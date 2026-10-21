-- Local-only additions when using the minimal email-auth fixture instead of
-- db/init.sql + directory migrations. Never apply this seed in production.
CREATE DATABASE IF NOT EXISTS askxuan_temple;
CREATE TABLE IF NOT EXISTS askxuan_temple.directory_tag (
 code VARCHAR(32) PRIMARY KEY,
 tag_group VARCHAR(16) NOT NULL,
 belief_code VARCHAR(32) NOT NULL DEFAULT '',
 name VARCHAR(32) NOT NULL,
 value VARCHAR(64) NOT NULL,
 sort INT NOT NULL DEFAULT 0,
 status VARCHAR(16) NOT NULL DEFAULT 'enabled'
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_temple.provider_directory_tag (
 provider_kind VARCHAR(16) NOT NULL,
 provider_code VARCHAR(64) NOT NULL,
 tag_code VARCHAR(32) NOT NULL,
 PRIMARY KEY(provider_kind,provider_code,tag_code)
) ENGINE=InnoDB;
INSERT IGNORE INTO askxuan_temple.directory_tag(code,tag_group,belief_code,name,value,sort,status) VALUES
('chan','tradition','han_buddhism','禅宗','chan',10,'enabled'),
('gelug','tradition','tibetan_buddhism','格鲁派','gelug',20,'enabled'),
('theravada','tradition','theravada_buddhism','上座部传统','theravada',30,'enabled'),
('quanzhen','tradition','daoism','全真派','quanzhen',40,'enabled'),
('mazu','tradition','folk','妈祖 / 天后','mazu',50,'enabled'),
('guandi','tradition','folk','关帝信俗','guandi',60,'enabled');
