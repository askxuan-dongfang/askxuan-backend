// Package reportcleanup is an operator-only, snapshot-guarded retirement tool.
// It never deletes points ledgers or model usage/cost records.
package reportcleanup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type Expected struct {
	Reports  int   `json:"reports"`
	Ledgers  int   `json:"ledgers"`
	Points   int64 `json:"points"`
	Sessions int   `json:"sessions"`
	Messages int   `json:"messages"`
}
type Table struct {
	Columns []string    `json:"columns"`
	Rows    [][]*string `json:"rows"`
}
type Backup struct {
	Version           int              `json:"version"`
	Batch             string           `json:"batch"`
	Tables            map[string]Table `json:"tables"`
	AccountLastLedger map[string]int64 `json:"accountLastLedger"`
}
type Manifest struct {
	Version            int      `json:"version"`
	Batch              string   `json:"batch"`
	SHA256             string   `json:"sha256"`
	Expected           Expected `json:"expected"`
	ReportIDs          []string `json:"reportIds"`
	LedgerIDs          []string `json:"ledgerIds"`
	SessionIDs         []string `json:"sessionIds"`
	PreservedUsageRows int      `json:"preservedUsageRows"`
}
type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

var batchPattern = regexp.MustCompile(`^[a-zA-Z0-9-]{8,48}$`)
var ErrGuard = errors.New("retirement snapshot/ownership/payment guard failed; generate a new reviewed plan")

const reportTable = "askxuan_ai.ai_report"
const ledgerTable = "askxuan_payment.points_ledger"
const accountTable = "askxuan_payment.points_account"
const receiptTable = "askxuan_payment.ai_report_points_retirement"

var backupTables = []string{reportTable, ledgerTable, accountTable, "askxuan_ai.ai_session", "askxuan_ai.ai_message", "askxuan_ai.ai_run", "askxuan_ai.ai_tool_call", "askxuan_ai.ai_usage_log"}

func read(ctx context.Context, q queryer, table, where, order string, args []any, lock bool) (Table, error) {
	var t Table
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	rows, e := q.QueryContext(ctx, "SELECT * FROM "+table+" WHERE "+where+" ORDER BY "+order+suffix, args...)
	if e != nil {
		return t, e
	}
	defer rows.Close()
	t.Columns, e = rows.Columns()
	if e != nil {
		return t, e
	}
	for rows.Next() {
		raw := make([]sql.NullString, len(t.Columns))
		dest := make([]any, len(raw))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if e = rows.Scan(dest...); e != nil {
			return t, e
		}
		row := make([]*string, len(raw))
		for i := range raw {
			if raw[i].Valid {
				v := raw[i].String
				row[i] = &v
			}
		}
		t.Rows = append(t.Rows, row)
	}
	return t, rows.Err()
}
func value(t Table, row []*string, col string) string {
	for i, c := range t.Columns {
		if c == col && row[i] != nil {
			return *row[i]
		}
	}
	return ""
}
func ids(t Table, col string) []string {
	out := []string{}
	for _, r := range t.Rows {
		out = append(out, value(t, r, col))
	}
	return out
}
func in(column string, values []string) (string, []any) {
	if len(values) == 0 {
		return "1=0", nil
	}
	p := make([]string, len(values))
	args := make([]any, len(values))
	for i, v := range values {
		p[i] = "?"
		args[i] = v
	}
	return column + " IN (" + strings.Join(p, ",") + ")", args
}
func integer(v string) int64 { n, _ := strconv.ParseInt(v, 10, 64); return n }
func fingerprint(b Backup) (string, error) {
	raw, e := json.Marshal(b)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), e
}

func snapshot(ctx context.Context, q queryer, batch string, lock bool) (Backup, Expected, error) {
	b := Backup{Version: 1, Batch: batch, Tables: map[string]Table{}, AccountLastLedger: map[string]int64{}}
	var counts Expected
	if !batchPattern.MatchString(batch) {
		return b, counts, ErrGuard
	}
	var e error
	b.Tables[reportTable], e = read(ctx, q, reportTable, "points_paid=1", "id", nil, lock)
	if e != nil {
		return b, counts, e
	}
	b.Tables[ledgerTable], e = read(ctx, q, ledgerTable, "event_key LIKE 'ai-report:%'", "id", nil, lock)
	if e != nil {
		return b, counts, e
	}
	reports, ledgers := b.Tables[reportTable], b.Tables[ledgerTable]
	counts.Reports = len(reports.Rows)
	counts.Ledgers = len(ledgers.Rows)
	ledgerByNo := map[string]string{}
	userSet := map[string]bool{}
	refs := []string{}
	for _, r := range ledgers.Rows {
		no, user := value(ledgers, r, "reference_no"), value(ledgers, r, "user_id")
		delta := integer(value(ledgers, r, "delta"))
		if no == "" || user == "" || value(ledgers, r, "event_key") != "ai-report:"+no || value(ledgers, r, "kind") != "redeem" || delta >= 0 || ledgerByNo[no] != "" {
			return b, counts, ErrGuard
		}
		// A retained original debit must never be refunded again under a new batch.
		// Even a rolled-back retirement requires explicit reconciliation, not reuse.
		var retired int
		if e = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+ledgerTable+" WHERE kind='ai_report_retired_refund' AND user_id=? AND reference_no=?", user, no).Scan(&retired); e != nil {
			return b, counts, e
		}
		if retired != 0 {
			return b, counts, ErrGuard
		}
		ledgerByNo[no] = user
		userSet[user] = true
		refs = append(refs, no)
		counts.Points -= delta
	}
	for _, table := range []string{"askxuan_payment.payment", "askxuan_payment.growth_quote", "askxuan_payment.growth_transaction"} {
		where, args := in("order_no", refs)
		var n int
		if e = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE order_type='ai_report' AND "+where, args...).Scan(&n); e != nil {
			return b, counts, e
		}
		if n > 0 {
			return b, counts, ErrGuard
		}
	}
	// Reject inconsistent surviving records; orphaned old ledger rows are refundable.
	where, args := in("report_no", refs)
	var inconsistent int
	if e = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+reportTable+" WHERE points_paid<>1 AND "+where, args...).Scan(&inconsistent); e != nil {
		return b, counts, e
	}
	if inconsistent > 0 {
		return b, counts, ErrGuard
	}
	sessionOwners := map[string]string{}
	sessionTitles := map[string]string{}
	for _, r := range reports.Rows {
		no, user := value(reports, r, "report_no"), value(reports, r, "user_id")
		if ledgerByNo[no] != user || value(reports, r, "status") != "ready" {
			return b, counts, ErrGuard
		}
		sid := value(reports, r, "chat_session_id")
		if sid != "0" && sid != "" {
			if sessionOwners[sid] != "" {
				return b, counts, ErrGuard
			}
			sessionOwners[sid] = user
			sessionTitles[sid] = "报告追问 · " + value(reports, r, "title")
		}
	}
	sessionIDs := []string{}
	for sid := range sessionOwners {
		sessionIDs = append(sessionIDs, sid)
	}
	where, args = in("id", sessionIDs)
	b.Tables["askxuan_ai.ai_session"], e = read(ctx, q, "askxuan_ai.ai_session", where, "id", args, lock)
	if e != nil {
		return b, counts, e
	}
	sessions := b.Tables["askxuan_ai.ai_session"]
	if len(sessions.Rows) != len(sessionIDs) {
		return b, counts, ErrGuard
	}
	for _, r := range sessions.Rows {
		sid := value(sessions, r, "id")
		if value(sessions, r, "user_id") != sessionOwners[sid] || value(sessions, r, "title") != sessionTitles[sid] {
			return b, counts, ErrGuard
		}
		var n int
		if e = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+reportTable+" WHERE chat_session_id=?", sid).Scan(&n); e != nil {
			return b, counts, e
		}
		if n != 1 {
			return b, counts, ErrGuard
		}
	}
	for _, table := range []string{"askxuan_ai.ai_message", "askxuan_ai.ai_run", "askxuan_ai.ai_usage_log"} {
		where, args = in("session_id", sessionIDs)
		b.Tables[table], e = read(ctx, q, table, where, "id", args, lock)
		if e != nil {
			return b, counts, e
		}
	}
	for _, table := range []string{"askxuan_ai.ai_message", "askxuan_ai.ai_run"} {
		t := b.Tables[table]
		for _, r := range t.Rows {
			state := value(t, r, "status")
			if state == "pending" || state == "running" || state == "streaming" || state == "generating" || state == "queued" {
				return b, counts, ErrGuard
			}
		}
	}
	runs := b.Tables["askxuan_ai.ai_run"]
	messages := b.Tables["askxuan_ai.ai_message"]
	messageIDs := map[string]bool{}
	for _, id := range ids(messages, "id") {
		messageIDs[id] = true
	}
	for _, r := range runs.Rows {
		if !messageIDs[value(runs, r, "message_id")] || value(runs, r, "user_id") != sessionOwners[value(runs, r, "session_id")] {
			return b, counts, ErrGuard
		}
	}
	where, args = in("run_id", ids(runs, "id"))
	b.Tables["askxuan_ai.ai_tool_call"], e = read(ctx, q, "askxuan_ai.ai_tool_call", where, "id", args, lock)
	if e != nil {
		return b, counts, e
	}
	users := []string{}
	for u := range userSet {
		users = append(users, u)
	}
	where, args = in("user_id", users)
	b.Tables[accountTable], e = read(ctx, q, accountTable, where, "user_id", args, lock)
	if e != nil {
		return b, counts, e
	}
	if len(b.Tables[accountTable].Rows) != len(users) {
		return b, counts, ErrGuard
	}
	for _, u := range users {
		var id int64
		if e = q.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM "+ledgerTable+" WHERE user_id=?", u).Scan(&id); e != nil {
			return b, counts, e
		}
		b.AccountLastLedger[u] = id
	}
	counts.Sessions = len(sessions.Rows)
	counts.Messages = len(messages.Rows)
	return b, counts, nil
}
func Plan(ctx context.Context, db *sql.DB, batch string, want Expected) (Backup, Manifest, error) {
	var m Manifest
	tx, e := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return Backup{}, m, e
	}
	defer tx.Rollback()
	b, got, e := snapshot(ctx, tx, batch, false)
	if e != nil {
		return b, m, e
	}
	if got != want {
		return b, m, fmt.Errorf("%w: expected %+v got %+v", ErrGuard, want, got)
	}
	hash, e := fingerprint(b)
	m = Manifest{Version: 1, Batch: batch, SHA256: hash, Expected: got, ReportIDs: ids(b.Tables[reportTable], "id"), LedgerIDs: ids(b.Tables[ledgerTable], "id"), SessionIDs: ids(b.Tables["askxuan_ai.ai_session"], "id"), PreservedUsageRows: len(b.Tables["askxuan_ai.ai_usage_log"].Rows)}
	return b, m, e
}
func verify(b Backup, m Manifest) error {
	hash, e := fingerprint(b)
	if e != nil {
		return e
	}
	if b.Version != 1 || m.Version != 1 || b.Batch != m.Batch || hash != m.SHA256 || !batchPattern.MatchString(b.Batch) {
		return ErrGuard
	}
	return nil
}
func receipt(ctx context.Context, tx *sql.Tx, m Manifest) (string, error) {
	var state, hash string
	e := tx.QueryRowContext(ctx, "SELECT status,manifest_sha256 FROM "+receiptTable+" WHERE batch_id=? FOR UPDATE", m.Batch).Scan(&state, &hash)
	if e == sql.ErrNoRows {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	if hash != m.SHA256 {
		return "", ErrGuard
	}
	return state, nil
}
func credit(ctx context.Context, tx *sql.Tx, b Backup, rollback bool) error {
	t := b.Tables[ledgerTable]
	for _, r := range t.Rows {
		user, id := value(t, r, "user_id"), value(t, r, "id")
		delta := -integer(value(t, r, "delta"))
		kind, prefix := "ai_report_retired_refund", "ai-report-retire:"
		if rollback {
			delta = -delta
			kind = "ai_report_retired_rollback"
			prefix = "ai-report-retire-rollback:"
		}
		var before int64
		if e := tx.QueryRowContext(ctx, "SELECT balance FROM "+accountTable+" WHERE user_id=? FOR UPDATE", user).Scan(&before); e != nil {
			return e
		}
		if rollback && before+delta < 0 {
			return ErrGuard
		}
		if _, e := tx.ExecContext(ctx, "UPDATE "+accountTable+" SET balance=balance+? WHERE user_id=?", delta, user); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO "+ledgerTable+"(user_id,event_key,kind,delta,balance_after,reference_no) VALUES(?,?,?,?,?,?)", user, prefix+b.Batch+":"+id, kind, delta, before+delta, value(t, r, "reference_no")); e != nil {
			return e
		}
	}
	return nil
}

func lockedTransaction(ctx context.Context, db *sql.DB) (*sql.Tx, func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	var locked int
	if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK('askxuan:ai-report-points-retirement',5)").Scan(&locked); err != nil || locked != 1 {
		conn.Close()
		return nil, nil, ErrGuard
	}
	release := func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('askxuan:ai-report-points-retirement')")
		_ = conn.Close()
	}
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		release()
		return nil, nil, err
	}
	return tx, release, nil
}
func Apply(ctx context.Context, db *sql.DB, b Backup, m Manifest) error {
	if e := verify(b, m); e != nil {
		return e
	}
	tx, release, e := lockedTransaction(ctx, db)
	if e != nil {
		return e
	}
	defer release()
	defer tx.Rollback()
	state, e := receipt(ctx, tx, m)
	if e != nil {
		return e
	}
	if state == "applied" {
		return nil
	}
	if state != "" {
		return ErrGuard
	}
	now, counts, e := snapshot(ctx, tx, b.Batch, true)
	if e != nil {
		return e
	}
	hash, e := fingerprint(now)
	if e != nil {
		return e
	}
	if hash != m.SHA256 || counts != m.Expected {
		return ErrGuard
	}
	if e = credit(ctx, tx, b, false); e != nil {
		return e
	}
	for _, table := range []string{"askxuan_ai.ai_tool_call", "askxuan_ai.ai_run", "askxuan_ai.ai_message", "askxuan_ai.ai_session", reportTable} {
		where, args := in("id", ids(b.Tables[table], "id"))
		res, e := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE "+where, args...)
		if e != nil {
			return e
		}
		n, e := res.RowsAffected()
		if e != nil || n != int64(len(b.Tables[table].Rows)) {
			return ErrGuard
		}
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO "+receiptTable+"(batch_id,manifest_sha256,status,reports,ledger_entries,points_refunded) VALUES(?,?,'applied',?,?,?)", m.Batch, m.SHA256, m.Expected.Reports, m.Expected.Ledgers, m.Expected.Points)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func Rollback(ctx context.Context, db *sql.DB, b Backup, m Manifest) error {
	if e := verify(b, m); e != nil {
		return e
	}
	tx, release, e := lockedTransaction(ctx, db)
	if e != nil {
		return e
	}
	defer release()
	defer tx.Rollback()
	state, e := receipt(ctx, tx, m)
	if e != nil {
		return e
	}
	if state == "rolled_back" {
		return nil
	}
	if state != "applied" {
		return ErrGuard
	}
	where, args := in("id", ids(b.Tables[ledgerTable], "id"))
	original, e := read(ctx, tx, ledgerTable, where, "id", args, true)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(original, b.Tables[ledgerTable]) {
		return ErrGuard
	}
	refunds := map[string]int64{}
	for _, row := range b.Tables[ledgerTable].Rows {
		user, id, delta := value(b.Tables[ledgerTable], row, "user_id"), value(b.Tables[ledgerTable], row, "id"), -integer(value(b.Tables[ledgerTable], row, "delta"))
		var n int
		if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+ledgerTable+" WHERE event_key=? AND user_id=? AND delta=? AND kind='ai_report_retired_refund'", "ai-report-retire:"+b.Batch+":"+id, user, delta).Scan(&n); e != nil {
			return e
		}
		if n != 1 {
			return ErrGuard
		}
		refunds[user] += delta
	}
	for _, row := range b.Tables[accountTable].Rows {
		user := value(b.Tables[accountTable], row, "user_id")
		var current int64
		if e = tx.QueryRowContext(ctx, "SELECT balance FROM "+accountTable+" WHERE user_id=? FOR UPDATE", user).Scan(&current); e != nil {
			return e
		}
		if current != integer(value(b.Tables[accountTable], row, "balance"))+refunds[user] {
			return ErrGuard
		}
	}
	for user, last := range b.AccountLastLedger {
		var n int
		if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+ledgerTable+" WHERE user_id=? AND id>? AND event_key NOT LIKE ?", user, last, "ai-report-retire:"+b.Batch+":%").Scan(&n); e != nil {
			return e
		}
		if n > 0 {
			return ErrGuard
		}
	}
	if e = credit(ctx, tx, b, true); e != nil {
		return e
	}
	safeColumn := regexp.MustCompile(`^[a-z_]+$`)
	for _, table := range []string{"askxuan_ai.ai_session", "askxuan_ai.ai_message", "askxuan_ai.ai_run", "askxuan_ai.ai_tool_call", reportTable} {
		t := b.Tables[table]
		cols := make([]string, len(t.Columns))
		marks := make([]string, len(t.Columns))
		for i, c := range t.Columns {
			if !safeColumn.MatchString(c) {
				return ErrGuard
			}
			cols[i] = "`" + c + "`"
			marks[i] = "?"
		}
		for _, row := range t.Rows {
			args := make([]any, len(row))
			for i, v := range row {
				if v != nil {
					args[i] = *v
				}
			}
			if _, e = tx.ExecContext(ctx, "INSERT INTO "+table+" ("+strings.Join(cols, ",")+") VALUES("+strings.Join(marks, ",")+")", args...); e != nil {
				return e
			}
		}
	}
	if _, e = tx.ExecContext(ctx, "UPDATE "+receiptTable+" SET status='rolled_back',updated_at=CURRENT_TIMESTAMP WHERE batch_id=?", m.Batch); e != nil {
		return e
	}
	return tx.Commit()
}
