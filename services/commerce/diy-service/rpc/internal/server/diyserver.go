package server

import (
	"context"

	"github.com/askxuan/diy-service/internal/svc"
	"github.com/askxuan/diy-service/rpc/diy"
	"github.com/askxuan/diy-service/rpc/internal/logic"
)

// diyServer DiyService gRPC 服务端实现
// 每个方法将请求转发到对应 logic 层处理
// 嵌入 UnimplementedDiyServiceServer 以满足 forward compatibility 要求
type diyServer struct {
	diy.UnimplementedDiyServiceServer
	svcCtx *svc.ServiceContext
}

// NewDiyServer 构造 DiyService gRPC 服务端实现
func NewDiyServer(svcCtx *svc.ServiceContext) diy.DiyServiceServer {
	return &diyServer{svcCtx: svcCtx}
}

func (s *diyServer) GetBlessingTask(ctx context.Context, req *diy.GetBlessingTaskReq) (*diy.BlessingTask, error) {
	l := logic.NewGetBlessingTaskLogic(ctx, s.svcCtx)
	return l.GetBlessingTask(req)
}

func (s *diyServer) GetBlessingTaskByOrderNo(ctx context.Context, req *diy.GetBlessingTaskByOrderNoReq) (*diy.BlessingTask, error) {
	l := logic.NewGetBlessingTaskByOrderNoLogic(ctx, s.svcCtx)
	return l.GetBlessingTaskByOrderNo(req)
}

func (s *diyServer) GetBlessingTaskByTaskNo(ctx context.Context, req *diy.GetBlessingTaskByTaskNoReq) (*diy.BlessingTask, error) {
	l := logic.NewGetBlessingTaskByTaskNoLogic(ctx, s.svcCtx)
	return l.GetBlessingTaskByTaskNo(req)
}

func (s *diyServer) ListBlessingTasks(ctx context.Context, req *diy.ListBlessingTasksReq) (*diy.ListBlessingTasksResp, error) {
	l := logic.NewListBlessingTasksLogic(ctx, s.svcCtx)
	return l.ListBlessingTasks(req)
}

func (s *diyServer) UpdateBlessingTaskStatus(ctx context.Context, req *diy.UpdateBlessingTaskStatusReq) (*diy.BlessingTask, error) {
	l := logic.NewUpdateBlessingTaskStatusLogic(ctx, s.svcCtx)
	return l.UpdateBlessingTaskStatus(req)
}

func (s *diyServer) CompleteBlessingTask(ctx context.Context, req *diy.CompleteBlessingTaskReq) (*diy.BlessingTask, error) {
	l := logic.NewCompleteBlessingTaskLogic(ctx, s.svcCtx)
	return l.CompleteBlessingTask(req)
}

func (s *diyServer) AssignBlessingTask(ctx context.Context, req *diy.AssignBlessingTaskReq) (*diy.BlessingTask, error) {
	l := logic.NewAssignBlessingTaskLogic(ctx, s.svcCtx)
	return l.AssignBlessingTask(req)
}
