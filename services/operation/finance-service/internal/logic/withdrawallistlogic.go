package logic

import (
	"context"

	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// WithdrawalListLogic 提现列表查询逻辑
type WithdrawalListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWithdrawalListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WithdrawalListLogic {
	return &WithdrawalListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// WithdrawalList 提现申请列表，支持按 applicantType/status 筛选 + 分页
func (l *WithdrawalListLogic) WithdrawalList(req *types.WithdrawalListReq) (*types.WithdrawalListResp, error) {
	list, total := model.ListWithdrawals(req.ApplicantType, req.Status, req.Page, req.Size)
	resp := &types.WithdrawalListResp{Total: total, Page: req.Page, Size: req.Size}
	for _, w := range list {
		resp.List = append(resp.List, types.Withdrawal{
			Id: w.Id, WithdrawalNo: w.WithdrawalNo, ApplicantType: w.ApplicantType,
			ApplicantId: w.ApplicantId, Amount: w.Amount, BankCard: w.BankCard,
			Status: w.Status, AuditTime: w.AuditTime, ProcessTime: w.ProcessTime,
			CreateTime: w.CreateTime,
		})
	}
	return resp, nil
}
