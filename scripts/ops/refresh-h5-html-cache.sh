#!/usr/bin/env bash
# Apply the H5 entrypoint cache rule without touching API, media, or hashed assets.
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
import sys
p=Path(sys.argv[1]);s=p.read_text();line='include /etc/nginx/snippets/askxuan-h5-html-cache.conf;'
if line not in s:
 assert 'location = /index.html' not in s
 p.write_text(s+'\n'+line+'\n')
PY
nginx -t
nginx -s reload
trap - ERR
