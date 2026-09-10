#!/usr/bin/env bash
# Apply a preverified DIY binary + static bundle without changing active source checkouts.
# Usage: deploy-diy-studio.sh RELEASE EXPECTED_PUBLIC
set -euo pipefail
umask 077
release="${1:?release}"; expected="${2:?expected public directory}"
[[ "$release" =~ ^[a-zA-Z0-9-]+$ && "$expected" == /var/www/askxuan/releases/*/public ]]
base=/opt/askxuan
candidate="$base/runtime/$release"; backup="$base/backups/$release"
public="/var/www/askxuan/releases/$release/public"
[[ "$(readlink -f /var/www/askxuan/public)" == "$expected" ]]
test -s "$candidate/diy"; test -s "$candidate/h5/index.html"; test -s "$candidate/admin/index.html"
test ! -e "$public"; mkdir -p "$public" "$backup"
cp -a "$expected/." "$public/"
cp -a "$candidate/h5/." "$public/"
mkdir -p "$public/admin"; cp -a "$candidate/admin/." "$public/admin/"
chmod -R a+rX "/var/www/askxuan/releases/$release"
python3 - "$candidate" "$backup" "$release" <<'PYDEPLOY'
import json,subprocess,sys
from pathlib import Path
candidate,backup,release=sys.argv[1:]
c=json.loads(subprocess.check_output(['docker','inspect','askxuan-diy-service']))[0]
oldtag='askxuan/diy-service:before-'+release
subprocess.run(['docker','tag',c['Image'],oldtag],check=True)
host=c['HostConfig'];cfg=c['Config'];networks={};volumes={}
d={'container_name':'askxuan-diy-service','image':'askxuan/diy-service:'+release,'restart':host['RestartPolicy']['Name'] or 'unless-stopped','environment':dict(v.split('=',1) for v in cfg['Env']),'volumes':[],'networks':{}}
for m in c['Mounts']:
 if m['Type']=='volume':volumes[m['Name']]={'external':True,'name':m['Name']}
 d['volumes'].append({'type':m['Type'],'source':m.get('Name') if m['Type']=='volume' else m['Source'],'target':m['Destination'],'read_only':not m['RW']})
d['ports']=[{'target':int(p.split('/')[0]),'published':b['HostPort'],'host_ip':b['HostIp'] or '0.0.0.0','protocol':p.split('/')[1]} for p,bindings in (host.get('PortBindings') or {}).items() for b in (bindings or [])]
for n,v in c['NetworkSettings']['Networks'].items():
 networks[n]={'external':True,'name':n};d['networks'][n]={'aliases':list(dict.fromkeys(['diy-service']+(v.get('Aliases') or [])))}
if host.get('ExtraHosts'):d['extra_hosts']=host['ExtraHosts']
if host.get('ReadonlyRootfs'):d['read_only']=True
for raw,key in [('CapAdd','cap_add'),('CapDrop','cap_drop'),('SecurityOpt','security_opt')]:
 if host.get(raw):d[key]=host[raw]
if host.get('Memory'):d['mem_limit']=host['Memory']
if host.get('NanoCpus'):d['cpus']=host['NanoCpus']/1e9
for path,definition in [(Path(candidate)/'compose.release.json',d),(Path(backup)/'compose.rollback.json',{**d,'image':oldtag})]:path.write_text(json.dumps({'services':{'diy-service':definition},'networks':networks,'volumes':volumes}))
(Path(backup)/'image-before.txt').write_text(c['Image']+'\n')
(Path(candidate)/'Dockerfile').write_text('FROM '+oldtag+'\nCOPY --chown=1000:1000 --chmod=755 diy /app/diy\n')
PYDEPLOY
docker build -t "askxuan/diy-service:$release" "$candidate" > "$candidate/build-diy.log" 2>&1
docker compose -p askxuan -f "$candidate/compose.release.json" config --quiet
# Back up only the DIY database. Password is consumed inside its own container.
docker exec askxuan-mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysqldump -uroot --single-transaction --databases askxuan_diy' > "$backup/diy-before.sql"
test -s "$backup/diy-before.sql"
[[ "$(readlink -f /var/www/askxuan/public)" == "$expected" ]]
docker exec -i askxuan-mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot' < "$candidate/20260911_diy_studio.sql"
docker exec -i askxuan-mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot' < "$candidate/20260911_diy_reference_assets.sql"
printf '%s\n' "$expected" > "$backup/previous-public"
rollback(){
 trap - ERR
 docker compose -p askxuan -f "$backup/compose.rollback.json" up -d --no-build --no-deps diy-service
 if [[ "$(readlink -f /var/www/askxuan/public)" == "$public" ]];then ln -s "$expected" "/var/www/askxuan/public.rollback-$release";mv -Tf "/var/www/askxuan/public.rollback-$release" /var/www/askxuan/public;fi
 echo ROLLED_BACK_DIY >&2
}
trap rollback ERR
docker compose -p askxuan -f "$candidate/compose.release.json" up -d --no-build --no-deps diy-service
healthy=false
for attempt in $(seq 1 60);do if [[ "$(docker inspect askxuan-diy-service --format '{{.State.Health.Status}}')" == healthy ]];then healthy=true;break;fi;sleep 2;done
"$healthy"
curl -fsS http://127.0.0.1:8080/api/v1/diy/materials > "$candidate/materials-smoke.json"
python3 - "$candidate/materials-smoke.json" <<'PYSMOKE'
import json,sys
r=json.load(open(sys.argv[1]));assert r['code']==0 and r['data']['list'];assert any(m.get('renderAssets') for m in r['data']['list'])
PYSMOKE
nginx -t
# Compare again immediately before the atomic pointer change: never replace a newer release.
[[ "$(readlink -f /var/www/askxuan/public)" == "$expected" ]]
ln -s "$public" "/var/www/askxuan/public.next-$release";mv -Tf "/var/www/askxuan/public.next-$release" /var/www/askxuan/public
cmp "$public/index.html" "$candidate/h5/index.html";cmp "$public/admin/index.html" "$candidate/admin/index.html"
python3 - "$candidate" "$expected" "$release" <<'PYVERSIONS'
from pathlib import Path
import sys
candidate,expected,release=sys.argv[1:];p=Path(candidate)
previous=Path('/opt/askxuan/runtime')/Path(expected).parent.name/'release.txt'
values=dict(line.split('=',1) for line in previous.read_text().splitlines() if '=' in line)
values.update(dict(line.split('=',1) for line in (p/'versions.txt').read_text().splitlines() if '=' in line))
values.update(release=release,inherited_from=Path(expected).parent.name)
(p/'release.txt').write_text(''.join(f'{k}={v}\n' for k,v in values.items()))
PYVERSIONS
touch "$candidate/DEPLOYED"
trap - ERR
echo "DEPLOYED_DIY $release"
