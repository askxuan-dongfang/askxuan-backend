#!/usr/bin/env bash
# Secrets are provided only to the protected production deployment job.
set -euo pipefail
umask 077
temp="$(mktemp -d)"
trap 'rm -rf "$temp"' EXIT
printf '%s\n' "$ECS_DEPLOY_KEY" > "$temp/key"
printf '%s\n' "$ECS_KNOWN_HOSTS" > "$temp/known_hosts"
unset ECS_DEPLOY_KEY ECS_KNOWN_HOSTS
if [[ "${1:-}" == --artifact ]]; then
  artifact="$temp/request.json"
  python3 "$(dirname "$0")/artifact_pointer.py" > "$artifact"
  unset GH_TOKEN
else
  artifact="$(python3 "$(dirname "$0")/retag_release.py" "${1:-release.tgz}" "$temp/release.tgz")"
fi
ssh -T -i "$temp/key" -o IdentitiesOnly=yes -o BatchMode=yes \
  -o StrictHostKeyChecking=yes -o UserKnownHostsFile="$temp/known_hosts" \
  -o ServerAliveInterval=30 -o ServerAliveCountMax=20 \
  "askxuan-ci@$ECS_HOST" deploy < "$artifact"
