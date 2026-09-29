package logic

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

var runningTasks sync.Map
var errUserStopped = errors.New("已停止生成，可点击重试")

type runningTask struct{ cancel context.CancelCauseFunc }

// Register synchronously so cancellation works immediately after POST returns.
func processAsync(s *svc.ServiceContext, sessionID, messageID int64) {
	limit := s.AIConfig.TaskTimeoutSeconds
	if limit < 60 {
		limit = 180
	}
	if limit > 600 {
		limit = 600
	}
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Duration(limit)*time.Second)
	ctx, cancel := context.WithCancelCause(timeoutCtx)
	task := &runningTask{cancel: cancel}
	if _, loaded := runningTasks.LoadOrStore(messageID, task); loaded {
		cancel(nil)
		timeoutCancel()
		return
	}
	go func() {
		defer timeoutCancel()
		defer cancel(nil)
		defer runningTasks.CompareAndDelete(messageID, task)
		err := processMessage(ctx, s, sessionID, messageID)
		runningTasks.CompareAndDelete(messageID, task)
		if err != nil {
			if errors.Is(context.Cause(ctx), errUserStopped) {
				err = errUserStopped
			}
			logx.Errorf("AI处理失败 session=%d message=%d: %v", sessionID, messageID, err)
			_ = s.ConversationModel.FailMessage(context.Background(), messageID, err.Error())
		}
	}()
}

// Handler must establish session ownership before calling this function.
func CancelMessage(id int64) bool {
	t, ok := runningTasks.Load(id)
	if ok {
		t.(*runningTask).cancel(errUserStopped)
	}
	return ok
}
