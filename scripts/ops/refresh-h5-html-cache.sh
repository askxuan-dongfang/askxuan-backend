#!/usr/bin/env bash
# Apply H5 entrypoint and install-manifest headers without touching API or media.
set -euo pipefail
umask 077
source_file="${1:?h5-html-cache.conf path}"
backup="${2:?backup directory}"
config=/etc/nginx/snippets/askxuan-app-locations.conf
snippet=/etc/nginx/snippets/askxuan-h5-html-cache.conf
mkdir -p "$backup"
test -f "$config"
test ! -e "$backup/app-locations-before.conf"
cp "$config" "$backup/app-locations-before.conf"
if [[ -e "$snippet" ]]; then cp "$snippet" "$backup/h5-html-cache-before.conf"; fi
rollback() {
 trap - ERR
 cp "$backup/app-locations-before.conf" "$config"
 if [[ -e "$backup/h5-html-cache-before.conf" ]]; then cp "$backup/h5-html-cache-before.conf" "$snippet"; fi
 nginx -t && nginx -s reload
}
trap rollback ERR
install -m 644 "$source_file" "$snippet"
python3 - "$config" <<'PY'
from pathlib import Path
import re, sys
p=Path(sys.argv[1]);s=p.read_text();line='include /etc/nginx/snippets/askxuan-h5-html-cache.conf;'
# Migrate only the known legacy location. Refuse custom blocks rather than
# silently overriding them or creating a duplicate exact-match location.
legacy=r'(?m)^location = /manifest\.webmanifest \{\s*try_files \$uri =404;\s*\}\n?'
s, removed=re.subn(legacy, '', s)
assert removed <= 1, 'Duplicate legacy manifest locations'
assert not re.search(r'location\s*=\s*/(?:manifest|master)\.webmanifest\b', s), 'Unexpected custom manifest location'
if line not in s:
 assert 'location = /index.html' not in s
 s=s+'\n'+line+'\n'
p.write_text(s)
PY
nginx -t
nginx -s reload
trap - ERR
