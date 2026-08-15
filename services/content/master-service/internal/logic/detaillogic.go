package logic

import (
	"context"
	"errors"

	"github.com/askxuan/common"
	"github.com/askxuan/master-service/internal/model"
	"github.com/askxuan/master-service/internal/svc"
	"github.com/askxuan/master-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// DetailLogic 法师详情查询逻辑
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

// Detail 按 ID（法师编码 code）查询法师详情
func (l *DetailLogic) Detail(req *types.DetailReq) (*types.Master, error) {
	m, err := l.svcCtx.MasterModel.FindByCode(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrMasterNotFound
		}
		return nil, err
	}
	if !isPublicMaster(m) {
		return nil, common.ErrMasterNotFound
	}
	resp := toTypeMaster(m)
	if tags, err := l.svcCtx.MasterModel.ListServiceTagsByMaster(l.ctx, m.Code); err == nil {
		for _, t := range tags {
			resp.ServiceTags = append(resp.ServiceTags, types.MasterServiceTagItem{
				ServiceCode: t.ServiceCode,
				Price:       t.Price,
			})
		}
	}
	return &resp, nil
}

func isPublicMaster(master *model.Master) bool {
	return master != nil &&
		master.AuthStatus == model.MasterAuthStatusVerified &&
		master.ShelfStatus == model.MasterShelfStatusOnShelf &&
		master.PlatformStatus == model.MasterPlatformStatusNormal
}
