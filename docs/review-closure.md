# Service review closure — 2026-09-27

The booking domain remains the source of truth. Review submission atomically records the review, completed→reviewed transition, audit and one durable outbox event. The review projection accepts full-temple execution without a master and preserves moderation on replay. `review_booking_context` holds provider/service names and the canonical booking reply. Public lists redact customer/order identifiers. Aggregate ratings include only normal booking reviews with verified booking context, independent of paging and star filtering.

Independent master identity is resolved from the authenticated profile, never reconstructed from a numeric ID. Provider list/detail/reply are scoped. Commerce review creation forwards the caller token to the order domain and requires ownership and completed receipt. New provider replies go through the booking domain, are scoped and may be submitted once. Public image attachments are limited to six; text and replies to 500 characters.

## Release order

1. Build the committed migration utility `services/content/review-service/cmd/migrate-context` for the ECS architecture. Run inside the review container using its own `etc/review.yaml`. This only creates the additive context table; no root database credential or cross-domain grants are needed.
2. Verify the only database contract delta from c12e97e is this additive table, preserve `/opt/askxuan/ci/config.json`, then update `backend_contract` under the publication lock to the new commit's contract hash.
3. Publish the exact CI backend artifact through the existing receiver; verify transaction and health.
4. Run `services/content/booking-service/cmd/replay-reviews` inside the booking container with its own config. It enqueues existing booking reviews under deterministic repair keys with action `review_synced`, which does not send fresh customer notifications. Existing ratings/content, original dates and hidden state are preserved. Do not insert synthetic production reviews.
5. Publish H5, verify anonymous ratings/profile reviews, own-account APIs without impersonation, and browser navigation. Preserve the prior release and additive table on application rollback.

## Verification

- `go test` in changed modules.
- Disposable MySQL tests: `TestReviewTransactionIntegration` (ownership, incomplete order rejection, concurrent duplicate rejection, outbox failure rollback) and `TestProjectionIntegration` (temple/independent master, aggregates, pagination, moderation replay, tenant filter).
- `TestMasterIdentityUsesProfileCode` and `TestCommerceReviewEligibility` verify caller-scoped upstream lookup.
- H5 `npm run build`, `npm run test:auth`, CI browser regressions, plus Ego Lite interaction checks.

This release is based on the deployed c12e97e backend. Unmigrated cash-wallet changes on main are intentionally excluded. Merge this branch into the cash-wallet line before that later release; do not revert its changes when reconciling.
