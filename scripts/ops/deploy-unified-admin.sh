#!/usr/bin/env bash
# ECS full application rebuild and release. Does not recreate databases or other projects.
# Usage: bash deploy-unified-admin.sh RELEASE BACKEND_SHA H5_SHA FRONTEND_SHA
set -euo pipefail
umask 077
release="${1:?release}"; backend_sha="${2:?backend SHA}"; h5_sha="${3:?H5 SHA}"; frontend_sha="${4:?frontend SHA}"
[[ "$release" =~ ^[a-zA-Z0-9-]+$ && "$backend_sha" =~ ^[0-9a-f]{7,40}$ && "$h5_sha" =~ ^[0-9a-f]{7,40}$ && "$frontend_sha" =~ ^[0-9a-f]{7,40}$ ]]
base=/opt/askxuan
candidate="$base/runtime/$release"; backup="$base/backups/$release"; public="/var/www/askxuan/releases/$release/public"
mkdir -p "$candidate/backend" "$candidate/frontend/apps/web-h5" "$backup"
test ! -f "$candidate/DEPLOYED"
source "$base/runtime/secrets.env"
tar -xzf "$base/runtime/askxuan-backend-$backend_sha.tar.gz" -C "$candidate/backend"
tar -xzf "$base/runtime/askxuan-frontend-$frontend_sha.tar.gz" -C "$candidate/frontend"
tar -xzf "$base/runtime/askxuan-h5-source-$h5_sha.tar.gz" -C "$candidate/frontend/apps/web-h5"
if [[ ! -f "$backup/prepared" ]]; then
 readlink -f /var/www/askxuan/public > "$backup/previous-public"
 tar --exclude='./.local' --exclude='./logs' --exclude='./.git' -czf "$backup/backend-before.tar.gz" -C "$base/backend" .
 cp "$base/backend/.docker/etc/marketing/marketing.yaml" "$backup/marketing.yaml"
 docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" askxuan-mysql mysqldump -h127.0.0.1 -uroot --single-transaction --routines --triggers --no-tablespaces --all-databases > "$backup/databases.sql"
 test -s "$backup/databases.sql"
 # Freeze current production bindings and runtime environment; never render credentials in logs.
 python3 - "$candidate" "$backup" "$release" <<'PY'
from pathlib import Path
import json,subprocess,sys
candidate,backup,release=map(str,sys.argv[1:]);root=Path(candidate)/'backend'
services={};rollback={};networks={};volumes={};mapping=[]
for mod in sorted(root.glob('services/*/*-service')):
 name=mod.name;c=json.loads(subprocess.check_output(['docker','inspect','askxuan-'+name]))[0]
 oldtag=f'askxuan/{name}:before-{release}'
 subprocess.run(['docker','tag',c['Image'],oldtag],check=True)
 cfg=c['Config'];host=c['HostConfig']
 definition={'container_name':c['Name'].lstrip('/'),'image':f'askxuan/{name}:{release}','restart':host['RestartPolicy']['Name'] or 'unless-stopped','environment':dict(x.split('=',1) for x in cfg['Env']),'volumes':[],'networks':{}}
 for mount in c['Mounts']:
  if mount['Type']=='volume':volumes[mount['Name']]={'external':True,'name':mount['Name']}
  definition['volumes'].append({'type':mount['Type'],'source':mount.get('Name') if mount['Type']=='volume' else mount['Source'],'target':mount['Destination'],'read_only':not mount['RW']})
 definition['ports']=[{'target':int(port.split('/')[0]),'published':binding['HostPort'],'host_ip':binding['HostIp'] or '0.0.0.0','protocol':port.split('/')[1]} for port,bindings in (host.get('PortBindings') or {}).items() for binding in (bindings or [])]
 for network,detail in c['NetworkSettings']['Networks'].items():
  networks[network]={'external':True,'name':network}
  definition['networks'][network]={'aliases':list(dict.fromkeys([name]+(detail.get('Aliases') or [])))}
 if host.get('ExtraHosts'):definition['extra_hosts']=host['ExtraHosts']
 if host.get('ReadonlyRootfs'):definition['read_only']=True
 for raw,key in [('CapAdd','cap_add'),('CapDrop','cap_drop'),('SecurityOpt','security_opt')]:
  if host.get(raw):definition[key]=host[raw]
 if host.get('Memory'):definition['mem_limit']=host['Memory']
 if host.get('NanoCpus'):definition['cpus']=host['NanoCpus']/1e9
 services[name]=definition;rollback[name]={**definition,'image':oldtag}
 mapping.append([name,str(mod.relative_to(root)),name.removesuffix('-service')])
for path,data in [(Path(candidate)/'compose.release.json',services),(Path(backup)/'compose.rollback.json',rollback)]:
 path.write_text(json.dumps({'services':data,'networks':networks,'volumes':volumes}))
(Path(candidate)/'services.tsv').write_text('\n'.join('\t'.join(row) for row in mapping)+'\n')
PY
 touch "$backup/prepared"
fi
# Older Docker versions encode an all-interface published port with an empty HostIp.
# Compose requires either an explicit address or omission; keep the existing binding.
python3 - "$candidate/compose.release.json" "$backup/compose.rollback.json" <<'PYPORT'
import json,sys
from pathlib import Path
for name in sys.argv[1:]:
 p=Path(name);data=json.loads(p.read_text())
 for service in data['services'].values():
  for port in service.get('ports',[]):
   if not port.get('host_ip'):port.pop('host_ip',None)
 p.write_text(json.dumps(data))
PYPORT
echo BACKUP_OK
docker compose -p askxuan -f "$candidate/compose.release.json" config --quiet
# Build every Go application, with gateway last at activation. The shared compiler cache is retained.
while IFS=$'\t' read -r service path binary; do
 docker build -f "$candidate/backend/build/docker/Dockerfile" --build-arg "SERVICE=$path" --build-arg "BINARY=$binary" -t "askxuan/$service:$release" "$candidate/backend" > "$candidate/build-$service.log" 2>&1
 echo "BUILT $service"
done < "$candidate/services.tsv"
# Build all web delivery targets on ECS from the exact uploaded Git archives.
node_image="${ASKXUAN_NODE_IMAGE:-node:22-bookworm-slim}"
if ! docker image inspect "$node_image" >/dev/null 2>&1 && docker image inspect askxuan/taibu-mcp:local >/dev/null 2>&1; then
 # The cached MCP image contains Node 20 (the apps require Node >= 20); use it only as a compiler.
 node_image=askxuan/taibu-mcp:local
fi
if ! docker image inspect "$node_image" >/dev/null 2>&1; then
 docker pull docker.1ms.run/library/node:22-bookworm-slim > "$candidate/node-pull.log" 2>&1
 docker tag docker.1ms.run/library/node:22-bookworm-slim "$node_image"
fi
for app in web-h5 web-platform-admin web-shop-admin web-temple-admin; do
 docker run --rm --user 0:0 -v "$candidate/frontend:/workspace" -v "$base/runtime/npm-cache:/root/.npm" -w "/workspace/apps/$app" "$node_image" sh -c 'npm ci --registry=https://registry.npmmirror.com && npm run build' > "$candidate/build-$app.log" 2>&1
 echo "BUILT $app"
done
mkdir -p "$public"
cp -a "$(cat "$backup/previous-public")/." "$public/"
cp -a "$candidate/frontend/apps/web-h5/dist/." "$public/"
for pair in 'web-platform-admin admin' 'web-shop-admin shop' 'web-temple-admin temple'; do
 read -r app target <<< "$pair"
 mkdir -p "$public/$target"
 cp -a "$candidate/frontend/apps/$app/dist/." "$public/$target/"
done
chmod -R a+rX "/var/www/askxuan/releases/$release"
# Nothing live is changed before every build succeeds.
docker exec -i -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" askxuan-mysql mysql -h127.0.0.1 -uroot < "$candidate/backend/scripts/db/20260909_free_rewards.sql"
rollback() {
 trap - ERR
 cp "$backup/marketing.yaml" "$base/backend/.docker/etc/marketing/marketing.yaml"
 docker compose -p askxuan -f "$backup/compose.rollback.json" up -d --no-build --no-deps
 ln -s "$(cat "$backup/previous-public")" /var/www/askxuan/public.rollback
 mv -Tf /var/www/askxuan/public.rollback /var/www/askxuan/public
 echo ROLLED_BACK >&2
}
trap rollback ERR
python3 - "$base/backend/.docker/etc/gateway/gateway.yaml" "$base/backend/.docker/etc/marketing/marketing.yaml" <<'PY'
from pathlib import Path
import re,sys
key=re.search(r'^  AccessSecret:\s*(.+)$',Path(sys.argv[1]).read_text(),re.M)
if not key:raise RuntimeError('gateway signing key missing')
p=Path(sys.argv[2]);text=p.read_text()
if re.search(r'^  AccessSecret:',text,re.M):text=re.sub(r'^  AccessSecret:.*$',lambda _:'  AccessSecret: '+key[1],text,flags=re.M)
else:text+='\nAuth:\n  AccessSecret: '+key[1]+'\n'
p.write_text(text)
PY
mapfile -t services < <(cut -f1 "$candidate/services.tsv" | grep -v '^gateway-service$')
docker compose -p askxuan -f "$candidate/compose.release.json" up -d --no-build --no-deps "${services[@]}"
docker compose -p askxuan -f "$candidate/compose.release.json" up -d --no-build --no-deps gateway-service
healthy=false
for attempt in $(seq 1 60); do
 if python3 - "$candidate/services.tsv" <<'PY'
import json,subprocess,sys
names=['askxuan-'+row.split('\t')[0] for row in open(sys.argv[1])]
containers=json.loads(subprocess.check_output(['docker','inspect',*names]))
sys.exit(0 if all(c['State']['Status']=='running' and c['State'].get('Health',{}).get('Status')=='healthy' for c in containers) else 1)
PY
 then healthy=true;break;fi
 sleep 2
done
"$healthy"
curl -fsS http://127.0.0.1:8080/api/v1/health > "$candidate/gateway-health.json"
curl -fsS http://127.0.0.1:8096/api/v1/marketing/rewards/campaigns > "$candidate/anonymous.json"
python3 - "$candidate/anonymous.json" <<'PY'
import json,sys
assert json.load(open(sys.argv[1]))['code']==40101
print('SERVICE_AUTH_OK')
PY
nginx -t
ln -s "$public" /var/www/askxuan/public.next
mv -Tf /var/www/askxuan/public.next /var/www/askxuan/public
# Synchronize maintained sources only after successful activation; keep production mounted configs.
tar -xzf "$base/runtime/askxuan-backend-$backend_sha.tar.gz" -C "$base/backend"
tar -xzf "$base/runtime/askxuan-frontend-$frontend_sha.tar.gz" -C "$base/frontend"
tar -xzf "$base/runtime/askxuan-h5-source-$h5_sha.tar.gz" -C "$base/frontend/apps/web-h5"
while IFS=$'\t' read -r service _; do docker tag "askxuan/$service:$release" "askxuan/$service:local";done < "$candidate/services.tsv"
printf 'backend=%s\nh5=%s\nfrontend=%s\nrelease=%s\n' "$backend_sha" "$h5_sha" "$frontend_sha" "$release" > "$candidate/release.txt"
touch "$candidate/DEPLOYED"
trap - ERR
echo "DEPLOYED $release"
