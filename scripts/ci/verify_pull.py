#!/usr/bin/env python3
"""Run the actual pull, switch and operator rollback on a disposable Actions runner."""

import http.server
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
from unittest.mock import patch
from urllib.request import urlopen

import ecs_receiver as receiver


def command(*args, **kwargs):
    subprocess.run(args, check=True, **kwargs)


def response():
    with urlopen('http://127.0.0.1:8080/api/v1/health', timeout=5) as res:
        return json.load(res)


def gateway_pull(directory, payload, release):
    pointer = receiver.artifact_request(io.BytesIO(payload))
    reference = receiver.download_artifact('backend', pointer, directory)
    manifest = json.loads(reference.read_text())
    receiver.validate_image_manifest(manifest)
    assert not receiver.run(['docker', 'ps', '-aq', '--filter', 'name=^/askxuan-gateway-service$']).strip()
    config = directory / 'etc'
    config.mkdir()
    (config / 'gateway.yaml').write_text('''
Name: gateway
Host: 0.0.0.0
Port: 8080
Auth:
  AccessSecret: 0f1cd42a59e88edbeaca2d37a2ce23079184ae217299fdc6547f20b7527d4af3
Security:
  AuthTarget: release-fixture:8080
  AuditTarget: release-fixture:8080
Etcd:
  Hosts: []
  Key: ""
Upstreams:
  - {Prefix: /api/v1/diy, Target: "release-fixture:8080"}
  - {Prefix: /api/v1/community, Target: "release-fixture:8080"}
  - {Prefix: /api/v1/products, Target: "release-fixture:8080"}
NoAuthPaths:
  - /api/v1/health
  - /api/v1/diy/materials
  - /api/v1/community/feed
  - /api/v1/products
''')
    fixture = directory / 'fixture'
    fixture.mkdir()
    (fixture / 'gateway').write_text('#!/bin/sh\nexec busybox-extras httpd -f -p 8080 -h /www\n')
    for route in ('health', 'diy/materials', 'community/feed', 'products'):
        page = fixture / 'www/api/v1' / route
        page.parent.mkdir(parents=True, exist_ok=True)
        page.write_text('{"code":0,"version":"old"}')
    recipe = Path(__file__).with_name('Dockerfile.image').read_text()
    (fixture / 'Dockerfile').write_text(recipe + '\nUSER root\nRUN apk add --no-cache busybox-extras\nCOPY www /www\nUSER appuser\n')
    baseline_image = 'askxuan-pull-fixture:gateway'
    command('docker', 'build', '--build-arg', 'BINARY=gateway', '-t', baseline_image, str(fixture))
    baseline_compose = directory / 'baseline.json'
    baseline_compose.write_text(json.dumps({'services': {
        'release-fixture': {'image': baseline_image},
        'release-redis': {'image': 'redis:7.0-alpine'},
        'gateway-service': {
            'container_name': 'askxuan-gateway-service', 'image': baseline_image,
            'ports': ['127.0.0.1:8080:8080'], 'volumes': [f'{config}:/app/etc:ro'],
            'environment': {'AUTH_REDIS_HOST': 'release-redis:6379'},
            'healthcheck': {'test': ['CMD', 'nc', '-z', '127.0.0.1', '8080'],
                            'interval': '1s', 'timeout': '1s', 'retries': 10},
        },
    }}))
    compose = ['docker', 'compose', '-p', 'askxuan', '-f', str(baseline_compose)]
    command(*compose, 'up', '-d')
    receiver.healthy(['gateway'])
    assert response()['version'] == 'old'
    old_image = receiver.run(['docker', 'inspect', '--format', '{{.Image}}', 'askxuan-gateway-service']).strip()
    baseline = {'components': {f'backend/{name}': {'input': component['input']}
                               for name, component in manifest['components'].items() if name != 'gateway'}}
    receiver.atomic_json(receiver.BASE / 'state.json', baseline)
    receiver.atomic_json(receiver.BASE / 'config.json', {'backend_contract': manifest['contract']})
    image = manifest['components']['gateway']['image']
    assert subprocess.run(['docker', 'image', 'inspect', image], capture_output=True).returncode != 0
    # This runner exercises the real gateway image with a gateway-only fixture.
    # Knowledge cutover and joint rollback are covered by test_weknora_release.py.
    with patch.object(receiver, 'prepare_weknora', return_value=({}, [])), \
         patch.object(receiver, 'smoke_weknora', return_value={'fixture': 'gateway-only'}):
        receiver.receive('backend', io.BufferedReader(io.BytesIO(payload)))
    installed = receiver.run(['docker', 'exec', 'askxuan-gateway-service', 'sha256sum', '/app/gateway']).decode().split()[0]
    assert installed == manifest['files']['payload/gateway/gateway']
    active = json.loads((receiver.BASE / 'state.json').read_text())
    assert active['components']['backend/gateway']['image'] == image
    assert response()['data']['service'] == 'gateway'
    receiver.rollback_latest(release)
    assert response()['version'] == 'old'
    assert receiver.run(['docker', 'inspect', '--format', '{{.Image}}', 'askxuan-gateway-service']).strip() == old_image
    assert json.loads((receiver.BASE / 'state.json').read_text()) == baseline
    command(*compose, 'down')
    print(json.dumps({'scope': 'backend', 'fixed_image': image, 'live_binary_sha256': installed,
                      'real_gateway': True, 'rollback_restored_previous_image': True}), flush=True)


def h5_pull(directory, payload, release):
    class API(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b'{"code":0}')

    command('openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
            '-subj', '/CN=localhost', '-keyout', str(directory / 'key.pem'), '-out', str(directory / 'cert.pem'),
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    (directory / 'nginx.conf').write_text(f'''
events {{}}
http {{
  server {{
    listen 443 ssl;
    ssl_certificate {directory}/cert.pem;
    ssl_certificate_key {directory}/key.pem;
    root {receiver.PUBLIC};
  }}
}}
''')
    command('docker', 'run', '-d', '--rm', '--name', 'askxuan-pull-nginx', '-p', '127.0.0.1:443:443',
            '-v', f'{directory}:{directory}:ro', '-v', f'{directory}/nginx.conf:/etc/nginx/nginx.conf:ro',
            'nginx:alpine')
    original_run = receiver.run
    receiver.run = lambda args, **opts: original_run(
        ['docker', 'exec', 'askxuan-pull-nginx', *args] if args == ['nginx', '-t'] else args, **opts)
    old = receiver.PUBLIC.resolve()
    baseline = {'components': {'web/admin': {'release': 'ci-web-fixture'}}}
    receiver.atomic_json(receiver.BASE / 'state.json', baseline)
    with http.server.ThreadingHTTPServer(('127.0.0.1', 8080), API) as api:
        threading.Thread(target=api.serve_forever, daemon=True).start()
        receiver.receive('h5', io.BufferedReader(io.BytesIO(payload)))
        receipt = receiver.BASE / 'releases' / release
        manifest = json.loads((receipt / 'manifest.json').read_text())
        for path, sha in manifest['files'].items():
            assert receiver.sha(receiver.PUBLIC / path.removeprefix('payload/h5/')) == sha
        assert (receiver.PUBLIC / 'admin/index.html').read_text() == 'admin baseline'
        receiver.rollback_latest(release)
        assert receiver.PUBLIC.resolve() == old
        assert (receiver.PUBLIC / 'index.html').read_text() == 'old H5'
        assert json.loads((receiver.BASE / 'state.json').read_text()) == baseline
        api.shutdown()
    command('docker', 'stop', 'askxuan-pull-nginx')
    print(json.dumps({'scope': 'h5', 'verified_files': len(manifest['files']), 'nginx': True,
                      'rollback_restored_previous_public': True}), flush=True)


if __name__ == '__main__':
    if os.environ.get('GITHUB_ACTIONS') != 'true':
        raise SystemExit('Run this verification on a disposable Actions runner')
    scope = sys.argv[1]
    payload = sys.stdin.buffer.read(16 * 1024 + 1)
    pointer = receiver.artifact_request(io.BytesIO(payload))
    release = f'ci-{scope}-{pointer["run"]}-{pointer["attempt"]}-{pointer["commit"][:12]}'
    with tempfile.TemporaryDirectory(prefix='askxuan-verify-pull-') as temp:
        directory = Path(temp)
        directory.chmod(0o755)
        receiver.BASE = directory / 'state'
        receiver.BASE.mkdir()
        receiver.PUBLIC = directory / 'public'
        receiver.RELEASES = directory / 'releases'
        baseline = receiver.RELEASES / 'previous' / 'public'
        (baseline / 'admin').mkdir(parents=True)
        (baseline / 'admin/index.html').write_text('admin baseline')
        (baseline / 'index.html').write_text('old H5')
        receiver.PUBLIC.symlink_to(baseline)
        if scope == 'backend':
            gateway_pull(directory, payload, release)
        else:
            h5_pull(directory, payload, release)
