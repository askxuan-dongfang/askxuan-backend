-- Richer, repeatable demo catalog for belief/temple/master/service browsing.
-- Religious venues are real; people named "演示法师/道长/讲师" are fictional demo profiles.
SET NAMES utf8mb4;

ALTER TABLE askxuan_temple.temple MODIFY COLUMN cover_image VARCHAR(500) NOT NULL DEFAULT '' COMMENT '封面图';
ALTER TABLE askxuan_temple.temple_image MODIFY COLUMN url VARCHAR(500) NOT NULL COMMENT '图片URL';
START TRANSACTION;

INSERT INTO askxuan_temple.temple
  (code,name,region,type,belief_code,sect,status,address,cover_image,rating,description)
VALUES
  ('T007','九华山化城寺','安徽池州','汉传佛教','han_buddhism','地藏法门','正常','安徽省池州市青阳县九华镇化城路41号','https://upload.wikimedia.org/wikipedia/commons/thumb/0/02/Huacheng_Temple_05.jpg/1280px-Huacheng_Temple_05.jpg',4.80,'九华山开山主寺之一，围绕地藏文化、礼佛祈愿与传统佛教文化开展服务。'),
  ('T008','北京雍和宫','北京东城','藏传佛教','tibetan_buddhism','格鲁派','正常','北京市东城区雍和宫大街12号','https://upload.wikimedia.org/wikipedia/commons/thumb/2/2b/Yonghe_Temple%2C_Beijing.JPG/1280px-Yonghe_Temple%2C_Beijing.JPG',4.80,'北京重要藏传佛教寺院，具有完整的历史建筑群和格鲁派文化传承。'),
  ('T009','青城山天师洞','四川成都','道教','daoism','正一派','正常','四川省成都市都江堰市青城山景区','https://upload.wikimedia.org/wikipedia/commons/thumb/d/de/%E9%9D%92%E5%9F%8E%E5%B1%B1%E5%A4%A9%E5%B8%88%E6%B4%9E-%E3%80%8C%E5%8F%A4%E5%B8%B8%E9%81%93%E8%A7%82%E3%80%8D%E9%97%A8%E6%A5%BC.jpg/1280px-%E9%9D%92%E5%9F%8E%E5%B1%B1%E5%A4%A9%E5%B8%88%E6%B4%9E-%E3%80%8C%E5%8F%A4%E5%B8%B8%E9%81%93%E8%A7%82%E3%80%8D%E9%97%A8%E6%A5%BC.jpg',4.70,'青城山古建筑群的重要组成部分，展示道教历史、科仪与清静修持传统。'),
  ('T010','湄洲妈祖祖庙','福建莆田','民间信仰','folk','妈祖信仰','正常','福建省莆田市秀屿区湄洲北大道988号','https://upload.wikimedia.org/wikipedia/commons/c/c0/%E7%A5%88%E5%B9%B4%E6%9C%9F%E9%97%B4%E7%9A%84%E6%B9%84%E6%B4%B2%E5%A6%88%E7%A5%96%E7%A5%96%E5%BA%993.jpg',4.90,'妈祖信俗的重要传承场所，承载海洋文化、民俗祈愿与非遗交流。')
ON DUPLICATE KEY UPDATE
  name=VALUES(name),region=VALUES(region),type=VALUES(type),belief_code=VALUES(belief_code),sect=VALUES(sect),
  address=VALUES(address),cover_image=VALUES(cover_image),description=VALUES(description);

INSERT INTO askxuan_master.master
  (code,dharma_name,lay_name,temple_code,position,belief_code,sect,type,auth_status,shelf_status,platform_status,specialties,avatar,rating)
VALUES
  ('M007','地藏法门演示法师','','T007','客堂法师','han_buddhism','地藏法门','佛教','已认证','on_shelf','normal','地藏文化,祈福,供灯','','4.70'),
  ('M008','格鲁派演示法师','','T008','文化讲师','tibetan_buddhism','格鲁派','佛教','已认证','on_shelf','normal','藏传文化,祈福,供灯','','4.70'),
  ('M009','青城演示道长','','T009','文化讲师','daoism','正一派','道教','已认证','on_shelf','normal','道教文化,祈福,养生','','4.60'),
  ('M010','妈祖文化演示讲师','','T010','文化讲师','folk','妈祖信仰','民间信仰','已认证','on_shelf','normal','妈祖文化,民俗祈愿,海洋文化','','4.80')
ON DUPLICATE KEY UPDATE
  temple_code=VALUES(temple_code),position=VALUES(position),belief_code=VALUES(belief_code),sect=VALUES(sect),type=VALUES(type),
  auth_status=VALUES(auth_status),shelf_status=VALUES(shelf_status),platform_status=VALUES(platform_status),specialties=VALUES(specialties);

INSERT INTO askxuan_temple.temple_image (temple_code,url,type,sort)
SELECT seed.temple_code,seed.url,seed.type,seed.sort
FROM (
  SELECT 'T007' temple_code,'https://upload.wikimedia.org/wikipedia/commons/thumb/0/02/Huacheng_Temple_05.jpg/1280px-Huacheng_Temple_05.jpg' url,'cover' type,0 sort
  UNION ALL SELECT 'T008','https://upload.wikimedia.org/wikipedia/commons/thumb/2/2b/Yonghe_Temple%2C_Beijing.JPG/1280px-Yonghe_Temple%2C_Beijing.JPG','cover',0
  UNION ALL SELECT 'T009','https://upload.wikimedia.org/wikipedia/commons/thumb/d/de/%E9%9D%92%E5%9F%8E%E5%B1%B1%E5%A4%A9%E5%B8%88%E6%B4%9E-%E3%80%8C%E5%8F%A4%E5%B8%B8%E9%81%93%E8%A7%82%E3%80%8D%E9%97%A8%E6%A5%BC.jpg/1280px-%E9%9D%92%E5%9F%8E%E5%B1%B1%E5%A4%A9%E5%B8%88%E6%B4%9E-%E3%80%8C%E5%8F%A4%E5%B8%B8%E9%81%93%E8%A7%82%E3%80%8D%E9%97%A8%E6%A5%BC.jpg','cover',0
  UNION ALL SELECT 'T010','https://upload.wikimedia.org/wikipedia/commons/c/c0/%E7%A5%88%E5%B9%B4%E6%9C%9F%E9%97%B4%E7%9A%84%E6%B9%84%E6%B4%B2%E5%A6%88%E7%A5%96%E7%A5%96%E5%BA%993.jpg','cover',0
) seed
WHERE NOT EXISTS (
  SELECT 1 FROM askxuan_temple.temple_image image
  WHERE image.temple_code=seed.temple_code AND image.type=seed.type
);

INSERT INTO askxuan_temple.temple_service
  (temple_code,service_code,service_name,price,time_slots,status)
VALUES
  ('T007','S001','平安祈福（演示）',168.00,'["09:00-10:00","14:00-15:00"]','on_shelf'),
  ('T007','S002','地藏供灯（演示）',88.00,'["10:00-11:00","15:00-16:00"]','on_shelf'),
  ('T007','S005','追思回向（演示）',498.00,'["14:00-15:30"]','on_shelf'),
  ('T008','S001','吉祥祈愿（演示）',268.00,'["10:00-11:00","15:00-16:00"]','on_shelf'),
  ('T008','S002','长明灯供养（演示）',168.00,'["09:00-10:00","14:00-15:00"]','on_shelf'),
  ('T008','S012','健康祈愿（演示）',368.00,'["10:00-11:00"]','on_shelf'),
  ('T009','S001','平安祈福（演示）',188.00,'["09:00-10:00","14:00-15:00"]','on_shelf'),
  ('T009','S007','顺星化太岁（演示）',388.00,'["10:00-11:30"]','on_shelf'),
  ('T009','S011','居家环境咨询（演示）',688.00,'["14:00-15:00","15:30-16:30"]','on_shelf'),
  ('T010','S001','妈祖平安祈愿（演示）',128.00,'["09:00-10:00","14:00-15:00"]','on_shelf'),
  ('T010','S003','敬香礼仪服务（演示）',68.00,'["08:30-09:30","15:00-16:00"]','on_shelf'),
  ('T010','S004','民俗还愿礼仪（演示）',268.00,'["10:00-11:00"]','on_shelf')
ON DUPLICATE KEY UPDATE
  service_name=VALUES(service_name),price=VALUES(price),time_slots=VALUES(time_slots),status=VALUES(status);

INSERT IGNORE INTO askxuan_temple.temple_service_slot
  (temple_service_id,slot_code,label,start_time,end_time,capacity,status,sort)
SELECT service.id,
  CONCAT('slot_',LPAD(slot.ord,2,'0')),
  slot.time_range,
  SUBSTRING_INDEX(slot.time_range,'-',1),
  SUBSTRING_INDEX(slot.time_range,'-',-1),
  10,'enabled',slot.ord
FROM askxuan_temple.temple_service service
JOIN JSON_TABLE(service.time_slots,'$[*]' COLUMNS (
  ord FOR ORDINALITY,
  time_range VARCHAR(32) PATH '$'
)) slot
WHERE service.temple_code IN ('T007','T008','T009','T010');

INSERT IGNORE INTO askxuan_temple.temple_service_intent_tag (temple_service_id,tag_code)
SELECT id,CASE service_code
  WHEN 'S001' THEN 'peace' WHEN 'S002' THEN 'peace' WHEN 'S003' THEN 'peace'
  WHEN 'S004' THEN 'rite' WHEN 'S005' THEN 'rite' WHEN 'S007' THEN 'taisui'
  WHEN 'S011' THEN 'career' WHEN 'S012' THEN 'peace'
END
FROM askxuan_temple.temple_service
WHERE temple_code IN ('T007','T008','T009','T010');

COMMIT;
