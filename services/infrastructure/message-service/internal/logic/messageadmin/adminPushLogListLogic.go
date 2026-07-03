// Code scaffolded by goctl. Safe to edit.

package messageadmin

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminPushLogListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminPushLogListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminPushLogListLogic {
	return &AdminPushLogListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminPushLogListLogic) AdminPushLogList(req *types.PushLogListReq) (*types.PushLogListResp, error) {
	page, size := req.Page, req.Size
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}

	list, total, err := l.svcCtx.PushLogModel.List(l.ctx, req.UserId, req.Status, req.BizType, page, size)
	if err != nil {
		l.Errorf("查询推送日志失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.PushLogListResp{Total: total, Page: page, Size: size, List: make([]types.PushLog, 0, len(list))}
	for _, item := range list {
		resp.List = append(resp.List, types.PushLog{
			Id:         item.Id,
			UserId:     item.UserId,
			PushType:   item.PushType,
			Title:      item.Title,
			Content:    item.Content,
			Status:     item.Status,
			BizType:    item.BizType,
			BizId:      item.BizId,
			CreateTime: item.CreateTime,
		})
	}
	return resp, nil
}
