-- DIY reads existing S006 offers; no copied catalog, price override or data rewrite.
GRANT SELECT ON askxuan_temple.temple TO 'diy_user'@'%';
GRANT SELECT ON askxuan_temple.temple_service TO 'diy_user'@'%';
GRANT SELECT ON askxuan_master.master TO 'diy_user'@'%';
GRANT SELECT ON askxuan_master.master_service_tag TO 'diy_user'@'%';
