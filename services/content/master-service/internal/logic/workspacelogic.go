package logic

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/master-service/internal/mq"
	"github.com/askxuan/master-service/internal/model"
	"github.com/askxuan/master-service/internal/svc"
	"github.com/askxuan/master-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 法师工作台 - 加持任务 Logic（修复 Gap-4/15） ============

// WorkspaceBlessingTaskListLogic 加持任务列表
type WorkspaceBlessingTaskListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceBlessingTaskListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceBlessingTaskListLogic {
	return &WorkspaceBlessingTaskListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WorkspaceBlessingTaskList 法师查询自己的加持任务列表
func (l *WorkspaceBlessingTaskListLogic) WorkspaceBlessingTaskList(req *types.BlessingTaskListReq) (*types.BlessingTaskListResp, error) {
	masterCode, err := l.currentMasterCode()
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

	list, total, err := l.svcCtx.BlessingTaskModel.FindByMasterId(l.ctx, masterCode, req.Status, page, size)
	if err != nil {
		return nil, err
	}

	out := make([]types.BlessingTask, 0, len(list))
	for _, t := range list {
		out = append(out, toTypeBlessingTask(t))
	}
	return &types.BlessingTaskListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

// WorkspaceBlessingTaskDetailLogic 加持任务详情
type WorkspaceBlessingTaskDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceBlessingTaskDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceBlessingTaskDetailLogic {
	return &WorkspaceBlessingTaskDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WorkspaceBlessingTaskDetail 查询加持任务详情（校验法师归属）
func (l *WorkspaceBlessingTaskDetailLogic) WorkspaceBlessingTaskDetail(req *types.BlessingTaskDetailReq) (*types.BlessingTask, error) {
	masterCode, err := l.currentMasterCode()
	if err != nil {
		return nil, err
	}
	task, err := l.svcCtx.BlessingTaskModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBlessingNotFound
		}
		return nil, err
	}
	if task.MasterCode != masterCode {
		return nil, common.ErrMasterIsolation
	}
	resp := toTypeBlessingTask(task)
	return &resp, nil
}

// WorkspaceBlessingAcceptLogic 接受任务（assigned → accepted，修复 Gap-15）
type WorkspaceBlessingAcceptLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceBlessingAcceptLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceBlessingAcceptLogic {
	return &WorkspaceBlessingAcceptLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WorkspaceBlessingAccept 法师接受加持任务
func (l *WorkspaceBlessingAcceptLogic) WorkspaceBlessingAccept(req *types.BlessingTaskActionReq) (*types.BlessingTask, error) {
	return transitBlessingTask(l.ctx, l.svcCtx, req.Id, model.BlessingTaskStatusAccepted)
}

// WorkspaceBlessingStartLogic 开始加持（accepted → in_progress，修复 Gap-15）
type WorkspaceBlessingStartLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceBlessingStartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceBlessingStartLogic {
	return &WorkspaceBlessingStartLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WorkspaceBlessingStart 法师开始执行加持
func (l *WorkspaceBlessingStartLogic) WorkspaceBlessingStart(req *types.BlessingTaskActionReq) (*types.BlessingTask, error) {
	return transitBlessingTask(l.ctx, l.svcCtx, req.Id, model.BlessingTaskStatusInProgress)
}

// WorkspaceBlessingRejectLogic 拒绝任务（assigned → rejected，修复 Gap-15）
type WorkspaceBlessingRejectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceBlessingRejectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceBlessingRejectLogic {
	return &WorkspaceBlessingRejectLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WorkspaceBlessingReject 法师拒绝加持任务
func (l *WorkspaceBlessingRejectLogic) WorkspaceBlessingReject(req *types.BlessingTaskActionReq) (*types.BlessingTask, error) {
	return transitBlessingTask(l.ctx, l.svcCtx, req.Id, model.BlessingTaskStatusRejected)
}

// transitBlessingTask 通用加持任务状态流转（校验归属 + 状态机 + 更新）
func transitBlessingTask(ctx context.Context, svcCtx *svc.ServiceContext, id int64, to string) (*types.BlessingTask, error) {
	masterCode, err := currentMasterCode(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	task, err := svcCtx.BlessingTaskModel.FindOne(ctx, id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBlessingNotFound
		}
		return nil, err
	}
	if task.MasterCode != masterCode {
		return nil, common.ErrMasterIsolation
	}
	if !model.CanTransitBlessingTask(task.Status, to) {
		return nil, common.ErrStatusInvalid
	}
	if err := svcCtx.BlessingTaskModel.UpdateStatus(ctx, id, to); err != nil {
		return nil, err
	}
	task.Status = to
	resp := toTypeBlessingTask(task)
	return &resp, nil
}

// WorkspaceBlessingCompleteLogic 完成加持（in_progress → completed，修复 Gap-4）
type WorkspaceBlessingCompleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceBlessingCompleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceBlessingCompleteLogic {
	return &WorkspaceBlessingCompleteLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WorkspaceBlessingComplete 法师完成加持（上传凭证 + 发 blessing.complete MQ）
func (l *WorkspaceBlessingCompleteLogic) WorkspaceBlessingComplete(req *types.BlessingTaskCompleteReq) (*types.BlessingTask, error) {
	masterCode, err := l.currentMasterCode()
	if err != nil {
		return nil, err
	}
	task, err := l.svcCtx.BlessingTaskModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBlessingNotFound
		}
		return nil, err
	}
	if task.MasterCode != masterCode {
		return nil, common.ErrMasterIsolation
	}
	if !model.CanTransitBlessingTask(task.Status, model.BlessingTaskStatusCompleted) {
		return nil, common.ErrStatusInvalid
	}

	certJSON := marshalStrSlice(req.CertificateUrls)
	if err := l.svcCtx.BlessingTaskModel.UpdateComplete(l.ctx, req.Id, certJSON); err != nil {
		return nil, err
	}

	// 发布 blessing.complete 事件通知 diy-service / temple-service
	if l.svcCtx.MqProducer != nil {
		_ = l.svcCtx.MqProducer.PublishBlessingComplete(l.ctx, mq.BlessingComplete{
			TaskNo:     task.TaskNo,
			DiyOrderId: task.DiyOrderNo,
			TempleCode: task.TempleCode,
			MasterCode: task.MasterCode,
			Status:     model.BlessingTaskStatusCompleted,
			Time:       time.Now().Format("2006-01-02 15:04:05"),
		})
	}

	task.Status = model.BlessingTaskStatusCompleted
	task.CertificateUrls = req.CertificateUrls
	resp := toTypeBlessingTask(task)
	return &resp, nil
}

// ============ 法师工作台 - 日程管理 Logic ============

// WorkspaceScheduleListLogic 日程列表
type WorkspaceScheduleListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceScheduleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceScheduleListLogic {
	return &WorkspaceScheduleListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WorkspaceScheduleList 查询排班日历
func (l *WorkspaceScheduleListLogic) WorkspaceScheduleList(req *types.ScheduleListReq) (*types.ScheduleListResp, error) {
	masterCode, err := l.currentMasterCode()
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

	list, total, err := l.svcCtx.MasterScheduleModel.FindByMasterId(l.ctx, masterCode, req.Date, page, size)
	if err != nil {
		return nil, err
	}

	out := make([]types.MasterSchedule, 0, len(list))
	for _, s := range list {
		out = append(out, toTypeSchedule(s))
	}
	return &types.ScheduleListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

// WorkspaceScheduleUpdateLogic 更新日程
type WorkspaceScheduleUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceScheduleUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceScheduleUpdateLogic {
	return &WorkspaceScheduleUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WorkspaceScheduleUpdate 更新排班（按 master_code + date upsert）
func (l *WorkspaceScheduleUpdateLogic) WorkspaceScheduleUpdate(req *types.ScheduleUpdateReq) (*types.ScheduleUpdateResp, error) {
	masterCode, err := l.currentMasterCode()
	if err != nil {
		return nil, err
	}
	status := req.Status
	if status == "" {
		status = model.ScheduleStatusAvailable
	}

	id, err := l.svcCtx.MasterScheduleModel.Upsert(l.ctx, &model.MasterSchedule{
		MasterCode:   masterCode,
		ScheduleDate: req.Date,
		TimeSlots:    marshalStrSlice(req.TimeSlots),
		Status:       status,
	})
	if err != nil {
		return nil, err
	}
	return &types.ScheduleUpdateResp{Id: id}, nil
}

// ============ 工作台共享辅助 ============

// currentMasterCode 从 JWT 取 masterId，查询法师编码
func currentMasterCode(ctx context.Context, svcCtx *svc.ServiceContext) (string, error) {
	masterId := middleware.MasterIDFromCtx(ctx)
	if masterId == 0 {
		return "", common.ErrUnauthorized
	}
	master, err := svcCtx.MasterModel.FindOne(ctx, masterId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return "", common.ErrMasterNotFound
		}
		return "", err
	}
	return master.Code, nil
}

// currentMasterCode 方法版本，供各 Logic 复用
func (l *WorkspaceBlessingTaskListLogic) currentMasterCode() (string, error) {
	return currentMasterCode(l.ctx, l.svcCtx)
}
func (l *WorkspaceBlessingTaskDetailLogic) currentMasterCode() (string, error) {
	return currentMasterCode(l.ctx, l.svcCtx)
}
func (l *WorkspaceBlessingAcceptLogic) currentMasterCode() (string, error) {
	return currentMasterCode(l.ctx, l.svcCtx)
}
func (l *WorkspaceBlessingCompleteLogic) currentMasterCode() (string, error) {
	return currentMasterCode(l.ctx, l.svcCtx)
}
func (l *WorkspaceScheduleListLogic) currentMasterCode() (string, error) {
	return currentMasterCode(l.ctx, l.svcCtx)
}
func (l *WorkspaceScheduleUpdateLogic) currentMasterCode() (string, error) {
	return currentMasterCode(l.ctx, l.svcCtx)
}

// toTypeBlessingTask 将 model.BlessingTask 转为 types.BlessingTask
func toTypeBlessingTask(t *model.BlessingTask) types.BlessingTask {
	return types.BlessingTask{
		Id:              t.Id,
		TaskNo:          t.TaskNo,
		DiyOrderNo:      t.DiyOrderNo,
		TempleCode:      t.TempleCode,
		MasterCode:      t.MasterCode,
		Status:          t.Status,
		CertificateUrls: t.CertificateUrls,
		AssignTime:      t.AssignTime,
		CompleteTime:    t.CompleteTime,
		CreateTime:      t.CreateTime,
	}
}

// toTypeSchedule 将 model.MasterSchedule 转为 types.MasterSchedule
func toTypeSchedule(s *model.MasterSchedule) types.MasterSchedule {
	return types.MasterSchedule{
		Id:         s.Id,
		MasterCode: s.MasterCode,
		Date:       s.ScheduleDate,
		TimeSlots:  unmarshalStrSlice(s.TimeSlots),
		Status:     s.Status,
		CreateTime: s.CreateTime,
	}
}

// marshalStrSlice 将字符串切片序列化为 JSON 数组字符串（DB 存储）
func marshalStrSlice(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	b, err := json.Marshal(ss)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// unmarshalStrSlice 将 JSON 数组字符串反序列化为字符串切片
func unmarshalStrSlice(s string) []string {
	if s == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return []string{}
	}
	return out
}
