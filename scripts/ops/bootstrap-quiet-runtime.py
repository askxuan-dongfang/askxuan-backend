#!/usr/bin/env python3
"""First installation only. Plan by default; --apply retains every recovery point.

Run a reviewed copy as root on ECS. Never run this from an uploaded release.
This script does not deploy the app, change nginx, or remove existing resources.
"""
import argparse
from datetime import datetime, timezone
import fcntl
import json
import os
from pathlib import Path
import re
import secrets
import shlex
import shutil
import stat
import subprocess
import time

BASE = Path('/opt/askxuan/ci')
SECRETS = Path('/opt/askxuan/runtime/secrets.env')
RUNTIME_BASE = 'askxuan/quiet-service:ci-runtime-base'
REDIS = 'askxuan-quiet-redis'
VOLUME = 'askxuan-quiet-redis-data'
ORIGINS = 'https://101.96.228.71,https://eliaukcloud.cn,https://www.eliaukcloud.cn'
MYSQL = ['docker', 'exec', '-i', '-e', 'MYSQL_PWD', 'askxuan-mysql',
         'mysql', '-uroot', '--batch', '--skip-column-names']


class Refused(Exception):
    """Only fixed, non-secret messages may cross the command boundary."""


def command(args, data=None, timeout=60):
    options = {}
    if args == MYSQL:
        # Read only on the server, at the MySQL command boundary. Do not put the
        # password in argv, stdin SQL, output, receipts, or another command's env.
        options['env'] = {**os.environ, 'MYSQL_PWD': mysql_password()}
    try:
        result = subprocess.run(args, input=data, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout, **options)
    except (OSError, ValueError, subprocess.TimeoutExpired):
        raise Refused('A required local command failed or timed out; inspect the root-only recovery receipt') from None
    if result.returncode:
        # Do not print command arguments, MySQL errors or Redis configuration.
        raise Refused('A required local command failed; inspect the root-only recovery receipt')
    return result.stdout.decode().strip()


def atomic(path, value):
    temp = path.with_suffix('.tmp')
    with open(temp, 'x', encoding='utf-8') as stream:
        os.chmod(temp, 0o600)
        json.dump(value, stream, indent=2, sort_keys=True)
        stream.write('\n')
        stream.flush()
        os.fsync(stream.fileno())
    os.replace(temp, path)


def root_path(path, regular=False):
    for item in (path, *path.parents):
        info = item.lstat()
        if info.st_uid != 0 or stat.S_ISLNK(info.st_mode) or info.st_mode & 0o022:
            raise Refused('Required runtime paths must be root-owned, non-symlink and not group/world writable')
    if regular and not stat.S_ISREG(path.lstat().st_mode):
        raise Refused('Expected an existing regular runtime file')


def mysql_password():
    try:
        root_path(SECRETS, regular=True)
        if stat.S_IMODE(SECRETS.lstat().st_mode) != 0o600:
            raise ValueError('invalid mode')
        values = []
        for line in SECRETS.read_text().splitlines():
            if '=' not in line or line.lstrip().startswith('#'):
                continue
            name, raw = line.split('=', 1)
            if name.strip() != 'MYSQL_ROOT_PASSWORD':
                continue
            parts = shlex.split(raw)
            if len(parts) != 1 or not parts[0] or '\x00' in parts[0]:
                raise ValueError('invalid scalar')
            values.append(parts[0])
        if len(values) != 1:
            raise ValueError('missing or ambiguous value')
        return values[0]
    except (OSError, ValueError, Refused):
        raise Refused('Validated server-local MySQL credential is unavailable') from None


def preflight(base_image):
    if not re.fullmatch(r'alpine@sha256:[0-9a-f]{64}', base_image):
        raise Refused('--base-image must be an explicit reviewed alpine@sha256 digest already present locally')
    if shutil.disk_usage(BASE).free < 3 * 1024**3:
        raise Refused('Need at least 3 GiB free for runtime preparation and later release rollback')
    for directory in (BASE / 'runtime', BASE / 'backups'):
        if directory.exists() or directory.is_symlink():
            root_path(directory)
    for journal in (BASE / 'releases').glob('*/transaction.json'):
        if json.loads(journal.read_text()).get('phase') not in ('complete', 'rolled-back'):
            raise Refused('An unfinished release transaction requires operator recovery')
    for journal in (BASE / 'backups').glob('quiet-bootstrap-*/receipt.json'):
        if json.loads(journal.read_text()).get('phase') != 'complete':
            raise Refused('An earlier quiet bootstrap needs operator recovery; no resources were removed')
    template = BASE / 'runtime/quiet.json'
    if any(p.exists() or p.is_symlink() for p in (template, template.with_suffix('.tmp'))):
        raise Refused('Quiet runtime template already exists; this tool only performs a first installation')
    for name in ('askxuan-quiet-service', REDIS):
        if command(['docker', 'ps', '-aq', '--filter', 'name=^/' + name + '$']):
            raise Refused('Quiet container already exists; this tool only performs a first installation')
    if command(['docker', 'volume', 'ls', '-q', '--filter', 'name=^' + VOLUME + '$']):
        raise Refused('Quiet Redis volume already exists; no existing storage may be reused or reset')
    if command(['docker', 'image', 'ls', '--format', '{{.Repository}}:{{.Tag}}', RUNTIME_BASE]):
        raise Refused('Quiet runtime base tag already exists; operator review is required')
    if command(['docker', 'network', 'inspect', '--format', '{{.Name}}', 'askxuan-net']) != 'askxuan-net':
        raise Refused('Existing askxuan-net is unavailable')
    for name in ('askxuan-mysql', 'askxuan-redis'):
        if command(['docker', 'inspect', '--format', '{{.State.Running}}', name]) != 'true':
            raise Refused('Existing MySQL and Redis containers must be running')
    source_id = command(['docker', 'image', 'inspect', '--format', '{{.Id}}', base_image])
    redis_id = command(['docker', 'inspect', '--format', '{{.Image}}', 'askxuan-redis'])
    if any(not re.fullmatch(r'sha256:[0-9a-f]{64}', value) for value in (source_id, redis_id)):
        raise Refused('Unexpected local base image identity')
    counts = command(MYSQL, b"SELECT COUNT(*) FROM information_schema.schemata WHERE SCHEMA_NAME='quiet_local';\nSELECT COUNT(*) FROM mysql.user WHERE User='quiet';\n")
    if counts.splitlines() != ['0', '0']:
        raise Refused('quiet_local database or quiet MySQL user already exists; no existing data may be changed')
    return {'baseImage': base_image, 'baseImageId': source_id, 'redisImageId': redis_id,
            'database': 'quiet_local', 'databaseUser': 'quiet', 'redisContainer': REDIS,
            'redisVolume': VOLUME, 'applicationContainer': 'askxuan-quiet-service',
            'runtimeTemplate': str(template), 'applicationPort': '127.0.0.1:18196',
            'network': 'askxuan-net', 'publicOrigins': ORIGINS}


def runtime_template(secret):
    return {'services': {'quiet-service': {
        'container_name': 'askxuan-quiet-service', 'image': RUNTIME_BASE,
        'restart': 'unless-stopped', 'user': '1000:1000', 'working_dir': '/app', 'entrypoint': ['/app/quiet'],
        'environment': {'QUIET_LISTEN': '0.0.0.0:8096', 'QUIET_MYSQL_ADDR': 'askxuan-mysql:3306',
                        'QUIET_MYSQL_PASSWORD': secret['mysqlPassword'], 'QUIET_REDIS_ADDR': REDIS + ':6379',
                        'QUIET_REDIS_PASSWORD': secret['redisPassword'], 'QUIET_CSRF_KEY': secret['csrfKey'],
                        'QUIET_REDIS_PREFIX': 'quiet-park-prod:', 'QUIET_PUBLIC_ORIGINS': ORIGINS,
                        'QUIET_AUTH_URL': 'https://101.96.228.71'},
        'ports': [{'target': 8096, 'published': '18196', 'host_ip': '127.0.0.1', 'protocol': 'tcp'}],
        'networks': {'askxuan-net': {}},
        'healthcheck': {'test': ['CMD', 'wget', '-q', '-O', '/dev/null', 'http://127.0.0.1:8096/health'],
                        'interval': '10s', 'timeout': '3s', 'start_period': '10s', 'retries': 3},
        'logging': {'driver': 'json-file', 'options': {'max-size': '10m', 'max-file': '3'}},
    }}, 'networks': {'askxuan-net': {'external': True, 'name': 'askxuan-net'}}, 'volumes': {}}


def apply(plan):
    backups = BASE / 'backups'
    backups.mkdir(mode=0o700, exist_ok=True)
    root_path(backups)
    stamp = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    backup = backups / ('quiet-bootstrap-' + stamp + '-' + secrets.token_hex(4))
    backup.mkdir(mode=0o700)
    secret = {'mysqlPassword': secrets.token_hex(32), 'redisPassword': secrets.token_hex(32), 'csrfKey': secrets.token_hex(32)}
    atomic(backup / 'recovery-secrets.json', secret)
    receipt = {**plan, 'phase': 'prepared', 'lastCompleted': 'prepared', 'recoveryDirectory': str(backup),
               'databaseBackup': 'not applicable: preflight verified database and user absent',
               'failurePolicy': 'stop and preserve all resources; never delete or reset database/storage'}
    def stage(name, complete=False):
        receipt['phase'] = name
        if complete:
            receipt['lastCompleted'] = name
        atomic(backup / 'receipt.json', receipt)
    stage('prepared', True)
    try:
        stage('building-runtime-base')
        dockerfile = ('FROM ' + plan['baseImage'] + '\n'
                      'RUN mkdir -p /app && chown 1000:1000 /app && '
                      'addgroup -g 1000 quiet && adduser -D -u 1000 -G quiet quiet\n'
                      'USER 1000:1000\nWORKDIR /app\n')
        # Empty build context: no app source, secrets or submitted release files.
        (backup / 'runtime.Dockerfile').write_text(dockerfile)
        command(['docker', 'build', '--pull=false', '--network=none', '-t', RUNTIME_BASE, '-'], dockerfile.encode(), timeout=180)
        command(['docker', 'run', '--rm', '--network', 'none', '--read-only', '--cap-drop', 'ALL',
                 '--security-opt', 'no-new-privileges', '--entrypoint', '/bin/sh', RUNTIME_BASE, '-ec',
                 'test "$(id -u)" = 1000; test "$(id -un)" = quiet; test -d /app; '
                 'test "$(stat -c %u /app)" = 1000; test -s /etc/ssl/certs/ca-certificates.crt; command -v wget >/dev/null'])
        stage('runtime-base-ready', True)

        stage('creating-mysql-database-and-user')
        password = secret['mysqlPassword']  # hex only, never shell-interpolated.
        sql = ("CREATE DATABASE quiet_local CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n"
               "CREATE USER 'quiet'@'%' IDENTIFIED BY '" + password + "';\n"
               "GRANT CREATE, SELECT, INSERT, UPDATE, DELETE ON quiet_local.* TO 'quiet'@'%';\n")
        command(MYSQL, sql.encode())
        stage('mysql-ready', True)

        stage('creating-redis-volume')
        command(['docker', 'volume', 'create', '--label', 'com.askxuan.scope=quiet-storage', VOLUME])
        stage('redis-volume-ready', True)
        stage('creating-redis-container')
        redis_id = command(['docker', 'run', '-d', '--name', REDIS, '--restart', 'unless-stopped',
                            '--network', 'askxuan-net', '--network-alias', REDIS,
                            '--mount', 'type=volume,source=' + VOLUME + ',target=/data',
                            plan['redisImageId'], 'redis-server', '--appendonly', 'yes',
                            '--protected-mode', 'yes', '--requirepass', secret['redisPassword']])
        if not re.fullmatch(r'[0-9a-f]{64}', redis_id):
            raise Refused('Unexpected new Redis container identity')
        receipt['redisContainerId'] = redis_id
        for attempt in range(15):
            try:
                pong = command(['docker', 'exec', '-i', REDIS, 'sh', '-ec',
                                'IFS= read -r REDISCLI_AUTH; export REDISCLI_AUTH; exec redis-cli ping'],
                               (secret['redisPassword'] + '\n').encode())
                if pong == 'PONG':
                    break
            except Refused:
                pass
            if attempt == 14:
                raise Refused('New isolated Redis did not become ready')
            time.sleep(1)
        stage('redis-ready', True)

        stage('writing-runtime-template')
        directory = BASE / 'runtime'
        directory.mkdir(mode=0o700, exist_ok=True)
        root_path(directory)
        if (directory / 'quiet.json').exists() or (directory / 'quiet.json').is_symlink():
            raise Refused('Quiet template appeared during initialization; refusing to overwrite it')
        atomic(backup / 'quiet.json', runtime_template(secret))
        atomic(directory / 'quiet.json', runtime_template(secret))
        stage('complete', True)
        return {'status': 'initialized', 'receipt': str(backup / 'receipt.json'),
                'next': 'Deploy only a verified quiet package through the separately reviewed quiet receiver'}
    except BaseException:
        receipt['failedDuring'] = receipt['phase']
        stage('failed')
        raise Refused('Initialization stopped; all existing and newly created data were preserved. Recovery receipt: ' + str(backup / 'receipt.json')) from None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base-image', required=True)
    parser.add_argument('--apply', action='store_true', help='Explicitly create new isolated quiet runtime resources')
    args = parser.parse_args()
    os.umask(0o077)
    if os.geteuid() != 0:
        raise Refused('Run the reviewed operator script as root')
    root_path(BASE)
    root_path(BASE / 'publish.lock', regular=True)
    # The existing lock is opened without creating or truncating any file in plan mode.
    with open(BASE / 'publish.lock', 'r+') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        plan = preflight(args.base_image)
        result = apply(plan) if args.apply else {'status': 'plan-only', **plan,
                  'next': 'Review the plan and rerun this exact command with --apply; no resources changed'}
        print(json.dumps(result, indent=2, sort_keys=True))


if __name__ == '__main__':
    try:
        main()
    except Refused as error:
        raise SystemExit('Quiet bootstrap refused: ' + str(error)) from None
    except Exception:
        raise SystemExit('Quiet bootstrap refused: preflight failed; no command output or secret is disclosed') from None
