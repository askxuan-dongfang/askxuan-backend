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
import stat

BASE = Path('/opt/askxuan/ci')
PUBLIC = Path('/var/www/askxuan/public')
RELEASES = Path('/var/www/askxuan/releases')
SERVICES = {'gateway', 'auth', 'user', 'temple', 'master', 'booking', 'community', 'review',
            'product', 'order', 'payment', 'diy', 'marketing', 'logistics', 'finance', 'audit',
            'message', 'file', 'ai', 'media'}
WEB = {'admin', 'temple'}
SHA = re.compile(r'^[0-9a-f]{40}$')
QUIET_BASE = 'askxuan/quiet-service:ci-runtime-base'


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
    allowed = SERVICES if scope == 'backend' else ({'quiet'} if scope == 'quiet' else (WEB if scope == 'web' else {'h5'}))
    if manifest.get('schema') != 1 or manifest.get('scope') != scope or set(manifest['components']) != allowed:
        raise ValueError('Release scope mismatch')
    if not re.fullmatch(r'ci-(backend|quiet|web|h5)-[a-zA-Z0-9-]{1,100}', manifest['release']) or not manifest['release'].startswith('ci-' + scope + '-'):
        raise ValueError('Invalid release identifier')
    actual = {p.relative_to(dest).as_posix(): sha(p) for p in (dest / 'payload').rglob('*') if p.is_file()}
    if actual != manifest['files']:
        raise ValueError('Payload checksum mismatch')
    if any(len(PurePosixPath(p).parts) < 3 or PurePosixPath(p).parts[1] not in allowed for p in actual):
        raise ValueError('Unexpected component path')
    for name in allowed:
        p = dest / 'payload' / name
        if scope in ('backend', 'quiet'):
            if set(f.name for f in p.iterdir()) != {name}:
                raise ValueError('Unexpected binary payload')
            header = (p / name).read_bytes()[:20]
            if header[:5] != b'\x7fELF\x02' or header[18:20] != b'\x3e\x00':
                raise ValueError('Expected Linux amd64 ELF binary')
        elif not (p / 'index.html').is_file():
            raise ValueError('Missing web index')
    if scope == 'web' and not (dest / 'payload/admin/legacy/shop/index.html').is_file():
        raise ValueError('Missing unified admin legacy shop entry')
    if scope == 'h5' and any((dest / 'payload/h5' / n).exists() for n in ('admin', 'shop', 'temple')):
        raise ValueError('H5 must not replace admin routes')
    return manifest


def validate_versions(manifest, state):
    for component, value in manifest['components'].items():
        key = ('backend/' if manifest['scope'] in ('backend', 'quiet') else 'web/') + component
        required = {'backend'} if manifest['scope'] in ('backend', 'quiet') else ({'frontend', 'h5'} if component == 'h5' else {'frontend'})
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


def trusted_json(path):
    # A template may contain runtime secrets. Only the separately installed,
    # root-owned file is trusted; no archive path can provide this configuration.
    for item in (path, *path.parents):
        info = item.lstat()
        if info.st_uid != 0 or info.st_mode & 0o022 or stat.S_ISLNK(info.st_mode):
            raise ValueError('Runtime template and parents must be root-owned, non-symlink and not group/world writable')
    if not stat.S_ISREG(path.lstat().st_mode):
        raise ValueError('Runtime template must be a regular file')
    return json.loads(path.read_text())


def quiet_template(image):
    definition = trusted_json(BASE / 'runtime/quiet.json')
    if set(definition) - {'services', 'networks', 'volumes'} or set(definition.get('services', {})) != {'quiet-service'}:
        raise ValueError('Quiet runtime template must contain only quiet-service')
    d = definition['services']['quiet-service']
    ports = [{'target': 8096, 'published': '18196', 'host_ip': '127.0.0.1', 'protocol': 'tcp'}]
    env = d.get('environment', {})
    required = ('QUIET_MYSQL_ADDR', 'QUIET_MYSQL_PASSWORD', 'QUIET_REDIS_ADDR', 'QUIET_CSRF_KEY', 'QUIET_PUBLIC_ORIGINS')
    if (d.get('container_name') != 'askxuan-quiet-service' or d.get('image') != QUIET_BASE or
            d.get('entrypoint') != ['/app/quiet'] or d.get('command') or d.get('user') != '1000:1000' or
            d.get('ports') != ports or d.get('volumes') or definition.get('volumes') or
            set(d.get('networks', {})) != {'askxuan-net'} or
            definition.get('networks') != {'askxuan-net': {'external': True, 'name': 'askxuan-net'}} or
            not isinstance(env, dict) or any(not isinstance(v, str) for v in env.values()) or
            any(not env.get(k) for k in required) or env.get('QUIET_LISTEN') != '0.0.0.0:8096' or
            env.get('QUIET_REDIS_PREFIX') != 'quiet-park-prod:' or not d.get('healthcheck', {}).get('test')):
        raise ValueError('Quiet runtime template does not match the reviewed isolated runtime contract')
    d['image'] = image
    return definition


def quiet_container():
    # Do not mistake a Docker daemon/permission failure for a first deployment.
    found = run(['docker', 'ps', '-aq', '--filter', 'name=^/askxuan-quiet-service$']).decode().strip()
    if not found:
        return None
    if not re.fullmatch(r'[0-9a-f]{12,64}', found):
        raise ValueError('Ambiguous quiet runtime container')
    return json.loads(run(['docker', 'inspect', 'askxuan-quiet-service']))[0]


def quiet_existing_definition(c, image):
    d, networks, volumes = compose_definition(c, image)
    check = c['Config'].get('Healthcheck', {})
    if not check.get('Test') or check['Test'] == ['NONE']:
        raise ValueError('Existing quiet runtime must have a healthcheck')
    d['healthcheck'] = {'test': check['Test']}
    for raw, key in [('Interval', 'interval'), ('Timeout', 'timeout'), ('StartPeriod', 'start_period'), ('StartInterval', 'start_interval')]:
        if check.get(raw):
            d['healthcheck'][key] = str(check[raw]) + 'ns'
    if check.get('Retries') is not None:
        d['healthcheck']['retries'] = check['Retries']
    return {'services': {'quiet-service': d}, 'networks': networks, 'volumes': volumes}


def deploy_quiet(candidate, manifest, state, journal):
    value = manifest['components']['quiet']
    c = quiet_container()
    if c and state.get('components', {}).get('backend/quiet', {}).get('input') == value['input']:
        return []
    # The operator must pin/provision this base separately. Never bootstrap it
    # from an arbitrary submitted image or execute a submitted Dockerfile/script.
    if subprocess.run(['docker', 'image', 'inspect', QUIET_BASE], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode:
        raise ValueError('Quiet runtime base is not provisioned')
    image = 'askxuan/quiet-service:' + manifest['release']
    current = quiet_existing_definition(c, image) if c else quiet_template(image)
    if c:
        before = quiet_existing_definition(c, c['Image'])
        atomic_compose(candidate / 'rollback.json', before)
    target = candidate / 'payload/quiet'
    (target / 'quiet').chmod(0o755)
    (target / 'Dockerfile').write_text('FROM %s\nCOPY --chown=1000:1000 --chmod=755 quiet /app/quiet\n' % QUIET_BASE)
    with open(candidate / 'build-quiet.log', 'wb') as log:
        subprocess.run(['docker', 'build', '-t', image, str(target)], stdout=log, stderr=log, check=True)
    atomic_compose(candidate / 'compose.json', current)
    journal['created_services'] = [] if c else ['quiet']
    journal['phase'] = 'switching-backend'
    atomic_json(candidate / 'transaction.json', journal)
    compose_up(candidate / 'compose.json')
    healthy(['quiet'])
    return ['quiet']


def remove_created_services(names):
    if names not in ([], ['quiet']):
        raise ValueError('Unexpected first-deploy cleanup scope')
    if names and quiet_container():
        # No compose down, volume removal, database cleanup or other service.
        run(['docker', 'rm', '-f', 'askxuan-quiet-service'])


def smoke_quiet():
    with urllib.request.urlopen('http://127.0.0.1:18196/health', timeout=15) as response:
        data = json.load(response)
        if data.get('status') != 'ok' or data.get('persistent') is not True or data.get('roomCapacity') != 20:
            raise RuntimeError('Quiet persistent runtime smoke failed')


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
    if manifest['scope'] == 'web':
        legacy = candidate / 'payload/admin/legacy/shop/index.html'
        for path in ('shop/', 'shop/login', 'shop/orders/compatibility-check'):
            with urllib.request.urlopen('https://127.0.0.1/' + path, context=context, timeout=15) as response:
                if hashlib.sha256(response.read()).hexdigest() != sha(legacy):
                    raise RuntimeError('Served legacy admin entry mismatch: ' + path)


def deploy(candidate, manifest, state):
    release = manifest['release']
    rollback = candidate / 'rollback.json'
    old_public = PUBLIC.resolve(strict=manifest['scope'] != 'quiet')
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
        elif manifest['scope'] == 'quiet':
            changed = deploy_quiet(candidate, manifest, state, journal)
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
            if manifest['scope'] == 'web':
                # /shop remains an address alias owned by the admin build, not a
                # separately built application. Replace all obsolete shop assets.
                legacy = web_public / 'shop'
                if legacy.is_symlink() or legacy.is_file():
                    legacy.unlink()
                elif legacy.exists():
                    shutil.rmtree(legacy)
                shutil.copytree(web_public / 'admin/legacy/shop', legacy)
            for p in web_public.parent.rglob('*'):
                p.chmod(0o755 if p.is_dir() else 0o644)
            web_public.parent.chmod(0o755)
            run(['nginx', '-t'], stderr=subprocess.STDOUT)
            journal['phase'] = 'switching-web'
            atomic_json(candidate / 'transaction.json', journal)
            switch_public(web_public, release)
            smoke_web(candidate, manifest)
        smoke_quiet() if manifest['scope'] == 'quiet' else smoke()
        for name, value in manifest['components'].items():
            key = ('backend/' if manifest['scope'] in ('backend', 'quiet') else 'web/') + name
            state.setdefault('components', {})[key] = {**value, 'release': release}
        if manifest['scope'] == 'web':
            state['components'].pop('web/shop', None)
            state.setdefault('aliases', {})['web/shop'] = {
                'owner': 'web/admin', 'entry': 'admin/legacy/shop/index.html', 'release': release,
            }
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
            healthy(['quiet'] if manifest['scope'] == 'quiet' else changed)
        if journal['phase'] == 'switching-backend':
            remove_created_services(journal.get('created_services', []))
        if PUBLIC.resolve() == web_public:
            switch_public(old_public, release + '-rollback')
        atomic_json(BASE / 'state.json', old_state)
        journal['phase'] = 'rolled-back'
        atomic_json(candidate / 'transaction.json', journal)
        raise


def main():
    os.umask(0o077)
    scope = sys.argv[1]
    if scope not in ('backend', 'quiet', 'web', 'h5'):
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
    """Operator-only CLI; the scope-specific sudo rules do not allow this argument."""
    if not re.fullmatch(r'ci-(backend|quiet|web|h5)-[a-zA-Z0-9-]{1,100}', release):
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
        remove_created_services(journal.get('created_services', []))
        if PUBLIC.resolve() == (RELEASES / release / 'public').resolve():
            switch_public(Path(journal['previous_public']), release + '-manual-rollback')
        if release.startswith('ci-quiet-'):
            if not journal.get('created_services'):
                smoke_quiet()
        else:
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
