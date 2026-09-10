#!/usr/bin/env python3
"""ECS-only, explicit additive migration and idempotent experience catalog import.
Run schema before deploying product-service; publish only after payment/order/finance deploy.
"""
import argparse, json, os, re, shlex, subprocess
from pathlib import Path


def sql_literal(value):
    if isinstance(value, (int, float)):
        return str(value)
    return "CONVERT(0x" + str(value).encode().hex() + " USING utf8mb4)" if str(value) else "''"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['schema', 'publish'])
    parser.add_argument('--apply', action='store_true', required=True)
    parser.add_argument('--config', default='/opt/askxuan/backend/.docker/etc/product/product.yaml')
    parser.add_argument('--manifest', default=str(Path(__file__).resolve().parents[1] / 'catalog-experience-products.json'))
    parser.add_argument('--backup-dir', required=True)
    args = parser.parse_args()
    os.umask(0o077)
    backup = Path(args.backup_dir); backup.mkdir(parents=True, exist_ok=True)
    dsn = re.search(r'DataSource:\s*[\"\']?([^\n\"\']+)', Path(args.config).read_text())[1]
    user, password, database = re.match(r'([^:]+):(.+)@tcp\([^)]*\)/([^?]+)', dsn).groups()
    assert database == 'askxuan_product'
    def run(query):
        command = 'export MYSQL_PWD=' + shlex.quote(password) + '\nexec mysql --default-character-set=utf8mb4 --batch --raw --skip-column-names -u' + shlex.quote(user) + ' ' + shlex.quote(database) + ' -e ' + shlex.quote(query)
        result = subprocess.run(['docker', 'exec', '-i', 'askxuan-mysql', 'sh'], input=command, text=True, capture_output=True)
        if result.returncode:
            # SQL can contain data, but never output connection credentials.
            raise RuntimeError(result.stderr[-2000:])
        return result.stdout.strip()
    dump = backup / ('catalog-before-' + args.mode + '.sql')
    if not dump.exists():
        command = 'export MYSQL_PWD=' + shlex.quote(password) + '\nexec mysqldump --default-character-set=utf8mb4 --single-transaction --skip-lock-tables --no-tablespaces -u' + shlex.quote(user) + ' ' + shlex.quote(database) + ' product product_sku product_image product_category'
        with dump.open('x') as out:
            result = subprocess.run(['docker', 'exec', '-i', 'askxuan-mysql', 'sh'], input=command, text=True, stdout=out, stderr=subprocess.PIPE)
        if result.returncode:
            dump.unlink(); raise RuntimeError(result.stderr[-1000:])
    if args.mode == 'schema':
        columns = {'is_experience': 'TINYINT(1) NOT NULL DEFAULT 0', 'source_name': "VARCHAR(100) NOT NULL DEFAULT ''", 'source_url': "VARCHAR(1000) NOT NULL DEFAULT ''", 'source_note': "VARCHAR(500) NOT NULL DEFAULT ''"}
        for name, definition in columns.items():
            if run("SHOW COLUMNS FROM product LIKE '" + name + "'"):
                continue
            run('ALTER TABLE product ADD COLUMN ' + name + ' ' + definition)
        print('SCHEMA_READY: four additive columns; existing products unchanged')
        return
    manifest = json.loads(Path(args.manifest).read_text())
    categories = {name: int(id_) for id_, name in (row.split('\t') for row in run('SELECT id,name FROM product_category').splitlines())}
    products = manifest['products']
    assert len(products) == 4 and len({p['productNo'] for p in products}) == 4
    statements = ["SET @catalog_lock=GET_LOCK('catalog-experiences-20260911',30)", "START TRANSACTION"]
    # The lock gates all inserts; final row validation detects an incomplete import.
    for p in products:
        assert p['isExperience'] and p['sourceUrl'].startswith('https://') and p['categoryName'] in categories
        assert len(p['productNo']) <= 32 and all(len(s['skuNo']) <= 32 for s in p['skus'])
        assert p['stock'] == sum(s['stock'] for s in p['skus'])
        old = run('SELECT is_experience,source_url FROM product WHERE product_no=' + sql_literal(p['productNo']))
        if old and old != '1\t' + p['sourceUrl']:
            raise RuntimeError('Existing product identity differs: ' + p['productNo'])
        values = [p['productNo'], p['name'], categories[p['categoryName']], p['description'], p['images'][0]['path'], 'on_shelf', p['price'], 0, p['stock'], p['tags'], 0, 1, p['sourceName'], p['sourceUrl'], p['sourceNote']]
        statements += [
            'INSERT INTO product(product_no,name,category_id,description,main_image,status,price,market_price,stock,tags,freight_template_id,is_experience,source_name,source_url,source_note) SELECT ' + ','.join(sql_literal(v) for v in values) + ' WHERE @catalog_lock=1 AND NOT EXISTS(SELECT 1 FROM product WHERE product_no=' + sql_literal(p['productNo']) + ')',
            'SET @created=ROW_COUNT()',
            'SET @pid=(SELECT id FROM product WHERE product_no=' + sql_literal(p['productNo']) + ')',
        ]
        for sku in p['skus']:
            statements.append('INSERT INTO product_sku(product_id,spec_name,spec_value,price,stock,sku_no) SELECT @pid,' + ','.join(sql_literal(sku[k]) for k in ['specName', 'specValue', 'price', 'stock', 'skuNo']) + ' WHERE @created=1')
        for sort, image in enumerate(p['images']):
            assert image['path'].startswith('/catalog-experiences/')
            statements.append('INSERT INTO product_image(product_id,image_url,sort,type) SELECT @pid,' + ','.join(sql_literal(v) for v in [image['path'], sort, 'main']) + ' WHERE @created=1')
    statements += ['COMMIT', "SELECT RELEASE_LOCK('catalog-experiences-20260911')"]
    run(';\n'.join(statements))
    rows = run("SELECT id,product_no,name,status,stock FROM product WHERE product_no LIKE 'EXP-20260911-%' ORDER BY id")
    print(rows)
    assert len(rows.splitlines()) == 4
    coverage = run("SELECT category_id,COUNT(*) FROM product WHERE product_no LIKE 'EXP-20260911-%' AND status='on_shelf' AND is_experience=1 GROUP BY category_id")
    assert len(coverage.splitlines()) == 2 and all(row.split('\t')[1] == '2' for row in coverage.splitlines())
    print('PUBLISHED_OR_ALREADY_PRESENT; existing records and consumed stock preserved')


if __name__ == '__main__':
    main()
