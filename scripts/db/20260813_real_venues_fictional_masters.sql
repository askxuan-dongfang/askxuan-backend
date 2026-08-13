-- Real venue facts/photos + explicitly fictional demo masters.
-- Repeatable against an existing environment.
SET NAMES utf8mb4;
START TRANSACTION;

CREATE TABLE IF NOT EXISTS askxuan_temple.temple_cover_source (
  temple_code VARCHAR(16) NOT NULL,
  image_url VARCHAR(500) NOT NULL,
  source_url VARCHAR(500) NOT NULL,
  attribution VARCHAR(255) NOT NULL DEFAULT '',
  license_name VARCHAR(64) NOT NULL DEFAULT '',
  license_url VARCHAR(500) NOT NULL DEFAULT '',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (temple_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='寺院封面实景照片来源与许可';

INSERT INTO askxuan_temple.temple
  (code,name,region,type,belief_code,sect,status,address,cover_image,rating,description)
VALUES
  ('T001','灵隐寺','浙江杭州','汉传佛教','han_buddhism','禅宗','正常','浙江省杭州市西湖区灵隐路法云弄1号','https://101.96.228.71/objects/askxuan/temp/20260813173807_T001.jpg',4.90,'灵隐寺创建于东晋咸和元年（326年），位于杭州西湖西面的飞来峰与北高峰之间，是杭州历史悠久的佛教寺院。'),
  ('T002','北京白云观','北京西城','道教','daoism','全真派','正常','北京市西城区白云观街9号','https://101.96.228.71/objects/askxuan/temp/20260813173756_T002.jpg',4.70,'北京白云观始建于唐代，是全真道重要祖庭和龙门派祖庭，也是北京现存规模较大的道教宫观。'),
  ('T003','嵩山少林寺','河南登封','汉传佛教','han_buddhism','禅宗','正常','河南省郑州市登封市嵩山少林景区','https://101.96.228.71/objects/askxuan/temp/20260813174105_T003.jpg',4.80,'嵩山少林寺始建于北魏太和十九年（495年），位于嵩山少室山五乳峰下，是中国佛教禅宗与少林文化的重要场所。'),
  ('T004','大昭寺','西藏拉萨','藏传佛教','tibetan_buddhism','各派共尊','正常','西藏自治区拉萨市城关区八廓西街2号','https://101.96.228.71/objects/askxuan/temp/20260813173802_T004.jpg',4.90,'大昭寺位于拉萨老城中心，始建于公元7世纪，是西藏现存重要古建筑和藏传佛教各教派共同尊崇的寺院。'),
  ('T005','普济禅寺','浙江舟山','汉传佛教','han_buddhism','禅宗','待审核','浙江省舟山市普陀区普陀山镇香华街','https://101.96.228.71/objects/askxuan/temp/20260813173810_T005.jpg',4.60,'普济禅寺位于普陀山白华顶南麓，是普陀山规模较大的寺院之一，也是普陀山佛教活动的重要场所。'),
  ('T006','武当山紫霄宫','湖北十堰','道教','daoism','武当道教','正常','湖北省十堰市丹江口市武当山特区紫霄村','https://101.96.228.71/objects/askxuan/temp/20260813173804_T006.jpg',4.70,'紫霄宫位于武当山展旗峰下，现存主体建筑形成于明代，是武当山古建筑群的重要组成部分。'),
  ('T007','九华山化城寺','安徽池州','汉传佛教','han_buddhism','地藏法门','正常','安徽省池州市青阳县九华山风景区九华街','https://101.96.228.71/objects/askxuan/temp/20260813173807_T007.jpg',4.80,'化城寺位于九华山九华街，是九华山历史悠久的开山寺院和当地佛教建筑群的重要组成部分。'),
  ('T008','雍和宫','北京东城','藏传佛教','tibetan_buddhism','格鲁派','正常','北京市东城区雍和宫大街12号','https://101.96.228.71/objects/askxuan/temp/20260813173803_T008.jpg',4.80,'雍和宫位于北京东城区，前身为清代皇家府邸，后改为藏传佛教寺院，是北京现存重要的藏传佛教建筑群。'),
  ('T009','青城山天师洞','四川都江堰','道教','daoism','正一派','正常','四川省成都市都江堰市青城山景区','https://101.96.228.71/objects/askxuan/temp/20260813174114_T009.jpg',4.70,'天师洞位于青城山前山，是青城山道教宫观与古建筑群的重要组成部分，现存建筑依山布局。'),
  ('T010','湄洲妈祖祖庙','福建莆田','民间信仰','folk','妈祖信俗','正常','福建省莆田市秀屿区湄洲北大道988号','https://101.96.228.71/objects/askxuan/temp/20260813173801_T010.jpg',4.90,'湄洲妈祖祖庙始建于北宋雍熙四年（987年），位于湄洲岛，是妈祖信俗的重要发祥地和传承场所。')
ON DUPLICATE KEY UPDATE
  name=VALUES(name),region=VALUES(region),type=VALUES(type),belief_code=VALUES(belief_code),sect=VALUES(sect),
  address=VALUES(address),cover_image=VALUES(cover_image),description=VALUES(description);

INSERT INTO askxuan_temple.temple_cover_source
  (temple_code,image_url,source_url,attribution,license_name,license_url)
VALUES
  ('T001','https://101.96.228.71/objects/askxuan/temp/20260813173807_T001.jpg','https://commons.wikimedia.org/wiki/File:Blubb_(10595970686).jpg','Ludger Heide','CC BY-SA 2.0','https://creativecommons.org/licenses/by-sa/2.0'),
  ('T002','https://101.96.228.71/objects/askxuan/temp/20260813173756_T002.jpg','https://commons.wikimedia.org/wiki/File:WhiteCloudpic1.jpg','Gene Zhang','CC BY 2.0','https://creativecommons.org/licenses/by/2.0'),
  ('T003','https://101.96.228.71/objects/askxuan/temp/20260813174105_T003.jpg','https://commons.wikimedia.org/wiki/File:20241103_Gate_of_Shaolin_Temple.jpg','Windmemories','CC BY-SA 4.0','https://creativecommons.org/licenses/by-sa/4.0'),
  ('T004','https://101.96.228.71/objects/askxuan/temp/20260813173802_T004.jpg','https://commons.wikimedia.org/wiki/File:Jokhang_Temple_Lhasa_Tibet_China_%E8%A5%BF%E8%97%8F_%E6%8B%89%E8%90%A8_%E5%A4%A7%E6%98%AD%E5%AF%BA_-_panoramio_(6).jpg','Hiroki Ogawa','CC BY 3.0','https://creativecommons.org/licenses/by/3.0'),
  ('T005','https://101.96.228.71/objects/askxuan/temp/20260813173810_T005.jpg','https://commons.wikimedia.org/wiki/File:Puji_Temple,_Putuo,_2019-05-11_20.jpg','Siyuwj','CC BY-SA 4.0','https://creativecommons.org/licenses/by-sa/4.0'),
  ('T006','https://101.96.228.71/objects/askxuan/temp/20260813173804_T006.jpg','https://commons.wikimedia.org/wiki/File:%E7%B4%AB%E9%9C%84%E5%AE%AB.jpg','gongfu_king','CC BY-SA 2.0','https://creativecommons.org/licenses/by-sa/2.0'),
  ('T007','https://101.96.228.71/objects/askxuan/temp/20260813173807_T007.jpg','https://commons.wikimedia.org/wiki/File:Huacheng_Temple_04.jpg','WQL','CC0','https://creativecommons.org/publicdomain/zero/1.0/'),
  ('T008','https://101.96.228.71/objects/askxuan/temp/20260813173803_T008.jpg','https://commons.wikimedia.org/wiki/File:Yonghe_Temple,_Beijing.JPG','Regina800809','CC BY-SA 3.0','https://creativecommons.org/licenses/by-sa/3.0'),
  ('T009','https://101.96.228.71/objects/askxuan/temp/20260813174114_T009.jpg','https://commons.wikimedia.org/wiki/File:%E9%9D%92%E5%9F%8E%E5%B1%B1%E5%A4%A9%E5%B8%88%E6%B4%9E-%E3%80%8C%E5%8F%A4%E5%B8%B8%E9%81%93%E8%A7%82%E3%80%8D%E9%97%A8%E6%A5%BC.jpg','Kcx36','CC BY-SA 4.0','https://creativecommons.org/licenses/by-sa/4.0'),
  ('T010','https://101.96.228.71/objects/askxuan/temp/20260813173801_T010.jpg','https://commons.wikimedia.org/wiki/File:%E7%A5%88%E5%B9%B4%E6%9C%9F%E9%97%B4%E7%9A%84%E6%B9%84%E6%B4%B2%E5%A6%88%E7%A5%96%E7%A5%96%E5%BA%993.jpg','向史公哲曰','CC BY-SA 4.0','https://creativecommons.org/licenses/by-sa/4.0')
ON DUPLICATE KEY UPDATE image_url=VALUES(image_url),source_url=VALUES(source_url),attribution=VALUES(attribution),license_name=VALUES(license_name),license_url=VALUES(license_url);

DELETE FROM askxuan_temple.temple_image WHERE type='cover' AND temple_code IN ('T001','T002','T003','T004','T005','T006','T007','T008','T009','T010');
INSERT INTO askxuan_temple.temple_image (temple_code,url,type,sort)
SELECT code,cover_image,'cover',0 FROM askxuan_temple.temple WHERE code IN ('T001','T002','T003','T004','T005','T006','T007','T008','T009','T010');

INSERT INTO askxuan_master.master
  (code,dharma_name,lay_name,temple_code,position,belief_code,sect,type,auth_status,shelf_status,platform_status,specialties,avatar,rating)
VALUES
  ('M001','明觉法师（演示）','林知远','T001','客堂法师','han_buddhism','禅宗','佛教','已认证','on_shelf','normal','禅修入门,佛教文化,祈愿礼仪','https://101.96.228.71/objects/askxuan/temp/20260813174243_M001.jpg',4.90),
  ('M002','玄和道长（演示）','赵清远','T002','经师','daoism','全真派','道教','已认证','on_shelf','normal','道教文化,科仪讲解,养生导引','https://101.96.228.71/objects/askxuan/temp/20260813174246_M002.jpg',4.80),
  ('M003','延澄法师（演示）','周安行','T003','禅修讲师','han_buddhism','禅宗','佛教','已认证','on_shelf','normal','禅修指导,少林文化,静心课程','https://101.96.228.71/objects/askxuan/temp/20260813174238_M003.jpg',4.80),
  ('M004','嘉措讲师（演示）','','T004','文化讲师','tibetan_buddhism','各派共尊','佛教','已认证','on_shelf','normal','藏传佛教文化,寺院历史,祈愿礼仪','https://101.96.228.71/objects/askxuan/temp/20260813174249_M004.jpg',4.90),
  ('M005','慧闻法师（演示）','孙明远','T005','客堂法师','han_buddhism','禅宗','佛教','待审核','off_shelf','normal','观音文化,佛教礼仪,静心交流','https://101.96.228.71/objects/askxuan/temp/20260813174250_M005.jpg',4.50),
  ('M006','守一道长（演示）','张云舟','T006','经师','daoism','武当道教','道教','已认证','on_shelf','normal','武当文化,太极养生,道教礼仪','https://101.96.228.71/objects/askxuan/temp/20260813174248_M006.jpg',4.70),
  ('M007','行愿法师（演示）','吴善行','T007','客堂法师','han_buddhism','地藏法门','佛教','已认证','on_shelf','normal','地藏文化,佛教礼仪,静心交流','https://101.96.228.71/objects/askxuan/temp/20260813173804_M007.png',4.70),
  ('M008','嘉木扬讲师（演示）','','T008','文化讲师','tibetan_buddhism','格鲁派','佛教','已认证','on_shelf','normal','藏传佛教文化,建筑讲解,祈愿礼仪','https://101.96.228.71/objects/askxuan/temp/20260813173803_M008.png',4.70),
  ('M009','静虚道长（演示）','陈守静','T009','经师','daoism','正一派','道教','已认证','on_shelf','normal','青城道教文化,养生导引,礼仪讲解','https://101.96.228.71/objects/askxuan/temp/20260813173803_M009.png',4.60),
  ('M010','林怀恩讲师（演示）','林怀恩','T010','文化讲师','folk','妈祖信俗','民间信仰','已认证','on_shelf','normal','妈祖文化,民俗礼仪,海洋文化','https://101.96.228.71/objects/askxuan/temp/20260813173807_M010.jpg',4.80)
ON DUPLICATE KEY UPDATE
  dharma_name=VALUES(dharma_name),lay_name=VALUES(lay_name),temple_code=VALUES(temple_code),position=VALUES(position),
  belief_code=VALUES(belief_code),sect=VALUES(sect),type=VALUES(type),auth_status=VALUES(auth_status),
  shelf_status=VALUES(shelf_status),platform_status=VALUES(platform_status),specialties=VALUES(specialties),avatar=VALUES(avatar),rating=VALUES(rating);

INSERT INTO askxuan_master.master_profile_ext (master_code,bio,pricing)
VALUES
  ('M001','虚构演示人物。设定为禅宗文化与基础禅修讲师，负责线上文化讲解和预约沟通。','演示服务价格以寺院服务目录为准'),
  ('M002','虚构演示人物。设定为全真道文化经师，侧重经典文化、礼仪和养生导引讲解。','演示服务价格以寺院服务目录为准'),
  ('M003','虚构演示人物。设定为少林禅修文化讲师，提供静心课程与少林文化介绍。','演示服务价格以寺院服务目录为准'),
  ('M004','虚构演示人物，不使用活佛等真实宗教头衔。设定为藏传佛教文化讲师。','演示服务价格以寺院服务目录为准'),
  ('M005','虚构演示人物。设定为普济禅寺客堂法师，当前资料仍处于平台演示审核流程。','审核通过后按寺院服务目录展示'),
  ('M006','虚构演示人物。设定为武当文化经师，侧重太极养生与道教礼仪介绍。','演示服务价格以寺院服务目录为准'),
  ('M007','虚构演示人物。设定为九华山地藏文化与佛教礼仪讲师。','演示服务价格以寺院服务目录为准'),
  ('M008','虚构演示人物。设定为雍和宫建筑与藏传佛教文化讲师。','演示服务价格以寺院服务目录为准'),
  ('M009','虚构演示人物。设定为青城山道教文化与养生导引讲师。','演示服务价格以寺院服务目录为准'),
  ('M010','虚构演示人物。设定为妈祖信俗与海洋民俗文化讲师。','演示服务价格以寺院服务目录为准')
ON DUPLICATE KEY UPDATE bio=VALUES(bio),pricing=VALUES(pricing);

UPDATE askxuan_auth.admin_account SET name='普济禅寺管理员' WHERE account='putuo_admin';
UPDATE askxuan_auth.admin_account SET name='武当山紫霄宫管理员' WHERE account='wudang_admin';
UPDATE askxuan_auth.admin_account SET name='明觉法师（演示）',temple_id='T001',master_id='M001',status='enabled' WHERE account='zhihai';

INSERT INTO askxuan_auth.admin_account (account,password,name,role_id,temple_id,master_id,shop_id,status)
VALUES
  ('xuanhe','123456','玄和道长（演示）',3,'T002','M002',0,'enabled'),
  ('yancheng','123456','延澄法师（演示）',3,'T003','M003',0,'enabled'),
  ('jiacuo','123456','嘉措讲师（演示）',3,'T004','M004',0,'enabled'),
  ('huiwen','123456','慧闻法师（演示）',3,'T005','M005',0,'disabled'),
  ('shouyi','123456','守一道长（演示）',3,'T006','M006',0,'enabled'),
  ('xingyuan','123456','行愿法师（演示）',3,'T007','M007',0,'enabled'),
  ('jiamuyang','123456','嘉木扬讲师（演示）',3,'T008','M008',0,'enabled'),
  ('jingxu','123456','静虚道长（演示）',3,'T009','M009',0,'enabled'),
  ('huaien','123456','林怀恩讲师（演示）',3,'T010','M010',0,'enabled')
ON DUPLICATE KEY UPDATE name=VALUES(name),role_id=VALUES(role_id),temple_id=VALUES(temple_id),master_id=VALUES(master_id),shop_id=0,status=VALUES(status);

UPDATE askxuan.extra_service SET name='灵隐寺·祈愿加持',description='明觉法师（演示）提供线上祈愿礼仪服务' WHERE code='E001';
UPDATE askxuan.extra_service SET name='北京白云观·道教文化祈愿',description='玄和道长（演示）提供道教文化与祈愿礼仪服务' WHERE code='E002';
UPDATE askxuan.extra_service SET name='嵩山少林寺·禅修祈愿',description='延澄法师（演示）提供禅修文化与祈愿服务' WHERE code='E003';
UPDATE askxuan.extra_service SET name='大昭寺·文化祈愿',description='嘉措讲师（演示）提供藏传佛教文化与祈愿礼仪讲解' WHERE code='E004';
UPDATE askxuan_finance.settlement SET target_name='明觉法师（演示）' WHERE settle_type='master' AND target_id='M001';

-- C 端演示用户不复用大师头像；仅清理旧样例路径，保留用户自行上传的头像。
UPDATE askxuan_user.user
SET avatar=''
WHERE id=1 AND avatar='/assets/master-avatar-zhihai.jpg';
UPDATE askxuan_review.review SET content='玄和道长（演示）的文化讲解清晰，整体体验安心。' WHERE review_no='RV20260620001';
UPDATE askxuan_review.review SET content='延澄法师（演示）的禅修讲解细致，整体体验不错。' WHERE review_no='RV20260625002';
UPDATE askxuan_audit.audit_queue SET content_snapshot='{"name":"普济禅寺","type":"汉传佛教","status":"待审核"}' WHERE biz_type='temple' AND biz_id='T005';
UPDATE askxuan_audit.audit_queue SET content_snapshot='{"name":"慧闻法师（演示）","credential":"演示资质资料"}' WHERE biz_type='master' AND biz_id='M005';

-- Keep compatibility copies aligned where they exist.
UPDATE askxuan.temple target JOIN askxuan_temple.temple source ON source.code=target.code
SET target.name=source.name,target.region=source.region,target.type=source.type,target.belief_code=source.belief_code,
    target.sect=source.sect,target.status=source.status,target.address=source.address,target.cover_image=source.cover_image,
    target.description=source.description;
UPDATE askxuan.master target JOIN askxuan_master.master source ON source.code=target.code
SET target.dharma_name=source.dharma_name,target.lay_name=source.lay_name,target.temple_code=source.temple_code,
    target.position=source.position,target.belief_code=source.belief_code,target.sect=source.sect,target.type=source.type,
    target.auth_status=source.auth_status,target.shelf_status=source.shelf_status,target.platform_status=source.platform_status,
    target.specialties=source.specialties,target.avatar=source.avatar,target.rating=source.rating;

COMMIT;
