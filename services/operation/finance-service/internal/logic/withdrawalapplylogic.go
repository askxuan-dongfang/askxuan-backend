package logic

import (
	"context"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/mq"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ============ 法师提现申请 Logic ============

// WithdrawalApplyLogic 法师提现申请逻辑
type WithdrawalApplyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWithdrawalApplyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WithdrawalApplyLogic {
	return &WithdrawalApplyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// WithdrawalApply 法师提交提现申请（masterId 从 JWT 获取）
func (l *WithdrawalApplyLogic) WithdrawalApply(req *types.WithdrawalApplyReq) (*types.WithdrawalApplyResp, error) {
	masterID := middleware.MasterIDFromCtx(l.ctx)
	if masterID == 0 {
		return nil, common.ErrUnauthorized
	}

	if req.Amount <= 0 {
		return nil, common.ErrParam
	}
	if req.BankCard == "" {
		return nil, common.ErrParamMissing
	}

	applicantID := strconv.FormatInt(masterID, 10)
	w := model.ApplyWithdrawal(model.SettleTypeMaster, applicantID, req.Amount, req.BankCard)

	// 发 MQ 通知（失败不阻断主流程）
	if l.svcCtx.MqProducer != nil {
		_ = l.svcCtx.MqProducer.PublishWithdrawalNotify(l.ctx, mq.WithdrawalNotify{
			WithdrawalId: strconv.FormatInt(w.Id, 10),
			UserId:       applicantID,
			Amount:       w.Amount,
			Status:       w.Status,
			Time:         w.CreateTime,
		})
	}

	return &types.WithdrawalApplyResp{
		Id:            w.Id,
		WithdrawalNo:  w.WithdrawalNo,
		ApplicantType: w.ApplicantType,
		ApplicantId:   w.ApplicantId,
		Amount:        w.Amount,
		Status:        w.Status,
		CreateTime:    w.CreateTime,
	}, nil
}
