package model

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Operation logs deliberately contain no request body, query, IP or contact data.
type OperationStart struct {
	RequestID     string   `json:"requestId"`
	ActorID       int64    `json:"actorId"`
	ActorRoles    []string `json:"actorRoles"`
	Action        string   `json:"action"`
	Method        string   `json:"method"`
	RouteTemplate string   `json:"routeTemplate"`
	TargetType    string   `json:"targetType"`
	TargetID      string   `json:"targetId"`
}

type OperationFinish struct {
	RequestID    string `json:"requestId"`
	Result       string `json:"result"`
	HTTPStatus   int    `json:"httpStatus"`
	BusinessCode int    `json:"businessCode"`
	DurationMs   int64  `json:"durationMs"`
}

type OperationReceipt struct {
	RequestID string `json:"requestId"`
	Result    string `json:"result"`
}

type OperationLog struct {
	OperationStart
	Result       string `json:"result"`
	HTTPStatus   int    `json:"httpStatus"`
	BusinessCode int    `json:"businessCode"`
	DurationMs   int64  `json:"durationMs"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
}

type OperationFilter struct {
	ActorID    int64
	Action     string
	Method     string
	TargetType string
	Result     string
	StartTime  time.Time
	EndTime    time.Time
	Page       int
	Size       int
}

type OperationList struct {
	List  []OperationLog `json:"list"`
	Total int64          `json:"total"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
}

type OperationStore interface {
	Start(context.Context, OperationStart) (OperationReceipt, error)
	Finish(context.Context, OperationFinish) (OperationReceipt, error)
	List(context.Context, OperationFilter) (OperationList, error)
}

type SQLOperationStore struct{ conn sqlx.SqlConn }

func NewOperationStore(conn sqlx.SqlConn) *SQLOperationStore {
	return &SQLOperationStore{conn: conn}
}

var (
	operationRequestID = regexp.MustCompile(`^[a-f0-9]{32}$`)
	operationCode      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{0,63}$`)
	operationTargetID  = regexp.MustCompile(`^[A-Za-z0-9_.:-]{0,128}$`)
	operationConflict  = common.NewBizError(common.ErrStatusInvalid.Code, "操作日志请求标识或终态冲突")
)

func validOperationMethod(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}

func validOperationAction(action string) bool {
	for _, capability := range adminpolicy.Catalog {
		if action == capability.Code {
			return true
		}
	}
	switch action {
	case "admin.write", "ai.admin.write", "growth.admin.write", "onboarding.admin.write":
		return true
	}
	return false
}

func validOperationRoute(method, route string) bool {
	if !validOperationMethod(method) {
		return false
	}
	if known, ok := adminpolicy.Match(method, route); ok && known.Template == route {
		return true
	}
	switch route {
	case "/api/v1/admin/*", "/api/v1/ai/admin/*", "/api/v1/payments/growth/admin/*", "/api/v1/auth/onboarding/*":
		return true
	}
	return false
}

func (r OperationStart) Validate() error {
	if !operationRequestID.MatchString(r.RequestID) || r.ActorID <= 0 || len(r.ActorRoles) == 0 || len(r.ActorRoles) > 32 || !validOperationAction(r.Action) || !operationCode.MatchString(r.TargetType) || !operationTargetID.MatchString(r.TargetID) || !validOperationRoute(r.Method, r.RouteTemplate) {
		return common.ErrParam
	}
	seen := make(map[string]bool, len(r.ActorRoles))
	for _, role := range r.ActorRoles {
		if !operationCode.MatchString(role) || seen[role] {
			return common.ErrParam
		}
		seen[role] = true
	}
	return nil
}

func terminalOperationResult(result string) bool {
	switch result {
	case "success", "denied", "failed", "unknown":
		return true
	}
	return false
}

func (r OperationFinish) Validate() error {
	unknownCode := r.BusinessCode == -1 && (r.Result == "unknown" || r.Result == "failed")
	if !operationRequestID.MatchString(r.RequestID) || !terminalOperationResult(r.Result) || r.HTTPStatus < 100 || r.HTTPStatus > 599 || (r.BusinessCode < 0 && !unknownCode) || r.BusinessCode > 999999999 || r.DurationMs < 0 || r.DurationMs > 86400000 {
		return common.ErrParam
	}
	return nil
}

func (f OperationFilter) Validate() error {
	if f.ActorID < 0 || f.Page < 1 || f.Page > 1000000 || f.Size < 1 || f.Size > 100 || (f.Action != "" && !operationCode.MatchString(f.Action)) || (f.TargetType != "" && !operationCode.MatchString(f.TargetType)) || (f.Method != "" && !validOperationMethod(f.Method)) || (f.Result != "" && f.Result != "pending" && !terminalOperationResult(f.Result)) || (!f.StartTime.IsZero() && !f.EndTime.IsZero() && f.StartTime.After(f.EndTime)) {
		return common.ErrParam
	}
	return nil
}

type operationRow struct {
	RequestID     string `db:"request_id"`
	ActorID       int64  `db:"actor_id"`
	ActorRoles    string `db:"actor_roles"`
	Action        string `db:"action"`
	Method        string `db:"method"`
	RouteTemplate string `db:"route_template"`
	TargetType    string `db:"target_type"`
	TargetID      string `db:"target_id"`
	Result        string `db:"result"`
	HTTPStatus    int    `db:"http_status"`
	BusinessCode  int    `db:"business_code"`
	DurationMs    int64  `db:"duration_ms"`
	StartedAt     string `db:"started_at"`
	FinishedAt    string `db:"finished_at"`
}

const operationColumns = `request_id,actor_id,actor_roles,action,method,route_template,target_type,target_id,result,http_status,business_code,duration_ms,DATE_FORMAT(started_at,'%Y-%m-%dT%H:%i:%s.%fZ') started_at,COALESCE(DATE_FORMAT(finished_at,'%Y-%m-%dT%H:%i:%s.%fZ'),'') finished_at`

func (row operationRow) matchesStart(r OperationStart, roles string) bool {
	return row.RequestID == r.RequestID && row.ActorID == r.ActorID && row.ActorRoles == roles && row.Action == r.Action && row.Method == r.Method && row.RouteTemplate == r.RouteTemplate && row.TargetType == r.TargetType && row.TargetID == r.TargetID
}

func (row operationRow) matchesFinish(r OperationFinish) bool {
	return row.Result == r.Result && row.HTTPStatus == r.HTTPStatus && row.BusinessCode == r.BusinessCode && row.DurationMs == r.DurationMs
}

func (s *SQLOperationStore) Start(ctx context.Context, req OperationStart) (OperationReceipt, error) {
	if err := req.Validate(); err != nil {
		return OperationReceipt{}, err
	}
	roles, _ := json.Marshal(req.ActorRoles)
	var receipt OperationReceipt
	err := s.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		// A duplicate never updates stored metadata. The unique key serializes starts.
		_, err := tx.ExecCtx(ctx, `INSERT INTO operation_log(request_id,actor_id,actor_roles,action,method,route_template,target_type,target_id,result,started_at) VALUES(?,?,?,?,?,?,?,?,'pending',UTC_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE request_id=request_id`, req.RequestID, req.ActorID, string(roles), req.Action, req.Method, req.RouteTemplate, req.TargetType, req.TargetID)
		if err != nil {
			return err
		}
		var row operationRow
		if err := tx.QueryRowCtx(ctx, &row, `SELECT `+operationColumns+` FROM operation_log WHERE request_id=? FOR UPDATE`, req.RequestID); err != nil {
			return err
		}
		if !row.matchesStart(req, string(roles)) {
			return operationConflict
		}
		receipt = OperationReceipt{RequestID: row.RequestID, Result: row.Result}
		return nil
	})
	return receipt, err
}

func (s *SQLOperationStore) Finish(ctx context.Context, req OperationFinish) (OperationReceipt, error) {
	if err := req.Validate(); err != nil {
		return OperationReceipt{}, err
	}
	var receipt OperationReceipt
	err := s.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var row operationRow
		if err := tx.QueryRowCtx(ctx, &row, `SELECT `+operationColumns+` FROM operation_log WHERE request_id=? FOR UPDATE`, req.RequestID); err != nil {
			if errors.Is(err, sqlx.ErrNotFound) {
				return operationConflict
			}
			return err
		}
		if row.Result != "pending" {
			if !row.matchesFinish(req) {
				return operationConflict
			}
		} else if _, err := tx.ExecCtx(ctx, `UPDATE operation_log SET result=?,http_status=?,business_code=?,duration_ms=?,finished_at=UTC_TIMESTAMP(3) WHERE request_id=? AND result='pending'`, req.Result, req.HTTPStatus, req.BusinessCode, req.DurationMs, req.RequestID); err != nil {
			return err
		}
		receipt = OperationReceipt{RequestID: req.RequestID, Result: req.Result}
		return nil
	})
	return receipt, err
}

func operationWhere(f OperationFilter) (string, []any) {
	var clauses []string
	var args []any
	add := func(clause string, arg any) { clauses = append(clauses, clause); args = append(args, arg) }
	if f.ActorID > 0 {
		add("actor_id=?", f.ActorID)
	}
	if f.Action != "" {
		add("action=?", f.Action)
	}
	if f.Method != "" {
		add("method=?", f.Method)
	}
	if f.TargetType != "" {
		add("target_type=?", f.TargetType)
	}
	if f.Result != "" {
		add("result=?", f.Result)
	}
	if !f.StartTime.IsZero() {
		add("started_at>=?", f.StartTime.UTC().Format("2006-01-02 15:04:05.999999"))
	}
	if !f.EndTime.IsZero() {
		add("started_at<=?", f.EndTime.UTC().Format("2006-01-02 15:04:05.999999"))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (s *SQLOperationStore) List(ctx context.Context, f OperationFilter) (OperationList, error) {
	resp := OperationList{List: []OperationLog{}, Page: f.Page, Size: f.Size}
	if err := f.Validate(); err != nil {
		return resp, err
	}
	where, args := operationWhere(f)
	if err := s.conn.QueryRowCtx(ctx, &resp.Total, `SELECT COUNT(*) FROM operation_log`+where, args...); err != nil {
		return resp, err
	}
	args = append(args, f.Size, (f.Page-1)*f.Size)
	var rows []operationRow
	if err := s.conn.QueryRowsCtx(ctx, &rows, `SELECT `+operationColumns+` FROM operation_log`+where+` ORDER BY started_at DESC,request_id DESC LIMIT ? OFFSET ?`, args...); err != nil {
		return resp, err
	}
	for _, row := range rows {
		var roles []string
		if err := json.Unmarshal([]byte(row.ActorRoles), &roles); err != nil {
			return resp, errors.New("invalid operation roles in storage")
		}
		resp.List = append(resp.List, OperationLog{OperationStart: OperationStart{RequestID: row.RequestID, ActorID: row.ActorID, ActorRoles: roles, Action: row.Action, Method: row.Method, RouteTemplate: row.RouteTemplate, TargetType: row.TargetType, TargetID: row.TargetID}, Result: row.Result, HTTPStatus: row.HTTPStatus, BusinessCode: row.BusinessCode, DurationMs: row.DurationMs, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt})
	}
	return resp, nil
}
