#!/bin/bash
# Generate container-network configs for local docker compose.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="${ROOT}/.docker/etc"

rm -rf "${OUT}"
mkdir -p "${OUT}"

render_service() {
  local name="$1"
  local src="$2"
  local dst="${OUT}/${name}"

  mkdir -p "${dst}"
  cp "${src}"/*.yaml "${dst}/"

  perl -pi -e '
    s/127\.0\.0\.1:3306/mysql:3306/g;
    s/localhost:6379/redis:6379/g;
    s/localhost:2379/etcd:2379/g;
    s/http:\/\/localhost:2379/http:\/\/etcd:2379/g;
    s/Host: localhost/Host: rabbitmq/g;
    s/Endpoint: localhost:9000/Endpoint: minio:9000/g;
    s/http:\/\/127\.0\.0\.1:10002/http:\/\/host.docker.internal:10002/g;
    s/http:\/\/localhost:8080/http:\/\/gateway-service:8080/g;
    s/ListenOn: 127\.0\.0\.1:9088/ListenOn: 0.0.0.0:9088/g;
  ' "${dst}"/*.yaml
}

render_service gateway  "${ROOT}/services/platform/gateway-service/etc"
render_service auth     "${ROOT}/services/platform/auth-service/etc"
render_service user     "${ROOT}/services/platform/user-service/etc"
render_service temple   "${ROOT}/services/content/temple-service/etc"
render_service master   "${ROOT}/services/content/master-service/etc"
render_service booking  "${ROOT}/services/content/booking-service/etc"
render_service community "${ROOT}/services/content/community-service/etc"
render_service review   "${ROOT}/services/content/review-service/etc"
render_service product  "${ROOT}/services/commerce/product-service/etc"
render_service order    "${ROOT}/services/commerce/order-service/etc"
render_service payment  "${ROOT}/services/commerce/payment-service/etc"
render_service diy      "${ROOT}/services/commerce/diy-service/etc"
render_service marketing "${ROOT}/services/operation/marketing-service/etc"
render_service logistics "${ROOT}/services/operation/logistics-service/etc"
render_service finance  "${ROOT}/services/operation/finance-service/etc"
render_service audit    "${ROOT}/services/operation/audit-service/etc"
render_service message  "${ROOT}/services/infrastructure/message-service/etc"
render_service file     "${ROOT}/services/infrastructure/file-service/etc"
render_service ai       "${ROOT}/services/infrastructure/ai-service/etc"
render_service media    "${ROOT}/services/infrastructure/media-service/etc"

# Gateway uses static in-compose service names so it remains stable even before
# any optional etcd discovery data is available.
perl -pi -e '
  s/Target: localhost:8081/Target: auth-service:8081/g;
  s/Target: localhost:8082/Target: user-service:8082/g;
  s/Target: localhost:8083/Target: temple-service:8083/g;
  s/Target: localhost:8084/Target: master-service:8084/g;
  s/Target: localhost:8085/Target: booking-service:8085/g;
  s/Target: localhost:8086/Target: product-service:8086/g;
  s/Target: localhost:8088/Target: diy-service:8088/g;
  s/Target: localhost:8089/Target: order-service:8089/g;
  s/Target: localhost:8090/Target: payment-service:8090/g;
  s/Target: localhost:8091/Target: finance-service:8091/g;
  s/Target: localhost:8092/Target: review-service:8092/g;
  s/Target: localhost:8093/Target: audit-service:8093/g;
  s/Target: localhost:8094/Target: message-service:8094/g;
  s/Target: localhost:8095/Target: logistics-service:8095/g;
  s/Target: localhost:8096/Target: marketing-service:8096/g;
  s/Target: localhost:8097/Target: file-service:8097/g;
  s/Target: localhost:8098/Target: ai-service:8098/g;
  s/Target: localhost:8099/Target: community-service:8099/g;
  s/Target: localhost:8100/Target: media-service:8100/g;
  s/Target: localhost:10002/Target: host.docker.internal:10002/g;
' "${OUT}/gateway/gateway.yaml"

echo "Rendered docker configs to ${OUT}"
