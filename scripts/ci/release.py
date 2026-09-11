#!/usr/bin/env python3
"""Build immutable release archives. Runs on CI, never on production ECS."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile

SERVICES = {
    'gateway': 'platform', 'auth': 'platform', 'user': 'platform',
    'temple': 'content', 'master': 'content', 'booking': 'content',
    'community': 'content', 'review': 'content',
    'product': 'commerce', 'order': 'commerce', 'payment': 'commerce', 'diy': 'commerce',
    'marketing': 'operation', 'logistics': 'operation', 'finance': 'operation', 'audit': 'operation',
    'message': 'infrastructure', 'file': 'infrastructure', 'ai': 'infrastructure', 'media': 'infrastructure',
}
APPS = {'admin': 'web-platform-admin', 'shop': 'web-shop-admin', 'temple': 'web-temple-admin', 'h5': 'web-h5'}


def git(root, *args):
    return subprocess.check_output(['git', '-C', str(root), *args], text=True).strip()


def digest(path):
    h = hashlib.sha256()
    with open(path, 'rb') as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b''):
            h.update(chunk)
    return h.hexdigest()


def tree_hash(root, paths):
    # Git object IDs cover names/content; no ignored build output or local secrets.
    return hashlib.sha256(git(root, 'ls-tree', '-r', 'HEAD', '--', *paths).encode()).hexdigest()


def contract(root):
    paths = ['db', 'scripts/db', 'build/docker/Dockerfile']
    paths += ['services/%s/%s-service/etc' % (group, name) for name, group in SERVICES.items()]
    return tree_hash(root, paths)


def build(scope, frontend, output):
    root = Path.cwd()
    frontend = Path(frontend).resolve()
    repos = {'backend': root} if scope == 'backend' else {'frontend': frontend}
    if scope == 'h5':
        repos['h5'] = frontend / 'apps/web-h5'
    history = {k: git(p, 'rev-list', 'HEAD').splitlines() for k, p in repos.items()}
    manifest = {'schema': 1, 'scope': scope, 'history': history, 'components': {}, 'files': {}}
    run = os.environ.get('GITHUB_RUN_ID', 'local')
    attempt = os.environ.get('GITHUB_RUN_ATTEMPT', '1')
    primary = 'backend' if scope == 'backend' else ('frontend' if scope == 'web' else 'h5')
    manifest['release'] = 'ci-%s-%s-%s-%s' % (scope, run, attempt, history[primary][0][:12])
    with tempfile.TemporaryDirectory(prefix='askxuan-release-') as tmp:
        stage = Path(tmp)
        payload = stage / 'payload'
        payload.mkdir()
        if scope == 'backend':
            manifest['contract'] = contract(root)
            for name, group in SERVICES.items():
                module = 'services/%s/%s-service' % (group, name)
                target = payload / name
                target.mkdir()
                print('Building ' + name, flush=True)
                subprocess.run(['go', 'build', '-trimpath', '-buildvcs=false', '-ldflags=-s -w -buildid=',
                                '-o', str(target / name), name + '.go'], cwd=root / module,
                               env={**os.environ, 'CGO_ENABLED': '0', 'GOOS': 'linux', 'GOARCH': 'amd64'}, check=True)
                manifest['components'][name] = {
                    'sources': {'backend': history['backend'][0]},
                    'input': tree_hash(root, ['common', 'go.work', 'go.work.sum', module]),
                }
        else:
            for name in (['admin', 'shop', 'temple'] if scope == 'web' else ['h5']):
                app = frontend / 'apps' / APPS[name]
                subprocess.run(['npm', 'ci'], cwd=app, check=True)
                subprocess.run(['npm', 'run', 'build'], cwd=app, check=True)
                shutil.copytree(app / 'dist', payload / name)
                sources = {'frontend': history['frontend'][0]}
                if name == 'h5':
                    sources['h5'] = history['h5'][0]
                manifest['components'][name] = {'sources': sources}
        for f in sorted(payload.rglob('*')):
            if f.is_symlink():
                raise ValueError('Symlinks are not permitted in release archives')
            if f.is_file():
                manifest['files'][f.relative_to(stage).as_posix()] = digest(f)
        (stage / 'manifest.json').write_text(json.dumps(manifest, sort_keys=True))
        with tarfile.open(output, 'w:gz') as archive:
            archive.add(stage / 'manifest.json', arcname='manifest.json')
            archive.add(payload, arcname='payload')
    print(json.dumps({'release': manifest['release'], 'components': list(manifest['components']),
                      'archive_sha256': digest(output)}, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('scope', choices=['backend', 'web', 'h5', 'contract'])
    parser.add_argument('--frontend', default='.')
    parser.add_argument('--output', default='release.tgz')
    args = parser.parse_args()
    if args.scope == 'contract':
        print(contract(Path.cwd()))
    else:
        build(args.scope, args.frontend, args.output)
