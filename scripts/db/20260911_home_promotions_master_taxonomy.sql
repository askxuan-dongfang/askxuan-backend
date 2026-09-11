-- Reviewed additive migration. Run only after backing up askxuan_marketing.banner,
-- askxuan_master.master and askxuan_master.master_service_tag.
SET NAMES utf8mb4;
USE askxuan_marketing;
SET @placement_exists=(SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='banner' AND column_name='placement');
SET @placement_ddl=IF(@placement_exists=0,'ALTER TABLE banner ADD COLUMN placement VARCHAR(32) NOT NULL DEFAULT ''customer_home'' AFTER title','SELECT 1');
PREPARE placement_stmt FROM @placement_ddl;
EXECUTE placement_stmt;
DEALLOCATE PREPARE placement_stmt;

-- Retire only known, non-functional legacy demo adverts; keep their records.
UPDATE banner SET status='disabled' WHERE
 (title='更新Banner' AND image_url='http://example.com/img.jpg') OR
 (title='新人首单立减' AND image_url='/banners/newuser.jpg' AND link_value='/promo/newuser') OR
 (title='灵隐寺祈福法会' AND image_url='/banners/lingyin.jpg');

USE askxuan_master;
-- Canonical taxonomy code. Old API clients remain compatible with taoism.
UPDATE master SET belief_code='daoism' WHERE belief_code='taoism';
-- Keep the original four profiles. New fictional examples cover the missing groups.
INSERT INTO master (code,dharma_name,lay_name,temple_code,position,belief_code,sect,type,auth_status,shelf_status,platform_status,manage_by,specialties,avatar,rating,consult_enabled,consult_fee,consult_valid_hours,consult_response_minutes)
SELECT 'W006','明觉居士（演示）','演示资料','','独立居士','tibetan_buddhism','格鲁派','藏传佛教','已认证','on_shelf','normal','platform','藏文化交流,静心禅修','',4.8,1,29,72,30
WHERE NOT EXISTS(SELECT 1 FROM master WHERE code='W006');
INSERT INTO master (code,dharma_name,lay_name,temple_code,position,belief_code,sect,type,auth_status,shelf_status,platform_status,manage_by,specialties,avatar,rating,consult_enabled,consult_fee,consult_valid_hours,consult_response_minutes)
SELECT 'W007','守礼先生（演示）','演示资料','','民俗文化顾问','folk','民俗文化','民间信仰','已认证','on_shelf','normal','platform','传统礼俗,民俗祈愿','',4.7,1,19,72,30
WHERE NOT EXISTS(SELECT 1 FROM master WHERE code='W007');
INSERT INTO master_service_tag(master_code,service_code,price,status)
SELECT 'W006','S001',68,'enabled' WHERE EXISTS(SELECT 1 FROM master WHERE code='W006' AND dharma_name='明觉居士（演示）') AND NOT EXISTS(SELECT 1 FROM master_service_tag WHERE master_code='W006' AND service_code='S001');
INSERT INTO master_service_tag(master_code,service_code,price,status)
SELECT 'W006','S002',48,'enabled' WHERE EXISTS(SELECT 1 FROM master WHERE code='W006' AND dharma_name='明觉居士（演示）') AND NOT EXISTS(SELECT 1 FROM master_service_tag WHERE master_code='W006' AND service_code='S002');
INSERT INTO master_service_tag(master_code,service_code,price,status)
SELECT 'W007','S001',58,'enabled' WHERE EXISTS(SELECT 1 FROM master WHERE code='W007' AND dharma_name='守礼先生（演示）') AND NOT EXISTS(SELECT 1 FROM master_service_tag WHERE master_code='W007' AND service_code='S001');
INSERT INTO master_service_tag(master_code,service_code,price,status)
SELECT 'W007','S010',88,'enabled' WHERE EXISTS(SELECT 1 FROM master WHERE code='W007' AND dharma_name='守礼先生（演示）') AND NOT EXISTS(SELECT 1 FROM master_service_tag WHERE master_code='W007' AND service_code='S010');
