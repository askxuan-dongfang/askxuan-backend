#!/usr/bin/env python3
"""Publish precisely the CI-tested release binaries and write the digest manifest."""

import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from ecs_receiver import extract, image_repository, validate_image_manifest


dockerfile = Path(__file__).with_name('Dockerfile.image').resolve()
runtime = hashlib.sha256(dockerfile.read_bytes()).hexdigest()
with tempfile.TemporaryDirectory(prefix='askxuan-image-build-') as temp:
    stage = Path(temp)
    manifest = extract(Path(sys.argv[1]), stage, 'backend')
    source = manifest['history']['backend'][0]
    if source != os.environ['GITHUB_SHA']:
        raise ValueError('Release must match the current CI commit')
    for name, component in manifest['components'].items():
        repository = image_repository(name)
        tag = f'{repository}:{manifest["release"]}'
        binary = manifest['files'][f'payload/{name}/{name}']
        subprocess.run([
            'docker', 'build', '--platform', 'linux/amd64', '--file', str(dockerfile),
            '--build-arg', f'BINARY={name}', '--tag', tag,
            '--label', f'org.opencontainers.image.revision={source}',
            '--label', 'org.opencontainers.image.source=https://github.com/askxuan-dongfang/askxuan-backend',
            '--label', f'org.askxuan.binary-sha256={binary}', str(stage / 'payload' / name),
        ], check=True)
        subprocess.run(['docker', 'push', tag], check=True)
        digests = json.loads(subprocess.check_output([
            'docker', 'image', 'inspect', '--format', '{{json .RepoDigests}}', tag,
        ], text=True))
        component['image'], = [digest for digest in digests if digest.startswith(repository + '@sha256:')]
        component['source_input'] = component['input']
        component['input'] = hashlib.sha256(f'{component["input"]}:{runtime}'.encode()).hexdigest()
    validate_image_manifest(manifest)
    Path(sys.argv[2]).write_text(json.dumps(manifest, sort_keys=True) + '\n')
