package reportcleanup

import (
	"context"
	"database/sql"
	"errors"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"sync"
	"testing"
)

func TestRetirementMySQL(t *testing.T) {
	dsn := os.Getenv("AI_REPORT_CLEANUP_TEST_DSN")
	if dsn == "" || os.Getenv("AI_REPORT_CLEANUP_TEST_ISOLATED") != "1" {
		t.Skip("isolated cleanup database not configured")
	}
	db, e := sql.Open("mysql", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(8)
	ctx := context.Background()
	exec := func(q string, args ...any) sql.Result {
		t.Helper()
		r, e := db.ExecContext(ctx, q, args...)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	count := func(q string, args ...any) int64 {
		t.Helper()
		var n int64
		if e := db.QueryRowContext(ctx, q, args...).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}
	sid, _ := exec(`INSERT INTO askxuan_ai.ai_session(session_no,user_id,skill_code,selection_mode,skill_version,title,status) VALUES('cleanup-session','cleanup-owner','general','explicit','1.0','报告追问 · cleanup fixture','active')`).LastInsertId()
	mid, _ := exec(`INSERT INTO askxuan_ai.ai_message(session_id,role,content,input_json,attachments_json,status) VALUES(?,'assistant','private fixture','{}','[]','completed')`, sid).LastInsertId()
	exec(`INSERT INTO askxuan_ai.ai_message(session_id,role,content,input_json,attachments_json,status) VALUES(?,'user','private question','{}','[]','completed')`, sid)
	rid, _ := exec(`INSERT INTO askxuan_ai.ai_run(run_no,session_id,message_id,user_id,skill_code,status) VALUES('cleanup-run',?,?,'cleanup-owner','general','completed')`, sid, mid).LastInsertId()
	exec(`INSERT INTO askxuan_ai.ai_tool_call(run_id,server_code,tool_name,arguments_summary,result_summary,status) VALUES(?,'fixture','fixture','{}','private tool result','completed')`, rid)
	exec(`INSERT INTO askxuan_ai.ai_usage_log(user_id,session_id,message_id,skill_code,provider,model,prompt_tokens,completion_tokens,total_tokens,cost_micros,status,latency_ms,error_message) VALUES('cleanup-owner',?,?,'general','fixture','fixture',10,10,20,100,'success',1,'')`, sid, mid)
	exec(`INSERT INTO askxuan_ai.ai_report(report_no,user_id,request_key,skill_code,title,version,question,inputs_json,chapters_json,price_cents,points_price,status,summary,content,points_paid,chat_session_id) VALUES('ARcleanup-kept','cleanup-owner','cleanup-report','dream','cleanup fixture','1','private','{}','[]',990,10,'ready','private summary','private report',1,?)`, sid)
	exec(`INSERT INTO askxuan_payment.points_account(user_id,balance) VALUES('cleanup-owner',80)`)
	for _, no := range []string{"ARcleanup-kept", "ARcleanup-orphan"} {
		exec(`INSERT INTO askxuan_payment.points_ledger(user_id,event_key,kind,delta,balance_after,reference_no) VALUES('cleanup-owner',?,'redeem',-10,80,?)`, "ai-report:"+no, no)
	}
	want := Expected{Reports: 1, Ledgers: 2, Points: 20, Sessions: 1, Messages: 2}
	wrong := want
	wrong.Points = 21
	if _, _, e = Plan(ctx, db, "fixture-retire", wrong); !errors.Is(e, ErrGuard) {
		t.Fatal("expanded amount not blocked", e)
	}
	exec(`UPDATE askxuan_ai.ai_session SET title='ordinary conversation' WHERE id=?`, sid)
	if _, _, e = Plan(ctx, db, "fixture-retire", want); !errors.Is(e, ErrGuard) {
		t.Fatal("ordinary session accepted", e)
	}
	exec(`UPDATE askxuan_ai.ai_session SET title='报告追问 · cleanup fixture' WHERE id=?`, sid)
	exec(`INSERT INTO askxuan_payment.payment(payment_no,idempotency_key,user_id,order_type,order_no,amount,channel,status,trade_no) VALUES('cleanup-protected-payment','cleanup-protected-payment','cleanup-owner','ai_report','ARcleanup-kept',9.9,'demo_balance','success','fixture')`)
	if _, _, e = Plan(ctx, db, "fixture-retire", want); !errors.Is(e, ErrGuard) {
		t.Fatal("financially referenced report accepted", e)
	}
	exec(`DELETE FROM askxuan_payment.payment WHERE payment_no='cleanup-protected-payment'`)
	b, m, e := Plan(ctx, db, "fixture-retire", want)
	if e != nil {
		t.Fatal(e)
	}
	exec(`UPDATE askxuan_payment.points_account SET balance=81 WHERE user_id='cleanup-owner'`)
	if e = Apply(ctx, db, b, m); !errors.Is(e, ErrGuard) {
		t.Fatal("stale account snapshot accepted", e)
	}
	exec(`UPDATE askxuan_payment.points_account SET balance=80 WHERE user_id='cleanup-owner'`)
	b, m, e = Plan(ctx, db, "fixture-retire", want)
	if e != nil {
		t.Fatal(e)
	}
	exec(`CREATE TRIGGER askxuan_payment.fail_retirement BEFORE INSERT ON askxuan_payment.points_ledger FOR EACH ROW BEGIN IF NEW.kind='ai_report_retired_refund' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fixture rollback'; END IF; END`)
	if e = Apply(ctx, db, b, m); e == nil {
		t.Fatal("injected failure passed")
	}
	if count(`SELECT balance FROM askxuan_payment.points_account WHERE user_id='cleanup-owner'`) != 80 || count(`SELECT COUNT(*) FROM askxuan_ai.ai_report WHERE report_no='ARcleanup-kept'`) != 1 {
		t.Fatal("partial cleanup committed")
	}
	exec(`DROP TRIGGER askxuan_payment.fail_retirement`)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- Apply(ctx, db, b, m) }()
	}
	wg.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal(e)
		}
	}
	if count(`SELECT balance FROM askxuan_payment.points_account WHERE user_id='cleanup-owner'`) != 100 || count(`SELECT COUNT(*) FROM askxuan_payment.points_ledger WHERE kind='ai_report_retired_refund'`) != 2 {
		t.Fatal("duplicate or missing refund")
	}
	if _, _, e = Plan(ctx, db, "fixture-different-batch", Expected{Ledgers: 2, Points: 20}); !errors.Is(e, ErrGuard) {
		t.Fatal("cross-batch double refund accepted", e)
	}
	for _, table := range []string{reportTable, "askxuan_ai.ai_session", "askxuan_ai.ai_message", "askxuan_ai.ai_run", "askxuan_ai.ai_tool_call"} {
		if count("SELECT COUNT(*) FROM "+table+" WHERE id=?", integer(ids(b.Tables[table], "id")[0])) != 0 {
			t.Fatal("content not deleted", table)
		}
	}
	if count(`SELECT COUNT(*) FROM askxuan_ai.ai_usage_log WHERE message_id=?`, mid) != 1 {
		t.Fatal("usage billing lost")
	}
	exec(`INSERT INTO askxuan_payment.points_ledger(user_id,event_key,kind,delta,balance_after,reference_no) VALUES('cleanup-owner','fixture-later','earn',1,101,'fixture')`)
	exec(`UPDATE askxuan_payment.points_account SET balance=101 WHERE user_id='cleanup-owner'`)
	if e = Rollback(ctx, db, b, m); !errors.Is(e, ErrGuard) {
		t.Fatal("rollback crossed later transaction", e)
	}
	exec(`DELETE FROM askxuan_payment.points_ledger WHERE event_key='fixture-later'`)
	exec(`UPDATE askxuan_payment.points_account SET balance=100 WHERE user_id='cleanup-owner'`)
	if e = Rollback(ctx, db, b, m); e != nil {
		t.Fatal(e)
	}
	if e = Rollback(ctx, db, b, m); e != nil {
		t.Fatal("rollback retry", e)
	}
	if count(`SELECT balance FROM askxuan_payment.points_account WHERE user_id='cleanup-owner'`) != 80 || count(`SELECT COUNT(*) FROM askxuan_ai.ai_report WHERE report_no='ARcleanup-kept'`) != 1 || count(`SELECT COUNT(*) FROM askxuan_payment.points_ledger WHERE user_id='cleanup-owner'`) != 6 {
		t.Fatal("rollback did not preserve/restored audited history")
	}
	if e = Apply(ctx, db, b, m); !errors.Is(e, ErrGuard) {
		t.Fatal("retired batch reopened", e)
	}
}
