package logic

import (
	"context"

	"github.com/askxuan/audit-service/internal/model"
	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// SensitiveWordDeleteLogic 删除敏感词逻辑
type SensitiveWordDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSensitiveWordDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SensitiveWordDeleteLogic {
	return &SensitiveWordDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SensitiveWordDelete 删除敏感词
func (l *SensitiveWordDeleteLogic) SensitiveWordDelete(req *types.SensitiveWordDeleteReq) error {
	if !model.DeleteSensitiveWord(req.Id) {
		return common.NewBizError(40404, "敏感词不存在")
	}
	return nil
}
