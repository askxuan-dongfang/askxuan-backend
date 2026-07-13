package logic

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/media-service/internal/model"
	"github.com/askxuan/media-service/internal/svc"
	"github.com/askxuan/media-service/internal/types"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func CreateUploadCredential(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UploadCredentialReq) (*types.UploadCredentialResp, error) {
	if req.UserId == "" || req.FileName == "" || !validMediaType(req.MediaType) || req.ContentType == "" || req.FileSize < 0 {
		return nil, common.ErrParam
	}
	ext := strings.ToLower(filepath.Ext(req.FileName))
	objectName := fmt.Sprintf("%s/%s/%d%s", req.MediaType, time.Now().Format("2006/01/02"), time.Now().UnixNano(), ext)
	media, err := svcCtx.MediaModel.Insert(ctx, &model.Media{
		OwnerId: req.UserId, MediaType: req.MediaType, ContentType: req.ContentType,
		FileName: req.FileName, ObjectName: objectName, Provider: svcCtx.MediaProvider.Name(), FileSize: req.FileSize,
	})
	if err != nil {
		return nil, common.ErrSystem
	}
	uploadURL, headers, expires, err := svcCtx.MediaProvider.PrepareUpload(ctx, objectName, req.ContentType)
	if err != nil {
		return nil, common.NewBizError(50310, "上传存储暂不可用")
	}
	return &types.UploadCredentialResp{MediaId: media.Id, UploadUrl: uploadURL, ObjectName: objectName, ExpiresIn: expires, UploadHeaders: headers}, nil
}

func CompleteUpload(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UploadCompleteReq) (*types.Media, error) {
	media, err := svcCtx.MediaModel.FindOne(ctx, req.Id)
	if err != nil {
		return nil, mapNotFound(err)
	}
	if media.OwnerId != req.UserId {
		return nil, common.ErrForbidden
	}
	if media.Status == "ready" {
		if req.CoverMediaId > 0 && req.CoverMediaId != media.CoverMediaId {
			return nil, common.ErrStatusInvalid
		}
		result := toMedia(media)
		return &result, nil
	}
	info, err := svcCtx.MediaProvider.Stat(ctx, media.ObjectName)
	if err != nil {
		return nil, common.NewBizError(40920, "上传对象尚未完成")
	}
	coverURL := ""
	if req.CoverMediaId > 0 {
		cover, findErr := svcCtx.MediaModel.FindOne(ctx, req.CoverMediaId)
		if findErr != nil || cover.OwnerId != req.UserId || cover.MediaType != "image" || cover.Status != "ready" {
			return nil, common.NewBizError(40921, "视频封面不可用")
		}
		coverURL = cover.PlaybackUrl
	}
	updated, err := svcCtx.MediaModel.Complete(ctx, media.Id, req.UserId, svcCtx.MediaProvider.PublicURL(media.ObjectName), coverURL, req.CoverMediaId, info.Size, info.ContentType)
	if err != nil {
		return nil, mapNotFound(err)
	}
	result := toMedia(updated)
	return &result, nil
}

func GetMedia(ctx context.Context, svcCtx *svc.ServiceContext, id int64, requester string) (*types.Media, error) {
	media, err := svcCtx.MediaModel.FindOne(ctx, id)
	if err != nil {
		return nil, mapNotFound(err)
	}
	if !canReadMedia(media, requester) {
		return nil, common.ErrForbidden
	}
	result := toMedia(media)
	return &result, nil
}

func canReadMedia(media *model.Media, requester string) bool {
	return requester == media.OwnerId || (requester != "" && media.Status == "ready" && media.AuditStatus == "approved")
}

func TranscodeCallback(ctx context.Context, svcCtx *svc.ServiceContext, req *types.MediaCallbackReq) (*types.EmptyResp, error) {
	if req.MediaId <= 0 || !validTranscodeStatus(req.Status) {
		return nil, common.ErrParam
	}
	media, err := svcCtx.MediaModel.FindOne(ctx, req.MediaId)
	if err != nil {
		return nil, mapNotFound(err)
	}
	applyTranscodeCallback(media, req)
	if err := svcCtx.MediaModel.UpdateTranscode(ctx, media); err != nil {
		return nil, common.ErrSystem
	}
	return &types.EmptyResp{Success: true}, nil
}

func applyTranscodeCallback(media *model.Media, req *types.MediaCallbackReq) {
	if req.ProviderTaskId != "" {
		media.ProviderTaskId = req.ProviderTaskId
	}
	media.Status = req.Status
	if req.PlaybackUrl != "" {
		media.PlaybackUrl = req.PlaybackUrl
	}
	if req.CoverUrl != "" {
		media.CoverUrl = req.CoverUrl
	}
	if req.Duration > 0 {
		media.Duration = req.Duration
	}
	media.ErrorMessage = req.ErrorMessage
}

func AuditCallback(ctx context.Context, svcCtx *svc.ServiceContext, req *types.AuditCallbackReq) (*types.EmptyResp, error) {
	if req.MediaId <= 0 || (req.AuditStatus != "approved" && req.AuditStatus != "rejected") {
		return nil, common.ErrParam
	}
	if err := svcCtx.MediaModel.UpdateAudit(ctx, req.MediaId, req.AuditStatus, req.Reason); err != nil {
		return nil, common.ErrSystem
	}
	return &types.EmptyResp{Success: true}, nil
}

func validMediaType(value string) bool {
	return value == "image" || value == "video" || value == "audio"
}

func validTranscodeStatus(value string) bool {
	return value == "processing" || value == "ready" || value == "failed"
}

func mapNotFound(err error) error {
	if err == sqlx.ErrNotFound {
		return common.NewBizError(40420, "媒体不存在")
	}
	return common.ErrSystem
}

func toMedia(value *model.Media) types.Media {
	return types.Media{
		Id: value.Id, MediaNo: value.MediaNo, OwnerId: value.OwnerId, MediaType: value.MediaType,
		ContentType: value.ContentType, FileName: value.FileName, ObjectName: value.ObjectName,
		Provider: value.Provider, ProviderTaskId: value.ProviderTaskId, Status: value.Status,
		AuditStatus: value.AuditStatus, PlaybackUrl: value.PlaybackUrl, CoverUrl: value.CoverUrl,
		CoverMediaId: value.CoverMediaId, Duration: value.Duration, FileSize: value.FileSize,
		ErrorMessage: value.ErrorMessage, CreateTime: value.CreateTime, UpdateTime: value.UpdateTime,
	}
}
