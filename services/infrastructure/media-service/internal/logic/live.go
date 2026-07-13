package logic

import (
	"context"
	"errors"

	"github.com/askxuan/common"
	"github.com/askxuan/media-service/internal/model"
	"github.com/askxuan/media-service/internal/provider"
	"github.com/askxuan/media-service/internal/svc"
	"github.com/askxuan/media-service/internal/types"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func LiveCapabilities(svcCtx *svc.ServiceContext) *types.LiveCapabilitiesResp {
	configured := svcCtx.LiveProvider.Configured()
	return &types.LiveCapabilitiesResp{
		Enabled: svcCtx.LiveEnabled, Provider: svcCtx.LiveProvider.Name(),
		Configured: configured, CanStart: svcCtx.LiveEnabled && configured,
	}
}

func CreateLiveRoom(ctx context.Context, svcCtx *svc.ServiceContext, req *types.LiveRoomCreateReq) (*types.LiveRoom, error) {
	if req.OwnerId == "" || req.MasterId == "" || req.Title == "" {
		return nil, common.ErrParam
	}
	if req.CoverMediaId > 0 {
		cover, err := svcCtx.MediaModel.FindOne(ctx, req.CoverMediaId)
		if err != nil || cover.OwnerId != req.OwnerId || cover.MediaType != "image" || cover.Status != "ready" {
			return nil, common.NewBizError(40921, "直播封面不可用")
		}
	}
	room, err := svcCtx.LiveRoomModel.Insert(ctx, &model.LiveRoom{
		OwnerId: req.OwnerId, MasterId: req.MasterId, Title: req.Title,
		CoverMediaId: req.CoverMediaId, Provider: svcCtx.LiveProvider.Name(), OpenimGroupId: req.OpenimGroupId,
	})
	if err != nil {
		return nil, common.ErrSystem
	}
	result := toLiveRoom(room, true)
	return &result, nil
}

func BindLiveOpenIM(ctx context.Context, svcCtx *svc.ServiceContext, req *types.LiveBindOpenIMReq) (*types.LiveRoom, error) {
	if req.Id <= 0 || req.OwnerId == "" || req.OpenimGroupId == "" {
		return nil, common.ErrParam
	}
	room, err := svcCtx.LiveRoomModel.BindOpenIM(ctx, req.Id, req.OwnerId, req.OpenimGroupId)
	if err != nil {
		return nil, mapLiveError(err)
	}
	result := toLiveRoom(room, true)
	return &result, nil
}

func StartLiveRoom(ctx context.Context, svcCtx *svc.ServiceContext, req *types.LiveRoomActionReq) (*types.LiveRoom, error) {
	if !svcCtx.LiveEnabled || !svcCtx.LiveProvider.Configured() {
		return nil, common.NewBizError(50320, "直播能力未配置")
	}
	room, err := svcCtx.LiveRoomModel.FindOne(ctx, req.Id)
	if err != nil {
		return nil, mapLiveError(err)
	}
	if room.OwnerId != req.OwnerId {
		return nil, common.ErrForbidden
	}
	if room.Status != "created" {
		return nil, common.ErrStatusInvalid
	}
	if room.OpenimGroupId == "" {
		return nil, common.NewBizError(40931, "请先绑定直播群聊")
	}
	session, err := svcCtx.LiveProvider.Start(ctx, room.RoomNo)
	if err != nil {
		if errors.Is(err, provider.ErrLiveUnavailable) {
			return nil, common.NewBizError(50320, "直播 Provider 不可用")
		}
		return nil, common.ErrSystem
	}
	room, err = svcCtx.LiveRoomModel.Start(ctx, room.Id, req.OwnerId, session.ProviderRoomId, session.PushURL, session.WatchURL)
	if err != nil {
		_ = svcCtx.LiveProvider.Close(ctx, session.ProviderRoomId)
		return nil, mapLiveError(err)
	}
	result := toLiveRoom(room, true)
	return &result, nil
}

func CloseLiveRoom(ctx context.Context, svcCtx *svc.ServiceContext, req *types.LiveRoomActionReq) (*types.LiveRoom, error) {
	room, err := svcCtx.LiveRoomModel.FindOne(ctx, req.Id)
	if err != nil {
		return nil, mapLiveError(err)
	}
	if room.OwnerId != req.OwnerId {
		return nil, common.ErrForbidden
	}
	if room.Status != "live" {
		return nil, common.ErrStatusInvalid
	}
	if err := svcCtx.LiveProvider.Close(ctx, room.ProviderRoomId); err != nil {
		return nil, common.NewBizError(50320, "关闭直播 Provider 失败")
	}
	room, err = svcCtx.LiveRoomModel.Close(ctx, room.Id, req.OwnerId)
	if err != nil {
		return nil, mapLiveError(err)
	}
	result := toLiveRoom(room, true)
	return &result, nil
}

func GetLiveRoom(ctx context.Context, svcCtx *svc.ServiceContext, id int64, requester string) (*types.LiveRoom, error) {
	room, err := svcCtx.LiveRoomModel.FindOne(ctx, id)
	if err != nil {
		return nil, mapLiveError(err)
	}
	if !canViewLiveRoom(room, requester) {
		return nil, common.ErrForbidden
	}
	result := toLiveRoom(room, requester != "" && requester == room.OwnerId)
	return &result, nil
}

func canViewLiveRoom(room *model.LiveRoom, requester string) bool {
	return requester == room.OwnerId || (requester != "" && room.Status == "live")
}

func ListLiveRooms(ctx context.Context, svcCtx *svc.ServiceContext, req *types.LiveRoomListReq) (*types.LiveRoomListResp, error) {
	rooms, err := svcCtx.LiveRoomModel.FindLive(ctx, req.MasterId, req.Limit)
	if err != nil {
		return nil, common.ErrSystem
	}
	result := make([]types.LiveRoom, 0, len(rooms))
	for index := range rooms {
		result = append(result, toLiveRoom(&rooms[index], false))
	}
	return &types.LiveRoomListResp{List: result}, nil
}

func mapLiveError(err error) error {
	if err == sqlx.ErrNotFound {
		return common.NewBizError(40430, "直播房间不存在或状态已变化")
	}
	return common.ErrSystem
}

func toLiveRoom(value *model.LiveRoom, includePush bool) types.LiveRoom {
	pushURL := ""
	if includePush {
		pushURL = value.PushUrl
	}
	return types.LiveRoom{
		Id: value.Id, RoomNo: value.RoomNo, OwnerId: value.OwnerId, MasterId: value.MasterId,
		Title: value.Title, CoverMediaId: value.CoverMediaId, Provider: value.Provider,
		Status: value.Status, OpenimGroupId: value.OpenimGroupId, PushUrl: pushURL,
		WatchUrl: value.WatchUrl, ProviderRoomId: value.ProviderRoomId,
		StartedAt: value.StartedAt, EndedAt: value.EndedAt, CreateTime: value.CreateTime, UpdateTime: value.UpdateTime,
	}
}
