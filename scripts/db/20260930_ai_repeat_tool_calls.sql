-- Run against askxuan_ai after backing up ai_tool_call. Preserves all rows.
-- Harness may call the same read-only tool more than once within one run.
-- id remains the unique invocation identity. The old run/name constraint
-- incorrectly blocked keyword refinement, retries and multi-step calculations.
SET @has_unique := (SELECT COUNT(*) FROM information_schema.statistics
 WHERE table_schema=DATABASE() AND table_name='ai_tool_call' AND index_name='uk_run_tool');
SET @has_index := (SELECT COUNT(*) FROM information_schema.statistics
 WHERE table_schema=DATABASE() AND table_name='ai_tool_call' AND index_name='idx_run_tool');
SET @ddl := IF(@has_unique>0,
 IF(@has_index=0,'ALTER TABLE ai_tool_call DROP INDEX uk_run_tool, ADD INDEX idx_run_tool (run_id,tool_name)',
 'ALTER TABLE ai_tool_call DROP INDEX uk_run_tool'),
 IF(@has_index=0,'ALTER TABLE ai_tool_call ADD INDEX idx_run_tool (run_id,tool_name)','SELECT 1'));
PREPARE tool_index_migration FROM @ddl;
EXECUTE tool_index_migration;
DEALLOCATE PREPARE tool_index_migration;
