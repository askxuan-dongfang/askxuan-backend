#!/bin/sh
set -eu
cd "$(dirname "$0")"
umask 077
mkdir -p .local/certs
if [ ! -f .local/stack.env ]; then
  {
    printf 'QUIET_REDIS_PASSWORD=%s\n' "$(openssl rand -hex 24)"
    printf 'QUIET_MYSQL_PASSWORD=%s\n' "$(openssl rand -hex 24)"
    printf 'QUIET_MYSQL_ROOT_PASSWORD=%s\n' "$(openssl rand -hex 24)"
    printf 'QUIET_CSRF_KEY=%s\n' "$(openssl rand -hex 32)"
  } > .local/stack.env
fi
if [ ! -f .local/certs/local.crt ]; then
  openssl req -x509 -newkey rsa:2048 -nodes -days 30 -subj '/CN=localhost' -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1' -keyout .local/certs/local.key -out .local/certs/local.crt >/dev/null 2>&1
fi
case "${1:-up}" in
  deps) docker compose --env-file .local/stack.env up -d --wait redis mysql ;;
  up) docker compose --env-file .local/stack.env up -d --build --wait ;;
  down) docker compose --env-file .local/stack.env down ;;
  test)
    set -a
    . ./.local/stack.env
    set +a
    QUIET_TEST_INTEGRATION=1 GOWORK=off go test -race -v ./...
    ;;
  *) printf 'Usage: %s [deps|up|down|test]\n' "$0" >&2; exit 2 ;;
esac
