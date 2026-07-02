// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package messageadmin

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/model"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminTemplateUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminTemplateUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminTemplateUpdateLogic {
	return &AdminTemplateUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminTemplateUpdateLogic) AdminTemplateUpdate(req *types.TemplateUpdateReq) (resp *types.IdResp, err error) {
	if err := l.svcCtx.TemplateModel.Update(l.ctx, &model.MessageTemplate{
		Id:              req.Id,
		TitleTemplate:   req.TitleTemplate,
		ContentTemplate: req.ContentTemplate,
		Variables:       req.Variables,
		Type:            req.Type,
	}); err != nil {
		l.Errorf("更新消息模板失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.IdResp{Id: req.Id}, nil
}
