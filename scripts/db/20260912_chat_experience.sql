SET NAMES utf8mb4;
SET @ddl = IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_booking' AND table_name='booking_chat_message' AND column_name='kind')=0, 'ALTER TABLE askxuan_booking.booking_chat_message ADD COLUMN kind VARCHAR(16) NOT NULL DEFAULT ''text''', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @ddl = IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_booking' AND table_name='booking_chat_message' AND column_name='attachment_json')=0, 'ALTER TABLE askxuan_booking.booking_chat_message ADD COLUMN attachment_json TEXT NULL', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @ddl = IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_booking' AND table_name='booking_chat_message' AND column_name='notify_status')=0, 'ALTER TABLE askxuan_booking.booking_chat_message ADD COLUMN notify_status VARCHAR(16) NOT NULL DEFAULT ''done''', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @ddl = IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_booking' AND table_name='booking_chat_message' AND column_name='notify_attempts')=0, 'ALTER TABLE askxuan_booking.booking_chat_message ADD COLUMN notify_attempts INT NOT NULL DEFAULT 0', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @ddl = IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_booking' AND table_name='booking_chat_message' AND column_name='notify_after')=0, 'ALTER TABLE askxuan_booking.booking_chat_message ADD COLUMN notify_after DATETIME NULL', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE askxuan_booking.booking_chat_message SET attachment_json='{}' WHERE attachment_json IS NULL;
CREATE TABLE IF NOT EXISTS askxuan_booking.chat_read_cursor (
 conversation_id VARCHAR(64) NOT NULL, reader_id VARCHAR(64) NOT NULL,
 through_id BIGINT NOT NULL DEFAULT 0, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(conversation_id,reader_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_booking.chat_attachment (
 id VARCHAR(36) NOT NULL PRIMARY KEY, conversation_id VARCHAR(64) NOT NULL, owner_id VARCHAR(64) NOT NULL,
 name VARCHAR(255) NOT NULL, content_type VARCHAR(128) NOT NULL, file_size BIGINT NOT NULL,
 duration DOUBLE NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 KEY idx_chat_attachment_owner(owner_id,created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
SET @ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_booking' AND table_name='booking_chat_message' AND column_name='push_status')=0,'ALTER TABLE askxuan_booking.booking_chat_message ADD COLUMN push_status VARCHAR(16) NOT NULL DEFAULT ''done''','SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_booking' AND table_name='booking_chat_message' AND column_name='push_after')=0,'ALTER TABLE askxuan_booking.booking_chat_message ADD COLUMN push_after DATETIME NULL','SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_message' AND table_name='device_token' AND column_name='chat_identity')=0,'ALTER TABLE askxuan_message.device_token ADD COLUMN chat_identity VARCHAR(64) NOT NULL DEFAULT ''''','SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_message' AND table_name='device_token' AND column_name='apns_environment')=0,'ALTER TABLE askxuan_message.device_token ADD COLUMN apns_environment VARCHAR(16) NOT NULL DEFAULT ''production''','SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
CREATE TABLE IF NOT EXISTS askxuan_booking.chat_push_delivery (
 message_id BIGINT NOT NULL,device_id BIGINT NOT NULL,status VARCHAR(16) NOT NULL DEFAULT 'pending',
 PRIMARY KEY(message_id,device_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_booking.chat_call (
 id VARCHAR(36) NOT NULL PRIMARY KEY,conversation_id VARCHAR(64) NOT NULL,
 caller_id VARCHAR(64) NOT NULL,callee_id VARCHAR(64) NOT NULL,kind VARCHAR(16) NOT NULL,
 state VARCHAR(16) NOT NULL DEFAULT 'ringing',active_key VARCHAR(64) NULL,
 offer MEDIUMTEXT NOT NULL,answer MEDIUMTEXT NULL,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,caller_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 callee_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,ended_at DATETIME NULL,
 UNIQUE KEY uk_active_call(active_key), KEY idx_chat_call_conversation(conversation_id,created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_booking.chat_call_signal (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,call_id VARCHAR(36) NOT NULL,sender_id VARCHAR(64) NOT NULL,
 client_id VARCHAR(64) NOT NULL,candidate TEXT NOT NULL,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE KEY uk_call_signal(call_id,sender_id,client_id), KEY idx_call_signal_poll(call_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS askxuan_booking.chat_call_push (
 call_id VARCHAR(36) NOT NULL,device_id BIGINT NOT NULL,PRIMARY KEY(call_id,device_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
SET @ddl=IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema='askxuan_booking' AND table_name='booking_chat_message' AND index_name='idx_chat_notify')=0,'ALTER TABLE askxuan_booking.booking_chat_message ADD KEY idx_chat_notify(notify_status,notify_after)','SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @ddl=IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema='askxuan_booking' AND table_name='booking_chat_message' AND index_name='idx_chat_push')=0,'ALTER TABLE askxuan_booking.booking_chat_message ADD KEY idx_chat_push(push_status,push_after)','SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @ddl=IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema='askxuan_booking' AND table_name='booking_chat_message' AND index_name='idx_chat_unread')=0,'ALTER TABLE askxuan_booking.booking_chat_message ADD KEY idx_chat_unread(receiver_id,booking_id,id)','SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @ddl=IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema='askxuan_booking' AND table_name='chat_call' AND index_name='idx_call_incoming')=0,'ALTER TABLE askxuan_booking.chat_call ADD KEY idx_call_incoming(callee_id,state,created_at)','SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @ddl=IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema='askxuan_message' AND table_name='device_token' AND index_name='idx_device_chat')=0,'ALTER TABLE askxuan_message.device_token ADD KEY idx_device_chat(chat_identity,status)','SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
