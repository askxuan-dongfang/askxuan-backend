package logic

import (
	"context"

	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// SensitiveWordListLogic 敏感词列表逻辑
type SensitiveWordListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSensitiveWordListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SensitiveWordListLogic {
	return &SensitiveWordListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SensitiveWordList 敏感词库列表，支持按分类/状态/关键词筛选
func (l *SensitiveWordListLogic) SensitiveWordList(req *types.SensitiveWordListReq) (*types.SensitiveWordListResp, error) {
	// TODO: 调用 model.ListSensitiveWords 查询
	return nil, common.ErrNotImplemented
}
