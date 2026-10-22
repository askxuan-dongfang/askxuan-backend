package logic

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/askxuan/common"
	"github.com/askxuan/finance-service/internal/types"
)

func TestWithdrawalProcessUnavailableWithoutStorageOrPayoutDependencies(t *testing.T) {
	// No model store or service dependencies are configured. The old path would
	// query storage and mutate two states; repeated requests must fail closed.
	l := NewWithdrawalProcessLogic(context.Background(), nil)
	for _, id := range []int64{1, 12345} {
		for attempt := 0; attempt < 2; attempt++ {
			resp, err := l.WithdrawalProcess(&types.WithdrawalProcessReq{Id: id})
			var biz *common.BizError
			if resp != nil || !errors.As(err, &biz) || biz.Code != common.ErrNotImplemented.Code {
				t.Fatalf("id=%d attempt=%d: response=%v error=%v", id, attempt, resp, err)
			}
			if !strings.Contains(biz.Msg, "未发起打款") || !strings.Contains(biz.Msg, "状态未改变") {
				t.Fatalf("response must explain that no payout or transition occurred: %s", biz.Msg)
			}
		}
	}
}

func TestWithdrawalProcessRejectsInvalidRequestWithoutDependencies(t *testing.T) {
	l := NewWithdrawalProcessLogic(context.Background(), nil)
	for _, req := range []*types.WithdrawalProcessReq{nil, {Id: 0}, {Id: -1}} {
		resp, err := l.WithdrawalProcess(req)
		if resp != nil || !errors.Is(err, common.ErrParam) {
			t.Fatalf("invalid request: response=%v error=%v", resp, err)
		}
	}
}
