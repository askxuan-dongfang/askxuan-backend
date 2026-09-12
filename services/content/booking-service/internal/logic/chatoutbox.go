package logic

import (
	"context"
	"github.com/askxuan/booking-service/internal/svc"
	commonim "github.com/askxuan/common/im"
	"github.com/zeromicro/go-zero/core/logx"
	"time"
)

// StartChatOutbox retries notifications for already-committed messages. A lease
// prevents concurrent workers from notifying the same row at the same time.
func StartChatOutbox(ctx context.Context, s *svc.ServiceContext) {
	go func() {
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				drainChatOutbox(ctx, s)
			}
		}
	}()
}
func drainChatOutbox(ctx context.Context, s *svc.ServiceContext) {
	var ids []int64
	if err := s.DB.QueryRowsCtx(ctx, &ids, "SELECT id FROM booking_chat_message WHERE status='sent' AND notify_status<>'done' AND (notify_after IS NULL OR notify_after<=NOW()) ORDER BY id LIMIT 30"); err != nil {
		logx.Errorf("chat outbox query failed: %v", err)
		return
	}
	for _, id := range ids {
		result, err := s.DB.ExecCtx(ctx, "UPDATE booking_chat_message SET notify_status='working',notify_after=NOW()+INTERVAL 45 SECOND,notify_attempts=notify_attempts+1 WHERE id=? AND notify_status<>'done' AND (notify_after IS NULL OR notify_after<=NOW())", id)
		if err != nil {
			continue
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			continue
		}
		var row struct {
			Conversation string `db:"booking_id"`
			ClientID     string `db:"client_message_id"`
			SourceType   string `db:"source_type"`
			Sender       string `db:"sender_id"`
			Receiver     string `db:"receiver_id"`
			Content      string `db:"content"`
		}
		err = s.DB.QueryRowCtx(ctx, &row, "SELECT booking_id,client_message_id,source_type,sender_id,receiver_id,content FROM booking_chat_message WHERE id=?", id)
		if err == nil {
			callCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
			err = s.IMClient.SendMessage(callCtx, &commonim.SendMsgReq{SendID: row.Sender, RecvID: row.Receiver, SenderName: "新消息", SenderPlatformID: 1, SessionType: 1, ContentType: 101, Content: map[string]string{"content": row.Content}, Ex: chatMessageMarker(row.SourceType, row.Conversation, row.ClientID)})
			cancel()
		}
		if err != nil {
			_, _ = s.DB.ExecCtx(ctx, "UPDATE booking_chat_message SET notify_status='pending',notify_after=NOW()+INTERVAL 30 SECOND WHERE id=?", id)
			logx.Errorf("chat wakeup pending message=%d: %v", id, err)
		} else if _, err = s.DB.ExecCtx(ctx, "UPDATE booking_chat_message SET notify_status='done',notify_after=NULL WHERE id=?", id); err != nil {
			logx.Errorf("chat outbox acknowledgement message=%d: %v", id, err)
		}
	}
}
