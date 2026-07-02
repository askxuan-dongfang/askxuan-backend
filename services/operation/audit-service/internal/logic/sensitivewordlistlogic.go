package logic

import (
	"context"

	"github.com/askxuan/audit-service/internal/model"
	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"

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
	list, total := model.ListSensitiveWords(req.Category, req.Status, req.Keyword, req.Page, req.Size)
	// 转换为 []types.SensitiveWord
	out := make([]types.SensitiveWord, 0, len(list))
	for _, sw := range list {
		out = append(out, types.SensitiveWord{
			Id:         sw.Id,
			Word:       sw.Word,
			Category:   sw.Category,
			Status:     sw.Status,
			CreateTime: sw.CreateTime,
		})
	}
	return &types.SensitiveWordListResp{
		Total: total,
		List:  out,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}
