#!/usr/bin/env python3
"""Destructive fixture checks only inside the labelled, ephemeral local test container."""
import json,subprocess
from pathlib import Path
root=Path(__file__).resolve().parent
container='askxuan-wish-test-20261014'
assert subprocess.check_output(['docker','inspect',container,'--format','{{index .Config.Labels "codex.task"}}'],text=True).strip()=='wish-completeness-20261014'
def sql(q):
 p=subprocess.run(['docker','exec','-i',container,'mysql','-uroot','-N','--default-character-set=utf8mb4'],input=q,text=True,capture_output=True)
 if p.returncode: raise RuntimeError(p.stderr)
 return p.stdout.strip()
def quote(s):return "'"+s.replace("'","''")+"'"
for user in ['product_user','temple_user']:sql(f"CREATE USER IF NOT EXISTS '{user}'@'%';")
sql("CREATE TABLE IF NOT EXISTS askxuan_temple.temple_image(id BIGINT AUTO_INCREMENT PRIMARY KEY,temple_code VARCHAR(32),url VARCHAR(500),type VARCHAR(32),sort INT,create_time DATETIME DEFAULT CURRENT_TIMESTAMP);")
schema=(root/'db/20261014_directory_media_schema.sql').read_text();sql(schema);sql(schema)
sql("INSERT IGNORE INTO askxuan_temple.temple_service_intent_tag VALUES(1,'wealth');")
defaults=(root/'db/20261014_service_intention_defaults.sql').read_text();sql(defaults)
assert sql("SELECT intent_mode FROM askxuan_temple.temple_service WHERE id=1;")=='custom'
sql("DELETE FROM askxuan_product.service_intention_rule WHERE service_code='S001';UPDATE askxuan_temple.temple_service SET intent_mode='custom' WHERE id=2;")
sql(defaults)
assert sql("SELECT COUNT(*) FROM askxuan_product.service_intention_rule WHERE service_code='S001';")=='0','rerun overwrote administrator rule'
assert sql("SELECT intent_mode FROM askxuan_temple.temple_service WHERE id=2;")=='custom','rerun overwrote explicit empty'
m=json.loads((root/'db/20261014_directory_media.json').read_text())
for code,cover in [('DT001',''),('DT002','/operator.jpg')]:
 e=m['temples'][code];sql(f"INSERT INTO askxuan_temple.temple VALUES({quote(code)},{quote(e['name'])},'正常','',{quote(cover)}) ON DUPLICATE KEY UPDATE cover_image=VALUES(cover_image);")
for code,name in [('DW001',m['masters']['DW001']['name']),('DW002','renamed person')]:
 sql(f"INSERT INTO askxuan_master.master VALUES({quote(code)},{quote(name)},'','demo','demo','','已认证','on_shelf','normal','platform') ON DUPLICATE KEY UPDATE avatar='',dharma_name=VALUES(dharma_name);")
media=(root/'db/20261014_directory_media.sql').read_text();sql(media);sql(media)
assert sql("SELECT COUNT(*) FROM askxuan_temple.temple_image WHERE temple_code='DT001';")=='2'
assert sql("SELECT COUNT(*) FROM askxuan_master.master_image WHERE master_code='DW001';")=='2'
assert sql("SELECT cover_image FROM askxuan_temple.temple WHERE code='DT002';")=='/operator.jpg'
assert sql("SELECT COUNT(*) FROM askxuan_master.master_image WHERE master_code='DW002';")=='0'
assert sql("SELECT COUNT(*) FROM askxuan_temple.temple_image WHERE temple_code='DT002';")=='0'
print('PASS: schema and seeds rerunnable; edited rules, explicit-empty bindings, operator uploads and renamed identities preserved')
