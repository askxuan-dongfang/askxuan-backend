#!/usr/bin/env bash
# Rebuild one allowed catalog flow service; preserve exact container config and rollback image.
# Usage: deploy-commerce-service.sh RELEASE BACKEND_SHA product|order|payment|finance
set -euo pipefail
umask 077
release="${1:?release}"; backend_sha="${2:?backend SHA}"
[[ "$release" =~ ^[a-zA-Z0-9-]+$ && "$backend_sha" =~ ^[0-9a-f]{7,40}$ ]]
service="${3:?service}"
case "$service" in product|order|payment) service_path="services/commerce/$service-service";; finance) service_path="services/operation/finance-service";; *) exit 2;; esac
base=/opt/askxuan
candidate="$base/runtime/$release"; backup="$base/backups/$release"
test ! -e "$candidate"
mkdir -p "$candidate/backend" "$backup"
tar -xzf "$base/runtime/askxuan-backend-$backend_sha.tar.gz" -C "$candidate/backend"
tar -czf "$backup/$service-source-before.tar.gz" -C "$base/backend" "$service_path" common
# Keep the exact current ports, mounts, environment and resource limits private.
python3 - "$candidate" "$backup" "$release" "$service" <<'PY'
import json,subprocess,sys
from pathlib import Path
candidate,backup,release,service=sys.argv[1:]
c=json.loads(subprocess.check_output(['docker','inspect','askxuan-'+service+'-service']))[0]
oldtag='askxuan/'+service+'-service:before-'+release
subprocess.run(['docker','tag',c['Image'],oldtag],check=True)
host=c['HostConfig']; cfg=c['Config']; networks={};volumes={}
d={'container_name':'askxuan-'+service+'-service','image':'askxuan/'+service+'-service:'+release,'restart':host['RestartPolicy']['Name'] or 'unless-stopped','environment':dict(v.split('=',1) for v in cfg['Env']),'volumes':[],'networks':{}}
for mount in c['Mounts']:
 if mount['Type']=='volume':volumes[mount['Name']]={'external':True,'name':mount['Name']}
 d['volumes'].append({'type':mount['Type'],'source':mount.get('Name') if mount['Type']=='volume' else mount['Source'],'target':mount['Destination'],'read_only':not mount['RW']})
d['ports']=[{'target':int(port.split('/')[0]),'published':binding['HostPort'],'host_ip':binding['HostIp'] or '0.0.0.0','protocol':port.split('/')[1]} for port,bindings in (host.get('PortBindings') or {}).items() for binding in (bindings or [])]
for network,detail in c['NetworkSettings']['Networks'].items():
 networks[network]={'external':True,'name':network}
 d['networks'][network]={'aliases':list(dict.fromkeys([service+'-service']+(detail.get('Aliases') or [])))}
if host.get('ExtraHosts'):d['extra_hosts']=host['ExtraHosts']
if host.get('ReadonlyRootfs'):d['read_only']=True
for raw,key in [('CapAdd','cap_add'),('CapDrop','cap_drop'),('SecurityOpt','security_opt')]:
 if host.get(raw):d[key]=host[raw]
if host.get('Memory'):d['mem_limit']=host['Memory']
if host.get('NanoCpus'):d['cpus']=host['NanoCpus']/1e9
for path,definition in [(Path(candidate)/'compose.release.json',d),(Path(backup)/'compose.rollback.json',{**d,'image':oldtag})]:
 path.write_text(json.dumps({'services':{service+'-service':definition},'networks':networks,'volumes':volumes}))
(Path(backup)/'image-before.txt').write_text(c['Image']+'\n')
PY
docker compose -p askxuan -f "$candidate/compose.release.json" config --quiet
docker build -f "$candidate/backend/build/docker/Dockerfile" --build-arg SERVICE="$service_path" --build-arg BINARY="$service" -t "askxuan/${service}-service:$release" "$candidate/backend" > "$candidate/build-$service.log" 2>&1
echo BUILT_$service
rollback() {
 trap - ERR
 docker compose -p askxuan -f "$backup/compose.rollback.json" up -d --no-build --no-deps ${service}-service
 tar -xzf "$backup/$service-source-before.tar.gz" -C "$base/backend"
 echo ROLLED_BACK_$service >&2
}
trap rollback ERR
docker compose -p askxuan -f "$candidate/compose.release.json" up -d --no-build --no-deps ${service}-service
healthy=false
for attempt in $(seq 1 60); do
 if [[ "$(docker inspect askxuan-${service}-service --format '{{.State.Health.Status}}')" == healthy ]];then healthy=true;break;fi
 sleep 2
done
"$healthy"
curl -fsS http://127.0.0.1:8080/api/v1/health > "$candidate/gateway-health.json"
tar -xzf "$base/runtime/askxuan-backend-$backend_sha.tar.gz" -C "$base/backend" "$service_path" common scripts/ops/deploy-commerce-service.sh
docker tag "askxuan/${service}-service:$release" askxuan/${service}-service:local
printf 'service_commit=%s\nrelease=%s\n' "$backend_sha" "$release" > "$candidate/release.txt"
touch "$candidate/DEPLOYED"
trap - ERR
echo "DEPLOYED_$service $release"
