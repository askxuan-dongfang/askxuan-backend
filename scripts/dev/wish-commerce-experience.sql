-- 问玄东方虚拟体验数据，供独立验收库使用。
-- 依赖仓库的初始化脚本和截至 20261120 的预约、商城迁移。
SET NAMES utf8mb4;
START TRANSACTION;

INSERT IGNORE INTO askxuan_temple.temple
  (code,name,region,type,belief_code,sect,status,auth_status,platform_status,shelf_status,description)
VALUES
  ('TEXP1120','清和体验院','体验场景','汉传佛教','han_buddhism','禅宗','正常','certified','normal','on_shelf','虚拟承办方，场次仅用于体验预约流程。');

INSERT IGNORE INTO askxuan_temple.temple_service
  (temple_code,service_code,service_name,price,time_slots,status,intent_mode)
VALUES
  ('TEXP1120','S001','平安祈福（体验）',18.00,'["10:30-11:30","18:00-19:00"]','on_shelf','custom');

SET @experience_service_id = (SELECT id FROM askxuan_temple.temple_service WHERE temple_code='TEXP1120' AND service_code='S001');
INSERT IGNORE INTO askxuan_temple.temple_service_intent_tag (temple_service_id,tag_code)
VALUES (@experience_service_id,'peace');
INSERT IGNORE INTO askxuan_temple.temple_service_slot
  (temple_service_id,slot_code,label,start_time,end_time,capacity,status,sort,participation_mode)
VALUES
  (@experience_service_id,'experience_onsite','体验到场','10:30','11:30',8,'enabled',1,'onsite'),
  (@experience_service_id,'experience_proxy','体验代办','18:00','19:00',12,'enabled',2,'proxy');

INSERT IGNORE INTO askxuan_logistics.freight_template
  (id,name,type,free_shipping,config,status)
VALUES
  (991120,'体验运费 · 按件','by_piece',0,
   '{"default":{"first":1,"firstFee":12,"next":1,"nextFee":3},"regions":{"上海市":{"first":2,"firstFee":8,"next":1,"nextFee":2}}}',
   'enabled');

INSERT IGNORE INTO askxuan_product.product
  (id,product_no,name,category_id,description,status,is_experience,price,stock,tags,freight_template_id)
VALUES
  (991120,'EXP-1120-INCENSE','莲纹香插（体验）',4,'模拟陶制香插，体验礼物挑选和运费核对。','on_shelf',1,69.00,12,'体验商品,送礼',991120),
  (991121,'EXP-1120-BRACELET','青木手串（体验）',1,'模拟日常手串，可从送礼和日常佩戴两个主题选购。','on_shelf',1,49.00,12,'体验商品,送礼,日常佩戴',991120);

INSERT IGNORE INTO askxuan_payment.points_product
  (id,name,category,description,points_price,stock,status)
VALUES
  (991120,'随身平安卡（体验）','体验好礼','虚拟积分商品，供测试兑换和取消返还。',120,10,'on_sale');

INSERT IGNORE INTO askxuan_user.user (id,mobile,nickname)
VALUES (991120,'fixture-wish-1120','体验用户');
INSERT IGNORE INTO askxuan_user.user_address (id,user_id,name,phone,province,city,district,detail,is_default)
VALUES
  (991120,991120,'体验用户','00000000000','上海市','上海市','浦东新区','虚拟收货地址 1 号',1),
  (991121,991120,'体验用户','00000000000','浙江省','杭州市','西湖区','虚拟收货地址 2 号',0);

INSERT IGNORE INTO askxuan_payment.points_account (user_id,balance) VALUES ('991120',500);
INSERT IGNORE INTO askxuan_payment.points_ledger (user_id,event_key,kind,delta,balance_after,reference_no)
VALUES ('991120','fixture:wish-commerce:20261120','fixture',500,500,'wishes-commerce-20261120');

COMMIT;
