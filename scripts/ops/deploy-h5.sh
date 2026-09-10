#!/usr/bin/env bash
# Build an exact H5 Git archive on ECS and activate an isolated static release.
# Usage: bash deploy-h5.sh RELEASE H5_SHA
# Upload /opt/askxuan/runtime/askxuan-h5-source-H5_SHA.tar.gz first.
set -euo pipefail
umask 077
release="${1:?release}"; h5_sha="${2:?H5 SHA}"
[[ "$release" =~ ^[a-zA-Z0-9-]+$ && "$h5_sha" =~ ^[0-9a-f]{7,40}$ ]]
base=/opt/askxuan
candidate="$base/runtime/$release"; backup="$base/backups/$release"
public="/var/www/askxuan/releases/$release/public"
previous=$(readlink -f /var/www/askxuan/public)
test -f "$previous/index.html"
test ! -e "$candidate"
test ! -e "$public"
mkdir -p "$candidate/frontend/apps/web-h5" "$backup" "$public"
printf '%s\n' "$previous" > "$backup/previous-public"
tar --exclude='./node_modules' --exclude='./dist' --exclude='./.git' -czf "$backup/h5-source-before.tar.gz" -C "$base/frontend/apps/web-h5" .
# Record inherited application provenance; this release changes no service images or databases.
previous_release=$(basename "$(dirname "$previous")")
cp "$base/runtime/$previous_release/release.txt" "$backup/previous-release.txt"
frontend_sha=$(sed -n 's/^frontend=//p' "$backup/previous-release.txt")
[[ "$frontend_sha" =~ ^[0-9a-f]{7,40}$ ]]
# H5 imports packages/domain-status using the frontend monorepo directory layout.
tar -xzf "$base/runtime/askxuan-frontend-$frontend_sha.tar.gz" -C "$candidate/frontend"
tar -xzf "$base/runtime/askxuan-h5-source-$h5_sha.tar.gz" -C "$candidate/frontend/apps/web-h5"
docker ps --format '{{.Names}} {{.Image}} {{.Status}}' > "$backup/containers-before.txt"
node_image="${ASKXUAN_NODE_IMAGE:-node:22-bookworm-slim}"
if ! docker image inspect "$node_image" >/dev/null 2>&1; then node_image=askxuan/taibu-mcp:local;fi
docker image inspect "$node_image" >/dev/null
docker run --rm --user 0:0 -v "$candidate/frontend:/workspace" -v "$base/runtime/npm-cache:/root/.npm" -w /workspace/apps/web-h5 "$node_image" sh -c 'npm ci --registry=https://registry.npmmirror.com && npm run build' > "$candidate/build-h5.log" 2>&1
test -s "$candidate/frontend/apps/web-h5/dist/index.html"
cp -a "$previous/." "$public/"
cp -a "$candidate/frontend/apps/web-h5/dist/." "$public/"
chmod -R a+rX "/var/www/askxuan/releases/$release"
# Verify compatibility apps were inherited byte-for-byte before activation.
for target in admin shop temple; do diff -qr "$previous/$target" "$public/$target" >/dev/null;done
nginx -t
curl -fsS http://127.0.0.1:8080/api/v1/health > "$candidate/gateway-health.json"
rollback() {
 trap - ERR
 ln -s "$previous" "/var/www/askxuan/public.rollback-$release"
 mv -Tf "/var/www/askxuan/public.rollback-$release" /var/www/askxuan/public
 tar -xzf "$backup/h5-source-before.tar.gz" -C "$base/frontend/apps/web-h5"
 echo ROLLED_BACK >&2
}
trap rollback ERR
ln -s "$public" "/var/www/askxuan/public.next-$release"
mv -Tf "/var/www/askxuan/public.next-$release" /var/www/askxuan/public
tar -xzf "$base/runtime/askxuan-h5-source-$h5_sha.tar.gz" -C "$base/frontend/apps/web-h5"
python3 - "$backup/previous-release.txt" "$candidate/release.txt" "$release" "$h5_sha" "$previous_release" <<'PY'
import sys
from pathlib import Path
old,new,release,h5,parent=sys.argv[1:]
values=dict(line.split('=',1) for line in Path(old).read_text().splitlines() if '=' in line)
values.update(h5=h5,release=release,inherited_from=parent)
Path(new).write_text(''.join(f'{k}={v}\n' for k,v in values.items()))
PY
cmp "$public/index.html" "$candidate/frontend/apps/web-h5/dist/index.html"
touch "$candidate/DEPLOYED"
trap - ERR
echo "DEPLOYED $release"
