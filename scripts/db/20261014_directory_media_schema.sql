SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_master.master_image (
 master_code VARCHAR(32) NOT NULL, url VARCHAR(500) NOT NULL,
 caption VARCHAR(255) NOT NULL DEFAULT '',kind VARCHAR(32) NOT NULL DEFAULT '',sort INT NOT NULL DEFAULT 0,
 name_guard VARCHAR(255) NOT NULL, avatar_guard VARCHAR(500) NOT NULL,
 PRIMARY KEY(master_code,url)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
SET @q=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_temple' AND table_name='temple_image' AND column_name='caption')=0, 'ALTER TABLE askxuan_temple.temple_image ADD COLUMN caption VARCHAR(255) NOT NULL DEFAULT ''''','SELECT 1');
PREPARE s FROM @q; EXECUTE s; DEALLOCATE PREPARE s;
SET @q=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_temple' AND table_name='temple_image' AND column_name='kind')=0, 'ALTER TABLE askxuan_temple.temple_image ADD COLUMN kind VARCHAR(32) NOT NULL DEFAULT ''''','SELECT 1');
PREPARE s FROM @q; EXECUTE s; DEALLOCATE PREPARE s;
SET @q=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_temple' AND table_name='temple_image' AND column_name='name_guard')=0, 'ALTER TABLE askxuan_temple.temple_image ADD COLUMN name_guard VARCHAR(255) NOT NULL DEFAULT ''''','SELECT 1');
PREPARE s FROM @q; EXECUTE s; DEALLOCATE PREPARE s;
SET @q=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_temple' AND table_name='temple_image' AND column_name='cover_guard')=0, 'ALTER TABLE askxuan_temple.temple_image ADD COLUMN cover_guard VARCHAR(500) NOT NULL DEFAULT ''''','SELECT 1');
PREPARE s FROM @q; EXECUTE s; DEALLOCATE PREPARE s;
