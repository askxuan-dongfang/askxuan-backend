// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package messageadmin

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminTemplateListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminTemplateListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminTemplateListLogic {
	return &AdminTemplateListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminTemplateListLogic) AdminTemplateList(req *types.TemplateListReq) (resp *types.TemplateListResp, err error) {
	list, total, err := l.svcCtx.TemplateModel.List(l.ctx, req.Type, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询模板列表失败: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.MessageTemplate, 0, len(list))
	for _, t := range list {
		out = append(out, types.MessageTemplate{
			Id:              t.Id,
			Code:            t.Code,
			TitleTemplate:   t.TitleTemplate,
			ContentTemplate: t.ContentTemplate,
			Variables:       t.Variables,
			Type:            t.Type,
			CreatedAt:       t.CreateTime,
		})
	}

	return &types.TemplateListResp{
		Total: total,
		List:  out,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}
