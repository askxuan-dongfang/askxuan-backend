package logic

import (
	"context"

	"github.com/askxuan/audit-service/internal/model"
	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// SensitiveWordCreateLogic 新增敏感词逻辑
type SensitiveWordCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSensitiveWordCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SensitiveWordCreateLogic {
	return &SensitiveWordCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SensitiveWordCreate 新增敏感词
func (l *SensitiveWordCreateLogic) SensitiveWordCreate(req *types.SensitiveWordCreateReq) (*types.SensitiveWordCreateResp, error) {
	sw := model.CreateSensitiveWord(req.Word, req.Category)
	return &types.SensitiveWordCreateResp{Id: sw.Id}, nil
}
