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

type ReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReadLogic {
	return &ReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReadLogic) Read(req *types.ReadReq) (*types.ReadResp, error) {
	userID := middleware.UserIDFromCtx(l.ctx)
	if userID == 0 {
		return nil, common.ErrUnauthorized
	}

	id, err := strconv.ParseInt(req.Id, 10, 64)
	if err != nil || id <= 0 {
		return nil, common.ErrParamInvalid
	}
	if err := l.svcCtx.MessageModel.MarkReadByUser(
		l.ctx,
		id,
		strconv.FormatInt(userID, 10),
	); err != nil {
		l.Errorf("mark message read failed: %v", err)
		return nil, common.ErrSystem
	}

	return &types.ReadResp{Id: id, IsRead: 1}, nil
}
