// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package messagemaster

import (
	"context"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// MasterMessageReadLogic 法师标记消息已读逻辑
type MasterMessageReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMasterMessageReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MasterMessageReadLogic {
	return &MasterMessageReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// MasterMessageRead 法师标记消息为已读（校验消息归属，防止越权）
func (l *MasterMessageReadLogic) MasterMessageRead(req *types.MasterMessageReadReq) (resp *types.ReadResp, err error) {
	masterID := middleware.MasterIDFromCtx(l.ctx)
	if masterID == 0 {
		return nil, common.ErrUnauthorized
	}
	userId := strconv.FormatInt(masterID, 10)

	if err := l.svcCtx.MessageModel.MarkReadByUser(l.ctx, req.Id, userId); err != nil {
		l.Errorf("标记法师消息已读失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.ReadResp{Id: req.Id, IsRead: 1}, nil
}
