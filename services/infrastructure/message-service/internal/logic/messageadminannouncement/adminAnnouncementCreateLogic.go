// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package messageadminannouncement

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/model"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminAnnouncementCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminAnnouncementCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminAnnouncementCreateLogic {
	return &AdminAnnouncementCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminAnnouncementCreateLogic) AdminAnnouncementCreate(req *types.AnnouncementCreateReq) (resp *types.IdResp, err error) {
	id, err := l.svcCtx.AnnouncementModel.Insert(l.ctx, &model.SystemAnnouncement{
		Title:          req.Title,
		Content:        req.Content,
		Type:           req.Type,
		TargetAudience: req.TargetAudience,
		Status:         "published",
	})
	if err != nil {
		l.Errorf("创建公告失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.IdResp{Id: id}, nil
}
