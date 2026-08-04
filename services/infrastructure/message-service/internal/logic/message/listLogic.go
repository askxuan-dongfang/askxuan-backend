// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package message

import (
	"context"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogic {
	return &ListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListLogic) List(req *types.ListReq) (*types.ListResp, error) {
	userID := middleware.UserIDFromCtx(l.ctx)
	if userID == 0 {
		return nil, common.ErrUnauthorized
	}

	page, size := req.Page, req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	list, total, err := l.svcCtx.MessageModel.List(
		l.ctx,
		strconv.FormatInt(userID, 10),
		req.IsRead,
		page,
		size,
	)
	if err != nil {
		l.Errorf("query message list failed: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.Message, 0, len(list))
	for _, item := range list {
		out = append(out, types.Message{
			Id:        item.Id,
			UserId:    item.UserId,
			Title:     item.Title,
			Content:   item.Content,
			BizType:   item.BizType,
			BizId:     item.BizId,
			IsRead:    item.IsRead,
			CreatedAt: item.CreateTime,
		})
	}

	return &types.ListResp{Total: total, List: out, Page: page, Size: size}, nil
}
