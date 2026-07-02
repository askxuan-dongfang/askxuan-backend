package logic

import (
	"context"
	"errors"

	"github.com/askxuan/common"
	"github.com/askxuan/temple-service/internal/mq"
	"github.com/askxuan/temple-service/internal/model"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/askxuan/temple-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 寺院管理台 Logic（修复 Gap-3 加持任务接收与分配） ============

// ---------- 寺院信息 ----------

// AdminTempleInfoLogic 本寺院信息
type AdminTempleInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminTempleInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminTempleInfoLogic {
	return &AdminTempleInfoLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminTempleInfoLogic) AdminTempleInfo(_ *types.DetailReq) (*types.Temple, error) {
	t, err := getCurrentTemple(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp := toTypeTemple(t)
	return &resp, nil
}

// AdminTempleUpdateLogic 编辑寺院信息
type AdminTempleUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminTempleUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminTempleUpdateLogic {
	return &AdminTempleUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminTempleUpdateLogic) AdminTempleUpdate(req *types.TempleUpdateReq) (*types.Temple, error) {
	t, err := getCurrentTemple(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	// 仅更新允许编辑的字段
	if req.Name != "" {
		t.Name = req.Name
	}
	if req.Region != "" {
		t.Region = req.Region
	}
	if req.Address != "" {
		t.Address = req.Address
	}
	if req.CoverImage != "" {
		t.CoverImage = req.CoverImage
	}
	if req.Description != "" {
		t.Description = req.Description
	}
	if err := l.svcCtx.TempleModel.Update(l.ctx, t); err != nil {
		l.Errorf("更新寺院信息失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := toTypeTemple(t)
	return &resp, nil
}

// ---------- 图片管理 ----------

// AdminImageCreateLogic 上传图片
type AdminImageCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminImageCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminImageCreateLogic {
	return &AdminImageCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminImageCreateLogic) AdminImageCreate(req *types.TempleImageCreateReq) (*types.TempleImageCreateResp, error) {
	t, err := getCurrentTemple(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	// 校验图片类型
	if req.Type != model.ImageTypeCover && req.Type != model.ImageTypeDetail && req.Type != model.ImageTypeHero {
		return nil, common.ErrParamInvalid
	}
	// 校验配额：cover/hero 单张，detail 最多9张
	if req.Type == model.ImageTypeCover || req.Type == model.ImageTypeHero {
		count, err := l.svcCtx.TempleImageModel.CountByTempleCodeAndType(l.ctx, t.Code, req.Type)
		if err != nil {
			l.Errorf("查询图片数量失败: %v", err)
			return nil, common.ErrSystem
		}
		if count > 0 {
			return nil, common.NewBizError(40904, "该类型图片已达上限（仅允许1张）")
		}
	} else if req.Type == model.ImageTypeDetail {
		count, err := l.svcCtx.TempleImageModel.CountByTempleCodeAndType(l.ctx, t.Code, req.Type)
		if err != nil {
			l.Errorf("查询图片数量失败: %v", err)
			return nil, common.ErrSystem
		}
		if count >= 9 {
			return nil, common.NewBizError(40904, "详情图最多9张")
		}
	}

	id, err := l.svcCtx.TempleImageModel.Insert(l.ctx, &model.TempleImage{
		TempleCode: t.Code,
		Url:        req.Url,
		Type:       req.Type,
		Sort:       req.Sort,
	})
	if err != nil {
		l.Errorf("新增寺院图片失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.TempleImageCreateResp{Id: id}, nil
}

// AdminImageDeleteLogic 删除图片
type AdminImageDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminImageDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminImageDeleteLogic {
	return &AdminImageDeleteLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminImageDeleteLogic) AdminImageDelete(req *types.TempleImageDeleteReq) error {
	t, err := getCurrentTemple(l.ctx, l.svcCtx)
	if err != nil {
		return err
	}
	// 校验图片归属本寺院
	img, err := l.svcCtx.TempleImageModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return common.ErrTempleNotFound
		}
		l.Errorf("查询图片失败: %v", err)
		return common.ErrSystem
	}
	if img.TempleCode != t.Code {
		return common.ErrTempleIsolation
	}
	if err := l.svcCtx.TempleImageModel.Delete(l.ctx, req.Id); err != nil {
		l.Errorf("删除图片失败: %v", err)
		return common.ErrSystem
	}
	return nil
}

// ---------- 服务管理 ----------

// AdminServiceListLogic 服务列表
type AdminServiceListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminServiceListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminServiceListLogic {
	return &AdminServiceListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminServiceListLogic) AdminServiceList() (*types.TempleServiceListResp, error) {
	t, err := getCurrentTemple(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	records, err := l.svcCtx.TempleServiceModel.FindByTempleId(l.ctx, t.Code)
	if err != nil {
		l.Errorf("查询寺院服务列表失败: %v", err)
		return nil, common.ErrSystem
	}
	list := make([]types.TempleService, 0, len(records))
	for _, s := range records {
		list = append(list, toTypeTempleService(s))
	}
	return &types.TempleServiceListResp{List: list}, nil
}

// AdminServiceCreateLogic 创建服务
type AdminServiceCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminServiceCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminServiceCreateLogic {
	return &AdminServiceCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminServiceCreateLogic) AdminServiceCreate(req *types.TempleServiceCreateReq) (*types.TempleServiceCreateResp, error) {
	t, err := getCurrentTemple(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	if req.ServiceCode == "" || req.ServiceName == "" {
		return nil, common.ErrParam
	}
	id, err := l.svcCtx.TempleServiceModel.Insert(l.ctx, &model.TempleServiceRecord{
		TempleCode:  t.Code,
		ServiceCode: req.ServiceCode,
		ServiceName: req.ServiceName,
		Price:       req.Price,
		TimeSlots:   req.TimeSlots,
	})
	if err != nil {
		l.Errorf("创建寺院服务失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.TempleServiceCreateResp{Id: id}, nil
}

// AdminServiceUpdateLogic 更新服务
type AdminServiceUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminServiceUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminServiceUpdateLogic {
	return &AdminServiceUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminServiceUpdateLogic) AdminServiceUpdate(req *types.TempleServiceUpdateReq) (*types.TempleService, error) {
	t, err := getCurrentTemple(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	record, err := l.svcCtx.TempleServiceModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrTempleNotFound
		}
		l.Errorf("查询寺院服务失败: %v", err)
		return nil, common.ErrSystem
	}
	if record.TempleCode != t.Code {
		return nil, common.ErrTempleIsolation
	}
	// 仅更新非空字段
	if req.ServiceName != "" {
		record.ServiceName = req.ServiceName
	}
	if req.Price > 0 {
		record.Price = req.Price
	}
	if req.TimeSlots != nil {
		record.TimeSlots = req.TimeSlots
	}
	if err := l.svcCtx.TempleServiceModel.Update(l.ctx, record); err != nil {
		l.Errorf("更新寺院服务失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := toTypeTempleService(record)
	return &resp, nil
}

// AdminServiceStatusLogic 服务上下架
type AdminServiceStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminServiceStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminServiceStatusLogic {
	return &AdminServiceStatusLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminServiceStatusLogic) AdminServiceStatus(req *types.TempleServiceStatusReq) (*types.TempleServiceStatusResp, error) {
	t, err := getCurrentTemple(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	if req.Status != model.TempleServiceStatusOnShelf && req.Status != model.TempleServiceStatusOffShelf {
		return nil, common.ErrParamInvalid
	}
	record, err := l.svcCtx.TempleServiceModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrTempleNotFound
		}
		l.Errorf("查询寺院服务失败: %v", err)
		return nil, common.ErrSystem
	}
	if record.TempleCode != t.Code {
		return nil, common.ErrTempleIsolation
	}
	if err := l.svcCtx.TempleServiceModel.UpdateStatus(l.ctx, req.Id, req.Status); err != nil {
		l.Errorf("更新服务状态失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.TempleServiceStatusResp{Id: req.Id, Status: req.Status}, nil
}

// ---------- 加持任务（修复 Gap-3） ----------

// AdminBlessingTaskListLogic 加持任务列表
type AdminBlessingTaskListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBlessingTaskListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBlessingTaskListLogic {
	return &AdminBlessingTaskListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBlessingTaskListLogic) AdminBlessingTaskList(req *types.BlessingTaskListReq) (*types.BlessingTaskListResp, error) {
	t, err := getCurrentTemple(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	list, total, err := l.svcCtx.BlessingTaskModel.FindByTempleId(l.ctx, t.Code, req.Status, page, size)
	if err != nil {
		l.Errorf("查询加持任务列表失败: %v", err)
		return nil, common.ErrSystem
	}
	out := make([]types.BlessingTask, 0, len(list))
	for _, b := range list {
		out = append(out, toTypeBlessingTask(b))
	}
	return &types.BlessingTaskListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

// AdminBlessingTaskDetailLogic 加持任务详情
type AdminBlessingTaskDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBlessingTaskDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBlessingTaskDetailLogic {
	return &AdminBlessingTaskDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBlessingTaskDetailLogic) AdminBlessingTaskDetail(req *types.BlessingTaskDetailReq) (*types.BlessingTask, error) {
	t, err := getCurrentTemple(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	task, err := l.svcCtx.BlessingTaskModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBlessingNotFound
		}
		l.Errorf("查询加持任务详情失败: %v", err)
		return nil, common.ErrSystem
	}
	if task.TempleCode != t.Code {
		return nil, common.ErrTempleIsolation
	}
	resp := toTypeBlessingTask(task)
	return &resp, nil
}

// AdminBlessingAssignLogic 分配法师（dispatched → assigned，修复 Gap-3 核心）
type AdminBlessingAssignLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBlessingAssignLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBlessingAssignLogic {
	return &AdminBlessingAssignLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBlessingAssignLogic) AdminBlessingAssign(req *types.BlessingAssignReq) (*types.BlessingTask, error) {
	t, err := getCurrentTemple(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	if req.MasterCode == "" {
		return nil, common.ErrParam
	}
	// 校验任务归属本寺院
	task, err := l.svcCtx.BlessingTaskModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBlessingNotFound
		}
		l.Errorf("查询加持任务失败: %v", err)
		return nil, common.ErrSystem
	}
	if task.TempleCode != t.Code {
		return nil, common.ErrTempleIsolation
	}
	// 校验状态流转
	if !model.CanTransitBlessingTask(task.Status, model.BlessingTaskStatusAssigned) {
		return nil, common.ErrStatusInvalid
	}
	// 分配法师
	updated, err := l.svcCtx.BlessingTaskModel.Assign(l.ctx, req.Id, req.MasterCode)
	if err != nil {
		l.Errorf("分配法师失败: %v", err)
		return nil, common.ErrSystem
	}
	// 发 blessing.assign MQ 事件通知 master-service（失败不阻断主流程）
	if l.svcCtx.MqProducer != nil {
		if err := l.svcCtx.MqProducer.PublishBlessingAssign(l.ctx, mq.BlessingAssign{
			TaskNo:     updated.TaskNo,
			TempleCode: updated.TempleCode,
			MasterCode: updated.MasterCode,
		}); err != nil {
			l.Errorf("发送 blessing.assign 事件失败: %v", err)
		}
	}
	resp := toTypeBlessingTask(updated)
	return &resp, nil
}

// ---------- 入驻申请 ----------

// AdminTempleApplyLogic 提交入驻申请
type AdminTempleApplyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminTempleApplyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminTempleApplyLogic {
	return &AdminTempleApplyLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminTempleApplyLogic) AdminTempleApply(req *types.TempleApplyReq) (*types.TempleApplyResp, error) {
	if req.TempleCode == "" || req.ApplicantName == "" {
		return nil, common.ErrParam
	}
	id, err := l.svcCtx.TempleAuditModel.Insert(l.ctx, &model.TempleAudit{
		TempleCode:    req.TempleCode,
		ApplicantName: req.ApplicantName,
		ContactPhone:  req.ContactPhone,
		CertUrls:      req.CertUrls,
		Status:        model.TempleAuditStatusPending,
	})
	if err != nil {
		l.Errorf("提交入驻申请失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.TempleApplyResp{Id: id}, nil
}

// ============ 通用辅助方法 ============

// getCurrentTemple 从 JWT context 取 templeId，查询当前寺院
func getCurrentTemple(ctx context.Context, svcCtx *svc.ServiceContext) (*model.Temple, error) {
	templeId := svc.TempleIDFromCtx(ctx)
	if templeId == 0 {
		return nil, common.ErrForbidden
	}
	t, err := svcCtx.TempleModel.FindOneByPk(ctx, templeId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrTempleNotFound
		}
		return nil, common.ErrSystem
	}
	return t, nil
}

// toTypeTemple model.Temple -> types.Temple（types.Id 为 code 字符串）
func toTypeTemple(t *model.Temple) types.Temple {
	return types.Temple{
		Id:          t.Code,
		Name:        t.Name,
		Region:      t.Region,
		Type:        t.Type,
		Sect:        t.Sect,
		Status:      t.Status,
		Address:     t.Address,
		CoverImage:  t.CoverImage,
		Rating:      t.Rating,
		Description: t.Description,
	}
}

// toTypeTempleService model.TempleServiceRecord -> types.TempleService
func toTypeTempleService(s *model.TempleServiceRecord) types.TempleService {
	return types.TempleService{
		Id:          s.Id,
		TempleCode:  s.TempleCode,
		ServiceCode: s.ServiceCode,
		ServiceName: s.ServiceName,
		Price:       s.Price,
		TimeSlots:   s.TimeSlots,
		Status:      s.Status,
		CreateTime:  s.CreateTime,
	}
}

// toTypeBlessingTask model.BlessingTask -> types.BlessingTask
func toTypeBlessingTask(b *model.BlessingTask) types.BlessingTask {
	return types.BlessingTask{
		Id:              b.Id,
		TaskNo:          b.TaskNo,
		DiyOrderNo:      b.DiyOrderNo,
		TempleCode:      b.TempleCode,
		MasterCode:      b.MasterCode,
		Status:          b.Status,
		CertificateUrls: b.CertificateUrls,
		AssignTime:      b.AssignTime,
		CompleteTime:    b.CompleteTime,
		CreateTime:      b.CreateTime,
	}
}

// toTypeAudit model.TempleAudit -> types.TempleAudit
func toTypeAudit(a *model.TempleAudit) types.TempleAudit {
	return types.TempleAudit{
		Id:            a.Id,
		TempleCode:    a.TempleCode,
		ApplicantName: a.ApplicantName,
		ContactPhone:  a.ContactPhone,
		CertUrls:      a.CertUrls,
		Status:        a.Status,
		AuditorId:     a.AuditorId,
		AuditRemark:   a.AuditRemark,
		CreateTime:    a.CreateTime,
	}
}
