SET NAMES utf8mb4;
-- Additive: rollback code without dropping private notes.
CREATE TABLE IF NOT EXISTS askxuan_ai.ai_experience_note (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, user_id VARCHAR(64) NOT NULL,
 request_key VARCHAR(64) NOT NULL, skill VARCHAR(32) NOT NULL, version VARCHAR(32) NOT NULL,
 payload JSON NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE KEY user_request(user_id,request_key), KEY user_history(user_id,id)
);
