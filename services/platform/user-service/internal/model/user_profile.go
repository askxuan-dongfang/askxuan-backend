package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// userProfileTable 用户画像表（位于 askxuan_user 库，需完全限定名跨库查询）
const userProfileTable = "askxuan_user.user_profile"

// UserProfileStats 用户画像统计（依据 init.sql user_profile 表）
type UserProfileStats struct {
	UserId         int64   `db:"user_id" json:"userId"`
	PreferenceTags string  `db:"preference_tags" json:"preferenceTags"` // 逗号分隔
	TotalOrders    int     `db:"total_orders" json:"totalOrders"`
	TotalSpent     float64 `db:"total_spent" json:"totalSpent"`
	LastActiveTime string  `db:"last_active_time" json:"lastActiveTime"`
	UpdateTime     string  `db:"update_time" json:"updateTime"`
}

// UserProfileModel 用户画像模型接口
type UserProfileModel interface {
	FindByID(ctx context.Context, userId int64) (*UserProfileStats, error)
}

type defaultUserProfileModel struct {
	conn sqlx.SqlConn
}

// NewUserProfileModel 构造用户画像模型
func NewUserProfileModel(conn sqlx.SqlConn) UserProfileModel {
	return &defaultUserProfileModel{conn: conn}
}

// FindByID 查询用户画像
func (m *defaultUserProfileModel) FindByID(ctx context.Context, userId int64) (*UserProfileStats, error) {
	var s UserProfileStats
	query := fmt.Sprintf(`SELECT user_id, preference_tags, total_orders, total_spent, IFNULL(last_active_time,'') AS last_active_time, update_time FROM %s WHERE user_id = ?`, userProfileTable)
	err := m.conn.QueryRowCtx(ctx, &s, query, userId)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// SplitTags 将 preference_tags 字符串拆分为切片
func SplitTags(tags string) []string {
	if tags == "" {
		return []string{}
	}
	parts := strings.Split(tags, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
