#!/usr/bin/env bash
# Run on the authorized ECS after uploading git archives to /opt/askxuan/runtime.
# Usage: bash deploy-free-rewards.sh RELEASE BACKEND_SHA H5_SHA FRONTEND_SHA
set -euo pipefail
umask 077
release="${1:?release required}"; backend_sha="${2:?backend SHA required}"; h5_sha="${3:?H5 SHA required}"; frontend_sha="${4:?frontend SHA required}"
[[ "$release" =~ ^[a-zA-Z0-9-]+$ && "$backend_sha" =~ ^[0-9a-f]{7,40}$ && "$h5_sha" =~ ^[0-9a-f]{7,40}$ && "$frontend_sha" =~ ^[0-9a-f]{7,40}$ ]]
base=/opt/askxuan
source "$base/runtime/secrets.env"
backup="$base/backups/$release"; candidate="$base/runtime/$release"; public="/var/www/askxuan/releases/$release/public"
mkdir -p "$backup" "$candidate/source"
if [[ ! -f "$backup/prepared" ]]; then
 readlink -f /var/www/askxuan/public > "$backup/previous-public"
 tar --exclude='./.local' --exclude='./logs' --exclude='./.git' -czf "$backup/backend-before.tar.gz" -C "$base/backend" .
 docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" askxuan-mysql mysqldump -h127.0.0.1 -uroot --single-transaction --routines --triggers --no-tablespaces --databases askxuan_marketing > "$backup/marketing.sql"
 test -s "$backup/marketing.sql"
 cp "$base/backend/.docker/etc/marketing/marketing.yaml" "$backup/marketing.yaml"
 docker tag "$(docker inspect askxuan-marketing-service --format '{{.Image}}')" "askxuan/marketing-service:before-$release"
 python3 - "$backup/environment.json" <<'PY'
import json,subprocess,sys
c=json.loads(subprocess.check_output(['docker','inspect','askxuan-marketing-service']))[0]
with open(sys.argv[1],'w') as f:json.dump({'services':{'marketing-service':{'environment':dict(x.split('=',1) for x in c['Config']['Env'])}}},f)
PY
 touch "$backup/prepared"
fi
echo BACKUP_OK
tar -xzf "$base/runtime/askxuan-backend-$backend_sha.tar.gz" -C "$candidate/source"
cd "$candidate/source"
docker build -f build/docker/Dockerfile --build-arg SERVICE=services/operation/marketing-service --build-arg BINARY=marketing -t "askxuan/marketing-service:$release" . > "$candidate/marketing-build.log" 2>&1
echo BUILD_OK
mkdir -p "$public"
cp -a "$(cat "$backup/previous-public")/." "$public/"
tar -xzf "$base/runtime/askxuan-h5-$h5_sha.tar.gz" -C "$public"
tar -xzf "$base/runtime/askxuan-admin-$frontend_sha.tar.gz" -C "$public"
chmod -R a+rX "/var/www/askxuan/releases/$release"
# The migration is additive, repeatable and contains no production sample activities.
docker exec -i -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" askxuan-mysql mysql -h127.0.0.1 -uroot < "$candidate/source/scripts/db/20260909_free_rewards.sql"
docker exec -i -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" askxuan-mysql mysql -h127.0.0.1 -uroot < "$candidate/source/scripts/db/20260910_points_rewards.sql"
docker exec -i -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" askxuan-mysql mysql -h127.0.0.1 -uroot < "$candidate/source/scripts/db/20260910_points_rewards_permissions.sql"
cd "$base/backend"
compose=(docker compose -p askxuan -f docker-compose.yml -f "$base/runtime/docker-compose.full.ecs.yml" -f "$backup/environment.json")
rollback() {
 trap - ERR
 cp "$backup/marketing.yaml" "$base/backend/.docker/etc/marketing/marketing.yaml"
 docker tag "askxuan/marketing-service:before-$release" askxuan/marketing-service:local
 "${compose[@]}" up -d --no-build --no-deps marketing-service
 ln -s "$(cat "$backup/previous-public")" /var/www/askxuan/public.rollback
 mv -Tf /var/www/askxuan/public.rollback /var/www/askxuan/public
 echo ROLLED_BACK >&2
}
trap rollback ERR
# Copy only the access signing key into the existing marketing config; never log it.
python3 - "$base/backend/.docker/etc/gateway/gateway.yaml" "$base/backend/.docker/etc/marketing/marketing.yaml" <<'PY'
from pathlib import Path
import re,sys
gateway=Path(sys.argv[1]).read_text();p=Path(sys.argv[2]);text=p.read_text()
m=re.search(r'^  AccessSecret:\s*(.+)$',gateway,re.M)
if not m:raise RuntimeError('gateway access signing key missing')
key=m.group(1)
if re.search(r'^  AccessSecret:',text,re.M):text=re.sub(r'^  AccessSecret:.*$',lambda _: '  AccessSecret: '+key,text,flags=re.M)
else:text+='\nAuth:\n  AccessSecret: '+key+'\n'
p.write_text(text)
PY
docker tag "askxuan/marketing-service:$release" askxuan/marketing-service:local
"${compose[@]}" up -d --no-build --no-deps marketing-service
healthy=false
for attempt in $(seq 1 30); do if [[ "$(docker inspect askxuan-marketing-service --format '{{.State.Health.Status}}')" = healthy ]]; then healthy=true;break;fi;sleep 2;done
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
tar -xzf "$base/runtime/askxuan-backend-$backend_sha.tar.gz" -C "$base/backend"
printf 'backend=%s\nh5=%s\nfrontend=%s\nrelease=%s\n' "$backend_sha" "$h5_sha" "$frontend_sha" "$release" > "$candidate/release.txt"
trap - ERR
echo "DEPLOYED $release"
