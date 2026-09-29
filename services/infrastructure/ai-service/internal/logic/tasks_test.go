package logic

import (
	"context"
	"errors"
	"testing"
)

func TestCancellationTargetsOnlyRunningMessage(t *testing.T) {
	ctx, c := context.WithCancelCause(context.Background())
	defer c(nil)
	task := &runningTask{cancel: c}
	runningTasks.Store(int64(99), task)
	defer runningTasks.Delete(int64(99))
	if CancelMessage(98) || ctx.Err() != nil {
		t.Fatal("cancelled unrelated message")
	}
	if !CancelMessage(99) || !errors.Is(context.Cause(ctx), errUserStopped) {
		t.Fatal("did not cancel task")
	}
}
