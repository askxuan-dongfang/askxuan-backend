package logic

import (
	"context"
	"errors"

	"github.com/askxuan/common"
	"github.com/askxuan/temple-service/internal/model"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/askxuan/temple-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// DetailLogic 寺院详情查询逻辑
type DetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DetailLogic {
	return &DetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Detail 按寺院编码(code)查询寺院信息
// C端仅返回正常或推荐状态的寺院，封禁/待审核寺院不对外展示。
func (l *DetailLogic) Detail(req *types.DetailReq) (*types.TempleDetail, error) {
	t, err := l.svcCtx.TempleModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrTempleNotFound
		}
		l.Errorf("查询寺院详情失败: %v", err)
		return nil, common.ErrSystem
	}
	if !model.IsTemplePublicStatus(t.Status) {
		return nil, common.ErrTempleNotFound
	}

	images, err := l.svcCtx.TempleImageModel.FindByTempleCode(l.ctx, t.Code)
	if err != nil {
		l.Errorf("查询寺院图片失败: templeCode=%s err=%v", t.Code, err)
		return nil, common.ErrSystem
	}
	services, err := l.svcCtx.TempleServiceModel.FindByTempleId(l.ctx, t.Code)
	if err != nil {
		l.Errorf("查询寺院服务失败: templeCode=%s err=%v", t.Code, err)
		return nil, common.ErrSystem
	}

	imageItems := make([]types.TempleImage, 0, len(images))
	for _, image := range images {
		imageItems = append(imageItems, types.TempleImage{
			Id: image.Id, TempleCode: image.TempleCode, Url: image.Url,
			Type: image.Type, Sort: image.Sort, CreateTime: image.CreateTime,
		})
	}

	publicServices := publicTempleServiceRecords(services)
	serviceItems := make([]types.TempleService, 0, len(publicServices))
	for _, service := range publicServices {
		item := toTypeTempleService(service)
		if tags, tagErr := l.svcCtx.TempleServiceModel.FindIntentTags(l.ctx, service.Id); tagErr == nil {
			item.IntentTags = tags
		}
		serviceItems = append(serviceItems, item)
	}

	return &types.TempleDetail{
		Temple:   withTempleServiceSummary(toTypeTemple(t), services),
		Images:   imageItems,
		Services: serviceItems,
	}, nil
}

func publicTempleServiceRecords(services []*model.TempleServiceRecord) []*model.TempleServiceRecord {
	public := make([]*model.TempleServiceRecord, 0, len(services))
	for _, service := range services {
		if service != nil && service.Status == model.TempleServiceStatusOnShelf {
			public = append(public, service)
		}
	}
	return public
}
