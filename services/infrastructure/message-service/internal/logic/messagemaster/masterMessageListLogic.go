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

func masterRecipientID(masterID int64) string {
	return "m_" + strconv.FormatInt(masterID, 10)
}

// MasterMessageListLogic 法师消息列表逻辑
type MasterMessageListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMasterMessageListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MasterMessageListLogic {
	return &MasterMessageListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// MasterMessageList 法师查询自己的站内消息（masterId 从 JWT 获取）
func (l *MasterMessageListLogic) MasterMessageList(req *types.MasterMessageListReq) (resp *types.ListResp, err error) {
	masterID := middleware.MasterIDFromCtx(l.ctx)
	if masterID == 0 {
		return nil, common.ErrUnauthorized
	}
	// Customer and master numeric IDs live in different tables and can collide.
	// Keep master recipients in the same m_<id> namespace used by OpenIM.
	userID := masterRecipientID(masterID)

	list, total, err := l.svcCtx.MessageModel.List(l.ctx, userID, req.IsRead, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询法师消息列表失败: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.Message, 0, len(list))
	for _, m := range list {
		out = append(out, types.Message{
			Id:        m.Id,
			UserId:    m.UserId,
			Title:     m.Title,
			Content:   m.Content,
			BizType:   m.BizType,
			BizId:     m.BizId,
			IsRead:    m.IsRead,
			CreatedAt: m.CreateTime,
		})
	}

	return &types.ListResp{
		Total: total,
		List:  out,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}
