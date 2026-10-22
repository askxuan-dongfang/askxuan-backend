package model

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type sandboxEventFailureConn struct{ sqlx.SqlConn }
type sandboxEventFailureSession struct{ sqlx.Session }

func (c sandboxEventFailureConn) TransactCtx(ctx context.Context, fn func(context.Context, sqlx.Session) error) error {
	return c.SqlConn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error { return fn(ctx, sandboxEventFailureSession{tx}) })
}
func (s sandboxEventFailureSession) ExecCtx(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if strings.HasPrefix(query, "INSERT INTO payout_sandbox_event") && len(args) > 3 && args[3] == "result" {
		return nil, errors.New("injected simulated receipt event failure")
	}
	return s.Session.ExecCtx(ctx, query, args...)
}

func TestPayoutSandboxIntegration(t *testing.T) {
	dsn := os.Getenv("PAYOUT_SANDBOX_TEST_DSN")
	if dsn == "" {
		t.Skip("set PAYOUT_SANDBOX_TEST_DSN to a disposable askxuan_admin_test_* database")
	}
	conn := sqlx.NewMysql(dsn)
	sqlx.DisableLog()
	ctx := context.Background()
	var name string
	if err := conn.QueryRowCtx(ctx, &name, "SELECT DATABASE()"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "askxuan_admin_test_") {
		t.Fatal("refusing non-test schema")
	}
	script, err := os.ReadFile("../../../../../scripts/db/20261022_payout_sandbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"payout_sandbox", "payout_sandbox_event"} {
		prefix := "CREATE TABLE IF NOT EXISTS " + table + " ("
		i := strings.Index(string(script), prefix)
		if i < 0 {
			t.Fatal("missing sandbox schema")
		}
		if _, err = conn.ExecCtx(ctx, strings.SplitN(string(script)[i:], ";", 2)[0]); err != nil {
			t.Fatal(err)
		}
	}
	// This schema has only the two simulator tables. Successful flow execution
	// cannot depend on any real withdrawal, settlement, transaction or ledger.
	var realTables int
	if err = conn.QueryRowCtx(ctx, &realTables, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('withdrawal','settlement','finance_transaction','finance_ledger_entry','finance_log')`); err != nil || realTables != 0 {
		t.Fatal("simulator fixture must not contain real finance tables")
	}
	repo := NewPayoutSandboxModel(conn)
	ids := []string{}
	var idsLock sync.Mutex
	t.Cleanup(func() {
		for _, id := range ids {
			_, _ = conn.ExecCtx(ctx, "DELETE FROM payout_sandbox_event WHERE sandbox_id=?", id)
			_, _ = conn.ExecCtx(ctx, "DELETE FROM payout_sandbox WHERE id=?", id)
		}
	})
	create := func() *PayoutSandbox {
		t.Helper()
		row, e := repo.Create(ctx, uuid.NewString(), 12345, "9")
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, row.ID)
		if !strings.HasPrefix(row.ID, "sandbox_") || row.Version != 1 || row.Attempt != 0 || row.Status != SandboxPendingReview {
			t.Fatal("invalid simulated creation")
		}
		return row
	}
	act := func(row *PayoutSandbox, action, outcome string) *PayoutSandbox {
		t.Helper()
		next, e := repo.Act(ctx, row.ID, PayoutSandboxAction{RequestID: uuid.NewString(), ActorID: "9", Version: row.Version, Action: action, Outcome: outcome})
		if e != nil {
			t.Fatal(e)
		}
		return next
	}
	row := create()
	again, err := repo.Create(ctx, row.CreateRequestID, row.AmountCents, "9")
	if err != nil || again.ID != row.ID {
		t.Fatal("creation retry duplicated instance")
	}
	if _, err = repo.Create(ctx, row.CreateRequestID, row.AmountCents+1, "9"); !errors.Is(err, ErrSandboxConflict) {
		t.Fatal("creation key accepted changed amount")
	}
	if _, err = repo.Create(ctx, row.CreateRequestID, row.AmountCents, "10"); !errors.Is(err, ErrSandboxConflict) {
		t.Fatal("creation key accepted changed actor")
	}
	approve := PayoutSandboxAction{RequestID: uuid.NewString(), ActorID: "9", Version: row.Version, Action: "approve"}
	row, err = repo.Act(ctx, row.ID, approve)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Act(ctx, row.ID, approve); err != nil {
		t.Fatal("idempotent action retry failed")
	}
	changed := approve
	changed.Outcome = SandboxSuccess
	if _, err = repo.Act(ctx, row.ID, changed); !errors.Is(err, ErrSandboxConflict) {
		t.Fatal("action key accepted changed payload")
	}
	row = act(row, "submit", SandboxFailed)
	if row.Status != SandboxFailed || row.Version != 4 || row.Attempt != 1 {
		t.Fatal("failed simulation did not record one attempt and two versions")
	}
	_, events, err := repo.Get(ctx, row.ID)
	if err != nil || len(events) != 4 {
		t.Fatalf("processing/result event count=%d err=%v", len(events), err)
	}
	if events[2].Stage != "processing" || events[2].ToStatus != "processing" || events[2].Outcome != "" || events[3].Stage != "result" || events[3].ToStatus != SandboxFailed {
		t.Fatal("processing and simulated receipt must be separate events")
	}
	row = act(row, "retry", SandboxUnknown)
	if row.Status != SandboxUnknown || row.Attempt != 2 || row.Version != 6 {
		t.Fatal("timeout simulation incorrect")
	}
	for _, action := range []string{"retry", "submit"} {
		if _, err = repo.Act(ctx, row.ID, PayoutSandboxAction{RequestID: uuid.NewString(), ActorID: "9", Version: row.Version, Action: action, Outcome: SandboxSuccess}); !errors.Is(err, ErrSandboxState) {
			t.Fatal("unknown result allowed duplicate simulated payment")
		}
	}
	row = act(row, "resolve", SandboxSuccess)
	if row.Status != SandboxSuccess || row.Attempt != 2 || row.Version != 7 {
		t.Fatal("query confirmation must not add a payment attempt")
	}
	if _, err = repo.Act(ctx, row.ID, PayoutSandboxAction{RequestID: uuid.NewString(), ActorID: "9", Version: 1, Action: "approve"}); !errors.Is(err, ErrSandboxConflict) {
		t.Fatal("stale version accepted")
	}
	failed := act(act(create(), "approve", ""), "submit", SandboxFailed)
	if next := act(failed, "retry", SandboxSuccess); next.Status != SandboxSuccess || next.Attempt != 2 {
		t.Fatal("confirmed failed attempt could not retry")
	}
	rejected := act(create(), "reject", "")
	if _, err = repo.Act(ctx, rejected.ID, PayoutSandboxAction{RequestID: uuid.NewString(), ActorID: "9", Version: rejected.Version, Action: "submit", Outcome: SandboxSuccess}); !errors.Is(err, ErrSandboxState) {
		t.Fatal("rejected instance could submit")
	}
	rollback := act(create(), "approve", "")
	failing := NewPayoutSandboxModel(sandboxEventFailureConn{conn})
	if _, err = failing.Act(ctx, rollback.ID, PayoutSandboxAction{RequestID: uuid.NewString(), ActorID: "9", Version: rollback.Version, Action: "submit", Outcome: SandboxSuccess}); err == nil {
		t.Fatal("simulated receipt event failure was swallowed")
	}
	afterRollback, rollbackEvents, err := repo.Get(ctx, rollback.ID)
	if err != nil || afterRollback.Status != SandboxApproved || afterRollback.Version != 2 || afterRollback.Attempt != 0 || len(rollbackEvents) != 2 {
		t.Fatal("event failure did not roll back state, attempt and processing event together")
	}

	// Concurrent creation is idempotent, then competing CAS commands have only
	// one winner and exactly one processing/result pair for the payment attempt.
	request := uuid.NewString()
	var wg sync.WaitGroup
	var createErrors atomic.Int32
	var concurrent *PayoutSandbox
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, e := repo.Create(ctx, request, 100, "9")
			if e != nil {
				createErrors.Add(1)
				return
			}
			idsLock.Lock()
			defer idsLock.Unlock()
			if concurrent == nil {
				concurrent = value
				ids = append(ids, value.ID)
			} else if concurrent.ID != value.ID {
				createErrors.Add(1)
			}
		}()
	}
	wg.Wait()
	if createErrors.Load() != 0 || concurrent == nil {
		t.Fatal("concurrent create lost idempotency")
	}
	compete := func(action, outcome string, version int64) {
		t.Helper()
		var wins, unexpected atomic.Int32
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, e := repo.Act(ctx, concurrent.ID, PayoutSandboxAction{RequestID: uuid.NewString(), ActorID: "9", Version: version, Action: action, Outcome: outcome})
				if e == nil {
					wins.Add(1)
				} else if !errors.Is(e, ErrSandboxConflict) {
					unexpected.Add(1)
				}
			}()
		}
		wg.Wait()
		if wins.Load() != 1 || unexpected.Load() != 0 {
			t.Fatalf("concurrent %s winners=%d unexpected=%d", action, wins.Load(), unexpected.Load())
		}
	}
	compete("approve", "", 1)
	compete("submit", SandboxSuccess, 2)
	final, events, err := repo.Get(ctx, concurrent.ID)
	if err != nil || final.Attempt != 1 || final.Version != 4 || len(events) != 4 {
		t.Fatal("competing commands duplicated attempts/events")
	}
	list, total, err := repo.List(ctx, "", 1, 1)
	if err != nil || total < 4 || len(list) != 1 {
		t.Fatal("real sandbox pagination failed")
	}
	_, total, err = repo.List(ctx, SandboxRejected, 1, 20)
	if err != nil || total != 1 {
		t.Fatal("real status filtering failed")
	}
}
