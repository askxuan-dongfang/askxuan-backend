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

type AdminTemplateCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminTemplateCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminTemplateCreateLogic {
	return &AdminTemplateCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminTemplateCreateLogic) AdminTemplateCreate(req *types.TemplateCreateReq) (resp *types.IdResp, err error) {
	id, err := l.svcCtx.TemplateModel.Insert(l.ctx, &model.MessageTemplate{
		Code:            req.Code,
		TitleTemplate:   req.TitleTemplate,
		ContentTemplate: req.ContentTemplate,
		Variables:       req.Variables,
		Type:            req.Type,
	})
	if err != nil {
		l.Errorf("创建消息模板失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.IdResp{Id: id}, nil
}
