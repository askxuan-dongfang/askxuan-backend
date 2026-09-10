-- Content approval publishes only media attached to the reviewed post. No data backfill.
GRANT UPDATE (audit_status, update_time) ON askxuan_media.media_asset TO 'community_user'@'%';
