-- Operator-provisioned demonstration accounts may have non-deliverable placeholder contacts.
-- Never assert email ownership for these accounts. Public registration remains verified.
ALTER TABLE askxuan_auth.auth_identity MODIFY verified_at DATETIME NULL DEFAULT NULL;
