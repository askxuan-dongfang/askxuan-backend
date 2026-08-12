-- 首页信仰与心愿运营分类动态化（可重复执行）
SET NAMES utf8mb4;

SET @belief_icon_exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA='askxuan_temple' AND TABLE_NAME='belief_profile' AND COLUMN_NAME='icon'
);
SET @sql := IF(@belief_icon_exists=0,
  "ALTER TABLE askxuan_temple.belief_profile ADD COLUMN icon VARCHAR(64) NOT NULL DEFAULT 'sparkles' AFTER cover_image",
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE askxuan_temple.belief_profile SET icon=CASE code
  WHEN 'han_buddhism' THEN 'leaf.fill'
  WHEN 'tibetan_buddhism' THEN 'flame.fill'
  WHEN 'daoism' THEN 'sparkles'
  WHEN 'folk' THEN 'seal.fill'
  ELSE IF(icon='', 'sparkles', icon)
END;

SET @intent_landing_type_exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA='askxuan_product' AND TABLE_NAME='intent_tag' AND COLUMN_NAME='landing_type'
);
SET @sql := IF(@intent_landing_type_exists=0,
  "ALTER TABLE askxuan_product.intent_tag ADD COLUMN landing_type VARCHAR(16) NOT NULL DEFAULT 'aggregate' AFTER icon, ADD COLUMN landing_value VARCHAR(64) NOT NULL DEFAULT '' AFTER landing_type, ADD COLUMN action_title VARCHAR(64) NOT NULL DEFAULT '' AFTER landing_value",
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE askxuan_product.intent_tag SET
  landing_type=CASE code WHEN 'diy' THEN 'diy' WHEN 'peace' THEN 'service' WHEN 'wealth' THEN 'service' WHEN 'love' THEN 'service' WHEN 'career' THEN 'service' WHEN 'study' THEN 'service' WHEN 'taisui' THEN 'service' WHEN 'rite' THEN 'service' ELSE landing_type END,
  landing_value=CASE code WHEN 'peace' THEN 'S001' WHEN 'wealth' THEN 'S009' WHEN 'love' THEN 'S008' WHEN 'career' THEN 'S010' WHEN 'study' THEN 'S013' WHEN 'taisui' THEN 'S007' WHEN 'rite' THEN 'S005' ELSE landing_value END,
  action_title=CASE code WHEN 'peace' THEN '办理平安祈福' WHEN 'wealth' THEN '办理财运祈福' WHEN 'love' THEN '办理姻缘祈愿' WHEN 'career' THEN '办理事业祈愿' WHEN 'study' THEN '办理学业祈愿' WHEN 'taisui' THEN '办理化太岁' WHEN 'diy' THEN '开始定制' WHEN 'rite' THEN '预约法事' ELSE action_title END;
