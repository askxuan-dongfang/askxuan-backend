package logic

import (
	"context"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/logistics-service/internal/mq"
	"github.com/askxuan/logistics-service/internal/model"
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

// TracksBatchSync 批量同步物流轨迹
// 查询所有非终态记录，mock 生成一条轨迹节点，按状态机推进状态（pending→in_transit→delivered→signed）
// status 变为 signed 时发 MQ 通知 order-service/diy-service 触发自动确认收货
func (l *TracksBatchSyncLogic) TracksBatchSync(req *types.TracksBatchSyncReq) (*types.TracksBatchSyncResp, error) {
	tracks, err := l.svcCtx.LogisticsTrackModel.FindNonTerminal(l.ctx)
	if err != nil {
		l.Errorf("查询非终态物流记录失败: %v", err)
		return nil, common.ErrSystem
	}

	total := len(tracks)
	success := 0
	failed := 0
	now := time.Now().Format("2006-01-02 15:04:05")

	for _, track := range tracks {
		// 计算下一个状态
		nextStatus := getNextTrackStatus(track.Status)
		if nextStatus == "" {
			failed++
			continue
		}

		// 生成 mock 轨迹节点
		desc := mockTraceDesc(nextStatus)
		track.Traces = append(track.Traces, model.TrackTrace{Time: now, Desc: desc})
		track.Status = nextStatus

		err := l.svcCtx.LogisticsTrackModel.Update(l.ctx, track)
		if err != nil {
			l.Errorf("更新物流轨迹失败 trackingNo=%s: %v", track.TrackingNo, err)
			failed++
			continue
		}
		success++

		// 签收时发 MQ 通知（BizType 映射为下游消费者期望的 OrderType）
		if nextStatus == model.TrackStatusSigned {
			_ = l.svcCtx.MqProducer.PublishLogisticsSync(l.ctx, mq.LogisticsSync{
				OrderId:   track.BizNo,
				OrderType: bizTypeToOrderType(track.BizType),
				ExpressNo: track.TrackingNo,
				Status:    "signed",
				Time:      now,
			})
		}
	}

	return &types.TracksBatchSyncResp{Total: total, Success: success, Failed: failed}, nil
}

// getNextTrackStatus 按状态机获取下一个状态
func getNextTrackStatus(current string) string {
	switch current {
	case model.TrackStatusPending:
		return model.TrackStatusInTransit
	case model.TrackStatusInTransit:
		return model.TrackStatusDelivered
	case model.TrackStatusDelivered:
		return model.TrackStatusSigned
	default:
		return ""
	}
}

// mockTraceDesc 生成 mock 轨迹描述
func mockTraceDesc(status string) string {
	switch status {
	case model.TrackStatusInTransit:
		return "快件已揽收，正在运输中"
	case model.TrackStatusDelivered:
		return "快件已到达派送网点，等待派送"
	case model.TrackStatusSigned:
		return "已签收，本人签收"
	default:
		return "物流状态更新"
	}
}

// bizTypeToOrderType 将物流 BizType 映射为下游消费者期望的 OrderType
// order → shop_order, diy → diy_order
func bizTypeToOrderType(bizType string) string {
	switch bizType {
	case model.BizTypeDiy:
		return "diy_order"
	default:
		return "shop_order"
	}
}
