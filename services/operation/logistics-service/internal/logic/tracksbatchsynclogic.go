package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/logistics-service/internal/svc"
	"github.com/askxuan/logistics-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// TracksBatchSyncLogic 批量同步物流轨迹逻辑
type TracksBatchSyncLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTracksBatchSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TracksBatchSyncLogic {
	return &TracksBatchSyncLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// TracksBatchSync rejects before reading or writing tracks until a real carrier
// integration exists. A synchronization request must never manufacture delivery
// events or publish a signed notification.
func (l *TracksBatchSyncLogic) TracksBatchSync(req *types.TracksBatchSyncReq) (*types.TracksBatchSyncResp, error) {
	return nil, common.NewBizError(common.ErrNotImplemented.Code, "真实物流同步尚未接入，物流轨迹和订单状态未改变")
}
