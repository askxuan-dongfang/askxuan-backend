package logic

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/askxuan/diy-service/rpc/diy"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetBlessingTaskLogic 按 ID 查询加持任务
type GetBlessingTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBlessingTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBlessingTaskLogic {
	return &GetBlessingTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetBlessingTaskLogic) GetBlessingTask(req *diy.GetBlessingTaskReq) (*diy.BlessingTask, error) {
	task, err := l.svcCtx.BlessingTaskModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "加持任务不存在")
		}
		return nil, err
	}
	return modelTaskToRPC(task), nil
}

// modelTaskToRPC 将 model.BlessingTask 转为 rpc diy.BlessingTask
func modelTaskToRPC(t *model.BlessingTask) *diy.BlessingTask {
	return &diy.BlessingTask{
		Id:              t.Id,
		TaskNo:          t.TaskNo,
		DiyOrderNo:      t.DiyOrderNo,
		TempleCode:      t.TempleCode,
		MasterCode:      t.MasterCode,
		Status:          t.Status,
		CertificateUrls: urlsToJSONStr(t.CertificateUrls),
		AssignTime:      t.AssignTime,
		CompleteTime:    t.CompleteTime,
		CreateTime:      t.CreateTime,
		UpdateTime:      t.UpdateTime,
	}
}

// urlsToJSONStr 将 []string 序列化为 JSON 字符串（proto 中 certificate_urls 为 string 类型）
func urlsToJSONStr(urls []string) string {
	if len(urls) == 0 {
		return "[]"
	}
	b, err := json.Marshal(urls)
	if err != nil {
		return "[]"
	}
	return string(b)
}
