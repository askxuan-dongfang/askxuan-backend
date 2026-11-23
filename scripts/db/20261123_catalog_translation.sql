-- Additive; apply after rewards, DIY publication, and temple lifecycle schemas.
CREATE TABLE IF NOT EXISTS askxuan_ai.catalog_translation (
 kind VARCHAR(32) NOT NULL,
 resource_id VARCHAR(32) NOT NULL,
 source_revision CHAR(64) NOT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'missing',
 version BIGINT NOT NULL DEFAULT 0,
 fields JSON NULL,
 published_fields JSON NULL,
 published_source_revision CHAR(64) NULL,
 published_source_fingerprints JSON NULL,
 editor_id BIGINT NULL,
 published_by BIGINT NULL,
 published_at DATETIME(6) NULL,
 lease_until DATETIME(6) NULL,
 updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
 PRIMARY KEY (kind,resource_id),
 KEY idx_catalog_translation_queue (status,lease_until,updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

-- Match DIY's current publication and qualifying-order gates. The AI user may
-- read only the customer-facing fields, never order or full snapshot contents.
CREATE OR REPLACE SQL SECURITY DEFINER VIEW askxuan_diy.catalog_public_design_text AS
SELECT s.id, p.id AS publication_id,
 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(IF(JSON_VALID(p.design_snapshot),p.design_snapshot,'{}'),'$.name')),'') AS title,
 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(IF(JSON_VALID(p.design_snapshot),p.design_snapshot,'{}'),'$.description')),'') AS description
FROM askxuan_diy.diy_design s
JOIN askxuan_diy.diy_design_publication p ON p.design_id=s.id AND p.id=s.publication_id
JOIN askxuan_diy.diy_order o ON o.id=p.order_id AND o.design_id=p.design_id AND o.user_id=p.creator_id
WHERE s.status='public' AND o.payment_status='success'
 AND o.status IN ('in_making','awaiting_blessing','blessing_in_progress','blessing_completed','awaiting_shipment','shipped','completed')
 AND CAST(JSON_UNQUOTE(JSON_EXTRACT(IF(JSON_VALID(p.design_snapshot),p.design_snapshot,'{}'),'$.id')) AS UNSIGNED)=p.design_id
 AND BINARY JSON_UNQUOTE(JSON_EXTRACT(IF(JSON_VALID(p.design_snapshot),p.design_snapshot,'{}'),'$.userId'))=BINARY p.creator_id
 AND EXISTS (SELECT 1 FROM askxuan_payment.payment pay WHERE pay.order_no=o.order_no
   AND pay.order_type='diy_order' AND pay.user_id=o.user_id AND pay.status='success'
   AND pay.channel IN ('wechat','alipay','balance','mock','demo_balance'))
 AND NOT EXISTS (SELECT 1 FROM askxuan_payment.refund f JOIN askxuan_payment.payment pay ON pay.id=f.payment_id
   WHERE pay.order_no=o.order_no AND pay.order_type='diy_order' AND f.status IN ('pending','processing','success'));

GRANT SELECT (id,name,description,status,freight_template_id) ON askxuan_product.product TO 'ai_user'@'%';
GRANT SELECT (product_id,spec_name,spec_value) ON askxuan_product.product_sku TO 'ai_user'@'%';
GRANT SELECT (id,type,free_shipping,status,config) ON askxuan_logistics.freight_template TO 'ai_user'@'%';
GRANT SELECT (id,name,category,description,status) ON askxuan_payment.points_product TO 'ai_user'@'%';
GRANT SELECT (id,title,description,rules,prize_name,published_at) ON askxuan_marketing.reward_campaign TO 'ai_user'@'%';
GRANT SELECT (id,campaign_id,name,kind) ON askxuan_marketing.reward_prize TO 'ai_user'@'%';
GRANT SELECT (id,name,spec,status) ON askxuan_diy.material TO 'ai_user'@'%';
GRANT SELECT ON askxuan_diy.catalog_public_design_text TO 'ai_user'@'%';
GRANT SELECT (id,code,name,description,address,auth_status,platform_status,shelf_status) ON askxuan_temple.temple TO 'ai_user'@'%';
GRANT SELECT (id,temple_code,service_name,status) ON askxuan_temple.temple_service TO 'ai_user'@'%';
GRANT SELECT (id,code,temple_code,manage_by,dharma_name,lay_name,position,specialties,auth_status,shelf_status,platform_status) ON askxuan_master.master TO 'ai_user'@'%';
GRANT SELECT (code,name,summary,description,status) ON askxuan_temple.belief_profile TO 'ai_user'@'%';

-- Checkout callers share the published-current read. They cannot read drafts.
GRANT SELECT (kind,resource_id,published_source_revision,published_fields) ON askxuan_ai.catalog_translation TO 'order_user'@'%';
GRANT SELECT (kind,resource_id,published_source_revision,published_fields) ON askxuan_ai.catalog_translation TO 'payment_user'@'%';
GRANT SELECT (kind,resource_id,published_source_revision,published_fields) ON askxuan_ai.catalog_translation TO 'marketing_user'@'%';
GRANT SELECT (id,name,description,status,freight_template_id) ON askxuan_product.product TO 'order_user'@'%';
GRANT SELECT (product_id,spec_name,spec_value) ON askxuan_product.product_sku TO 'order_user'@'%';
GRANT SELECT (id,type,free_shipping,status,config) ON askxuan_logistics.freight_template TO 'order_user'@'%';
