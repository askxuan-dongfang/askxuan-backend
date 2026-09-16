-- Additive only: existing completed orders remain historical records without fabricated receipts.
USE askxuan_booking;
CREATE TABLE IF NOT EXISTS booking_receipt (
 sequence BIGINT NOT NULL AUTO_INCREMENT UNIQUE,
 id VARCHAR(36) PRIMARY KEY, booking_no VARCHAR(64) NOT NULL,
 summary TEXT NOT NULL, operator_id VARCHAR(64) NOT NULL, operator_type VARCHAR(32) NOT NULL,
 digest CHAR(64) NOT NULL, create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 INDEX booking_receipts(booking_no,create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS booking_receipt_file (
 id VARCHAR(36) PRIMARY KEY, booking_no VARCHAR(64) NOT NULL, owner_id VARCHAR(64) NOT NULL,
 receipt_id VARCHAR(36) NOT NULL DEFAULT '', name VARCHAR(255) NOT NULL,
 content_type VARCHAR(64) NOT NULL, file_size BIGINT NOT NULL, sha256 CHAR(64) NOT NULL,
 create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 INDEX receipt_files(booking_no,receipt_id), INDEX owner_files(owner_id,create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
