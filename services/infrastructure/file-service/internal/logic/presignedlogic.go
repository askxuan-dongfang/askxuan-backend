package logic

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/file-service/internal/svc"
	"github.com/askxuan/file-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// PresignLogic 预签名 URL 逻辑
type PresignLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPresignLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PresignLogic {
	return &PresignLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Presigned 生成预签名 URL
// - operate=upload：返回 PresignedPutObject URL，前端直传
// - operate=download：返回 PresignedGetObject URL，前端下载
func (l *PresignLogic) Presigned(req *types.PresignReq) (*types.PresignResp, error) {
	if l.svcCtx.MinIOClient == nil {
		return nil, common.ErrOssService
	}

	// 生成 objectName
	objectName := req.ObjectName
	if objectName == "" {
		if req.FileName == "" {
			return nil, common.ErrParam
		}
		objectName = buildObjectName(req.ObjectType, req.FileName)
	}

	expire := time.Duration(l.svcCtx.PresignExpire) * time.Second
	var u string
	var err error
	if req.Operate == "download" {
		pu, e := l.svcCtx.MinIOClient.PresignedGetObject(l.ctx, l.svcCtx.Bucket, objectName, expire, nil)
		if e != nil {
			err = e
		} else {
			u = pu.String()
		}
	} else {
		// 默认 upload
		pu, e := l.svcCtx.MinIOClient.PresignedPutObject(l.ctx, l.svcCtx.Bucket, objectName, expire)
		if e != nil {
			err = e
		} else {
			u = pu.String()
		}
	}
	if err != nil {
		l.Errorf("生成预签名 URL 失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.PresignResp{
		UploadUrl:  u,
		ObjectName: objectName,
		ExpiresIn:  l.svcCtx.PresignExpire,
	}, nil
}

// buildObjectName 按业务目录 + 时间戳 + 原始文件名生成对象名
// 例：temples/20260630153005/abc.jpg
func buildObjectName(objectType, fileName string) string {
	if objectType == "" {
		objectType = "temp"
	}
	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	// 去掉中文/空格，简单用时间戳前缀避免重名
	stamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("%s/%s_%s%s", objectType, stamp, base, ext)
}
