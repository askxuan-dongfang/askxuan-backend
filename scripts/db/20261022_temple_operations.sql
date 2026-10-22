-- Requires the platform authorization version migration. Additive, no account rebinding.
INSERT INTO askxuan_auth.role (name,code,description,authz_scope)
SELECT '寺院接待','temple_reception','本寺预约接待、履约与异常跟进','temple'
WHERE NOT EXISTS (SELECT 1 FROM askxuan_auth.role WHERE code='temple_reception');
INSERT INTO askxuan_auth.role (name,code,description,authz_scope)
SELECT '寺院运营','temple_operations','本寺资料、人员、服务、排班与履约运营','temple'
WHERE NOT EXISTS (SELECT 1 FROM askxuan_auth.role WHERE code='temple_operations');
INSERT INTO askxuan_auth.role (name,code,description,authz_scope)
SELECT '寺院财务只读','temple_finance','本寺预约合同统计与结算查看、导出；不可打款','temple'
WHERE NOT EXISTS (SELECT 1 FROM askxuan_auth.role WHERE code='temple_finance');
SET @has_revision=(SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_master' AND table_name='master_schedule' AND column_name='revision');
SET @ddl=IF(@has_revision=0,'ALTER TABLE askxuan_master.master_schedule ADD COLUMN revision BIGINT NOT NULL DEFAULT 0','SELECT 1');
PREPARE temple_ops_stmt FROM @ddl; EXECUTE temple_ops_stmt; DEALLOCATE PREPARE temple_ops_stmt;
CREATE TABLE IF NOT EXISTS askxuan_booking.temple_exception_note (
 id BIGINT NOT NULL AUTO_INCREMENT,
 temple_code VARCHAR(16) NOT NULL,
 source_kind VARCHAR(16) NOT NULL,
 source_id VARCHAR(64) NOT NULL,
 operator_id BIGINT NOT NULL,
 operator_role VARCHAR(64) NOT NULL,
 request_id VARCHAR(64) NOT NULL,
 action VARCHAR(16) NOT NULL,
 content VARCHAR(4000) NOT NULL,
 create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(id),
 UNIQUE KEY exception_request(temple_code,operator_id,request_id),
 KEY exception_source(temple_code,source_kind,source_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- A separate mutex row serializes edits/reservations without granting profile writes to booking.
CREATE TABLE IF NOT EXISTS askxuan_master.master_schedule_guard (
 master_code VARCHAR(16) NOT NULL,
 PRIMARY KEY(master_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
GRANT SELECT,INSERT,UPDATE ON askxuan_master.master_schedule_guard TO 'booking_user'@'%';
-- Booking validates the shared calendar in its reservation transaction; it never edits master profiles or schedules.
GRANT SELECT (id,code) ON askxuan_master.master TO 'booking_user'@'%';
GRANT SELECT (master_code,schedule_date,status,time_slots) ON askxuan_master.master_schedule TO 'booking_user'@'%';
GRANT SELECT (id,task_no,temple_code,status) ON askxuan_diy.blessing_task TO 'booking_user'@'%';
