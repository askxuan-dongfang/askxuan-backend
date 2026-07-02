// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package messageadminannouncement

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminAnnouncementStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminAnnouncementStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminAnnouncementStatusLogic {
	return &AdminAnnouncementStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminAnnouncementStatusLogic) AdminAnnouncementStatus(req *types.AnnouncementStatusReq) (resp *types.IdResp, err error) {
	if err := l.svcCtx.AnnouncementModel.UpdateStatus(l.ctx, req.Id, req.Status); err != nil {
		l.Errorf("更新公告状态失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.IdResp{Id: req.Id}, nil
}
