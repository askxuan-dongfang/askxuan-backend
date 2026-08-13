-- Keep public temple detail assets and one demo management account per temple in sync.
-- Repeatable against an existing environment.
SET NAMES utf8mb4;
START TRANSACTION;

INSERT INTO askxuan_temple.temple_image (temple_code,url,type,sort)
SELECT temple.code,temple.cover_image,'cover',0
FROM askxuan_temple.temple temple
WHERE temple.cover_image <> ''
  AND NOT EXISTS (
    SELECT 1 FROM askxuan_temple.temple_image image
    WHERE image.temple_code=temple.code AND image.type='cover'
  );

INSERT INTO askxuan_auth.admin_account
  (account,password,name,role_id,temple_id,master_id,shop_id,status)
VALUES
  ('lingyin_admin','123456','灵隐寺管理员',2,'T001','',0,'enabled'),
  ('baiyun_admin','123456','白云观管理员',2,'T002','',0,'enabled'),
  ('shaolin_admin','123456','少林寺管理员',2,'T003','',0,'enabled'),
  ('dazhao_admin','123456','大昭寺管理员',2,'T004','',0,'enabled'),
  ('putuo_admin','123456','普陀山管理员',2,'T005','',0,'disabled'),
  ('wudang_admin','123456','武当山管理员',2,'T006','',0,'enabled'),
  ('jiuhua_admin','123456','九华山化城寺管理员',2,'T007','',0,'enabled'),
  ('yonghe_admin','123456','北京雍和宫管理员',2,'T008','',0,'enabled'),
  ('qingcheng_admin','123456','青城山天师洞管理员',2,'T009','',0,'enabled'),
  ('mazu_admin','123456','湄洲妈祖祖庙管理员',2,'T010','',0,'enabled')
ON DUPLICATE KEY UPDATE
  name=VALUES(name),role_id=VALUES(role_id),temple_id=VALUES(temple_id),
  master_id='',shop_id=0;

INSERT IGNORE INTO askxuan_temple.temple_admin (temple_code,account_id,role)
SELECT account.temple_id,account.id,'admin'
FROM askxuan_auth.admin_account account
WHERE account.role_id=2 AND account.temple_id IN
  ('T001','T002','T003','T004','T005','T006','T007','T008','T009','T010');

COMMIT;

GRANT SELECT, INSERT, DELETE ON askxuan_temple.temple_admin TO 'auth_user'@'%';
