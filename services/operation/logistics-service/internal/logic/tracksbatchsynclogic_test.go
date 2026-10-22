package logic

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/askxuan/common"
	"github.com/askxuan/logistics-service/internal/types"
)

func TestTracksBatchSyncUnavailableWithoutStorageOrQueueDependencies(t *testing.T) {
	// nil dependencies make any attempted track query/update or MQ publish fail
	// the test. Both all-track and explicitly selected requests must do no work.
	l := NewTracksBatchSyncLogic(context.Background(), nil)
	for _, req := range []*types.TracksBatchSyncReq{nil, {}, {TrackingNos: []string{"TEST-NOT-A-REAL-WAYBILL"}}} {
		for attempt := 0; attempt < 2; attempt++ {
			resp, err := l.TracksBatchSync(req)
			var biz *common.BizError
			if resp != nil || !errors.As(err, &biz) || biz.Code != common.ErrNotImplemented.Code {
				t.Fatalf("attempt=%d: response=%v error=%v", attempt, resp, err)
			}
			if !strings.Contains(biz.Msg, "尚未接入") || !strings.Contains(biz.Msg, "状态未改变") {
				t.Fatalf("response must explain that no track/order transition occurred: %s", biz.Msg)
			}
		}
	}
}
