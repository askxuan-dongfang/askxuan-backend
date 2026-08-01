package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	commonim "github.com/askxuan/common/im"
	"github.com/askxuan/common/middleware"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const maxChatMessageLength = 2000

type chatParticipant struct {
	SenderType string
	SenderID   string
	ReceiverID string
	SenderName string
}

type ChatListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatListLogic {
	return &ChatListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ChatListLogic) List(req *types.ChatListReq) (*types.ChatListResp, error) {
	page, size := normalizeChatPage(req.Page, req.Size, 20)
	userID := ""
	masterCode := ""
	isMaster := middleware.MasterIDFromCtx(l.ctx) > 0
	if isMaster {
		master, err := l.svcCtx.MasterClient.GetByID(l.ctx, middleware.MasterIDFromCtx(l.ctx))
		if err != nil {
			return nil, common.ErrDependencyUnavailable
		}
		masterCode = master.Code
	} else {
		var err error
		userID, err = authenticatedUserID(l.ctx)
		if err != nil {
			return nil, err
		}
	}
	rows, total, err := l.svcCtx.ChatModel.ListConversations(l.ctx, userID, masterCode, page, size)
	if err != nil {
		l.Errorf("查询付费预约会话失败: %v", err)
		return nil, common.ErrSystem
	}
	list := make([]types.ChatConversation, 0, len(rows))
	for _, row := range rows {
		peerID, peerName := row.MasterCode, row.MasterName
		if isMaster {
			peerID, peerName = row.UserId, "预约用户"
		}
		list = append(list, types.ChatConversation{
			BookingId: row.BookingId, PeerId: peerID, PeerName: peerName,
			TempleName: row.TempleName, ServiceName: row.ServiceName, BookingDate: row.BookingDate,
			LastMessage: row.LastMessage, LastMessageAt: row.LastMessageAt, CanChat: true,
		})
	}
	return &types.ChatListResp{Total: total, List: list, Page: page, Size: size}, nil
}

type ChatMessageListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatMessageListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatMessageListLogic {
	return &ChatMessageListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ChatMessageListLogic) List(req *types.ChatMessageListReq) (*types.ChatMessageListResp, error) {
	booking, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		return nil, common.ErrSystem
	}
	if _, err := resolveChatParticipant(l.ctx, l.svcCtx, booking); err != nil {
		return nil, err
	}
	page, size := normalizeChatPage(req.Page, req.Size, 50)
	rows, total, err := l.svcCtx.ChatModel.ListMessages(l.ctx, booking.Id, page, size)
	if err != nil {
		l.Errorf("查询预约聊天记录失败: %v", err)
		return nil, common.ErrSystem
	}
	list := make([]types.ChatMessage, 0, len(rows))
	for _, row := range rows {
		list = append(list, toChatMessage(row))
	}
	return &types.ChatMessageListResp{Total: total, List: list, Page: page, Size: size}, nil
}

type ChatMessageSendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatMessageSendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatMessageSendLogic {
	return &ChatMessageSendLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ChatMessageSendLogic) Send(req *types.ChatMessageSendReq) (*types.ChatMessage, error) {
	content := strings.TrimSpace(req.Content)
	if content == "" || len([]rune(content)) > maxChatMessageLength {
		return nil, common.ErrParamInvalid
	}
	booking, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		return nil, common.ErrSystem
	}
	participant, err := resolveChatParticipant(l.ctx, l.svcCtx, booking)
	if err != nil {
		return nil, err
	}
	clientMessageID := strings.TrimSpace(req.ClientMessageId)
	if clientMessageID == "" {
		clientMessageID = fmt.Sprintf("askxuan-%d-%s", time.Now().UnixNano(), participant.SenderID)
	}
	row, created, err := l.svcCtx.ChatModel.Insert(l.ctx, &model.BookingChatMessage{
		BookingId: booking.Id, ClientMessageId: clientMessageID, SenderType: participant.SenderType,
		SenderId: participant.SenderID, ReceiverId: participant.ReceiverID, Content: content, Status: "pending",
	})
	if err != nil {
		l.Errorf("创建预约聊天消息失败: %v", err)
		return nil, common.ErrSystem
	}
	deliver, deliveryContent := chatDeliveryDecision(created, row, content)
	if !deliver {
		message := toChatMessage(row)
		return &message, nil
	}
	content = deliveryContent
	err = l.svcCtx.IMClient.SendMessage(l.ctx, &commonim.SendMsgReq{
		SendID: participant.SenderID, RecvID: participant.ReceiverID, SenderName: participant.SenderName,
		SenderPlatformID: 1, SessionType: 1, ContentType: 101, Content: map[string]string{"content": content},
		Ex: "askxuan-booking:" + booking.Id + ":" + clientMessageID,
	})
	if err != nil {
		_ = l.svcCtx.ChatModel.UpdateDelivery(l.ctx, row.Id, "failed", "")
		l.Errorf("OpenIM 投递预约聊天消息失败: %v", err)
		return nil, common.ErrDependencyUnavailable
	}
	_ = l.svcCtx.ChatModel.UpdateDelivery(l.ctx, row.Id, "sent", "")
	row.Status = "sent"
	message := toChatMessage(row)
	return &message, nil
}

func chatDeliveryDecision(created bool, row *model.BookingChatMessage, requestedContent string) (bool, string) {
	if created {
		return true, requestedContent
	}
	if row != nil && row.Status == "failed" {
		// An idempotent retry resends the original payload, not new text supplied
		// with an already-used client message ID.
		return true, row.Content
	}
	if row != nil {
		return false, row.Content
	}
	return false, requestedContent
}

func resolveChatParticipant(ctx context.Context, svcCtx *svc.ServiceContext, booking *model.Booking) (*chatParticipant, error) {
	if booking.PaymentStatus != model.PaymentStatusSuccess || booking.Status == model.StatusCancelled || booking.MasterId == "" {
		return nil, common.ErrBookingChatUnavailable
	}
	if masterID := middleware.MasterIDFromCtx(ctx); masterID > 0 {
		master, err := svcCtx.MasterClient.GetByID(ctx, masterID)
		if err != nil {
			return nil, common.ErrDependencyUnavailable
		}
		if master.Code != booking.MasterId {
			return nil, common.ErrForbidden
		}
		return &chatParticipant{SenderType: "master", SenderID: "m_" + strconv.FormatInt(master.Id, 10), ReceiverID: "u_" + booking.UserId, SenderName: booking.MasterName}, nil
	}
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if userID != booking.UserId {
		return nil, common.ErrForbidden
	}
	master, err := svcCtx.MasterClient.GetByCode(ctx, booking.MasterId)
	if err != nil {
		return nil, common.ErrDependencyUnavailable
	}
	return &chatParticipant{SenderType: "customer", SenderID: "u_" + userID, ReceiverID: "m_" + strconv.FormatInt(master.Id, 10), SenderName: "预约用户"}, nil
}

// AuthorizeOpenIMMessage is used by OpenIM's synchronous before-send webhook.
// Booking text must carry the server-issued marker so multiple bookings between
// the same customer and master remain isolated.
func AuthorizeOpenIMMessage(ctx context.Context, svcCtx *svc.ServiceContext, sendID, recvID, ex string) (*model.Booking, error) {
	userID, masterNumericID, isBookingPair, err := parseOpenIMBookingPair(sendID, recvID)
	if err != nil {
		return nil, common.ErrBookingChatUnavailable
	}
	if !isBookingPair {
		return nil, nil
	}
	bookingID, _, marked := parseBookingMessageMarker(ex)
	if !marked {
		return nil, common.ErrBookingChatUnavailable
	}
	booking, err := svcCtx.BookingModel.FindOne(ctx, bookingID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingChatUnavailable
		}
		return nil, common.ErrSystem
	}
	if !bookingHasChatEntitlement(booking, userID) {
		return nil, common.ErrBookingChatUnavailable
	}
	master, err := svcCtx.MasterClient.GetByCode(ctx, booking.MasterId)
	if err != nil {
		return nil, common.ErrDependencyUnavailable
	}
	if master.Id != masterNumericID {
		return nil, common.ErrBookingChatUnavailable
	}
	return booking, nil
}

func bookingHasChatEntitlement(booking *model.Booking, userID string) bool {
	return booking != nil && booking.UserId == userID && booking.MasterId != "" &&
		booking.PaymentStatus == model.PaymentStatusSuccess && booking.Status != model.StatusCancelled
}

func RecordOpenIMMessage(ctx context.Context, svcCtx *svc.ServiceContext, callback OpenIMCallbackMessage) error {
	booking, err := AuthorizeOpenIMMessage(ctx, svcCtx, callback.SendID, callback.RecvID, callback.Ex)
	if err != nil || booking == nil {
		return err
	}
	clientMessageID := callback.ClientMsgID
	if bookingID, markedClientID, ok := parseBookingMessageMarker(callback.Ex); ok && bookingID == booking.Id {
		clientMessageID = markedClientID
	}
	if clientMessageID == "" {
		clientMessageID = callback.ServerMsgID
	}
	if clientMessageID == "" {
		return nil
	}
	senderType := "customer"
	if strings.HasPrefix(callback.SendID, "m_") {
		senderType = "master"
	}
	row, _, err := svcCtx.ChatModel.Insert(ctx, &model.BookingChatMessage{
		BookingId: booking.Id, ClientMessageId: clientMessageID, OpenIMServerMsgId: callback.ServerMsgID,
		SenderType: senderType, SenderId: callback.SendID, ReceiverId: callback.RecvID,
		Content: decodeOpenIMText(callback.Content), Status: "sent",
	})
	if err != nil {
		return err
	}
	return svcCtx.ChatModel.UpdateDelivery(ctx, row.Id, "sent", callback.ServerMsgID)
}

type OpenIMCallbackMessage struct {
	SendID      string
	RecvID      string
	ClientMsgID string
	ServerMsgID string
	Content     string
	Ex          string
}

func parseOpenIMBookingPair(sendID, recvID string) (string, int64, bool, error) {
	var userRaw, masterRaw string
	switch {
	case strings.HasPrefix(sendID, "u_") && strings.HasPrefix(recvID, "m_"):
		userRaw, masterRaw = strings.TrimPrefix(sendID, "u_"), strings.TrimPrefix(recvID, "m_")
	case strings.HasPrefix(sendID, "m_") && strings.HasPrefix(recvID, "u_"):
		userRaw, masterRaw = strings.TrimPrefix(recvID, "u_"), strings.TrimPrefix(sendID, "m_")
	default:
		return "", 0, false, nil
	}
	if userRaw == "" {
		return "", 0, true, fmt.Errorf("empty user id")
	}
	masterID, err := strconv.ParseInt(masterRaw, 10, 64)
	if err != nil || masterID <= 0 {
		return "", 0, true, fmt.Errorf("invalid master id")
	}
	return userRaw, masterID, true, nil
}

func parseBookingMessageMarker(ex string) (string, string, bool) {
	const prefix = "askxuan-booking:"
	if !strings.HasPrefix(ex, prefix) {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(ex, prefix), ":", 2)
	return parts[0], func() string {
		if len(parts) == 2 {
			return parts[1]
		}
		return ""
	}(), len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

func decodeOpenIMText(content string) string {
	var payload struct {
		Content string `json:"content"`
	}
	if json.Unmarshal([]byte(content), &payload) == nil && payload.Content != "" {
		return payload.Content
	}
	return content
}

func normalizeChatPage(page, size, defaultSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = defaultSize
	}
	return page, size
}

func toChatMessage(row *model.BookingChatMessage) types.ChatMessage {
	return types.ChatMessage{Id: row.Id, BookingId: row.BookingId, ClientMessageId: row.ClientMessageId,
		SenderType: row.SenderType, SenderId: row.SenderId, ReceiverId: row.ReceiverId,
		Content: row.Content, Status: row.Status, CreateTime: row.CreateTime}
}
