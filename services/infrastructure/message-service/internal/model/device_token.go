package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const deviceTokenTable = "device_token"

type DeviceToken struct {
	Id          int64  `db:"id" json:"id"`
	UserId      string `db:"user_id" json:"userId"`
	ClientType  string `db:"client_type" json:"clientType"`
	Platform    string `db:"platform" json:"platform"`
	DeviceToken string `db:"device_token" json:"deviceToken"`
	BundleId    string `db:"bundle_id" json:"bundleId"`
	AppVersion  string `db:"app_version" json:"appVersion"`
	Status      string `db:"status" json:"status"`
	CreateTime  string `db:"create_time" json:"createTime"`
	UpdateTime  string `db:"update_time" json:"updateTime"`
}

type DeviceTokenModel interface {
	Upsert(ctx context.Context, d *DeviceToken) (int64, error)
	Deactivate(ctx context.Context, userId, deviceToken string) (int64, error)
}

type defaultDeviceTokenModel struct {
	conn sqlx.SqlConn
}

func NewDeviceTokenModel(conn sqlx.SqlConn) DeviceTokenModel {
	return &defaultDeviceTokenModel{conn: conn}
}

func (m *defaultDeviceTokenModel) Upsert(ctx context.Context, data *DeviceToken) (int64, error) {
	const query = `INSERT INTO ` + deviceTokenTable + ` (user_id, client_type, platform, device_token, bundle_id, app_version, status)
VALUES (?, ?, ?, ?, ?, ?, 'active')
ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id), user_id=VALUES(user_id), client_type=VALUES(client_type), platform=VALUES(platform),
bundle_id=VALUES(bundle_id), app_version=VALUES(app_version), status='active', update_time=CURRENT_TIMESTAMP`
	res, err := m.conn.ExecCtx(ctx, query,
		data.UserId, data.ClientType, data.Platform, data.DeviceToken, data.BundleId, data.AppVersion)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (m *defaultDeviceTokenModel) Deactivate(ctx context.Context, userId, deviceToken string) (int64, error) {
	const query = `UPDATE ` + deviceTokenTable + ` SET status='inactive', update_time=CURRENT_TIMESTAMP WHERE user_id=? AND device_token=?`
	res, err := m.conn.ExecCtx(ctx, query, userId, deviceToken)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
