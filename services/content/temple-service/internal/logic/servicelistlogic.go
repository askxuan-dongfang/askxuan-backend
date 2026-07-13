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

// ServiceListLogic C端寺院服务列表查询逻辑
type ServiceListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceListLogic {
	return &ServiceListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// ServiceList 查询指定寺院的上架服务列表
// C端仅返回状态为「正常」的寺院的上架服务
func (l *ServiceListLogic) ServiceList(req *types.TempleServiceListReq) (*types.TempleServiceListResp, error) {
	// 校验寺院存在且状态正常
	t, err := l.svcCtx.TempleModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrTempleNotFound
		}
		l.Errorf("查询寺院失败: %v", err)
		return nil, common.ErrSystem
	}
	if t.Status != model.TempleStatusNormal {
		return nil, common.ErrTempleNotFound
	}

	records, err := l.svcCtx.TempleServiceModel.FindByTempleId(l.ctx, t.Code)
	if err != nil {
		l.Errorf("查询寺院服务列表失败: %v", err)
		return nil, common.ErrSystem
	}
	// 仅返回上架服务
	list := make([]types.TempleService, 0, len(records))
	for _, s := range records {
		if s.Status != model.TempleServiceStatusOnShelf {
			continue
		}
		item := toTypeTempleService(s)
		if tags, tagErr := l.svcCtx.TempleServiceModel.FindIntentTags(l.ctx, s.Id); tagErr == nil {
			item.IntentTags = tags
		}
		list = append(list, item)
	}
	return &types.TempleServiceListResp{List: list}, nil
}
