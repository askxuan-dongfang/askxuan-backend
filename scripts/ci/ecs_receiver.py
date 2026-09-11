#!/usr/bin/python3
"""Root-owned forced SSH command. Receives data, never executes submitted scripts.

Install manually from a reviewed commit. All writers share this process's flock.
Production state and compose files remain root-only because they contain env vars.
"""
import fcntl
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import signal
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request
import urllib.parse
import ssl

BASE = Path('/opt/askxuan/ci')
PUBLIC = Path('/var/www/askxuan/public')
RELEASES = Path('/var/www/askxuan/releases')
SERVICES = {'gateway', 'auth', 'user', 'temple', 'master', 'booking', 'community', 'review',
            'product', 'order', 'payment', 'diy', 'marketing', 'logistics', 'finance', 'audit',
            'message', 'file', 'ai', 'media'}
WEB = {'admin', 'shop', 'temple'}
SHA = re.compile(r'^[0-9a-f]{40}$')


def run(args, **kwargs):
    return subprocess.check_output(args, **kwargs)


def sha(path):
    h = hashlib.sha256()
    with open(path, 'rb') as f:
        for data in iter(lambda: f.read(1048576), b''):
            h.update(data)
    return h.hexdigest()


def atomic_json(path, value):
    tmp = path.with_suffix('.tmp')
    with open(tmp, 'w') as f:
        json.dump(value, f, indent=2)
        f.flush()
        os.fsync(f.fileno())
    os.replace(tmp, path)


def atomic_compose(path, value):
    # Compose interpolates dollar expressions even in JSON array commands and
    # environment values. Docker inspect contains literal container values.
    # Escape once at serialization so Compose restores those exact values.
    def escaped(item):
        if isinstance(item, str):
            return item.replace('$', '$$')
        if isinstance(item, list):
            return [escaped(v) for v in item]
        if isinstance(item, dict):
            return {k: escaped(v) for k, v in item.items()}
        return item
    atomic_json(path, escaped(value))


def extract(archive, dest, scope):
    with tarfile.open(archive, 'r:gz') as tar:
        members = tar.getmembers()
        if len(members) > 20000 or sum(m.size for m in members) > 3 * 1024**3:
            raise ValueError('Archive exceeds release limits')
        names = set()
        for m in members:
            p = PurePosixPath(m.name)
            if p.is_absolute() or '..' in p.parts or str(p) != m.name or m.name in names:
                raise ValueError('Unsafe or duplicate archive path')
            if not (m.isfile() or m.isdir()) or m.name not in ('manifest.json', 'payload') and not m.name.startswith('payload/'):
                raise ValueError('Unsupported archive member')
            names.add(m.name)
        # Copy regular files ourselves: never honor uid, mode, links, devices or tar metadata.
        for m in members:
            p = dest / m.name
            if m.isdir():
                p.mkdir(parents=True, exist_ok=True)
            else:
                p.parent.mkdir(parents=True, exist_ok=True)
                with tar.extractfile(m) as src, open(p, 'xb') as target:
                    shutil.copyfileobj(src, target)
    manifest = json.loads((dest / 'manifest.json').read_text())
    allowed = SERVICES if scope == 'backend' else (WEB if scope == 'web' else {'h5'})
    if manifest.get('schema') != 1 or manifest.get('scope') != scope or set(manifest['components']) != allowed:
        raise ValueError('Release scope mismatch')
    if not re.fullmatch(r'ci-(backend|web|h5)-[a-zA-Z0-9-]{1,100}', manifest['release']):
        raise ValueError('Invalid release identifier')
    actual = {p.relative_to(dest).as_posix(): sha(p) for p in (dest / 'payload').rglob('*') if p.is_file()}
    if actual != manifest['files']:
        raise ValueError('Payload checksum mismatch')
    if any(len(PurePosixPath(p).parts) < 3 or PurePosixPath(p).parts[1] not in allowed for p in actual):
        raise ValueError('Unexpected component path')
    for name in allowed:
        p = dest / 'payload' / name
        if scope == 'backend':
            if set(f.name for f in p.iterdir()) != {name}:
                raise ValueError('Unexpected binary payload')
            header = (p / name).read_bytes()[:20]
            if header[:5] != b'\x7fELF\x02' or header[18:20] != b'\x3e\x00':
                raise ValueError('Expected Linux amd64 ELF binary')
        elif not (p / 'index.html').is_file():
            raise ValueError('Missing web index')
    if scope != 'backend' and any((dest / 'payload/h5' / n).exists() for n in ('admin', 'shop', 'temple')):
        raise ValueError('H5 must not replace admin routes')
    return manifest


def validate_versions(manifest, state):
    for component, value in manifest['components'].items():
        key = ('backend/' if manifest['scope'] == 'backend' else 'web/') + component
        required = {'backend'} if manifest['scope'] == 'backend' else ({'frontend', 'h5'} if component == 'h5' else {'frontend'})
        if set(value['sources']) != required:
            raise ValueError('Missing source provenance')
        for source, commit in value['sources'].items():
            history = manifest['history'][source]
            if not SHA.fullmatch(commit) or not history or history[0] != commit or any(not SHA.fullmatch(c) for c in history):
                raise ValueError('Invalid Git history')
            previous = state.get('components', {}).get(key, {}).get('sources', {}).get(source)
            if previous and previous not in history:
                raise ValueError('Stale/divergent release: %s %s' % (key, source))


def compose_definition(c, image):
    host, cfg = c['HostConfig'], c['Config']
    name = c['Name'].lstrip('/')
    d = {'container_name': name, 'image': image,
         'restart': host['RestartPolicy']['Name'] or 'unless-stopped',
         'environment': dict(v.split('=', 1) for v in cfg['Env']), 'volumes': [], 'networks': {}}
    networks, volumes = {}, {}
    for m in c['Mounts']:
        if m['Type'] not in ('bind', 'volume'):
            raise ValueError('Unsupported runtime mount type')
        if m['Type'] == 'volume':
            volumes[m['Name']] = {'external': True, 'name': m['Name']}
        d['volumes'].append({'type': m['Type'], 'source': m['Name'] if m['Type'] == 'volume' else m['Source'],
                             'target': m['Destination'], 'read_only': not m['RW']})
    d['ports'] = [{'target': int(p.split('/')[0]), 'published': b['HostPort'],
                   'host_ip': b['HostIp'] or '0.0.0.0', 'protocol': p.split('/')[1]}
                  for p, bindings in (host.get('PortBindings') or {}).items() for b in (bindings or [])]
    for n, v in c['NetworkSettings']['Networks'].items():
        networks[n] = {'external': True, 'name': n}
        d['networks'][n] = {'aliases': list(dict.fromkeys([name.removeprefix('askxuan-')] + (v.get('Aliases') or [])))}
    for raw, key in [('ExtraHosts', 'extra_hosts'), ('CapAdd', 'cap_add'), ('CapDrop', 'cap_drop'),
                     ('SecurityOpt', 'security_opt'), ('ReadonlyRootfs', 'read_only'), ('Memory', 'mem_limit')]:
        if host.get(raw):
            d[key] = host[raw]
    if host.get('NanoCpus'):
        d['cpus'] = host['NanoCpus'] / 1e9
    if host.get('LogConfig', {}).get('Type'):
        d['logging'] = {'driver': host['LogConfig']['Type'], 'options': host['LogConfig'].get('Config') or {}}
    for raw, key in [('User', 'user'), ('WorkingDir', 'working_dir'), ('Entrypoint', 'entrypoint'), ('Cmd', 'command')]:
        if cfg.get(raw):
            d[key] = cfg[raw]
    return d, networks, volumes


def compose_up(path):
    # Docker output/errors can contain env values. Keep full output in root-only logs.
    with open(path.parent / (path.stem + '.log'), 'ab') as log:
        subprocess.run(['docker', 'compose', '-p', 'askxuan', '-f', str(path), 'up', '-d', '--no-build', '--no-deps'],
                       stdout=log, stderr=log, check=True)


def healthy(names):
    deadline = time.monotonic() + 180
    pending = set(names)
    while pending and time.monotonic() < deadline:
        for name in list(pending):
            c = json.loads(run(['docker', 'inspect', 'askxuan-' + name + '-service']))[0]
            if c['State'].get('Health', {}).get('Status') == 'healthy':
                pending.remove(name)
        if pending:
            time.sleep(3)
    if pending:
        raise RuntimeError('Unhealthy services: ' + ','.join(sorted(pending)))


def smoke():
    for route in ('health', 'diy/materials', 'community/feed', 'products'):
        with urllib.request.urlopen('http://127.0.0.1:8080/api/v1/' + route, timeout=15) as r:
            data = json.load(r)
            if data.get('code') != 0:
                raise RuntimeError('API smoke failed: ' + route)


def switch_public(target, release):
    link = PUBLIC.parent / ('public.next-' + release)
    link.symlink_to(target)
    os.replace(link, PUBLIC)


def smoke_web(candidate, manifest):
    # Loopback TLS uses the production virtual host's certificate; compare bytes,
    # while SSH host-key pinning authenticates the server for the CI transport.
    context = ssl._create_unverified_context()
    for name in manifest['components']:
        prefix = '' if name == 'h5' else name + '/'
        root = candidate / 'payload' / name
        targets = [root / 'index.html']
        targets += sorted(root.rglob('*.js'))[:1]
        for file in targets:
            path = prefix + file.relative_to(root).as_posix()
            with urllib.request.urlopen('https://127.0.0.1/' + urllib.parse.quote(path), context=context, timeout=15) as response:
                if hashlib.sha256(response.read()).hexdigest() != sha(file):
                    raise RuntimeError('Served web content mismatch: ' + path)


def deploy(candidate, manifest, state):
    release = manifest['release']
    rollback = candidate / 'rollback.json'
    old_public = PUBLIC.resolve(strict=True)
    web_public = (RELEASES / release / 'public').resolve()
    changed = []
    old_state = json.loads(json.dumps(state))
    journal = {'release': release, 'phase': 'preparing', 'previous_public': str(old_public), 'state_before': old_state}
    atomic_json(candidate / 'transaction.json', journal)
    try:
        if manifest['scope'] == 'backend':
            current = {'services': {}, 'networks': {}, 'volumes': {}}
            before = {'services': {}, 'networks': {}, 'volumes': {}}
            for name, value in manifest['components'].items():
                if state.get('components', {}).get('backend/' + name, {}).get('input') == value['input']:
                    continue
                c = json.loads(run(['docker', 'inspect', 'askxuan-' + name + '-service']))[0]
                base_tag = 'askxuan/%s-service:ci-runtime-base' % name
                # Freeze one existing runtime base; do not stack a binary layer every release.
                if subprocess.run(['docker', 'image', 'inspect', base_tag], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode:
                    run(['docker', 'tag', c['Image'], base_tag])
                image = 'askxuan/%s-service:%s' % (name, release)
                target = candidate / 'payload' / name
                (target / name).chmod(0o755)
                (target / 'Dockerfile').write_text('FROM %s\nCOPY --chown=1000:1000 --chmod=755 %s /app/%s\n' % (base_tag, name, name))
                with open(candidate / ('build-' + name + '.log'), 'wb') as log:
                    subprocess.run(['docker', 'build', '-t', image, str(target)], stdout=log, stderr=log, check=True)
                d, nets, vols = compose_definition(c, image)
                current['services'][name + '-service'] = d
                before['services'][name + '-service'] = {**d, 'image': c['Image']}
                for obj in (current, before):
                    obj['networks'].update(nets)
                    obj['volumes'].update(vols)
                changed.append(name)
            if changed:
                atomic_compose(candidate / 'compose.json', current)
                atomic_compose(rollback, before)
                journal['phase'] = 'switching-backend'
                atomic_json(candidate / 'transaction.json', journal)
                compose_up(candidate / 'compose.json')
                healthy(changed)
        else:
            shutil.copytree(old_public, web_public)
            if 'h5' in manifest['components']:
                for p in web_public.iterdir():
                    if p.name not in ('admin', 'shop', 'temple'):
                        shutil.rmtree(p) if p.is_dir() else p.unlink()
                shutil.copytree(candidate / 'payload/h5', web_public, dirs_exist_ok=True)
            for name in manifest['components']:
                if name != 'h5':
                    dest = web_public / name
                    if dest.exists():
                        shutil.rmtree(dest)
                    shutil.copytree(candidate / 'payload' / name, dest)
            for p in web_public.parent.rglob('*'):
                p.chmod(0o755 if p.is_dir() else 0o644)
            web_public.parent.chmod(0o755)
            run(['nginx', '-t'], stderr=subprocess.STDOUT)
            journal['phase'] = 'switching-web'
            atomic_json(candidate / 'transaction.json', journal)
            switch_public(web_public, release)
            smoke_web(candidate, manifest)
        smoke()
        for name, value in manifest['components'].items():
            key = ('backend/' if manifest['scope'] == 'backend' else 'web/') + name
            state.setdefault('components', {})[key] = {**value, 'release': release}
        state['last_release'] = release
        atomic_json(BASE / 'state.json', state)
        journal['phase'] = 'complete'
        journal['changed_services'] = changed
        atomic_json(candidate / 'transaction.json', journal)
        print(json.dumps({'status': 'deployed', 'release': release, 'changed_services': changed,
                          'components': list(manifest['components'])}), flush=True)
    except BaseException:
        # Roll back the entire affected component set, and record recovery evidence.
        if journal['phase'] == 'switching-backend' and rollback.exists():
            compose_up(rollback)
            healthy(changed)
        if PUBLIC.resolve() == web_public:
            switch_public(old_public, release + '-rollback')
        atomic_json(BASE / 'state.json', old_state)
        journal['phase'] = 'rolled-back'
        atomic_json(candidate / 'transaction.json', journal)
        raise


def main():
    os.umask(0o077)
    scope = sys.argv[1]
    if scope not in ('backend', 'web', 'h5'):
        raise ValueError('Unknown scope')
    BASE.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='incoming-', dir=BASE) as temp:
        archive = Path(temp) / 'release.tgz'
        size = 0
        with open(archive, 'wb') as target:
            for data in iter(lambda: sys.stdin.buffer.read(1048576), b''):
                size += len(data)
                if size > 1024**3:
                    raise ValueError('Upload exceeds 1 GiB')
                target.write(data)
        with open(BASE / 'publish.lock', 'w') as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            if shutil.disk_usage(BASE).free < 3 * 1024**3:
                raise RuntimeError('Need at least 3 GiB free for release and rollback')
            unpacked = Path(temp) / 'unpacked'
            unpacked.mkdir()
            manifest = extract(archive, unpacked, scope)
            state = json.loads((BASE / 'state.json').read_text())
            validate_versions(manifest, state)
            if scope == 'backend' and manifest.get('contract') != json.loads((BASE / 'config.json').read_text())['backend_contract']:
                raise ValueError('Database/runtime template changed: apply reviewed migration/config, then update contract baseline')
            for transaction in (BASE / 'releases').glob('*/transaction.json'):
                phase = json.loads(transaction.read_text())['phase']
                if phase not in ('complete', 'rolled-back'):
                    raise RuntimeError('An unfinished transaction needs operator recovery: ' + transaction.parent.name)
            candidate = BASE / 'releases' / manifest['release']
            if candidate.exists():
                raise ValueError('Release identifier already exists; rerun with a new attempt')
            candidate.parent.mkdir(exist_ok=True)
            shutil.move(str(unpacked), candidate)
            deploy(candidate, manifest, state)
            # Successful archives/binaries are redundant with built images and manifest hashes.
            shutil.rmtree(candidate / 'payload')


def rollback_latest(release):
    """Operator-only CLI; the three sudo rules do not allow this argument."""
    if not re.fullmatch(r'ci-(backend|web|h5)-[a-zA-Z0-9-]{1,100}', release):
        raise ValueError('Invalid release identifier')
    os.umask(0o077)
    with open(BASE / 'publish.lock', 'w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        state = json.loads((BASE / 'state.json').read_text())
        if state.get('last_release') != release:
            raise ValueError('Only the latest global release can be rolled back automatically')
        candidate = BASE / 'releases' / release
        journal = json.loads((candidate / 'transaction.json').read_text())
        if journal['phase'] != 'complete':
            raise ValueError('Release is not complete')
        if (candidate / 'rollback.json').exists():
            compose_up(candidate / 'rollback.json')
            healthy(journal['changed_services'])
        if PUBLIC.resolve() == (RELEASES / release / 'public').resolve():
            switch_public(Path(journal['previous_public']), release + '-manual-rollback')
        smoke()
        atomic_json(BASE / 'state.json', journal['state_before'])
        journal['phase'] = 'rolled-back'
        atomic_json(candidate / 'transaction.json', journal)
        print(json.dumps({'status': 'rolled-back', 'release': release}), flush=True)


if __name__ == '__main__':
    try:
        signal.signal(signal.SIGHUP, signal.SIG_IGN)
        if len(sys.argv) == 3 and sys.argv[1] == '--rollback':
            rollback_latest(sys.argv[2])
        else:
            main()
    except Exception as exc:
        # Never dump Docker inspect, environment, full command output or a secret-bearing compose.
        print('DEPLOY FAILED: %s' % str(exc), file=sys.stderr)
        sys.exit(1)
