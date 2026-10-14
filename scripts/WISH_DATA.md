# Service wishes and demo media

Seven wish entries stay fixed. Standard services inherit `service_intention_rule` unless the provider service has `intent_mode=custom`. A custom empty set deliberately excludes that service. Both platform administration and temple service editing can restore inheritance. Editing a master's service price preserves the existing row and override.

Run `scripts/test-wish-mysql.sh` with Docker available. It creates a labelled temporary MySQL 8 container on localhost:13316, exercises real queries, rollback, media guards and repeat migrations, then removes it. The Go integration fixtures refuse any other DSN. Unit/handler checks: `go test ./services/commerce/product-service/internal/... ./services/content/temple-service/internal/... ./services/content/master-service/internal/...`.

For an existing deployment, back up affected tables and apply in this order:

1. `db/20261014_service_intention_defaults.sql` (relative to this scripts directory).
2. `db/20261014_directory_media_schema.sql`.
3. Publish H5 static assets and then `db/20261014_directory_media.sql` for an explicitly enabled demo dataset.
4. Publish updated product, master and temple services and admin applications.

Fresh `db/init.sql` includes the schemas/default rules but does not silently add the fictional catalog. The 72 DT / 144 DM+DW media backfill only matches exact demo names and empty/unchanged demo images. It never overwrites operator images. Master and temple gallery reads suppress mismatched names or changed primary images.

To regenerate the demo mappings, run `python3 scripts/build-directory-examples.py` followed by `python3 scripts/build-directory-media.py --h5 /path/to/web-h5`. The H5 repository stores 20 generated source images with 40 JPEG renditions, prompt provenance, explicit shared-demo labels and media identity tests. This is a reusable illustration pool, not 216 unique portraits or real location photography.
