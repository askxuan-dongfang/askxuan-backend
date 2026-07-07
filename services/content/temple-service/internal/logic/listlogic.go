package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/temple-service/internal/model"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/askxuan/temple-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ListLogic 寺院列表查询逻辑
type ListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogic {
	return &ListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// List 寺院列表查询，支持按宗派(sect)/类型(type)/地区(region)/服务(serviceCode)筛选 + 分页
// C端仅返回状态为「正常」的寺院
func (l *ListLogic) List(req *types.ListReq) (*types.ListResp, error) {
	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	list, total, err := l.svcCtx.TempleModel.FindList(l.ctx, model.TempleFilter{
		Sect:   req.Sect,
		Type:   req.Type,
		Region: req.Region,
		Status: model.TempleStatusNormal,
	}, page, size)
	if err != nil {
		l.Errorf("查询寺院列表失败: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.Temple, 0, len(list))
	for _, t := range list {
		services, err := l.svcCtx.TempleServiceModel.FindByTempleId(l.ctx, t.Code)
		if err != nil {
			l.Errorf("查询寺院服务失败: templeCode=%s err=%v", t.Code, err)
			return nil, common.ErrSystem
		}
		if req.ServiceCode != "" && !hasOnShelfService(services, req.ServiceCode) {
			continue
		}
		out = append(out, withTempleServiceSummary(toTypeTemple(t), services))
	}
	if req.ServiceCode != "" {
		total = int64(len(out))
	}
	return &types.ListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

func hasOnShelfService(services []*model.TempleServiceRecord, serviceCode string) bool {
	for _, s := range services {
		if s.Status == model.TempleServiceStatusOnShelf && s.ServiceCode == serviceCode {
			return true
		}
	}
	return false
}
