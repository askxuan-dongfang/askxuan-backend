-- Additive service progress records; no existing booking or receipt is rewritten.
USE askxuan_booking;
CREATE TABLE IF NOT EXISTS booking_progress (
 sequence BIGINT NOT NULL AUTO_INCREMENT UNIQUE,
 id VARCHAR(36) PRIMARY KEY,
 booking_no VARCHAR(64) NOT NULL,
 kind VARCHAR(16) NOT NULL,
 content TEXT NOT NULL,
 operator_id VARCHAR(64) NOT NULL,
 operator_type VARCHAR(32) NOT NULL,
 create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 INDEX booking_progress_order(booking_no,sequence)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS booking_progress_file (
 progress_id VARCHAR(36) NOT NULL,
 file_id VARCHAR(36) NOT NULL UNIQUE,
 PRIMARY KEY(progress_id,file_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
