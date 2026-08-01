SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS askxuan_booking.booking_chat_message (
  id BIGINT NOT NULL AUTO_INCREMENT,
  booking_id VARCHAR(32) NOT NULL COMMENT '已支付预约单号',
  client_message_id VARCHAR(128) NOT NULL COMMENT '客户端幂等消息ID',
  openim_server_msg_id VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'OpenIM服务端消息ID',
  sender_type VARCHAR(16) NOT NULL COMMENT 'customer/master',
  sender_id VARCHAR(64) NOT NULL COMMENT 'OpenIM发送方ID',
  receiver_id VARCHAR(64) NOT NULL COMMENT 'OpenIM接收方ID',
  content VARCHAR(2000) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/sent/failed',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_booking_chat_client_msg (booking_id,client_message_id),
  KEY idx_booking_chat_history (booking_id,status,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='付费预约聊天消息';

SET @booking_chat_index_columns = (
  SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',')
  FROM information_schema.statistics
  WHERE table_schema='askxuan_booking' AND table_name='booking_chat_message'
    AND index_name='uk_booking_chat_client_msg'
);
SET @booking_chat_index_ddl = CASE
  WHEN @booking_chat_index_columns='client_message_id' THEN
    'ALTER TABLE askxuan_booking.booking_chat_message DROP INDEX uk_booking_chat_client_msg, ADD UNIQUE KEY uk_booking_chat_client_msg (booking_id,client_message_id)'
  WHEN @booking_chat_index_columns IS NULL THEN
    'ALTER TABLE askxuan_booking.booking_chat_message ADD UNIQUE KEY uk_booking_chat_client_msg (booking_id,client_message_id)'
  ELSE 'SELECT 1'
END;
PREPARE booking_chat_index_stmt FROM @booking_chat_index_ddl;
EXECUTE booking_chat_index_stmt;
DEALLOCATE PREPARE booking_chat_index_stmt;
