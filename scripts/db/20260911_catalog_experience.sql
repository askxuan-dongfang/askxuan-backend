-- Apply once before deploying product-service. No existing products or stock are reset.
USE askxuan_product;
ALTER TABLE product
 ADD COLUMN is_experience TINYINT(1) NOT NULL DEFAULT 0,
 ADD COLUMN source_name VARCHAR(100) NOT NULL DEFAULT '',
 ADD COLUMN source_url VARCHAR(1000) NOT NULL DEFAULT '',
 ADD COLUMN source_note VARCHAR(500) NOT NULL DEFAULT '';
