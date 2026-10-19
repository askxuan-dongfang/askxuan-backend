-- Report numbers contain the AR prefix plus a 32-character UUID (34 total).
-- Match the checkout API's existing 64-character order identifier contract.
-- Widening preserves existing values and does not change payment identifiers.
ALTER TABLE askxuan_payment.payment MODIFY COLUMN order_no VARCHAR(64) NOT NULL;
