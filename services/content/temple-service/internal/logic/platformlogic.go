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

// ============ 平台管理台 Logic（平台超管审核寺院入驻） ============

// PlatformTempleListLogic 全部寺院列表（含待审核/封禁等非常规状态）
type PlatformTempleListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformTempleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformTempleListLogic {
	return &PlatformTempleListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// PlatformTempleList 平台查询所有寺院（status 为空时不过滤状态）
func (l *PlatformTempleListLogic) PlatformTempleList(req *types.ListReq) (*types.ListResp, error) {
	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	// 平台视角：不强制 status=正常，filter.Status 留空以返回全部
	list, total, err := l.svcCtx.TempleModel.FindList(l.ctx, model.TempleFilter{
		BeliefCode: req.BeliefCode,
		Sect:       req.Sect,
		Type:       req.Type,
		Region:     req.Region,
	}, page, size)
	if err != nil {
		l.Errorf("平台查询寺院列表失败: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.Temple, 0, len(list))
	for _, t := range list {
		services, err := l.svcCtx.TempleServiceModel.FindByTempleId(l.ctx, t.Code)
		if err != nil {
			l.Errorf("平台查询寺院服务失败: templeCode=%s err=%v", t.Code, err)
			return nil, common.ErrSystem
		}
		out = append(out, withTempleServiceSummary(toTypeTemple(t), services))
	}
	return &types.ListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

// PlatformTempleDetail 返回平台管理视角的寺院详情，包含待审核和封禁寺院。
func PlatformTempleDetail(ctx context.Context, svcCtx *svc.ServiceContext, req *types.DetailReq) (*types.TempleDetail, error) {
	temple, err := svcCtx.TempleModel.FindOne(ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrTempleNotFound
		}
		return nil, common.ErrSystem
	}

	images, err := svcCtx.TempleImageModel.FindByTempleCode(ctx, temple.Code)
	if err != nil {
		return nil, common.ErrSystem
	}
	services, err := svcCtx.TempleServiceModel.FindByTempleId(ctx, temple.Code)
	if err != nil {
		return nil, common.ErrSystem
	}

	imageItems := make([]types.TempleImage, 0, len(images))
	for _, image := range images {
		imageItems = append(imageItems, types.TempleImage{
			Id:         image.Id,
			TempleCode: image.TempleCode,
			Url:        image.Url,
			Type:       image.Type,
			Sort:       image.Sort,
			CreateTime: image.CreateTime,
		})
	}
	serviceItems := make([]types.TempleService, 0, len(services))
	for _, service := range services {
		item := toTypeTempleService(service)
		if tags, tagErr := svcCtx.TempleServiceModel.FindIntentTags(ctx, service.Id); tagErr == nil {
			item.IntentTags = tags
		}
		serviceItems = append(serviceItems, item)
	}
	item := withTempleServiceSummary(toTypeTemple(temple), services)
	return &types.TempleDetail{Temple: item, Images: imageItems, Services: serviceItems}, nil
}

// PlatformAuditListLogic 入驻审核列表
type PlatformAuditListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformAuditListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformAuditListLogic {
	return &PlatformAuditListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// PlatformAuditList 查询入驻审核列表，支持按 status 筛选 + 分页
func (l *PlatformAuditListLogic) PlatformAuditList(req *types.TempleAuditListReq) (*types.TempleAuditListResp, error) {
	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	list, total, err := l.svcCtx.TempleAuditModel.FindList(l.ctx, req.TempleCode, req.Status, page, size)
	if err != nil {
		l.Errorf("查询入驻审核列表失败: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.TempleAudit, 0, len(list))
	for _, a := range list {
		out = append(out, toTypeAudit(a))
	}
	return &types.TempleAuditListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

// PlatformAuditFirstPassLogic 初审通过（pending → first_pass）
type PlatformAuditFirstPassLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformAuditFirstPassLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformAuditFirstPassLogic {
	return &PlatformAuditFirstPassLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// PlatformAuditFirstPass 初审通过
func (l *PlatformAuditFirstPassLogic) PlatformAuditFirstPass(req *types.TempleAuditActionReq) (*types.TempleAudit, error) {
	return transitAuditStatus(l.Logger, l.ctx, l.svcCtx, req.Id, model.TempleAuditStatusFirstPass, req.AuditRemark)
}

// PlatformAuditFinalPassLogic 终审通过（first_pass → final_pass，同时将寺院状态置为正常）
type PlatformAuditFinalPassLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformAuditFinalPassLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformAuditFinalPassLogic {
	return &PlatformAuditFinalPassLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// PlatformAuditFinalPass 终审通过：更新审核状态 + 寺院状态置为正常
func (l *PlatformAuditFinalPassLogic) PlatformAuditFinalPass(req *types.TempleAuditActionReq) (*types.TempleAudit, error) {
	auditorId := svc.UserIDFromCtx(l.ctx)

	audit, err := l.svcCtx.TempleAuditModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrTempleNotFound
		}
		l.Errorf("查询审核记录失败: %v", err)
		return nil, common.ErrSystem
	}
	if !model.CanTransitTempleAudit(audit.Status, model.TempleAuditStatusFinalPass) {
		return nil, common.ErrStatusInvalid
	}

	if err := l.svcCtx.TempleAuditModel.FinalPass(l.ctx, req.Id, audit.TempleCode, auditorId, req.AuditRemark); err != nil {
		l.Errorf("终审通过事务失败: %v", err)
		return nil, common.ErrSystem
	}

	// 返回更新后的审核记录。
	updated, err := l.svcCtx.TempleAuditModel.FindOne(l.ctx, req.Id)
	if err != nil {
		l.Errorf("终审通过-重新查询审核记录失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := toTypeAudit(updated)
	return &resp, nil
}

// PlatformAuditRejectLogic 审核驳回（pending/first_pass → rejected）
type PlatformAuditRejectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformAuditRejectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformAuditRejectLogic {
	return &PlatformAuditRejectLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// PlatformAuditReject 驳回审核申请
func (l *PlatformAuditRejectLogic) PlatformAuditReject(req *types.TempleAuditActionReq) (*types.TempleAudit, error) {
	return transitAuditStatus(l.Logger, l.ctx, l.svcCtx, req.Id, model.TempleAuditStatusRejected, req.AuditRemark)
}

// PlatformTempleStatusLogic 封禁/推荐/恢复正常寺院状态
type PlatformTempleStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformTempleStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformTempleStatusLogic {
	return &PlatformTempleStatusLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// PlatformTempleStatus 平台更新寺院运营状态：normal/banned/recommended。
func (l *PlatformTempleStatusLogic) PlatformTempleStatus(req *types.TemplePlatformStatusReq) (*types.TemplePlatformStatusResp, error) {
	targetStatus, ok := normalizeTemplePlatformStatus(req.Status)
	if !ok {
		return nil, common.ErrParamInvalid
	}

	t, err := l.svcCtx.TempleModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrTempleNotFound
		}
		l.Errorf("平台查询寺院失败: %v", err)
		return nil, common.ErrSystem
	}
	// 待审核寺院必须通过入驻审核状态机，不允许用运营状态接口绕过审核。
	if t.Status == model.TempleStatusPending && targetStatus != model.TempleStatusBanned {
		return nil, common.ErrStatusInvalid
	}
	if err := l.svcCtx.TempleModel.UpdateStatus(l.ctx, t.Id, targetStatus); err != nil {
		l.Errorf("平台更新寺院状态失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.TemplePlatformStatusResp{
		Id:     t.Code,
		Status: targetStatus,
	}, nil
}

func normalizeTemplePlatformStatus(status string) (string, bool) {
	switch status {
	case "normal", model.TempleStatusNormal:
		return model.TempleStatusNormal, true
	case "banned", model.TempleStatusBanned:
		return model.TempleStatusBanned, true
	case "recommended", model.TempleStatusRecommend:
		return model.TempleStatusRecommend, true
	default:
		return "", false
	}
}

// ============ 通用辅助方法 ============

// transitAuditStatus 审核状态流转通用逻辑：校验 → 更新状态 → 返回最新记录
func transitAuditStatus(
	l logx.Logger,
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	id int64, targetStatus, remark string,
) (*types.TempleAudit, error) {
	auditorId := svc.UserIDFromCtx(ctx)

	audit, err := svcCtx.TempleAuditModel.FindOne(ctx, id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrTempleNotFound
		}
		l.Errorf("查询审核记录失败: %v", err)
		return nil, common.ErrSystem
	}
	if !model.CanTransitTempleAudit(audit.Status, targetStatus) {
		return nil, common.ErrStatusInvalid
	}
	if err := svcCtx.TempleAuditModel.UpdateStatus(ctx, id, targetStatus, auditorId, remark); err != nil {
		l.Errorf("更新审核状态失败: %v", err)
		return nil, common.ErrSystem
	}

	updated, err := svcCtx.TempleAuditModel.FindOne(ctx, id)
	if err != nil {
		l.Errorf("重新查询审核记录失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := toTypeAudit(updated)
	return &resp, nil
}
