package model

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// This test never logs a DSN and only removes the rows it created.
func TestOperationLogMySQLIntegration(t *testing.T) {
	dsn := os.Getenv("ADMIN_OPERATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set ADMIN_OPERATION_TEST_DSN to a disposable askxuan_admin_test_* database")
	}
	// Reject ordinary/non-test DSNs before connecting, then verify the actual schema too.
	base := strings.SplitN(dsn, "?", 2)[0]
	i := strings.LastIndex(base, "/")
	if i < 0 || !strings.HasPrefix(base[i+1:], "askxuan_admin_test_") || len(base[i+1:]) <= len("askxuan_admin_test_") {
		t.Fatal("refusing non-test database DSN")
	}
	conn := sqlx.NewMysql(dsn)
	sqlx.DisableLog()
	ctx := context.Background()
	var database string
	if err := conn.QueryRowCtx(ctx, &database, "SELECT DATABASE()"); err != nil {
		t.Fatal("cannot connect to isolated test database")
	}
	if !strings.HasPrefix(database, "askxuan_admin_test_") || len(database) <= len("askxuan_admin_test_") {
		t.Fatal("refusing non-test database")
	}
	migration, err := os.ReadFile("../../../../../scripts/db/20261022_admin_operation_log.sql")
	if err != nil {
		t.Fatal(err)
	}
	ddl := strings.ReplaceAll(string(migration), "askxuan_audit.operation_log", "operation_log")
	if _, err := conn.ExecCtx(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	store := NewOperationStore(conn)
	actorID := 800000000 + time.Now().UnixNano()%100000000
	var ids []string
	newStart := func() OperationStart {
		t.Helper()
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			t.Fatal(err)
		}
		req := testOperationStart()
		req.RequestID = hex.EncodeToString(raw[:])
		req.ActorID = actorID
		ids = append(ids, req.RequestID)
		return req
	}
	t.Cleanup(func() {
		for _, id := range ids {
			if _, err := conn.ExecCtx(context.Background(), "DELETE FROM operation_log WHERE request_id=? AND actor_id=?", id, actorID); err != nil {
				t.Error("could not remove an isolated operation-log fixture")
			}
		}
	})
	req := newStart()
	const concurrency = 8
	var wg sync.WaitGroup
	startErrors := make(chan error, concurrency)
	for n := 0; n < concurrency; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			receipt, err := store.Start(ctx, req)
			if err == nil && (receipt.RequestID != req.RequestID || receipt.Result != "pending") {
				err = errors.New("wrong start receipt")
			}
			startErrors <- err
		}()
	}
	wg.Wait()
	close(startErrors)
	for err := range startErrors {
		if err != nil {
			t.Fatalf("concurrent identical start failed: %v", err)
		}
	}
	var count int64
	if err := conn.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM operation_log WHERE request_id=?", req.RequestID); err != nil || count != 1 {
		t.Fatal("duplicate starts created more than one row")
	}
	for _, mutate := range []func(*OperationStart){
		func(r *OperationStart) { r.ActorID++ }, func(r *OperationStart) { r.ActorRoles = []string{"auditor"} }, func(r *OperationStart) { r.Action = "admin.write" }, func(r *OperationStart) { r.Method = "POST"; r.RouteTemplate = "/api/v1/admin/*" }, func(r *OperationStart) { r.TargetType = "masters" }, func(r *OperationStart) { r.TargetID = "18" },
	} {
		bad := req
		mutate(&bad)
		_, err := store.Start(ctx, bad)
		var biz *common.BizError
		if !errors.As(err, &biz) || biz.Code != common.ErrStatusInvalid.Code {
			t.Fatal("conflicting start metadata was accepted")
		}
	}
	finishA := OperationFinish{RequestID: req.RequestID, Result: "success", HTTPStatus: 200, BusinessCode: 0, DurationMs: 17}
	finishB := OperationFinish{RequestID: req.RequestID, Result: "denied", HTTPStatus: 200, BusinessCode: 40301, DurationMs: 18}
	finishErrors := make(chan error, 2)
	for _, finish := range []OperationFinish{finishA, finishB} {
		wg.Add(1)
		go func(f OperationFinish) { defer wg.Done(); _, err := store.Finish(ctx, f); finishErrors <- err }(finish)
	}
	wg.Wait()
	close(finishErrors)
	wins, conflicts := 0, 0
	for err := range finishErrors {
		if err == nil {
			wins++
			continue
		}
		var biz *common.BizError
		if errors.As(err, &biz) && biz.Code == common.ErrStatusInvalid.Code {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("concurrent finalization did not keep a single immutable terminal result")
	}
	list, err := store.List(ctx, OperationFilter{ActorID: actorID, Page: 1, Size: 20})
	if err != nil || len(list.List) != 1 || list.Total != 1 {
		t.Fatalf("cannot read isolated operation: %v", err)
	}
	row := list.List[0]
	if row.StartedAt == "" || row.FinishedAt == "" || row.ActorID != actorID || len(row.ActorRoles) != 1 || row.RouteTemplate != req.RouteTemplate {
		t.Fatal("stored metadata/timestamps changed")
	}
	if _, err := time.Parse(time.RFC3339Nano, row.StartedAt); err != nil {
		t.Fatal("invalid UTC start timestamp")
	}
	winner := finishA
	if row.Result == "denied" {
		winner = finishB
	}
	for n := 0; n < 3; n++ {
		if _, err := store.Finish(ctx, winner); err != nil {
			t.Fatal("identical finish is not idempotent")
		}
	}
	winner.DurationMs++
	if _, err := store.Finish(ctx, winner); err == nil {
		t.Fatal("terminal duration was overwritten")
	}
	if receipt, err := store.Start(ctx, req); err != nil || receipt.Result != row.Result {
		t.Fatal("late duplicate start reset final state")
	}
	missing := finishA
	missing.RequestID = newStart().RequestID
	if _, err := store.Finish(ctx, missing); err == nil {
		t.Fatal("finalization without start was accepted")
	}
	for n := 0; n < 4; n++ {
		next := newStart()
		next.Action = "admin.write"
		next.TargetType = "masters"
		next.RouteTemplate = "/api/v1/admin/*"
		if _, err := store.Start(ctx, next); err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			if _, err := store.Finish(ctx, OperationFinish{RequestID: next.RequestID, Result: "unknown", HTTPStatus: 200, BusinessCode: -1, DurationMs: 2}); err != nil {
				t.Fatal(err)
			}
		}
	}
	filter := OperationFilter{ActorID: actorID, Action: "admin.write", TargetType: "masters", Result: "pending", Method: "PUT", Page: 1, Size: 2, StartTime: time.Now().Add(-time.Hour), EndTime: time.Now().Add(time.Hour)}
	first, err := store.List(ctx, filter)
	if err != nil || first.Total != 3 || len(first.List) != 2 {
		t.Fatalf("filtered pagination incorrect: %v total=%d count=%d", err, first.Total, len(first.List))
	}
	filter.Page = 2
	second, err := store.List(ctx, filter)
	if err != nil || second.Total != 3 || len(second.List) != 1 {
		t.Fatal("second page incorrect")
	}
	for _, a := range first.List {
		if a.RequestID == second.List[0].RequestID {
			t.Fatal("pagination duplicated a row")
		}
		if a.FinishedAt != "" || a.Result != "pending" {
			t.Fatal("unfinished request falsely marked complete")
		}
	}
	filter.Page = 3
	empty, err := store.List(ctx, filter)
	if err != nil || empty.Total != 3 || len(empty.List) != 0 || empty.List == nil {
		t.Fatal("empty page must have stable total and [] list")
	}
	filter.Page = 1
	filter.Result = "unknown"
	unknown, err := store.List(ctx, filter)
	if err != nil || unknown.Total != 1 || unknown.List[0].BusinessCode != -1 {
		t.Fatal("unknown upstream result lost sentinel")
	}
}
