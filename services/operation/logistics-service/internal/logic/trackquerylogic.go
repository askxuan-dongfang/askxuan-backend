package logic

import (
	"context"
	"errors"

	"github.com/askxuan/common"
	"github.com/askxuan/logistics-service/internal/svc"
	"github.com/askxuan/logistics-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// TrackQueryLogic 物流轨迹查询逻辑
type TrackQueryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTrackQueryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TrackQueryLogic {
	return &TrackQueryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// TrackQuery 按物流单号查询轨迹，返回 traces 轨迹数组
func (l *TrackQueryLogic) TrackQuery(req *types.TrackQueryReq) (*types.TrackQueryResp, error) {
	track, err := l.svcCtx.LogisticsTrackModel.FindByTrackingNo(l.ctx, req.TrackingNo)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrParam
		}
		l.Errorf("查询物流轨迹失败: %v", err)
		return nil, common.ErrSystem
	}
	traces := make([]types.TrackTrace, 0, len(track.Traces))
	for _, t := range track.Traces {
		traces = append(traces, types.TrackTrace{Time: t.Time, Desc: t.Desc})
	}
	return &types.TrackQueryResp{
		TrackingNo:   track.TrackingNo,
		ExpressCode:  track.ExpressCode,
		ExpressName:  track.ExpressName,
		BizType:      track.BizType,
		BizNo:        track.BizNo,
		Status:       track.Status,
		Traces:       traces,
		LastSyncTime: track.LastSyncTime,
	}, nil
}
