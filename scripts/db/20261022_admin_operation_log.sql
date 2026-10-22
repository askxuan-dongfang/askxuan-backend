-- Additive only. Apply once to the existing audit database; no historical queue is changed.
-- All times are UTC. No body/query/IP/email/token columns are permitted here.
CREATE TABLE IF NOT EXISTS askxuan_audit.operation_log (
  request_id CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  actor_id BIGINT NOT NULL,
  actor_roles VARCHAR(2200) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  method VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  route_template VARCHAR(256) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  result ENUM('pending','success','denied','failed','unknown') NOT NULL DEFAULT 'pending',
  http_status SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  business_code INT NOT NULL DEFAULT 0,
  duration_ms BIGINT UNSIGNED NOT NULL DEFAULT 0,
  started_at DATETIME(3) NOT NULL,
  finished_at DATETIME(3) NULL,
  PRIMARY KEY (request_id),
  KEY idx_operation_started (started_at,request_id),
  KEY idx_operation_actor_started (actor_id,started_at),
  KEY idx_operation_result_started (result,started_at),
  KEY idx_operation_action_started (action,started_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
