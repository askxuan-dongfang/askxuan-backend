# Disposable account-system integration tests

Run only against a dedicated, disposable MySQL instance. The tests require
`ACCOUNT_SYSTEM_TEST_ISOLATED=1` and a DSN whose default database is exactly
`askxuan_account_test`. Create that empty database first. The fixture also creates
`askxuan_user`, `askxuan_auth`, `askxuan_payment`, `askxuan_temple` and
`askxuan_master` on that same disposable instance; it must not share an instance
with development or production data.

From `services/platform/user-service` run `go test -v ./internal/model -run TestRealUserConsumptionMySQL`.
From `services/platform/auth-service` run `go test -v ./internal/logic -run TestAccountDetailsMySQL`.
The tests remove only their reserved fixture rows (910001–910099 and 930001–930099).
Never use this fixture as a production migration.

Consumption uses `payment` as the collection source, including booking,
consultation, shop_order, diy_order and ai_report. Real channels are wechat,
alipay and balance; mock, demo_balance, EXO- experience orders and recharges are
excluded. A business order is counted once even if it has payment retries or a
refund history. Money is the sum of actual real-channel payments less successful
refunds, clamped per payment to zero; a fully refunded payment contributes zero.
Pending or failed refunds do not reduce money. Amounts are aggregated as integer
cents and converted to yuan only in the API. An anomalous double collection is
still two real collections in money but one business order in the order count.
No business-order snapshots or stale profile aggregates are added to these totals.
