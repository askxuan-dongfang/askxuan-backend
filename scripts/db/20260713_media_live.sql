CREATE DATABASE IF NOT EXISTS askxuan_media CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS askxuan_media.media_asset (
  id BIGINT NOT NULL AUTO_INCREMENT,
  media_no VARCHAR(32) NOT NULL,
  owner_id VARCHAR(64) NOT NULL,
  media_type VARCHAR(16) NOT NULL COMMENT 'image/video/audio',
  content_type VARCHAR(128) NOT NULL DEFAULT '',
  file_name VARCHAR(255) NOT NULL DEFAULT '',
  object_name VARCHAR(512) NOT NULL,
  provider VARCHAR(32) NOT NULL DEFAULT 'local_minio',
  provider_task_id VARCHAR(128) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'uploading' COMMENT 'uploading/uploaded/processing/ready/failed',
  audit_status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/approved/rejected',
  playback_url VARCHAR(1000) NOT NULL DEFAULT '',
  cover_url VARCHAR(1000) NOT NULL DEFAULT '',
  cover_media_id BIGINT NOT NULL DEFAULT 0,
  duration DECIMAL(10,3) NOT NULL DEFAULT 0,
  file_size BIGINT NOT NULL DEFAULT 0,
  error_message VARCHAR(500) NOT NULL DEFAULT '',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_media_no (media_no),
  UNIQUE KEY uk_object_name (object_name),
  KEY idx_owner_status (owner_id,status),
  KEY idx_audit_status (audit_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='统一媒体资产';

CREATE TABLE IF NOT EXISTS askxuan_media.live_room (
  id BIGINT NOT NULL AUTO_INCREMENT,
  room_no VARCHAR(32) NOT NULL,
  owner_id VARCHAR(64) NOT NULL,
  master_id VARCHAR(64) NOT NULL,
  title VARCHAR(120) NOT NULL,
  cover_media_id BIGINT NOT NULL DEFAULT 0,
  provider VARCHAR(32) NOT NULL DEFAULT 'disabled',
  status VARCHAR(16) NOT NULL DEFAULT 'created' COMMENT 'created/live/ended/failed',
  openim_group_id VARCHAR(128) NOT NULL DEFAULT '',
  push_url VARCHAR(1000) NOT NULL DEFAULT '',
  watch_url VARCHAR(1000) NOT NULL DEFAULT '',
  provider_room_id VARCHAR(128) NOT NULL DEFAULT '',
  started_at DATETIME NULL,
  ended_at DATETIME NULL,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_room_no (room_no),
  KEY idx_master_status (master_id,status),
  KEY idx_openim_group (openim_group_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播房间';

CREATE USER IF NOT EXISTS 'media_user'@'%' IDENTIFIED BY 'Askxuan2026!';
GRANT ALL PRIVILEGES ON askxuan_media.* TO 'media_user'@'%';
FLUSH PRIVILEGES;
