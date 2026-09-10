#!/usr/bin/env bash
# Exact H5/admin build. Keep the latest deployed shop/temple apps and all unrelated services.
set -euo pipefail
umask 077
release="${1:?release}"; h5_sha="${2:?H5 SHA}"; frontend_sha="${3:?frontend SHA}"; community_sha="${4:?community SHA}"
[[ "$release" =~ ^[a-zA-Z0-9-]+$ && "$h5_sha" =~ ^[0-9a-f]{7,40}$ && "$frontend_sha" =~ ^[0-9a-f]{7,40}$ && "$community_sha" =~ ^[0-9a-f]{7,40}$ ]]
base=/opt/askxuan; candidate="$base/runtime/$release"; backup="$base/backups/$release"; public="/var/www/askxuan/releases/$release/public"
previous=$(readlink -f /var/www/askxuan/public)
test -f "$previous/index.html"; test ! -e "$candidate"; test ! -e "$public"
mkdir -p "$candidate/frontend/apps/web-h5" "$backup" "$public"
printf '%s\n' "$previous" > "$backup/previous-public"
parent=$(basename "$(dirname "$previous")")
cp "$base/runtime/$parent/release.txt" "$backup/previous-release.txt"
tar --exclude='./node_modules' --exclude='./dist' --exclude='./.git' -czf "$backup/h5-source-before.tar.gz" -C "$base/frontend/apps/web-h5" .
tar -czf "$backup/admin-community-source-before.tar.gz" -C "$base/frontend" apps/web-platform-admin/src/api/community.ts apps/web-platform-admin/src/views/audit/ContentDesignView.vue
tar -xzf "$base/runtime/askxuan-frontend-$frontend_sha.tar.gz" -C "$candidate/frontend"
tar -xzf "$base/runtime/askxuan-h5-source-$h5_sha.tar.gz" -C "$candidate/frontend/apps/web-h5"
node_image="${ASKXUAN_NODE_IMAGE:-node:22-bookworm-slim}"
if ! docker image inspect "$node_image" >/dev/null 2>&1; then node_image=askxuan/taibu-mcp:local;fi
for app in web-h5 web-platform-admin; do
 docker run --rm --user 0:0 -v "$candidate/frontend:/workspace" -v "$base/runtime/npm-cache:/root/.npm" -w "/workspace/apps/$app" "$node_image" sh -c 'npm ci --registry=https://registry.npmmirror.com && npm run build' > "$candidate/build-$app.log" 2>&1
 test -s "$candidate/frontend/apps/$app/dist/index.html"
 echo "BUILT $app"
done
# Refuse activation if another task has changed the live release during the build.
test "$(readlink -f /var/www/askxuan/public)" = "$previous"
cp -a "$previous/." "$public/"
cp -a "$candidate/frontend/apps/web-h5/dist/." "$public/"
cp -a "$candidate/frontend/apps/web-platform-admin/dist/." "$public/admin/"
for target in shop temple; do diff -qr "$previous/$target" "$public/$target" >/dev/null;done
chmod -R a+rX "/var/www/askxuan/releases/$release"
nginx -t
curl -fsS http://127.0.0.1:8080/api/v1/health > "$candidate/gateway-health.json"
rollback() {
 trap - ERR
 ln -s "$previous" "/var/www/askxuan/public.rollback-$release"
 mv -Tf "/var/www/askxuan/public.rollback-$release" /var/www/askxuan/public
 tar -xzf "$backup/h5-source-before.tar.gz" -C "$base/frontend/apps/web-h5"
 tar -xzf "$backup/admin-community-source-before.tar.gz" -C "$base/frontend"
 echo ROLLED_BACK_COMMUNITY_WEB >&2
}
trap rollback ERR
ln -s "$public" "/var/www/askxuan/public.next-$release"
mv -Tf "/var/www/askxuan/public.next-$release" /var/www/askxuan/public
tar -xzf "$base/runtime/askxuan-h5-source-$h5_sha.tar.gz" -C "$base/frontend/apps/web-h5"
tar -xzf "$base/runtime/askxuan-frontend-$frontend_sha.tar.gz" -C "$base/frontend" apps/web-platform-admin/src/api/community.ts apps/web-platform-admin/src/views/audit/ContentDesignView.vue
python3 - "$backup/previous-release.txt" "$candidate/release.txt" "$release" "$h5_sha" "$frontend_sha" "$community_sha" "$parent" <<'PY'
import sys
from pathlib import Path
old,new,release,h5,frontend,community,parent=sys.argv[1:]
values=dict(line.split('=',1) for line in Path(old).read_text().splitlines() if '=' in line)
values.update(h5=h5,frontend=frontend,community_service=community,release=release,inherited_from=parent)
Path(new).write_text(''.join(f'{k}={v}\n' for k,v in values.items()))
PY
cmp "$public/index.html" "$candidate/frontend/apps/web-h5/dist/index.html"
touch "$candidate/DEPLOYED"
trap - ERR
echo "DEPLOYED_COMMUNITY_WEB $release"
