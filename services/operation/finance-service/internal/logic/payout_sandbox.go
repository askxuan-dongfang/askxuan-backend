package logic

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const sandboxRecipient = "虚拟收款对象（无真实账户）"

type PayoutSandboxLogic struct {
	ctx        context.Context
	repository model.PayoutSandboxRepository
}

func NewPayoutSandboxLogic(ctx context.Context, _ *svc.ServiceContext) *PayoutSandboxLogic {
	return &PayoutSandboxLogic{ctx: ctx, repository: model.DefaultPayoutSandboxModel()}
}

func RequirePayoutSandboxSuper(ctx context.Context) error {
	kind, _ := ctx.Value(middleware.CtxKeyUserType).(string)
	if kind != "admin" || middleware.UserIDFromCtx(ctx) <= 0 {
		return common.ErrForbidden
	}
	for _, role := range middleware.RolesFromCtx(ctx) {
		if role == "platform_super" {
			return nil
		}
	}
	return common.ErrForbidden
}
func sandboxUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == value
}
func sandboxID(value string) bool {
	return strings.HasPrefix(value, "sandbox_") && sandboxUUID(strings.TrimPrefix(value, "sandbox_"))
}
func sandboxStatus(value string) bool {
	return value == "" || value == model.SandboxPendingReview || value == model.SandboxApproved || value == model.SandboxRejected || value == model.SandboxSuccess || value == model.SandboxFailed || value == model.SandboxUnknown
}
func sandboxError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, sqlx.ErrNotFound):
		return common.NewBizError(40472, "模拟打款记录不存在")
	case errors.Is(err, model.ErrSandboxConflict):
		return common.NewBizError(40972, "模拟记录版本或请求内容冲突，请刷新后重试")
	case errors.Is(err, model.ErrSandboxState):
		return common.NewBizError(40973, "当前模拟状态不允许此操作；未知结果必须先模拟查单确认")
	default:
		logx.WithContext(ctx).Errorf("payout sandbox: %v", err)
		return common.ErrSystem
	}
}
func sandboxRecord(row *model.PayoutSandbox) types.PayoutSandboxRecord {
	return types.PayoutSandboxRecord{ID: row.ID, Simulated: true, Recipient: sandboxRecipient, AmountCents: row.AmountCents, Status: row.Status, Version: row.Version, Attempt: row.Attempt, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func (l *PayoutSandboxLogic) List(req *types.PayoutSandboxListReq) (*types.PayoutSandboxListResp, error) {
	if err := RequirePayoutSandboxSuper(l.ctx); err != nil {
		return nil, err
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Size == 0 {
		req.Size = 20
	}
	if !sandboxStatus(req.Status) || req.Page < 1 || req.Page > 1000000 || req.Size < 1 || req.Size > 100 {
		return nil, common.ErrParamInvalid
	}
	rows, total, err := l.repository.List(l.ctx, req.Status, req.Page, req.Size)
	if err != nil {
		return nil, sandboxError(l.ctx, err)
	}
	response := &types.PayoutSandboxListResp{Simulated: true, Total: total, Page: req.Page, Size: req.Size, List: []types.PayoutSandboxRecord{}}
	for _, row := range rows {
		response.List = append(response.List, sandboxRecord(row))
	}
	return response, nil
}
func (l *PayoutSandboxLogic) Detail(id string) (*types.PayoutSandboxDetailResp, error) {
	if err := RequirePayoutSandboxSuper(l.ctx); err != nil {
		return nil, err
	}
	if !sandboxID(id) {
		return nil, common.ErrParamInvalid
	}
	row, events, err := l.repository.Get(l.ctx, id)
	if err != nil {
		return nil, sandboxError(l.ctx, err)
	}
	response := &types.PayoutSandboxDetailResp{Simulated: true, Record: sandboxRecord(row), Events: []types.PayoutSandboxEvent{}}
	for _, e := range events {
		response.Events = append(response.Events, types.PayoutSandboxEvent{ID: e.ID, RequestID: e.RequestID, Stage: e.Stage, Action: e.Action, FromStatus: e.FromStatus, ToStatus: e.ToStatus, Outcome: e.Outcome, Version: e.Version, Attempt: e.Attempt, ActorID: e.ActorID, CreatedAt: e.CreatedAt})
	}
	return response, nil
}
func (l *PayoutSandboxLogic) Create(req *types.PayoutSandboxCreateReq) (*types.PayoutSandboxRecord, error) {
	if err := RequirePayoutSandboxSuper(l.ctx); err != nil {
		return nil, err
	}
	if !sandboxUUID(req.RequestID) || req.AmountCents < 1 || req.AmountCents > 1000000 {
		return nil, common.ErrParamInvalid
	}
	row, err := l.repository.Create(l.ctx, req.RequestID, req.AmountCents, strconv.FormatInt(middleware.UserIDFromCtx(l.ctx), 10))
	if err != nil {
		return nil, sandboxError(l.ctx, err)
	}
	result := sandboxRecord(row)
	return &result, nil
}
func (l *PayoutSandboxLogic) Act(id string, req *types.PayoutSandboxActionReq) (*types.PayoutSandboxRecord, error) {
	if err := RequirePayoutSandboxSuper(l.ctx); err != nil {
		return nil, err
	}
	if !sandboxID(id) || !sandboxUUID(req.RequestID) || req.Version < 1 || req.Version > 1000000000 {
		return nil, common.ErrParamInvalid
	}
	valid := false
	switch req.Action {
	case "approve", "reject":
		valid = req.Outcome == ""
	case "submit", "retry":
		valid = req.Outcome == model.SandboxSuccess || req.Outcome == model.SandboxFailed || req.Outcome == model.SandboxUnknown
	case "resolve":
		valid = req.Outcome == model.SandboxSuccess || req.Outcome == model.SandboxFailed
	}
	if !valid {
		return nil, common.ErrParamInvalid
	}
	row, err := l.repository.Act(l.ctx, id, model.PayoutSandboxAction{RequestID: req.RequestID, Action: req.Action, Outcome: req.Outcome, Version: req.Version, ActorID: strconv.FormatInt(middleware.UserIDFromCtx(l.ctx), 10)})
	if err != nil {
		return nil, sandboxError(l.ctx, err)
	}
	result := sandboxRecord(row)
	return &result, nil
}
