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

## [ERR-20260718-001] backend service path assumptions

**Logged**: 2026-07-18T16:30:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Audit reads initially assumed services lived at the repository root and used an unmatched zsh glob.

### Error
```text
sed: review-service/review.go: No such file or directory
zsh: no matches found: services/content/review-service/*.api
```

### Context
- Backend services are grouped under `services/<domain>/<service>`.
- zsh aborts commands when a glob has no match.

### Suggested Fix
Discover paths with `rg --files` first and use `rg -g` filters instead of shell globs in audit commands.

### Metadata
- Reproducible: yes
- Related Files: go.work

---

## [ERR-20260718-002] booking review event image contract

**Logged**: 2026-07-18T16:40:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
The new booking-reviewed event passed `[]string` images into the review service's JSON string field.

### Error
```text
cannot use req.Images (variable of type []string) as string value in struct literal
```

### Context
- Booking API uses an array for review images.
- The review database stores the array as JSON text.

### Suggested Fix
Serialize the array at the booking event boundary and test the shared event payload shape.

### Metadata
- Reproducible: yes
- Related Files: services/content/booking-service/internal/logic/reviewlogic.go

---

## [ERR-20260718-003] zsh workspace module iteration

**Logged**: 2026-07-18T17:00:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The first full-module loop used zsh's read-only `modules` parameter, then assumed command substitution would split on newlines.

### Error
```text
zsh: read-only variable: modules
cd: no such file or directory: ./common\n./services/...
```

### Suggested Fix
Pipe module paths into `while IFS= read -r module_path`; do not rely on implicit word splitting in zsh.

### Metadata
- Reproducible: yes
- Related Files: go.work

---

## [ERR-20260718-004] sandboxed Xcode simulator access

**Logged**: 2026-07-18T17:05:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: tests

### Summary
Sandboxed `xcodebuild` could not access CoreSimulator logs and services, so workspace discovery failed before project evaluation.

### Error
```text
CoreSimulatorService connection became invalid
Error opening log file ... Operation not permitted
```

### Suggested Fix
Run approved Xcode validation outside the filesystem sandbox and use `generic/platform=iOS` with signing disabled for compile-only checks.

### Metadata
- Reproducible: yes
- Related Files: apps/ios-customer, apps/ios-master

---

## [ERR-20260715-011] mysql-multitable-delete-needs-default-database

**Logged**: 2026-07-15T23:59:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: acceptance-tests

### Summary
Fully qualified tables in a multi-table MySQL delete still failed with `ERROR 1046: No database selected` in the cleanup session.

### Resolution
Start the cleanup block with `USE askxuan_booking`; the acceptance rerun now leaves zero `accept-%` bookings and zero inventory mismatches.

---

## [ERR-20260715-009] docker-exec-heredoc-without-stdin

**Logged**: 2026-07-15T23:56:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: acceptance-tests

### Summary
Acceptance cleanup used a heredoc with `docker exec` but omitted `-i`, so MySQL received no SQL while the command still exited successfully.

### Resolution
Use `docker exec -i` for heredoc input and target the payment service database `askxuan_payment`.

---

## [ERR-20260715-010] shell-backticks-in-rg-pattern

**Logged**: 2026-07-15T23:56:00+08:00
**Priority**: low
**Status**: resolved
**Area**: inspection

### Summary
A double-quoted ripgrep pattern containing Markdown backticks triggered shell command substitution.

### Resolution
Use a single-quoted regex when searching SQL identifiers that contain backticks.

---

## [ERR-20260715-008] frontend-mock-package-path

**Logged**: 2026-07-15T23:52:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend-tests

### Summary
The Mock build was first invoked from nonexistent `packages/mock`.

### Resolution
Discover package paths from tracked `package.json` files; the actual package is `packages/mock-server`.

---

## [ERR-20260715-007] jwt-authoritative-user-still-required-by-parser

**Logged**: 2026-07-15T23:46:00+08:00
**Priority**: high
**Status**: resolved
**Area**: booking-api

### Summary
The booking create handler rejected the new JWT-authoritative request before business logic because `CreateReq.UserId` was still required by the go-zero HTTP parser.

### Resolution
Mark the compatibility `userId` field optional; business logic continues to reject a supplied ID that differs from the JWT subject.

---

## [ERR-20260715-006] go-work-root-recursive-pattern

**Logged**: 2026-07-15T23:32:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
`go test ./common/... ./services/...` at the repository root fails because `services` is only a container for independent modules listed in `go.work`.

### Resolution
Read `go.work` and run `go test ./...` from each listed module; do not report the root recursive pattern as a full test.

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
## [ERR-20260715-004] gofmt-deleted-files

**Logged**: 2026-07-15T22:20:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Formatting a raw `git diff --name-only` list failed because it included deleted Go files.

### Resolution
Format existing files from scoped service directories; when formatting a diff list, filter deleted paths first.

---
## [ERR-20260715-005] mobile-booking-query-inference

**Logged**: 2026-07-15T23:20:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend

### Summary
The backup React Native client did not preserve callback parameter inference through its React Query setup.

### Resolution
Annotate service and availability slot callback values with the shared contract types.

---

## [ERR-20260814-001] sandbox-httptest-listener

**Logged**: 2026-08-14T00:50:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The restricted workspace sandbox blocked Go `httptest` from opening a loopback listener.

### Error
`httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted`

### Resolution
Rerun the unchanged full workspace suite with approved non-sandbox execution. All modules passed.

---

## [ERR-20260902-001] ecs-mysql-password-drift

**Logged**: 2026-09-02T21:37:00+08:00
**Priority**: low
**Status**: resolved
**Area**: infra

### Summary
The ECS MySQL container's baked-in `MYSQL_ROOT_PASSWORD` and an old migration fallback no longer matched the running database, so backup attempts failed before migration.

### Resolution
Source the current host runtime secret, pass it transiently as `MYSQL_PWD`, verify the backup before migration, and never print the credential value.

---
## [ERR-20260903-002] ecs-compose-project-directory

**Logged**: 2026-09-03T09:20:00+08:00
**Priority**: low
**Status**: resolved
**Area**: infra

### Summary
Running the ECS override Compose file from `/opt/askxuan/runtime` resolved relative bind mounts against that directory and mounted an empty AI configuration path.

### Resolution
Always pass `--project-directory /opt/askxuan/backend` when using the runtime override file so repository-relative build contexts and volume mounts resolve consistently.

---
## [ERR-20260903-003] media-runtime-credential-drift

**Logged**: 2026-09-03T20:10:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: infra

### Summary
Recreating media-service exposed that its repository YAML still contained development MySQL and MinIO credentials while ECS used rotated runtime credentials.

### Resolution
Support `MEDIA_MYSQL_DATASOURCE` and runtime MinIO credential overrides, inject them through Compose, and keep production values only in the restricted ECS secrets file.

---
