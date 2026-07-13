package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const mediaRows = "id,media_no,owner_id,media_type,content_type,file_name,object_name,provider,provider_task_id,status,audit_status,playback_url,cover_url,cover_media_id,duration,file_size,error_message,create_time,update_time"

type Media struct {
	Id             int64   `db:"id"`
	MediaNo        string  `db:"media_no"`
	OwnerId        string  `db:"owner_id"`
	MediaType      string  `db:"media_type"`
	ContentType    string  `db:"content_type"`
	FileName       string  `db:"file_name"`
	ObjectName     string  `db:"object_name"`
	Provider       string  `db:"provider"`
	ProviderTaskId string  `db:"provider_task_id"`
	Status         string  `db:"status"`
	AuditStatus    string  `db:"audit_status"`
	PlaybackUrl    string  `db:"playback_url"`
	CoverUrl       string  `db:"cover_url"`
	CoverMediaId   int64   `db:"cover_media_id"`
	Duration       float64 `db:"duration"`
	FileSize       int64   `db:"file_size"`
	ErrorMessage   string  `db:"error_message"`
	CreateTime     string  `db:"create_time"`
	UpdateTime     string  `db:"update_time"`
}

type MediaModel interface {
	Insert(ctx context.Context, media *Media) (*Media, error)
	FindOne(ctx context.Context, id int64) (*Media, error)
	Complete(ctx context.Context, id int64, ownerId, playbackURL, coverURL string, coverMediaId, fileSize int64, contentType string) (*Media, error)
	UpdateTranscode(ctx context.Context, media *Media) error
	UpdateAudit(ctx context.Context, id int64, status, reason string) error
}

type mediaModel struct{ conn sqlx.SqlConn }

func NewMediaModel(conn sqlx.SqlConn) MediaModel { return &mediaModel{conn: conn} }

func (m *mediaModel) Insert(ctx context.Context, media *Media) (*Media, error) {
	if media.MediaNo == "" {
		media.MediaNo = fmt.Sprintf("MED%d", time.Now().UnixNano())
	}
	if media.Status == "" {
		media.Status = "uploading"
	}
	if media.AuditStatus == "" {
		media.AuditStatus = "pending"
	}
	result, err := m.conn.ExecCtx(ctx, `INSERT INTO media_asset(media_no,owner_id,media_type,content_type,file_name,object_name,provider,status,audit_status,file_size) VALUES(?,?,?,?,?,?,?,?,?,?)`, media.MediaNo, media.OwnerId, media.MediaType, media.ContentType, media.FileName, media.ObjectName, media.Provider, media.Status, media.AuditStatus, media.FileSize)
	if err != nil {
		return nil, err
	}
	media.Id, err = result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, media.Id)
}

func (m *mediaModel) FindOne(ctx context.Context, id int64) (*Media, error) {
	var media Media
	err := m.conn.QueryRowCtx(ctx, &media, `SELECT `+mediaRows+` FROM media_asset WHERE id=?`, id)
	return &media, err
}

func (m *mediaModel) Complete(ctx context.Context, id int64, ownerId, playbackURL, coverURL string, coverMediaId, fileSize int64, contentType string) (*Media, error) {
	result, err := m.conn.ExecCtx(ctx, `UPDATE media_asset SET status='ready',playback_url=?,cover_url=?,cover_media_id=?,file_size=?,content_type=?,error_message='',update_time=CURRENT_TIMESTAMP WHERE id=? AND owner_id=? AND status IN ('uploading','uploaded','failed')`, playbackURL, coverURL, coverMediaId, fileSize, contentType, id, ownerId)
	if err != nil {
		return nil, err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return nil, sqlx.ErrNotFound
	}
	return m.FindOne(ctx, id)
}

func (m *mediaModel) UpdateTranscode(ctx context.Context, media *Media) error {
	result, err := m.conn.ExecCtx(ctx, `UPDATE media_asset SET provider_task_id=?,status=?,playback_url=?,cover_url=?,duration=?,error_message=?,update_time=CURRENT_TIMESTAMP WHERE id=?`, media.ProviderTaskId, media.Status, media.PlaybackUrl, media.CoverUrl, media.Duration, media.ErrorMessage, media.Id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		_, findErr := m.FindOne(ctx, media.Id)
		return findErr
	}
	return nil
}

func (m *mediaModel) UpdateAudit(ctx context.Context, id int64, status, reason string) error {
	result, err := m.conn.ExecCtx(ctx, `UPDATE media_asset SET audit_status=?,error_message=?,update_time=CURRENT_TIMESTAMP WHERE id=?`, status, reason, id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		_, findErr := m.FindOne(ctx, id)
		return findErr
	}
	return nil
}
