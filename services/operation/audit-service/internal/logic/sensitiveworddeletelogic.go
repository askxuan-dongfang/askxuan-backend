package logic

import (
	"context"

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
	// TODO: 删除 sensitive_word 记录 + 更新 Redis 缓存
	return common.ErrNotImplemented
}
