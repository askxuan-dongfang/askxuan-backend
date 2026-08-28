-- C 端手机号注册初始化补齐。
-- 新注册由 user-service 在同一事务写入 user + user_profile；本迁移只修复历史缺口。
INSERT IGNORE INTO askxuan_user.user_profile
  (user_id, preference_tags, total_orders, total_spent, last_active_time)
SELECT id, '', 0, 0.00, create_time
FROM askxuan_user.user;
