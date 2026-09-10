#!/usr/bin/env python3
"""Use the running DIY service's database identity without logging credentials."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys

container = json.loads(subprocess.check_output(['docker', 'inspect', 'askxuan-diy-service']))[0]
mount = next(m for m in container['Mounts'] if m['Destination'] == '/app/etc')
config = (Path(mount['Source']) / 'diy.yaml').read_text()
raw = re.search(r'^\s*DataSource:\s*(.+?)\s*$', config, re.MULTILINE).group(1)
if raw.startswith('"'):
    raw = json.loads(raw)
elif raw.startswith("'") and raw.endswith("'"):
    raw = raw[1:-1].replace("''", "'")
identity, destination = raw.rsplit('@tcp(', 1)
user, password = identity.split(':', 1)
database = destination.rsplit(')/', 1)[1].split('?', 1)[0]
assert database == 'askxuan_diy', 'Only the DIY database is permitted'
environment = {**os.environ, 'MYSQL_PWD': password}
command = ['docker', 'exec', '-i', '-e', 'MYSQL_PWD', 'askxuan-mysql']
action = sys.argv[1]
if action == 'dump':
    command += ['mysqldump', '--default-character-set=utf8mb4', '-u', user, '--single-transaction', '--no-tablespaces', '--set-gtid-purged=OFF', database]
elif action == 'apply':
    command += ['mysql', '--default-character-set=utf8mb4', '-u', user, database]
else:
    raise ValueError('Expected dump or apply')
subprocess.run(command, env=environment, check=True)
