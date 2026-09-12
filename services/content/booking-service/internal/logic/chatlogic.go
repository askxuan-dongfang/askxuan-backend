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

type chatEntitlement struct {
	SourceType  string
	SourceID    string
	UserID      string
	MasterCode  string
	MasterName  string
	ServiceName string
	TempleName  string
	ExpiresAt   string
	CanSend     bool
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
	readerID := "u_" + userID
	if isMaster {
		readerID = "m_" + strconv.FormatInt(middleware.MasterIDFromCtx(l.ctx), 10)
	}
	if len([]rune(req.Query)) > 100 {
		return nil, common.ErrParamInvalid
	}
	rows, total, err := l.svcCtx.ChatModel.ListConversations(l.ctx, userID, masterCode, page, size, strings.TrimSpace(req.Query), readerID, req.UnreadOnly)
	if err != nil {
		l.Errorf("查询付费预约会话失败: %v", err)
		return nil, common.ErrSystem
	}
	list := make([]types.ChatConversation, 0, len(rows))
	masterOpenIMIDs := make(map[string]string)
	for _, row := range rows {
		peerID, peerName := row.MasterCode, row.MasterName
		peerOpenIMID := ""
		if isMaster {
			peerID, peerName = row.UserId, "预约用户"
			peerOpenIMID = "u_" + row.UserId
		} else if cached, ok := masterOpenIMIDs[row.MasterCode]; ok {
			peerOpenIMID = cached
		} else if master, lookupErr := l.svcCtx.MasterClient.GetByCode(l.ctx, row.MasterCode); lookupErr == nil {
			peerOpenIMID = "m_" + strconv.FormatInt(master.Id, 10)
			masterOpenIMIDs[row.MasterCode] = peerOpenIMID
		} else {
			l.Errorf("查询会话对端 OpenIM ID 失败: masterCode=%s err=%v", row.MasterCode, lookupErr)
		}
		reader := "u_" + row.UserId
		other := peerOpenIMID
		if isMaster {
			reader = "m_" + strconv.FormatInt(middleware.MasterIDFromCtx(l.ctx), 10)
			other = "u_" + row.UserId
		}
		unread, read, peerRead, err := chatReadState(l.ctx, l.svcCtx, row.BookingId, reader, other)
		if err != nil {
			return nil, common.ErrSystem
		}
		name, avatar := chatPeerProfile(l.ctx, l.svcCtx, peerOpenIMID, peerName)
		list = append(list, types.ChatConversation{
			UnreadCount: unread, ReadThrough: read, PeerReadThrough: peerRead,
			ConversationId: row.BookingId, SourceType: row.SourceType, SourceId: row.BookingId,
			BookingId: func() string {
				if row.SourceType == "booking" {
					return row.BookingId
				}
				return ""
			}(),
			PeerId: peerID, PeerOpenIMId: peerOpenIMID, PeerName: name, PeerAvatar: avatar,
			TempleName: row.TempleName, ServiceName: row.ServiceName, BookingDate: row.BookingDate,
			ExpiresAt: row.ExpiresAt, LastMessage: row.LastMessage, LastMessageAt: row.LastMessageAt,
			CanChat: row.ChatStatus == "active",
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
	entitlement, err := loadChatEntitlement(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		return nil, err
	}
	participant, err := resolveChatParticipant(l.ctx, l.svcCtx, entitlement, false)
	if err != nil {
		return nil, err
	}
	page, size := normalizeChatPage(req.Page, req.Size, 50)
	if req.BeforeId < 0 || req.AfterId < 0 || (req.BeforeId > 0 && req.AfterId > 0) {
		return nil, common.ErrParamInvalid
	}
	var rows []*model.BookingChatMessage
	var total int64
	var more bool
	if page > 1 && req.BeforeId == 0 && req.AfterId == 0 {
		rows, total, err = l.svcCtx.ChatModel.ListMessages(l.ctx, entitlement.SourceID, page, size)
		more = int64(page*size) < total
	} else {
		rows, more, err = l.svcCtx.ChatModel.MessagesCursor(l.ctx, entitlement.SourceID, req.BeforeId, req.AfterId, size)
		total = int64(len(rows))
		if more {
			total++
		}
	}
	if err != nil {
		return nil, common.ErrSystem
	}
	_, read, peerRead, err := chatReadState(l.ctx, l.svcCtx, entitlement.SourceID, participant.SenderID, participant.ReceiverID)
	if err != nil {
		return nil, common.ErrSystem
	}
	list := make([]types.ChatMessage, 0, len(rows))
	for _, row := range rows {
		msg := toChatMessage(row)
		msg.Read = row.SenderId == participant.SenderID && row.Id <= peerRead
		list = append(list, msg)
	}
	return &types.ChatMessageListResp{Total: total, List: list, Page: page, Size: size, HasMore: more, ReadThrough: read, PeerReadThrough: peerRead}, nil

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
	entitlement, err := loadChatEntitlement(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		return nil, err
	}
	participant, err := resolveChatParticipant(l.ctx, l.svcCtx, entitlement, false)
	if err != nil {
		return nil, err
	}
	clientMessageID := strings.TrimSpace(req.ClientMessageId)
	if len(clientMessageID) > 128 {
		return nil, common.ErrParamInvalid
	}
	if clientMessageID == "" {
		clientMessageID = fmt.Sprintf("askxuan-%d-%s", time.Now().UnixNano(), participant.SenderID)
	}
	// Return the original accepted payload even if a retry arrives after expiry.
	if existing, findErr := l.svcCtx.ChatModel.FindByClientMessageID(l.ctx, req.Id, clientMessageID); findErr == nil {
		if existing.SenderId != participant.SenderID {
			return nil, common.ErrForbidden
		}
		if existing.Status == "sent" {
			message := toChatMessage(existing)
			return &message, nil
		}
	} else if !errors.Is(findErr, sqlx.ErrNotFound) {
		return nil, common.ErrSystem
	}
	if _, err = resolveChatParticipant(l.ctx, l.svcCtx, entitlement, true); err != nil {
		return nil, err
	}
	content, kind, attachment, err := validateChatPayload(l.ctx, l.svcCtx, req, participant.SenderID)
	if err != nil {
		return nil, err
	}
	row, created, err := l.svcCtx.ChatModel.Insert(l.ctx, &model.BookingChatMessage{
		BookingId: entitlement.SourceID, SourceType: entitlement.SourceType, ClientMessageId: clientMessageID,
		SenderType: participant.SenderType, SenderId: participant.SenderID, ReceiverId: participant.ReceiverID,
		Content: content, Kind: kind, AttachmentJSON: attachment, Status: "sent",
	})
	if err != nil {
		return nil, common.ErrSystem
	}
	if row.SenderId != participant.SenderID {
		return nil, common.ErrForbidden
	}
	if !created && row.Status != "sent" {
		// Resume legacy failed/pending rows without replacing their original content.
		if _, err = l.svcCtx.DB.ExecCtx(l.ctx, "UPDATE booking_chat_message SET status='sent',notify_status='pending',notify_after=NOW() WHERE id=? AND sender_id=?", row.Id, participant.SenderID); err != nil {
			return nil, common.ErrSystem
		}
		row.Status = "sent"
	}
	// Committed business history is the acknowledgement. The durable outbox wakes
	// the other client independently, so an IM outage never loses accepted messages.
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

func loadChatEntitlement(ctx context.Context, svcCtx *svc.ServiceContext, sourceID string) (*chatEntitlement, error) {
	if booking, err := svcCtx.BookingModel.FindOne(ctx, sourceID); err == nil {
		return &chatEntitlement{SourceType: "booking", SourceID: booking.Id, UserID: booking.UserId,
			MasterCode: booking.MasterId, MasterName: booking.MasterName, ServiceName: booking.ServiceName, TempleName: booking.TempleName,
			CanSend: bookingHasChatEntitlement(booking, booking.UserId)}, nil
	} else if !errors.Is(err, sqlx.ErrNotFound) {
		return nil, common.ErrSystem
	}
	consultation, err := svcCtx.ConsultationModel.FindOne(ctx, sourceID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrConsultationNotFound
		}
		return nil, common.ErrSystem
	}
	canSend := consultation.PaymentStatus == model.ConsultationPaymentSuccess && consultation.Status == model.ConsultationStatusActive
	if canSend {
		expiry, parseErr := time.ParseInLocation("2006-01-02 15:04:05", consultation.ExpiresAt, time.FixedZone("CST", 8*3600))
		canSend = parseErr == nil && expiry.After(time.Now())
	}
	return &chatEntitlement{SourceType: "consultation", SourceID: consultation.Id, UserID: consultation.UserId,
		MasterCode: consultation.MasterId, MasterName: consultation.MasterName, ServiceName: "即时咨询", TempleName: consultation.TempleName, ExpiresAt: consultation.ExpiresAt, CanSend: canSend}, nil
}

func resolveChatParticipant(ctx context.Context, svcCtx *svc.ServiceContext, entitlement *chatEntitlement, requireSend bool) (*chatParticipant, error) {
	if entitlement == nil || entitlement.MasterCode == "" {
		return nil, common.ErrBookingChatUnavailable
	}
	if requireSend && !entitlement.CanSend {
		if entitlement.SourceType == "consultation" {
			return nil, common.ErrConsultationExpired
		}
		return nil, common.ErrBookingChatUnavailable
	}
	if masterID := middleware.MasterIDFromCtx(ctx); masterID > 0 {
		master, err := svcCtx.MasterClient.GetByID(ctx, masterID)
		if err != nil {
			return nil, common.ErrDependencyUnavailable
		}
		if master.Code != entitlement.MasterCode {
			return nil, common.ErrForbidden
		}
		return &chatParticipant{SenderType: "master", SenderID: "m_" + strconv.FormatInt(master.Id, 10), ReceiverID: "u_" + entitlement.UserID, SenderName: entitlement.MasterName}, nil
	}
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if userID != entitlement.UserID {
		return nil, common.ErrForbidden
	}
	master, err := svcCtx.MasterClient.GetByCode(ctx, entitlement.MasterCode)
	if err != nil {
		return nil, common.ErrDependencyUnavailable
	}
	return &chatParticipant{SenderType: "customer", SenderID: "u_" + userID, ReceiverID: "m_" + strconv.FormatInt(master.Id, 10), SenderName: "咨询用户"}, nil
}

// AuthorizeOpenIMMessage is used by OpenIM's synchronous before-send webhook.
// Booking text must carry the server-issued marker so multiple bookings between
// the same customer and master remain isolated.
func AuthorizeOpenIMMessage(ctx context.Context, svcCtx *svc.ServiceContext, sendID, recvID, ex string) (*chatEntitlement, error) {
	userID, masterNumericID, isBookingPair, err := parseOpenIMBookingPair(sendID, recvID)
	if err != nil {
		return nil, common.ErrBookingChatUnavailable
	}
	if !isBookingPair {
		return nil, nil
	}
	sourceType, sourceID, clientID, marked := parseChatMessageMarker(ex)
	if !marked {
		return nil, common.ErrBookingChatUnavailable
	}
	entitlement, err := loadChatEntitlement(ctx, svcCtx, sourceID)
	if err != nil || entitlement.SourceType != sourceType || entitlement.UserID != userID {
		return nil, common.ErrBookingChatUnavailable
	}
	master, err := svcCtx.MasterClient.GetByCode(ctx, entitlement.MasterCode)
	if err != nil {
		return nil, common.ErrDependencyUnavailable
	}
	if master.Id != masterNumericID {
		return nil, common.ErrBookingChatUnavailable
	}
	row, rowErr := svcCtx.ChatModel.FindByClientMessageID(ctx, sourceID, clientID)
	if rowErr != nil || row.SenderId != sendID || row.ReceiverId != recvID || row.Status != "sent" {
		return nil, common.ErrBookingChatUnavailable
	}
	return entitlement, nil
}

func bookingHasChatEntitlement(booking *model.Booking, userID string) bool {
	return booking != nil && booking.UserId == userID && booking.MasterId != "" &&
		booking.PaymentStatus == model.PaymentStatusSuccess && booking.Status != model.StatusCancelled
}

func RecordOpenIMMessage(ctx context.Context, svcCtx *svc.ServiceContext, callback OpenIMCallbackMessage) error {
	_, sourceID, clientID, ok := parseChatMessageMarker(callback.Ex)
	if !ok {
		return nil
	}
	row, err := svcCtx.ChatModel.FindByClientMessageID(ctx, sourceID, clientID)
	if err != nil {
		return err
	}
	if row.SenderId != callback.SendID || row.ReceiverId != callback.RecvID {
		return common.ErrForbidden
	}
	// Callbacks never create/overwrite business messages or mark them read.
	return svcCtx.ChatModel.UpdateDelivery(ctx, row.Id, row.Status, callback.ServerMsgID)
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

func chatMessageMarker(sourceType, sourceID, clientMessageID string) string {
	return "askxuan-chat:" + sourceType + ":" + sourceID + ":" + clientMessageID
}

func parseChatMessageMarker(ex string) (string, string, string, bool) {
	const prefix = "askxuan-chat:"
	if strings.HasPrefix(ex, prefix) {
		parts := strings.SplitN(strings.TrimPrefix(ex, prefix), ":", 3)
		if len(parts) == 3 && (parts[0] == "booking" || parts[0] == "consultation") && parts[1] != "" && parts[2] != "" {
			return parts[0], parts[1], parts[2], true
		}
		return "", "", "", false
	}
	bookingID, clientID, ok := parseBookingMessageMarker(ex)
	if !ok {
		return "", "", "", false
	}
	return "booking", bookingID, clientID, true
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
	var attachment *types.ChatAttachment
	if row.AttachmentJSON != "" && row.AttachmentJSON != "{}" {
		_ = json.Unmarshal([]byte(row.AttachmentJSON), &attachment)
	}
	return types.ChatMessage{Kind: row.Kind, Attachment: attachment, Id: row.Id, ConversationId: row.BookingId, SourceType: row.SourceType,
		BookingId: func() string {
			if row.SourceType == "booking" {
				return row.BookingId
			}
			return ""
		}(), ClientMessageId: row.ClientMessageId,
		SenderType: row.SenderType, SenderId: row.SenderId, ReceiverId: row.ReceiverId,
		Content: row.Content, Status: row.Status, CreateTime: row.CreateTime}
}

func chatReadState(ctx context.Context, s *svc.ServiceContext, id, reader, peer string) (int64, int64, int64, error) {
	unread, err := s.ChatModel.Unread(ctx, id, reader)
	if err != nil {
		return 0, 0, 0, err
	}
	read, err := s.ChatModel.ReadCursor(ctx, id, reader)
	if err != nil {
		return 0, 0, 0, err
	}
	other, err := s.ChatModel.ReadCursor(ctx, id, peer)
	return unread, read, other, err
}
func ChatDetail(ctx context.Context, s *svc.ServiceContext, id string) (*types.ChatConversation, error) {
	ent, err := loadChatEntitlement(ctx, s, id)
	if err != nil {
		return nil, err
	}
	p, err := resolveChatParticipant(ctx, s, ent, false)
	if err != nil {
		return nil, err
	}
	unread, read, peerRead, err := chatReadState(ctx, s, id, p.SenderID, p.ReceiverID)
	if err != nil {
		return nil, common.ErrSystem
	}
	name, peerID := ent.MasterName, ent.MasterCode
	if p.SenderType == "master" {
		name = "咨询用户"
		peerID = ent.UserID
	}
	name, avatar := chatPeerProfile(ctx, s, p.ReceiverID, name)
	result := &types.ChatConversation{ConversationId: id, SourceId: id, SourceType: ent.SourceType, PeerId: peerID, PeerName: name, PeerAvatar: avatar, PeerOpenIMId: p.ReceiverID, CanChat: ent.CanSend, ServiceName: ent.ServiceName, TempleName: ent.TempleName, ExpiresAt: ent.ExpiresAt, UnreadCount: unread, ReadThrough: read, PeerReadThrough: peerRead}
	if ent.SourceType == "booking" {
		result.BookingId = id
	}
	return result, nil
}
func ChatMarkRead(ctx context.Context, s *svc.ServiceContext, req *types.ChatReadReq) (map[string]int64, error) {
	if req.ThroughId < 0 {
		return nil, common.ErrParamInvalid
	}
	ent, err := loadChatEntitlement(ctx, s, req.Id)
	if err != nil {
		return nil, err
	}
	p, err := resolveChatParticipant(ctx, s, ent, false)
	if err != nil {
		return nil, err
	}
	through, err := s.ChatModel.MarkRead(ctx, req.Id, p.SenderID, req.ThroughId)
	if err != nil {
		return nil, common.ErrSystem
	}
	return map[string]int64{"readThrough": through}, nil
}

func chatPeerProfile(ctx context.Context, s *svc.ServiceContext, id, fallback string) (string, string) {
	var profile struct {
		Name   string `db:"name"`
		Avatar string `db:"avatar"`
	}
	query := "SELECT nickname name,avatar FROM askxuan_user.user WHERE id=?"
	key := strings.TrimPrefix(id, "u_")
	if strings.HasPrefix(id, "m_") {
		query = "SELECT dharma_name name,avatar FROM askxuan_master.master WHERE id=?"
		key = strings.TrimPrefix(id, "m_")
	}
	if err := s.DB.QueryRowCtx(ctx, &profile, query, key); err != nil {
		return fallback, ""
	}
	if profile.Name == "" {
		profile.Name = fallback
	}
	return profile.Name, profile.Avatar
}
func ChatUnread(ctx context.Context, s *svc.ServiceContext) (map[string]int64, error) {
	reader := ""
	where := ""
	key := ""
	if id := middleware.MasterIDFromCtx(ctx); id > 0 {
		m, err := s.MasterClient.GetByID(ctx, id)
		if err != nil {
			return nil, common.ErrDependencyUnavailable
		}
		reader = "m_" + strconv.FormatInt(id, 10)
		key = m.Code
		where = "x.master_code=?"
	} else {
		u, err := authenticatedUserID(ctx)
		if err != nil {
			return nil, err
		}
		reader = "u_" + u
		key = u
		where = "x.user_id=?"
	}
	var count int64
	query := `SELECT COUNT(*) FROM booking_chat_message m JOIN (SELECT booking_no id,user_id,master_code FROM booking WHERE payment_status='success' AND status<>'cancelled' UNION ALL SELECT order_no id,user_id,master_code FROM consultation_order WHERE payment_status='success') x ON x.id=m.booking_id LEFT JOIN chat_read_cursor c ON c.conversation_id=m.booking_id AND c.reader_id=? WHERE m.receiver_id=? AND m.status='sent' AND m.id>COALESCE(c.through_id,0) AND ` + where
	if err := s.DB.QueryRowCtx(ctx, &count, query, reader, reader, key); err != nil {
		return nil, common.ErrSystem
	}
	return map[string]int64{"count": count}, nil
}
