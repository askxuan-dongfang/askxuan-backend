#!/usr/bin/env bash
# Install reviewed beta-only routes; restore every touched config on failure.
set -euo pipefail
umask 077
source_dir="${1:?reviewed deploy/nginx directory}"
backup="${2:?new root-only backup directory}"
config=/etc/nginx/snippets/askxuan-app-locations.conf
limits=/etc/nginx/conf.d/00-askxuan-quiet-limits.conf
locations=/etc/nginx/snippets/askxuan-quiet-locations.conf
exec 9<>/opt/askxuan/ci/publish.lock
flock -x 9
test -f "$source_dir/quiet-limits.conf"
test -f "$source_dir/quiet-locations.conf"
test -f "$config"
test ! -e "$backup"
mkdir -m 700 "$backup"
cp "$config" "$backup/app-locations-before.conf"
if [[ -e "$limits" ]]; then cp "$limits" "$backup/limits-before.conf"; fi
if [[ -e "$locations" ]]; then cp "$locations" "$backup/locations-before.conf"; fi
rollback() {
  trap - EXIT HUP INT TERM
  if [[ "$committed" = 1 ]]; then return; fi
  cp "$backup/app-locations-before.conf" "$config"
  if [[ -e "$backup/limits-before.conf" ]]; then cp "$backup/limits-before.conf" "$limits"; else rm -f "$limits"; fi
  if [[ -e "$backup/locations-before.conf" ]]; then cp "$backup/locations-before.conf" "$locations"; else rm -f "$locations"; fi
  nginx -t && nginx -s reload
}
committed=0
trap rollback EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
install -m 644 "$source_dir/quiet-limits.conf" "$limits"
install -m 644 "$source_dir/quiet-locations.conf" "$locations"
python3 - "$config" <<'PY'
from pathlib import Path
import re, sys
p = Path(sys.argv[1])
s = p.read_text()
line = 'include /etc/nginx/snippets/askxuan-quiet-locations.conf;'
assert not re.search(r'location\s+(?:[=~^*]+\s+)?/quiet-(?:api|ws|health)', s), 'Existing custom quiet routes need review'
assert s.count(line) <= 1, 'Duplicate quiet include'
if line not in s:
    p.write_text(s + '\n' + line + '\n')
PY
nginx -t
nginx -s reload
committed=1
trap - EXIT HUP INT TERM
