package logic

import (
	"context"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/common/push"
	"github.com/zeromicro/go-zero/core/logx"
	"os"
	"time"
)

func StartChatPush(ctx context.Context, s *svc.ServiceContext) {
	client, err := push.NewAPNS(os.Getenv("APNS_KEY_FILE"), os.Getenv("APNS_KEY_ID"), os.Getenv("APNS_TEAM_ID"))
	if err != nil {
		logx.Info("Real APNs push is disabled until its credentials are configured")
		return
	}
	go func() {
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				drainChatPush(ctx, s, client)
				drainChatCallPush(ctx, s, client)
			}
		}
	}()
}
func drainChatPush(ctx context.Context, s *svc.ServiceContext, a *push.APNS) {
	_, _ = s.DB.ExecCtx(ctx, "UPDATE booking_chat_message SET push_status='done' WHERE push_status<>'done' AND create_time<NOW()-INTERVAL 1 DAY")
	var rows []struct {
		ID           int64  `db:"id"`
		Conversation string `db:"booking_id"`
		Receiver     string `db:"receiver_id"`
	}
	if err := s.DB.QueryRowsCtx(ctx, &rows, "SELECT id,booking_id,receiver_id FROM booking_chat_message WHERE status='sent' AND push_status<>'done' AND push_after<=NOW() ORDER BY id LIMIT 30"); err != nil {
		logx.Errorf("chat push query: %v", err)
		return
	}
	for _, m := range rows {
		res, err := s.DB.ExecCtx(ctx, "UPDATE booking_chat_message SET push_after=NOW()+INTERVAL 2 MINUTE WHERE id=? AND push_status<>'done' AND push_after<=NOW()", m.ID)
		if err != nil {
			continue
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			continue
		}
		read, err := s.ChatModel.ReadCursor(ctx, m.Conversation, m.Receiver)
		if err != nil {
			continue
		}
		if read >= m.ID {
			_, _ = s.DB.ExecCtx(ctx, "UPDATE booking_chat_message SET push_status='done' WHERE id=?", m.ID)
			continue
		}
		var devices []struct {
			ID          int64  `db:"id"`
			Token       string `db:"device_token"`
			Bundle      string `db:"bundle_id"`
			Environment string `db:"apns_environment"`
		}
		if err = s.DB.QueryRowsCtx(ctx, &devices, "SELECT id,device_token,bundle_id,apns_environment FROM askxuan_message.device_token WHERE chat_identity=? AND platform='ios' AND status='active'", m.Receiver); err != nil {
			continue
		}
		complete := true
		for _, d := range devices {
			var done int64
			if err = s.DB.QueryRowCtx(ctx, &done, "SELECT COUNT(*) FROM chat_push_delivery WHERE message_id=? AND device_id=? AND status='done'", m.ID, d.ID); err != nil {
				complete = false
				continue
			}
			if done > 0 {
				continue
			}
			// Lock screen notifications intentionally contain no consultation content.
			payload := map[string]any{"aps": map[string]any{"alert": map[string]string{"title": "问玄东方", "body": "你收到一条新的咨询消息"}, "sound": "default", "thread-id": m.Conversation}, "conversationId": m.Conversation}
			permanent, sendErr := a.Send(ctx, d.Token, d.Bundle, d.Environment, "chat-"+m.Conversation, payload)
			if permanent {
				_, _ = s.DB.ExecCtx(ctx, "UPDATE askxuan_message.device_token SET status='inactive' WHERE id=? AND device_token=?", d.ID, d.Token)
			}
			if sendErr != nil && !permanent {
				logx.Errorf("chat push message=%d device=%d: %v", m.ID, d.ID, sendErr)
				complete = false
				continue
			}
			if _, err = s.DB.ExecCtx(ctx, "INSERT INTO chat_push_delivery(message_id,device_id,status) VALUES(?,?,'done') ON DUPLICATE KEY UPDATE status='done'", m.ID, d.ID); err != nil {
				complete = false
			}
		}
		if complete {
			_, _ = s.DB.ExecCtx(ctx, "UPDATE booking_chat_message SET push_status='done' WHERE id=?", m.ID)
		}
	}
}

// Invitations expire quickly, so an offline device never rings for an old call.
func drainChatCallPush(ctx context.Context, s *svc.ServiceContext, a *push.APNS) {
	var calls []ChatCall
	if err := s.DB.QueryRowsCtx(ctx, &calls, "SELECT "+callColumns+" FROM chat_call WHERE state='ringing' AND active_key IS NOT NULL AND created_at>NOW()-INTERVAL 50 SECOND ORDER BY created_at LIMIT 20"); err != nil {
		return
	}
	for _, c := range calls {
		var devices []struct {
			ID          int64  `db:"id"`
			Token       string `db:"device_token"`
			Bundle      string `db:"bundle_id"`
			Environment string `db:"apns_environment"`
		}
		if err := s.DB.QueryRowsCtx(ctx, &devices, "SELECT id,device_token,bundle_id,apns_environment FROM askxuan_message.device_token WHERE chat_identity=? AND platform='ios' AND status='active'", c.Callee); err != nil {
			continue
		}
		for _, d := range devices {
			var count int64
			if err := s.DB.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM chat_call_push WHERE call_id=? AND device_id=?", c.ID, d.ID); err != nil || count > 0 {
				continue
			}
			payload := map[string]any{"aps": map[string]any{"alert": map[string]string{"title": "问玄东方", "body": "你收到一个咨询通话邀请，点击接听"}, "sound": "default", "thread-id": c.Conversation}, "conversationId": c.Conversation, "callId": c.ID}
			permanent, err := a.SendUntil(ctx, d.Token, d.Bundle, d.Environment, "call-"+c.ID, payload, time.Now().Add(10*time.Second))
			if permanent {
				_, _ = s.DB.ExecCtx(ctx, "UPDATE askxuan_message.device_token SET status='inactive' WHERE id=? AND device_token=?", d.ID, d.Token)
			}
			if err == nil || permanent {
				_, _ = s.DB.ExecCtx(ctx, "INSERT IGNORE INTO chat_call_push(call_id,device_id) VALUES(?,?)", c.ID, d.ID)
			}
		}
	}
}
