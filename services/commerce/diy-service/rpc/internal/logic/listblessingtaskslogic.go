package logic

import (
	"context"

	"github.com/askxuan/diy-service/rpc/diy"
	"github.com/askxuan/diy-service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// ListBlessingTasksLogic 查询加持任务列表（支持按 masterCode/templeCode/status 筛选 + 分页）
type ListBlessingTasksLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBlessingTasksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBlessingTasksLogic {
	return &ListBlessingTasksLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListBlessingTasksLogic) ListBlessingTasks(req *diy.ListBlessingTasksReq) (*diy.ListBlessingTasksResp, error) {
	page := req.Page
	size := req.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}

	list, total, err := l.svcCtx.BlessingTaskModel.FindList(l.ctx, req.MasterCode, req.TempleCode, req.Status, int(page), int(size))
	if err != nil {
		return nil, err
	}

	tasks := make([]*diy.BlessingTask, 0, len(list))
	for _, t := range list {
		tasks = append(tasks, modelTaskToRPC(t))
	}
	return &diy.ListBlessingTasksResp{
		Tasks: tasks,
		Total: total,
	}, nil
}
