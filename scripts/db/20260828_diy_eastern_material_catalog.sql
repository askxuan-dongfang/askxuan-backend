-- DIY 东方材料目录与跨端渲染参数（可重复执行）
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS add_diy_material_render_columns;
DELIMITER //
CREATE PROCEDURE add_diy_material_render_columns(IN schema_name VARCHAR(64))
BEGIN
  SET @table_exists := (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA=schema_name AND TABLE_NAME='material'
  );
  IF @table_exists > 0 THEN
    SET @column_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=schema_name AND TABLE_NAME='material' AND COLUMN_NAME='material_type');
    SET @sql := IF(@column_exists=0, CONCAT('ALTER TABLE `', schema_name, '`.`material` ADD COLUMN material_type VARCHAR(32) NOT NULL DEFAULT ''gemstone'' AFTER five_elements'), 'SELECT 1');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    SET @column_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=schema_name AND TABLE_NAME='material' AND COLUMN_NAME='shape');
    SET @sql := IF(@column_exists=0, CONCAT('ALTER TABLE `', schema_name, '`.`material` ADD COLUMN shape VARCHAR(32) NOT NULL DEFAULT ''round'' AFTER material_type'), 'SELECT 1');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    SET @column_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=schema_name AND TABLE_NAME='material' AND COLUMN_NAME='diameter_mm');
    SET @sql := IF(@column_exists=0, CONCAT('ALTER TABLE `', schema_name, '`.`material` ADD COLUMN diameter_mm DECIMAL(5,2) NOT NULL DEFAULT 10.00 AFTER shape'), 'SELECT 1');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    SET @column_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=schema_name AND TABLE_NAME='material' AND COLUMN_NAME='color_hex');
    SET @sql := IF(@column_exists=0, CONCAT('ALTER TABLE `', schema_name, '`.`material` ADD COLUMN color_hex VARCHAR(16) NOT NULL DEFAULT ''#8A6E4A'' AFTER diameter_mm'), 'SELECT 1');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    SET @column_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=schema_name AND TABLE_NAME='material' AND COLUMN_NAME='texture_key');
    SET @sql := IF(@column_exists=0, CONCAT('ALTER TABLE `', schema_name, '`.`material` ADD COLUMN texture_key VARCHAR(32) NOT NULL DEFAULT ''plain'' AFTER color_hex'), 'SELECT 1');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    SET @column_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=schema_name AND TABLE_NAME='material' AND COLUMN_NAME='finish');
    SET @sql := IF(@column_exists=0, CONCAT('ALTER TABLE `', schema_name, '`.`material` ADD COLUMN finish VARCHAR(32) NOT NULL DEFAULT ''polished'' AFTER texture_key'), 'SELECT 1');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    SET @column_exists := (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=schema_name AND TABLE_NAME='material' AND COLUMN_NAME='translucency');
    SET @sql := IF(@column_exists=0, CONCAT('ALTER TABLE `', schema_name, '`.`material` ADD COLUMN translucency DECIMAL(4,3) NOT NULL DEFAULT 0.000 AFTER finish'), 'SELECT 1');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END//
DELIMITER ;

CALL add_diy_material_render_columns('askxuan');
CALL add_diy_material_render_columns('askxuan_diy');
DROP PROCEDURE add_diy_material_render_columns;

DROP TEMPORARY TABLE IF EXISTS `diy_material_catalog`;
CREATE TEMPORARY TABLE `diy_material_catalog` LIKE `askxuan`.`material`;

INSERT INTO `diy_material_catalog` (`name`,`spec`,`unit_price`,`unit`,`category`,`five_elements`,`material_type`,`shape`,`diameter_mm`,`color_hex`,`texture_key`,`finish`,`translucency`,`image`,`stock`,`status`) VALUES
('小叶紫檀圆珠','10mm',28.00,'颗','main_bead','wood','wood','round',10,'#6B2727','wood_grain','polished',0.00,'/assets/materials/rosewood.jpg',500,'on_shelf'),
('星月菩提','10mm',18.00,'颗','main_bead','wood','seed','round',10,'#D8C99F','bodhi','natural',0.00,'/assets/materials/bodhi.jpg',500,'on_shelf'),
('凤眼菩提','10mm',22.00,'颗','main_bead','wood','seed','round',10,'#9C6B42','bodhi','natural',0.00,'/assets/materials/rudraksha.jpg',500,'on_shelf'),
('白玉','8mm',35.00,'颗','main_bead','earth','jade','round',8,'#E7E4D7','jade_cloud','polished',0.28,'/assets/materials/jade.jpg',300,'on_shelf'),
('青金石','10mm',25.00,'颗','main_bead','water','gemstone','round',10,'#234B91','lapis','polished',0.05,'/assets/materials/lapis.jpg',300,'on_shelf'),
('南红玛瑙','8mm',32.00,'颗','main_bead','fire','gemstone','round',8,'#B93631','agate','polished',0.22,'/assets/materials/agate.jpg',300,'on_shelf'),
('蜜蜡','10mm',45.00,'颗','main_bead','earth','organic','round',10,'#D99518','amber','polished',0.42,'/assets/materials/amber.jpg',260,'on_shelf'),
('黑曜石','10mm',12.00,'颗','main_bead','water','gemstone','round',10,'#202726','obsidian','polished',0.02,'/assets/materials/obsidian.jpg',500,'on_shelf'),
('藏银三通','10mm',48.00,'个','three_way','metal','metal','three_way',10,'#AEB8BA','metal','brushed',0.00,'/assets/materials/silver-three-way.jpg',120,'on_shelf'),
('蜜蜡佛头','12mm',68.00,'个','buddha_head','earth','organic','buddha_head',12,'#D99518','amber','carved',0.36,'/assets/materials/amber-head.jpg',120,'on_shelf'),
('花丝莲花吊坠','15mm',20.00,'个','pendant','metal','metal','pendant',15,'#C7A45A','metal','carved',0.00,'/assets/materials/lotus-pendant.jpg',200,'on_shelf'),
('白水晶隔片','6mm',2.50,'颗','spacer','water','crystal','disc',6,'#E8F1F4','crystal','faceted',0.82,'/assets/materials/crystal-spacer.jpg',1000,'on_shelf'),
('流苏配饰','',28.00,'个','tassel','fire','textile','tassel',14,'#A92F35','silk','woven',0.00,'/assets/materials/tassel.jpg',180,'on_shelf'),
('弹力绳','',2.00,'根','cord','wood','cord','cord',0,'#8B6B4A','cord','woven',0.00,'/assets/materials/cord.jpg',1000,'on_shelf'),
('东陵玉','8mm',6.00,'颗','main_bead','wood','jade','round',8,'#5D936C','jade_cloud','polished',0.24,'',600,'on_shelf'),
('和田青白玉','8mm',42.00,'颗','main_bead','metal','jade','round',8,'#DCE2D5','jade_cloud','polished',0.30,'',260,'on_shelf'),
('岫玉','10mm',8.00,'颗','main_bead','wood','jade','round',10,'#91AD75','jade_cloud','polished',0.32,'',500,'on_shelf'),
('紫水晶','8mm',12.00,'颗','main_bead','fire','crystal','round',8,'#79579E','crystal','faceted',0.58,'',500,'on_shelf'),
('茶晶','10mm',16.00,'颗','main_bead','earth','crystal','round',10,'#725546','crystal','polished',0.48,'',400,'on_shelf'),
('粉晶','8mm',10.00,'颗','main_bead','fire','crystal','round',8,'#D98FA1','crystal','polished',0.54,'',600,'on_shelf'),
('海蓝宝','8mm',38.00,'颗','main_bead','water','crystal','faceted',8,'#77ACC4','crystal','faceted',0.66,'',220,'on_shelf'),
('黄水晶切面珠','8mm',22.00,'颗','main_bead','earth','crystal','faceted',8,'#D6A42C','crystal','faceted',0.62,'',320,'on_shelf'),
('虎眼石','10mm',9.00,'颗','main_bead','earth','gemstone','round',10,'#A36C22','tiger_eye','polished',0.08,'',500,'on_shelf'),
('绿松石工艺珠','10mm',35.00,'颗','main_bead','wood','gemstone','round',10,'#4B9B93','turquoise','polished',0.03,'',240,'on_shelf'),
('仿古工艺天珠','12x30mm',88.00,'颗','main_bead','earth','ceramic','barrel',12,'#5A3727','dzi','matte',0.00,'',120,'on_shelf'),
('朱砂工艺珠','8mm',10.00,'颗','main_bead','fire','ceramic','round',8,'#A92F35','cinnabar','matte',0.00,'',500,'on_shelf'),
('雷击枣木','10mm',18.00,'颗','main_bead','wood','wood','round',10,'#6E4429','wood_grain','natural',0.00,'',260,'on_shelf'),
('桃木','10mm',4.00,'颗','main_bead','wood','wood','round',10,'#B06E52','wood_grain','natural',0.00,'',800,'on_shelf'),
('崖柏','10mm',8.00,'颗','main_bead','wood','wood','round',10,'#9A6038','wood_grain','polished',0.00,'',500,'on_shelf'),
('沉香木','8mm',30.00,'颗','main_bead','wood','wood','round',8,'#4B3428','wood_grain','natural',0.00,'',180,'on_shelf'),
('金刚菩提','12mm',12.00,'颗','main_bead','wood','seed','round',12,'#74462D','seed','natural',0.00,'',400,'on_shelf'),
('椰蒂','8mm',5.00,'颗','main_bead','water','seed','disc',8,'#302722','seed','polished',0.00,'',700,'on_shelf'),
('青花瓷珠','10mm',15.00,'颗','main_bead','water','ceramic','round',10,'#E8E5DA','porcelain','glazed',0.04,'',360,'on_shelf'),
('景泰蓝掐丝珠','10mm',26.00,'颗','main_bead','metal','metal','round',10,'#276C79','cloisonne','polished',0.00,'',240,'on_shelf'),
('莲花琉璃珠','10mm',16.00,'颗','main_bead','fire','glass','round',10,'#A85672','glass','carved',0.55,'',360,'on_shelf'),
('铜鎏金隔片','6mm',3.00,'颗','spacer','metal','metal','disc',6,'#B98532','metal','brushed',0.00,'',800,'on_shelf'),
('祥云银色隔片','6mm',6.00,'颗','spacer','metal','metal','disc',6,'#B9C2C4','metal','carved',0.00,'',600,'on_shelf'),
('绿松石三通','14mm',78.00,'个','three_way','wood','gemstone','three_way',14,'#4B9B93','turquoise','carved',0.02,'',100,'on_shelf'),
('檀木佛头','12mm',28.00,'个','buddha_head','wood','wood','buddha_head',12,'#6B372B','wood_grain','carved',0.00,'',200,'on_shelf'),
('朱砂葫芦吊坠','18mm',38.00,'个','pendant','fire','ceramic','pendant',18,'#A92F35','cinnabar','carved',0.00,'',160,'on_shelf'),
('和田玉平安扣','18mm',120.00,'个','pendant','earth','jade','pendant',18,'#E0E3D4','jade_cloud','polished',0.28,'',80,'on_shelf'),
('木鱼小吊坠','15mm',18.00,'个','pendant','wood','wood','pendant',15,'#895436','wood_grain','carved',0.00,'',220,'on_shelf'),
('中国结流苏','',28.00,'个','tassel','fire','textile','tassel',14,'#B52F39','silk','woven',0.00,'',260,'on_shelf'),
('五色编绳','',5.00,'根','cord','earth','cord','cord',0,'#A35B43','cord','woven',0.00,'',900,'on_shelf'),
('玉线','',2.00,'根','cord','wood','cord','cord',0,'#DDD7C9','cord','woven',0.00,'',1200,'on_shelf');

UPDATE `askxuan`.`material` material
JOIN `diy_material_catalog` catalog ON catalog.name=material.name
SET material.spec=catalog.spec,
    material.unit_price=catalog.unit_price,
    material.unit=catalog.unit,
    material.category=catalog.category,
    material.five_elements=catalog.five_elements,
    material.material_type=catalog.material_type,
    material.shape=catalog.shape,
    material.diameter_mm=catalog.diameter_mm,
    material.color_hex=catalog.color_hex,
    material.texture_key=catalog.texture_key,
    material.finish=catalog.finish,
    material.translucency=catalog.translucency,
    material.image=IF(catalog.image='',material.image,catalog.image),
    material.stock=catalog.stock,
    material.status=catalog.status;

INSERT INTO `askxuan`.`material` (`name`,`spec`,`unit_price`,`unit`,`category`,`five_elements`,`material_type`,`shape`,`diameter_mm`,`color_hex`,`texture_key`,`finish`,`translucency`,`image`,`stock`,`status`)
SELECT catalog.name,catalog.spec,catalog.unit_price,catalog.unit,catalog.category,catalog.five_elements,catalog.material_type,catalog.shape,catalog.diameter_mm,catalog.color_hex,catalog.texture_key,catalog.finish,catalog.translucency,catalog.image,catalog.stock,catalog.status
FROM `diy_material_catalog` catalog
WHERE NOT EXISTS (
  SELECT 1 FROM `askxuan`.`material` material WHERE material.name=catalog.name
);

INSERT INTO `askxuan_diy`.`material` (`id`,`name`,`spec`,`unit_price`,`unit`,`category`,`five_elements`,`material_type`,`shape`,`diameter_mm`,`color_hex`,`texture_key`,`finish`,`translucency`,`image`,`stock`,`status`)
SELECT `id`,`name`,`spec`,`unit_price`,`unit`,`category`,`five_elements`,`material_type`,`shape`,`diameter_mm`,`color_hex`,`texture_key`,`finish`,`translucency`,`image`,`stock`,`status`
FROM `askxuan`.`material`
ON DUPLICATE KEY UPDATE
`name`=VALUES(`name`),`spec`=VALUES(`spec`),`unit_price`=VALUES(`unit_price`),`unit`=VALUES(`unit`),`category`=VALUES(`category`),`five_elements`=VALUES(`five_elements`),`material_type`=VALUES(`material_type`),`shape`=VALUES(`shape`),`diameter_mm`=VALUES(`diameter_mm`),`color_hex`=VALUES(`color_hex`),`texture_key`=VALUES(`texture_key`),`finish`=VALUES(`finish`),`translucency`=VALUES(`translucency`),`image`=IF(VALUES(`image`)='',`image`,VALUES(`image`)),`stock`=VALUES(`stock`),`status`=VALUES(`status`);

UPDATE `askxuan_diy`.`material_sku` sku
JOIN `askxuan_diy`.`material` material ON material.id=sku.material_id
SET sku.spec=material.spec,sku.price=material.unit_price,sku.stock=material.stock;

INSERT INTO `askxuan_diy`.`material_sku` (`material_id`,`spec`,`price`,`stock`)
SELECT material.id,material.spec,material.unit_price,material.stock
FROM `askxuan_diy`.`material` material
WHERE NOT EXISTS (
  SELECT 1 FROM `askxuan_diy`.`material_sku` sku WHERE sku.material_id=material.id
);

DROP TEMPORARY TABLE `diy_material_catalog`;
