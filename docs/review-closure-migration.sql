-- Run with the existing review-service database account before deploying.
CREATE TABLE IF NOT EXISTS review_booking_context (
 booking_id VARCHAR(64) NOT NULL PRIMARY KEY,
 temple_code VARCHAR(64) NOT NULL DEFAULT '',
 temple_name VARCHAR(255) NOT NULL DEFAULT '',
 master_name VARCHAR(255) NOT NULL DEFAULT '',
 service_name VARCHAR(255) NOT NULL DEFAULT '',
 master_reply TEXT NOT NULL,
 INDEX idx_review_temple (temple_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
