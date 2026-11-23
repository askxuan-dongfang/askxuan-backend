-- Freeze the reviewed terms with the same transaction as each new English checkout.
CREATE TABLE IF NOT EXISTS askxuan_order.catalog_checkout_snapshot (
  id BIGINT NOT NULL PRIMARY KEY,
  payload JSON NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS askxuan_payment.catalog_checkout_snapshot LIKE askxuan_order.catalog_checkout_snapshot;
CREATE TABLE IF NOT EXISTS askxuan_marketing.catalog_checkout_snapshot LIKE askxuan_order.catalog_checkout_snapshot;
