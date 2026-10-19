-- Additive parent metadata only. Provider classifications and tag assignments stay intact.
SET @directory_parent_ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='askxuan_temple' AND table_name='directory_tag' AND column_name='belief_code'),
  'SELECT 1',
  'ALTER TABLE askxuan_temple.directory_tag ADD COLUMN belief_code VARCHAR(32) NOT NULL DEFAULT '''' AFTER tag_group'
);
PREPARE directory_parent_stmt FROM @directory_parent_ddl;
EXECUTE directory_parent_stmt;
DEALLOCATE PREPARE directory_parent_stmt;
UPDATE askxuan_temple.directory_tag SET belief_code = CASE
  WHEN code IN ('chan','pure_land','tiantai','huayan') THEN 'han_buddhism'
  WHEN code IN ('gelug','nyingma','kagyu','sakya') THEN 'tibetan_buddhism'
  WHEN code='theravada' THEN 'theravada_buddhism'
  WHEN code IN ('quanzhen','zhengyi') THEN 'daoism'
  WHEN code IN ('mazu','guandi','chenghuang','wangye','fude','baosheng','nezha','hung_shing','wong_tai_sin','nine_emperors','tua_pek_kong','datuk_gong','sanshan') THEN 'folk'
  ELSE belief_code END
WHERE tag_group='tradition' AND belief_code='';
