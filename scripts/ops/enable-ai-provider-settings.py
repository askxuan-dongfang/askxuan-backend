#!/usr/bin/env python3
"""Run as root on ECS to add private persistent AI settings storage.

Uses the installed release receiver, keeps the existing AI image and all provider
values, and restarts only AI. No credential values are printed. Safe to rerun.
"""
import base64
import fcntl
import importlib.machinery
import json
import os
from pathlib import Path
import secrets
import shlex
import subprocess
import time


def main():
    if os.geteuid() != 0:
        raise SystemExit('Run on ECS as root')
    os.umask(0o077)
    with open('/opt/askxuan/ci/publish.lock', 'a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        path = Path('/opt/askxuan/runtime/secrets.env')
        original = path.read_text()
        values = {}
        for line in original.splitlines():
            if '=' in line and not line.startswith('#'):
                name, raw = line.split('=', 1)
                parts = shlex.split(raw)
                values[name] = parts[0] if parts else ''
        container = json.loads(subprocess.check_output(['docker', 'inspect', 'askxuan-ai-service'], text=True))[0]
        env = dict(item.split('=', 1) for item in container['Config']['Env'] if '=' in item)
        dest = '/app/provider-settings'
        source = '/opt/askxuan/runtime/ai-provider-settings'
        if env.get('AI_SETTINGS_DIR') == dest and env.get('AI_SETTINGS_ENCRYPTION_KEY') and any(m['Destination'] == dest for m in container['Mounts']):
            print(json.dumps({'already_enabled': True, 'directory': source}))
            return
        # Avoid interrupting active generation while adding the mount.
        query = 'SELECT (SELECT COUNT(*) FROM askxuan_ai.ai_message WHERE status=\'pending\')+(SELECT COUNT(*) FROM askxuan_ai.ai_report WHERE status=\'generating\');'
        password = values['MYSQL_ROOT_PASSWORD']
        script = 'export MYSQL_PWD=' + shlex.quote(password) + '\nexec mysql -uroot -Nse ' + shlex.quote(query)
        count = subprocess.check_output(['docker', 'exec', '-i', 'askxuan-mysql', 'sh'], input=script, text=True).strip()
        if count != '0':
            raise SystemExit('Active AI work detected; retry after generation completes')
        directory = Path(source)
        directory.mkdir(mode=0o700, exist_ok=True)
        directory.chmod(0o700)
        os.chown(directory, 1000, 1000)
        if (directory / 'settings.enc').exists() and not values.get('AI_SETTINGS_ENCRYPTION_KEY'):
            raise SystemExit('Encrypted settings already exist; restore the matching key before enabling')
        encryption_key = values.get('AI_SETTINGS_ENCRYPTION_KEY') or base64.b64encode(secrets.token_bytes(32)).decode()
        changes = {'AI_SETTINGS_DIR': dest, 'AI_SETTINGS_ENCRYPTION_KEY': encryption_key}
        backup = Path('/opt/askxuan/backups') / ('ai-provider-settings-' + time.strftime('%Y%m%dT%H%M%S'))
        backup.mkdir(mode=0o700)
        receiver = importlib.machinery.SourceFileLoader('receiver', '/usr/local/sbin/askxuan-ci-receiver').load_module()
        definition, networks, volumes = receiver.compose_definition(container, container['Image'])
        before = {'services': {'ai-service': definition}, 'networks': networks, 'volumes': volumes}
        receiver.atomic_compose(backup / 'rollback.json', before)
        receiver.atomic_json(backup / 'previous-values.json', {key: values.get(key) for key in changes})
        candidate = json.loads(json.dumps(before))
        service = candidate['services']['ai-service']
        service['environment'].update(changes)
        service['volumes'] = [v for v in service.get('volumes', []) if v.get('target') != dest]
        service['volumes'].append({'type': 'bind', 'source': source, 'target': dest, 'read_only': False})
        receiver.atomic_compose(backup / 'candidate.json', candidate)
        lines = [line for line in original.splitlines() if line.split('=', 1)[0] not in changes]
        lines += [name + '=' + shlex.quote(value) for name, value in changes.items()]
        tmp = path.with_suffix('.ai-settings.tmp')
        tmp.write_text('\n'.join(lines) + '\n'); tmp.chmod(0o600); os.replace(tmp, path)
        try:
            receiver.compose_up(backup / 'candidate.json')
            receiver.healthy(['ai'])
        except BaseException:
            tmp.write_text(original); tmp.chmod(0o600); os.replace(tmp, path)
            receiver.compose_up(backup / 'rollback.json')
            raise
        print(json.dumps({'enabled': True, 'directory': source, 'backup': str(backup), 'changed_services': ['ai'], 'healthy': True}))


if __name__ == '__main__':
    main()
