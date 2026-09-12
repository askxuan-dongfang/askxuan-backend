package logic

import (
	"context"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"time"
)

// Remove only abandoned uploads. Sent attachments and conversation history are
// retained. Sending a draft older than six days is rejected before cleanup.
func StartChatMaintenance(ctx context.Context, s *svc.ServiceContext) {
	go func() {
		timer := time.NewTicker(time.Hour)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				cleanChatTemporaryData(ctx, s)
			}
		}
	}()
}
func cleanChatTemporaryData(ctx context.Context, s *svc.ServiceContext) {
	var ids []string
	if err := s.DB.QueryRowsCtx(ctx, &ids, `SELECT a.id FROM chat_attachment a WHERE a.created_at<NOW()-INTERVAL 7 DAY AND NOT EXISTS(SELECT 1 FROM booking_chat_message m WHERE m.booking_id=a.conversation_id AND JSON_UNQUOTE(JSON_EXTRACT(m.attachment_json,'$.id'))=a.id) LIMIT 100`); err == nil {
		for _, id := range ids {
			if _, err := uuid.Parse(id); err != nil {
				continue
			}
			if err := os.Remove(filepath.Join(chatFilesDir(), id)); err == nil || os.IsNotExist(err) {
				_, _ = s.DB.ExecCtx(ctx, "DELETE FROM chat_attachment WHERE id=? AND created_at<NOW()-INTERVAL 7 DAY", id)
			}
		}
	}
	// SDP and ICE addresses are needed only for live call setup.
	_, _ = s.DB.ExecCtx(ctx, "DELETE FROM chat_call_signal WHERE created_at<NOW()-INTERVAL 1 DAY LIMIT 1000")
	_, _ = s.DB.ExecCtx(ctx, "UPDATE chat_call SET offer='',answer='' WHERE active_key IS NULL AND created_at<NOW()-INTERVAL 1 DAY AND offer<>'' LIMIT 1000")
}
