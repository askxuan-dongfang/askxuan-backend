CREATE DATABASE IF NOT EXISTS askxuan_community CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS askxuan_community.post (
  id BIGINT NOT NULL AUTO_INCREMENT,
  post_no VARCHAR(32) NOT NULL,
  master_id VARCHAR(64) NOT NULL,
  owner_id VARCHAR(64) NOT NULL,
  type VARCHAR(16) NOT NULL COMMENT 'article/video',
  title VARCHAR(120) NOT NULL,
  content TEXT,
  cover_media_id BIGINT NOT NULL DEFAULT 0,
  belief_code VARCHAR(32) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'draft' COMMENT 'draft/pending/approved/rejected/off_shelf',
  audit_id BIGINT NOT NULL DEFAULT 0,
  audit_remark VARCHAR(255) NOT NULL DEFAULT '',
  like_count BIGINT NOT NULL DEFAULT 0,
  comment_count BIGINT NOT NULL DEFAULT 0,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id), UNIQUE KEY uk_post_no (post_no),
  KEY idx_feed (status,create_time), KEY idx_master_status (master_id,status), KEY idx_belief (belief_code,status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='大师广场帖子';

CREATE TABLE IF NOT EXISTS askxuan_community.post_asset (
  id BIGINT NOT NULL AUTO_INCREMENT,
  post_no VARCHAR(32) NOT NULL,
  media_id BIGINT NOT NULL,
  asset_type VARCHAR(16) NOT NULL COMMENT 'image/video',
  sort INT NOT NULL DEFAULT 0,
  PRIMARY KEY (id), UNIQUE KEY uk_post_media (post_no,media_id), KEY idx_post_sort (post_no,sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='帖子媒体引用';

CREATE TABLE IF NOT EXISTS askxuan_community.post_like (
  post_no VARCHAR(32) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (post_no,user_id), KEY idx_user (user_id,create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='帖子点赞';

CREATE TABLE IF NOT EXISTS askxuan_community.post_comment (
  id BIGINT NOT NULL AUTO_INCREMENT,
  comment_no VARCHAR(32) NOT NULL,
  post_no VARCHAR(32) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  content VARCHAR(500) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/approved/rejected',
  audit_id BIGINT NOT NULL DEFAULT 0,
  audit_remark VARCHAR(255) NOT NULL DEFAULT '',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id), UNIQUE KEY uk_comment_no (comment_no), KEY idx_post_status (post_no,status,create_time), KEY idx_audit (status,create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='帖子评论';

CREATE TABLE IF NOT EXISTS askxuan_community.master_follow (
  master_id VARCHAR(64) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (master_id,user_id), KEY idx_user (user_id,create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户关注大师';

CREATE USER IF NOT EXISTS 'community_user'@'%' IDENTIFIED BY 'Askxuan2026!';
GRANT ALL PRIVILEGES ON askxuan_community.* TO 'community_user'@'%';
GRANT SELECT ON askxuan_media.media_asset TO 'community_user'@'%';
GRANT SELECT,INSERT,UPDATE ON askxuan_audit.audit_queue TO 'community_user'@'%';
GRANT SELECT,INSERT ON askxuan_audit.audit_log TO 'community_user'@'%';
FLUSH PRIVILEGES;
