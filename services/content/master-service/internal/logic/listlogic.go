package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/askxuan/master-service/internal/model"
	"github.com/askxuan/master-service/internal/svc"
	"github.com/askxuan/master-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ListLogic 法师列表查询逻辑
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

// List 法师列表查询，支持按宗派(sect)/类型(type)/寺院(templeId)筛选 + 分页
func (l *ListLogic) List(req *types.ListReq) (*types.ListResp, error) {
	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	list, total, err := l.svcCtx.MasterModel.FindCList(l.ctx, req.BeliefCode, req.Sect, req.Type, req.TempleId, req.ManageBy, page, size)
	if err != nil {
		return nil, err
	}

	out := make([]types.Master, 0, len(list))
	for _, m := range list {
		out = append(out, l.toTypeMaster(m))
	}

	return &types.ListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

// toTypeMaster 将 model.Master 转为 types.Master
// model.Master.Code → types.Master.Id（业务编码 M001）
// model.Master.Specialties（逗号分隔）→ []string
func (l *ListLogic) toTypeMaster(m *model.Master) types.Master {
	return toTypeMasterWithTempleName(m, l.templeName(m.TempleCode))
}

func toTypeMaster(m *model.Master) types.Master {
	return toTypeMasterWithTempleName(m, "")
}

func toTypeMasterWithTempleName(m *model.Master, templeName string) types.Master {
	return types.Master{
		Id:                     m.Code,
		DharmaName:             m.DharmaName,
		LayName:                m.LayName,
		TempleId:               m.TempleCode,
		TempleName:             templeName,
		Position:               m.Position,
		BeliefCode:             m.BeliefCode,
		Sect:                   m.Sect,
		Type:                   m.Type,
		AuthStatus:             m.AuthStatus,
		ShelfStatus:            m.ShelfStatus,
		PlatformStatus:         m.PlatformStatus,
		ManageBy:               m.ManageBy,
		Specialties:            splitSpecialties(m.Specialties),
		Avatar:                 m.Avatar,
		Rating:                 m.Rating,
		ConsultEnabled:         m.ConsultEnabled,
		ConsultFee:             m.ConsultFee,
		ConsultValidHours:      m.ConsultValidHours,
		ConsultResponseMinutes: m.ConsultResponseMinutes,
	}
}

func (l *ListLogic) templeName(templeCode string) string {
	if templeCode == "" {
		return ""
	}
	name, err := l.svcCtx.MasterModel.FindTempleNameByCode(l.ctx, templeCode)
	if err != nil {
		if !errors.Is(err, sqlx.ErrNotFound) {
			l.Errorf("查询法师所属寺院失败: templeCode=%s err=%v", templeCode, err)
		}
		return ""
	}
	return name
}

// splitSpecialties 将逗号分隔的专长字符串转为切片
func splitSpecialties(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// joinSpecialties 将专长切片转为逗号分隔字符串
func joinSpecialties(ss []string) string {
	return strings.Join(ss, ",")
}
