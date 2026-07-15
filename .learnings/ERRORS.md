# Errors

## [ERR-20260715-001] seed consistency migration

**Logged**: 2026-07-15T21:49:33+08:00
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
The first data-consistency migration used `booking_no` for tables whose deployed column is `booking_id`.

### Error
```text
ERROR 1054 (42S22) at line 17: Unknown column 'booking_no' in 'where clause'
```

### Context
- The migration targeted `askxuan_booking.booking_status_log` and `booking_review`.
- The failed statements ran inside an uncommitted transaction.

### Suggested Fix
Verify deployed columns with `SHOW COLUMNS` before writing cross-version data migrations; use `booking_id` for both tables.

### Metadata
- Reproducible: yes
- Related Files: scripts/db/20260715_seed_data_consistency.sql

---

## [ERR-20260715-004] workspace root go test

**Logged**: 2026-07-15T22:00:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Running `go test ./...` at the backend workspace root is invalid because the root is not itself one of the modules listed in `go.work`.

### Error
```text
pattern ./...: directory prefix . does not contain modules listed in go.work or their selected dependencies
```

### Context
- The affected booking module test passed when run from its module directory.
- The final suite iterated every `go.work` `DiskPath` explicitly.

### Suggested Fix
For full backend verification, iterate `go work edit -json | jq -r '.Use[].DiskPath'` and run `go test ./...` inside each module.

### Metadata
- Reproducible: yes
- Related Files: go.work

---

## [ERR-20260715-003] idempotent temple catalog migration

**Logged**: 2026-07-15T21:58:00+08:00
**Priority**: high
**Status**: resolved
**Area**: backend

### Summary
`INSERT IGNORE` duplicated temple services because an older deployed table lacked the unique index present in `db/init.sql`.

### Error
```text
Repeated migration increased temple_service rows from 4 to 60.
```

### Context
- Forward schema drift meant `(temple_code, service_code)` was not unique.
- The migration was intentionally executed twice to expose this condition.

### Suggested Fix
Deduplicate by canonical minimum ID, preserve tag mappings, add the missing unique index, then seed with `INSERT IGNORE`.

### Metadata
- Reproducible: yes
- Related Files: scripts/db/20260715_seed_data_consistency.sql

---

## [ERR-20260715-002] booking service validation rollout

**Logged**: 2026-07-15T21:52:30+08:00
**Priority**: high
**Status**: resolved
**Area**: backend

### Summary
The new temple-service validation initially failed for valid bookings because `booking_user` lacked read access to `askxuan_temple.temple_service`.

### Error
```text
Error 1142 (42000): SELECT command denied to user 'booking_user' for table 'temple_service'
```

### Context
- Existing grants covered `temple`, `service_type`, and `master` only.
- The logic initially mapped every query error to a misleading business 404.

### Suggested Fix
Add the least-privilege SELECT grant and distinguish `sqlx.ErrNotFound` from infrastructure errors.

### Metadata
- Reproducible: yes
- Related Files: scripts/db/microservice-migration.sql, services/content/booking-service/internal/logic/createlogic.go

---
