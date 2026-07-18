package logic

import (
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/file-service/internal/svc"
	"github.com/askxuan/file-service/internal/types"
	"github.com/minio/minio-go/v7"
)

const backupPrefix = "backups/"

func ListBackups(ctx context.Context, svcCtx *svc.ServiceContext) (*types.BackupListResp, error) {
	if !svcCtx.Config.Backup.Enabled || svcCtx.MinIOClient == nil || svcCtx.BackupBucket == "" {
		return nil, common.NewBizError(7010, "备份服务未启用")
	}
	items := make([]types.BackupItem, 0)
	for object := range svcCtx.MinIOClient.ListObjects(ctx, svcCtx.BackupBucket, minio.ListObjectsOptions{Prefix: backupPrefix, Recursive: true}) {
		if object.Err != nil {
			return nil, object.Err
		}
		if !strings.HasSuffix(object.Key, ".sql.gz") {
			continue
		}
		items = append(items, backupItem(object.Key, object.Size, object.LastModified))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Time > items[j].Time })
	return &types.BackupListResp{List: items}, nil
}

func CreateBackup(ctx context.Context, svcCtx *svc.ServiceContext) (*types.BackupItem, error) {
	cfg := svcCtx.Config.Backup
	if !cfg.Enabled || svcCtx.MinIOClient == nil || len(cfg.Schemas) == 0 {
		return nil, common.NewBizError(7010, "备份服务未启用或未配置数据库")
	}
	command := cfg.DumpCommand
	if command == "" {
		command = "mysqldump"
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	now := time.Now()
	id := "BK" + now.Format("20060102150405")
	filename := "askxuan_" + now.Format("20060102_150405") + ".sql.gz"
	objectName := backupPrefix + filename
	temp, err := os.CreateTemp("", "askxuan-backup-*.sql.gz")
	if err != nil {
		return nil, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	args := mysqlArgs(cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLUser)
	args = append(args, "--single-transaction", "--routines", "--events", "--triggers", "--databases")
	args = append(args, cfg.Schemas...)
	cmd := exec.CommandContext(commandCtx, command, args...)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+cfg.MySQLPassword)
	gzipWriter := gzip.NewWriter(temp)
	cmd.Stdout = gzipWriter
	var stderr strings.Builder
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	closeGzipErr := gzipWriter.Close()
	closeFileErr := temp.Close()
	if runErr != nil {
		return nil, fmt.Errorf("数据库导出失败: %w: %s", runErr, strings.TrimSpace(stderr.String()))
	}
	if closeGzipErr != nil || closeFileErr != nil {
		return nil, fmt.Errorf("压缩备份失败: %v %v", closeGzipErr, closeFileErr)
	}

	reader, err := os.Open(tempName)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	stat, err := reader.Stat()
	if err != nil {
		return nil, err
	}
	_, err = svcCtx.MinIOClient.PutObject(ctx, svcCtx.BackupBucket, objectName, reader, stat.Size(), minio.PutObjectOptions{
		ContentType:  "application/gzip",
		UserMetadata: map[string]string{"backup-id": id, "backup-type": "manual"},
	})
	if err != nil {
		return nil, err
	}
	item := backupItem(objectName, stat.Size(), now)
	item.Id = id
	return &item, nil
}

func BackupDownload(ctx context.Context, svcCtx *svc.ServiceContext, filename string) (*types.BackupDownloadResp, error) {
	objectName, err := validateBackupFilename(filename)
	if err != nil {
		return nil, err
	}
	if _, err = svcCtx.MinIOClient.StatObject(ctx, svcCtx.BackupBucket, objectName, minio.StatObjectOptions{}); err != nil {
		return nil, common.NewBizError(7011, "备份文件不存在")
	}
	expires := time.Duration(svcCtx.PresignExpire) * time.Second
	url, err := svcCtx.MinIOClient.PresignedGetObject(ctx, svcCtx.BackupBucket, objectName, expires, nil)
	if err != nil {
		return nil, err
	}
	return &types.BackupDownloadResp{Url: url.String(), ExpiresIn: svcCtx.PresignExpire}, nil
}

func RestoreBackup(ctx context.Context, svcCtx *svc.ServiceContext, filename, confirm string) (*types.BackupActionResp, error) {
	cfg := svcCtx.Config.Backup
	if !cfg.Enabled || !cfg.RestoreEnabled {
		return nil, common.NewBizError(7012, "当前环境禁止数据库恢复")
	}
	if confirm != filename {
		return nil, common.NewBizError(7013, "恢复确认文件名不匹配")
	}
	objectName, err := validateBackupFilename(filename)
	if err != nil {
		return nil, err
	}
	object, err := svcCtx.MinIOClient.GetObject(ctx, svcCtx.BackupBucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, common.NewBizError(7011, "备份文件不存在")
	}
	defer object.Close()
	if _, err = object.Stat(); err != nil {
		return nil, common.NewBizError(7011, "备份文件不存在")
	}
	gzipReader, err := gzip.NewReader(object)
	if err != nil {
		return nil, common.NewBizError(7014, "备份文件损坏")
	}
	defer gzipReader.Close()

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := cfg.RestoreCommand
	if command == "" {
		command = "mysql"
	}
	cmd := exec.CommandContext(commandCtx, command, mysqlArgs(cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLUser)...)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+cfg.MySQLPassword)
	cmd.Stdin = gzipReader
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("数据库恢复失败: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return &types.BackupActionResp{Success: true, Message: "数据库恢复完成"}, nil
}

func mysqlArgs(host string, port int, user string) []string {
	if port <= 0 {
		port = 3306
	}
	return []string{"--host=" + host, fmt.Sprintf("--port=%d", port), "--user=" + user, "--protocol=tcp", "--default-character-set=utf8mb4"}
}

func validateBackupFilename(filename string) (string, error) {
	if filename == "" || filepath.Base(filename) != filename || !strings.HasPrefix(filename, "askxuan_") || !strings.HasSuffix(filename, ".sql.gz") {
		return "", common.ErrParam
	}
	return backupPrefix + filename, nil
}

func backupItem(objectName string, size int64, modified time.Time) types.BackupItem {
	filename := filepath.Base(objectName)
	id := strings.TrimSuffix(strings.TrimPrefix(filename, "askxuan_"), ".sql.gz")
	return types.BackupItem{
		Id: "BK" + strings.ReplaceAll(id, "_", ""), Filename: filename,
		Size: float64(size) / 1024 / 1024, SizeBytes: size, Type: "manual",
		Status: "success", Time: modified.Format(time.RFC3339), ObjectName: objectName,
	}
}
