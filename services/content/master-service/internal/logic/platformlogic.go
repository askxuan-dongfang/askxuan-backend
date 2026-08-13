package logic

import (
	"context"
	"errors"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/master-service/internal/model"
	"github.com/askxuan/master-service/internal/svc"
	"github.com/askxuan/master-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 平台管理台 - 法师资质审核 Logic ============

// errAuditNotFound 审核记录不存在
var errAuditNotFound = common.NewBizError(40414, "审核记录不存在")

// PlatformMasterListLogic 平台法师全量列表。
type PlatformMasterListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformMasterListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformMasterListLogic {
	return &PlatformMasterListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *PlatformMasterListLogic) PlatformMasterList(req *types.PlatformMasterListReq) (*types.ListResp, error) {
	page, size := req.Page, req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	list, total, err := l.svcCtx.MasterModel.FindPlatformList(l.ctx, req.BeliefCode, req.Sect, req.Type, req.TempleId, req.AuthStatus, req.ShelfStatus, req.PlatformStatus, page, size)
	if err != nil {
		return nil, err
	}
	out := make([]types.Master, 0, len(list))
	for _, master := range list {
		out = append(out, toTypeMasterWithTempleName(master, l.templeName(master.TempleCode)))
	}
	return &types.ListResp{Total: total, List: out, Page: page, Size: size}, nil
}

func (l *PlatformMasterListLogic) templeName(templeCode string) string {
	name, err := l.svcCtx.MasterModel.FindTempleNameByCode(l.ctx, templeCode)
	if err != nil {
		return ""
	}
	return name
}

// PlatformAuditListLogic 资质审核列表
type PlatformAuditListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformAuditListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformAuditListLogic {
	return &PlatformAuditListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// PlatformAuditList 平台查询法师资质审核列表（按 status 筛选 + 分页）
func (l *PlatformAuditListLogic) PlatformAuditList(req *types.MasterAuditListReq) (*types.MasterAuditListResp, error) {
	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	list, total, err := l.svcCtx.MasterAuditModel.FindAuditList(l.ctx, req.Status, page, size)
	if err != nil {
		return nil, err
	}

	out := make([]types.MasterAudit, 0, len(list))
	for _, a := range list {
		out = append(out, toTypeAudit(a))
	}
	return &types.MasterAuditListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

// PlatformAuditPassLogic 审核通过
type PlatformAuditPassLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformAuditPassLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformAuditPassLogic {
	return &PlatformAuditPassLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// PlatformAuditPass 审核通过（pending → pass + 更新法师 AuthStatus 为已认证 + 自动上架）
func (l *PlatformAuditPassLogic) PlatformAuditPass(req *types.MasterAuditActionReq) (*types.MasterAudit, error) {
	audit, err := l.svcCtx.MasterAuditModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, errAuditNotFound
		}
		return nil, err
	}
	if !model.CanTransitMasterAudit(audit.Status, model.MasterAuditStatusPass) {
		return nil, common.ErrStatusInvalid
	}

	auditorId := middleware.UserIDFromCtx(l.ctx)
	if err := l.svcCtx.MasterAuditModel.UpdateStatus(l.ctx, req.Id, model.MasterAuditStatusPass, auditorId, req.AuditRemark); err != nil {
		return nil, err
	}

	// 审核通过：更新法师认证状态为已认证，并自动上架
	if err := l.svcCtx.MasterModel.UpdateAuthStatus(l.ctx, audit.MasterCode, model.MasterAuthStatusVerified); err != nil {
		return nil, err
	}
	master, err := l.svcCtx.MasterModel.FindByCode(l.ctx, audit.MasterCode)
	if err == nil && master != nil {
		_ = l.svcCtx.MasterModel.UpdateShelfStatus(l.ctx, master.Id, model.MasterShelfStatusOnShelf)
	}

	audit.Status = model.MasterAuditStatusPass
	audit.AuditorId = auditorId
	audit.AuditRemark = req.AuditRemark
	resp := toTypeAudit(audit)
	return &resp, nil
}

// PlatformAuditRejectLogic 审核驳回
type PlatformAuditRejectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformAuditRejectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformAuditRejectLogic {
	return &PlatformAuditRejectLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// PlatformAuditReject 审核拒绝（pending → rejected）
func (l *PlatformAuditRejectLogic) PlatformAuditReject(req *types.MasterAuditActionReq) (*types.MasterAudit, error) {
	audit, err := l.svcCtx.MasterAuditModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, errAuditNotFound
		}
		return nil, err
	}
	if !model.CanTransitMasterAudit(audit.Status, model.MasterAuditStatusRejected) {
		return nil, common.ErrStatusInvalid
	}

	auditorId := middleware.UserIDFromCtx(l.ctx)
	if err := l.svcCtx.MasterAuditModel.UpdateStatus(l.ctx, req.Id, model.MasterAuditStatusRejected, auditorId, req.AuditRemark); err != nil {
		return nil, err
	}

	audit.Status = model.MasterAuditStatusRejected
	audit.AuditorId = auditorId
	audit.AuditRemark = req.AuditRemark
	resp := toTypeAudit(audit)
	return &resp, nil
}

// PlatformMasterStatusLogic 封禁/解禁法师
type PlatformMasterStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformMasterStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformMasterStatusLogic {
	return &PlatformMasterStatusLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// PlatformMasterStatus 平台更新法师状态（normal/banned，req.Id 为法师编码 code）
func (l *PlatformMasterStatusLogic) PlatformMasterStatus(req *types.MasterPlatformStatusReq) (*types.MasterPlatformStatusResp, error) {
	if req.Status != model.MasterPlatformStatusNormal && req.Status != model.MasterPlatformStatusBanned {
		return nil, common.ErrParamInvalid
	}
	master, err := l.svcCtx.MasterModel.FindByCode(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrMasterNotFound
		}
		return nil, err
	}
	if err := l.svcCtx.MasterModel.UpdatePlatformStatus(l.ctx, master.Id, req.Status); err != nil {
		return nil, err
	}
	return &types.MasterPlatformStatusResp{Id: req.Id, Status: req.Status}, nil
}

func (l *PlatformMasterStatusLogic) PlatformMasterConsultConfig(req *types.MasterConsultConfigReq) (*types.Master, error) {
	if req.ConsultFee <= 0 || req.ConsultValidHours < 1 || req.ConsultValidHours > 720 || req.ConsultResponseMinutes < 1 || req.ConsultResponseMinutes > 1440 {
		return nil, common.ErrParamInvalid
	}
	master, err := l.svcCtx.MasterModel.FindByCode(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrMasterNotFound
		}
		return nil, err
	}
	if err := l.svcCtx.MasterModel.UpdateConsultConfig(l.ctx, master.Id, req.ConsultEnabled, req.ConsultFee, req.ConsultValidHours, req.ConsultResponseMinutes); err != nil {
		return nil, err
	}
	master.ConsultEnabled, master.ConsultFee = req.ConsultEnabled, req.ConsultFee
	master.ConsultValidHours, master.ConsultResponseMinutes = req.ConsultValidHours, req.ConsultResponseMinutes
	templeName, _ := l.svcCtx.MasterModel.FindTempleNameByCode(l.ctx, master.TempleCode)
	resp := toTypeMasterWithTempleName(master, templeName)
	return &resp, nil
}

// ============ 平台审核共享辅助 ============

// toTypeAudit 将 model.MasterAudit 转为 types.MasterAudit
func toTypeAudit(a *model.MasterAudit) types.MasterAudit {
	return types.MasterAudit{
		Id:             a.Id,
		MasterCode:     a.MasterCode,
		TempleCode:     a.TempleCode,
		CredentialUrls: unmarshalStrSlice(a.CredentialUrls),
		Status:         a.Status,
		AuditorId:      a.AuditorId,
		AuditRemark:    a.AuditRemark,
		CreateTime:     a.CreateTime,
	}
}
