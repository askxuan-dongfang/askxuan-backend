// Code scaffolded by goctl. Safe to edit.

package messagecustomer

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/model"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendMessageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSendMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendMessageLogic {
	return &SendMessageLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *SendMessageLogic) SendMessage(req *types.SendMessageReq) (*types.SendMessageResp, error) {
	content := strings.TrimSpace(req.Content)
	if strings.TrimSpace(req.UserId) == "" || content == "" {
		return nil, common.ErrParamMissing
	}

	recipientID, err := resolveConsultRecipient(req.ConversationId)
	if err != nil {
		return nil, common.ErrParam
	}
	id, err := l.svcCtx.MessageModel.Insert(l.ctx, &model.Message{
		UserId:  recipientID,
		Title:   "新的咨询消息",
		Content: fmt.Sprintf("用户%s：%s", req.UserId, content),
		BizType: "consult",
		BizId:   req.ConversationId,
		IsRead:  0,
	})
	if err != nil {
		l.Errorf("发送咨询消息失败: %v", err)
		return nil, common.ErrSystem
	}

	if _, err := l.svcCtx.PushLogModel.Insert(l.ctx, &model.PushLog{
		UserId:   recipientID,
		PushType: "app_push",
		Title:    "新的咨询消息",
		Content:  content,
		Status:   "pending",
		BizType:  "consult",
		BizId:    req.ConversationId,
	}); err != nil {
		l.Errorf("写入推送日志失败（不阻断主流程）: %v", err)
	}
	return &types.SendMessageResp{Id: id}, nil
}

// resolveConsultRecipient 从会话 ID 解析接收方用户 ID
// 支持格式：master:<id>、M<code>（如 M001）、纯数字
func resolveConsultRecipient(conversationID string) (string, error) {
	id := strings.TrimSpace(conversationID)
	if id == "" {
		return "", fmt.Errorf("conversationId is empty")
	}
	if strings.HasPrefix(id, "master:") {
		rid := strings.TrimPrefix(id, "master:")
		if rid == "" {
			return "", fmt.Errorf("invalid master: prefix")
		}
		return rid, nil
	}
	if strings.HasPrefix(id, "M") {
		n, err := strconv.ParseInt(strings.TrimLeft(strings.TrimPrefix(id, "M"), "0"), 10, 64)
		if err != nil || n <= 0 {
			return "", fmt.Errorf("invalid master code: %s", id)
		}
		return strconv.FormatInt(n, 10), nil
	}
	if _, err := strconv.ParseInt(id, 10, 64); err == nil {
		return id, nil
	}
	return "", fmt.Errorf("invalid conversationId: %s", id)
}
