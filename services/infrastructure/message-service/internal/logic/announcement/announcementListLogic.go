// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package announcement

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnnouncementListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnnouncementListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnnouncementListLogic {
	return &AnnouncementListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnnouncementListLogic) AnnouncementList(req *types.AnnouncementListReq) (resp *types.AnnouncementListResp, err error) {
	list, total, err := l.svcCtx.AnnouncementModel.ListPublished(l.ctx, req.Type, req.TargetAudience, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询公告列表失败: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.SystemAnnouncement, 0, len(list))
	for _, a := range list {
		out = append(out, types.SystemAnnouncement{
			Id:             a.Id,
			Title:          a.Title,
			Content:        a.Content,
			Type:           a.Type,
			TargetAudience: a.TargetAudience,
			Status:         a.Status,
			PublishTime:    a.PublishTime,
			CreateTime:     a.CreateTime,
		})
	}

	return &types.AnnouncementListResp{
		Total: total,
		List:  out,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}
