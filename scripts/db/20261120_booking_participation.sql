USE askxuan_temple;

SET @slot_ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='temple_service_slot' AND column_name='participation_mode'),
  'SELECT 1',
  'ALTER TABLE temple_service_slot ADD COLUMN participation_mode VARCHAR(16) NULL'
);
PREPARE slot_stmt FROM @slot_ddl;
EXECUTE slot_stmt;
DEALLOCATE PREPARE slot_stmt;
