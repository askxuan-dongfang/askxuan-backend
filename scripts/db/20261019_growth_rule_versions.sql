-- Immutable growth rule history and cross-device upgrade delivery. No balances or switches change.
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_rule_release (
 version VARCHAR(32) PRIMARY KEY, effective_at BIGINT NOT NULL,
 summary VARCHAR(2000) NOT NULL, config_json JSON NOT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'published', created_at BIGINT NOT NULL,
 approved_by VARCHAR(64) NOT NULL DEFAULT '', KEY growth_rule_effective(status,effective_at)
) ENGINE=InnoDB;
INSERT IGNORE INTO askxuan_payment.growth_rule_release(version,effective_at,summary,config_json,created_at)
VALUES('growth-v1',0,'五级成长规则首版：有效消费与非消费任务，权益按已开放范围执行。','{"levels":[{"code":1,"name":"初识","threshold":0,"goodsRateBps":10000,"serviceRateBps":10000,"couponMinimumCents":0,"couponValueCents":0,"benefit":"成长档案"},{"code":2,"name":"同行","threshold":200,"goodsRateBps":9800,"serviceRateBps":10000,"couponMinimumCents":10000,"couponValueCents":500,"benefit":"等级徽章"},{"code":3,"name":"笃行","threshold":1000,"goodsRateBps":9700,"serviceRateBps":9900,"couponMinimumCents":20000,"couponValueCents":1000,"benefit":"月度成长回顾"},{"code":4,"name":"致远","threshold":3000,"goodsRateBps":9500,"serviceRateBps":9800,"couponMinimumCents":30000,"couponValueCents":2000,"benefit":"平台客服优先分流"},{"code":5,"name":"恒久","threshold":8000,"goodsRateBps":9200,"serviceRateBps":9500,"couponMinimumCents":50000,"couponValueCents":5000,"benefit":"年度数字成长纪念卡"}],"tasks":[{"kind":"profile","award":20,"limit":1,"period":"lifetime","used":0,"available":false},{"kind":"learning","award":3,"limit":1,"period":"day","used":0,"available":false},{"kind":"note","award":2,"limit":1,"period":"day","used":0,"available":false},{"kind":"activity","award":20,"limit":2,"period":"month","used":0,"available":false},{"kind":"review","award":5,"limit":4,"period":"month","used":0,"available":false}],"monthlyGrowthLimit":200,"monthlyDiscountLimitCents":30000,"goodsDiscountCapCents":10000,"serviceDiscountCapCents":5000,"observationDays":7,"couponDays":30}',UNIX_TIMESTAMP());
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_level_notice (
 user_id VARCHAR(64) PRIMARY KEY, level TINYINT NOT NULL DEFAULT 1, updated_at BIGINT NOT NULL
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS askxuan_payment.growth_account_rule (user_id VARCHAR(64) PRIMARY KEY, version VARCHAR(32) NOT NULL) ENGINE=InnoDB;
