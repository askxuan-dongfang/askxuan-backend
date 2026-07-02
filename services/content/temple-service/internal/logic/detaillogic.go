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
// C端仅返回状态为「正常」的寺院，封禁/待审核寺院不对外展示
func (l *DetailLogic) Detail(req *types.DetailReq) (*types.Temple, error) {
	t, err := l.svcCtx.TempleModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrTempleNotFound
		}
		l.Errorf("查询寺院详情失败: %v", err)
		return nil, common.ErrSystem
	}
	// 非正常状态的寺院对 C端 不可见
	if t.Status != model.TempleStatusNormal {
		return nil, common.ErrTempleNotFound
	}
	resp := toTypeTemple(t)
	return &resp, nil
}
