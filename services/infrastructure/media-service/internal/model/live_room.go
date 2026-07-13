package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const liveRoomRows = "id,room_no,owner_id,master_id,title,cover_media_id,provider,status,openim_group_id,push_url,watch_url,provider_room_id,COALESCE(DATE_FORMAT(started_at,'%Y-%m-%d %H:%i:%s'), '') AS started_at,COALESCE(DATE_FORMAT(ended_at,'%Y-%m-%d %H:%i:%s'), '') AS ended_at,create_time,update_time"

type LiveRoom struct {
	Id             int64  `db:"id"`
	RoomNo         string `db:"room_no"`
	OwnerId        string `db:"owner_id"`
	MasterId       string `db:"master_id"`
	Title          string `db:"title"`
	CoverMediaId   int64  `db:"cover_media_id"`
	Provider       string `db:"provider"`
	Status         string `db:"status"`
	OpenimGroupId  string `db:"openim_group_id"`
	PushUrl        string `db:"push_url"`
	WatchUrl       string `db:"watch_url"`
	ProviderRoomId string `db:"provider_room_id"`
	StartedAt      string `db:"started_at"`
	EndedAt        string `db:"ended_at"`
	CreateTime     string `db:"create_time"`
	UpdateTime     string `db:"update_time"`
}

type LiveRoomModel interface {
	Insert(ctx context.Context, room *LiveRoom) (*LiveRoom, error)
	FindOne(ctx context.Context, id int64) (*LiveRoom, error)
	FindLive(ctx context.Context, masterId string, limit int) ([]LiveRoom, error)
	BindOpenIM(ctx context.Context, id int64, ownerId, groupId string) (*LiveRoom, error)
	Start(ctx context.Context, id int64, ownerId, providerRoomId, pushURL, watchURL string) (*LiveRoom, error)
	Close(ctx context.Context, id int64, ownerId string) (*LiveRoom, error)
}

type liveRoomModel struct{ conn sqlx.SqlConn }

func NewLiveRoomModel(conn sqlx.SqlConn) LiveRoomModel { return &liveRoomModel{conn: conn} }

func (m *liveRoomModel) Insert(ctx context.Context, room *LiveRoom) (*LiveRoom, error) {
	room.RoomNo = fmt.Sprintf("LIVE%d", time.Now().UnixNano())
	room.Status = "created"
	result, err := m.conn.ExecCtx(ctx, `INSERT INTO live_room(room_no,owner_id,master_id,title,cover_media_id,provider,status,openim_group_id) VALUES(?,?,?,?,?,?,?,?)`, room.RoomNo, room.OwnerId, room.MasterId, room.Title, room.CoverMediaId, room.Provider, room.Status, room.OpenimGroupId)
	if err != nil {
		return nil, err
	}
	room.Id, err = result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, room.Id)
}

func (m *liveRoomModel) FindOne(ctx context.Context, id int64) (*LiveRoom, error) {
	var room LiveRoom
	err := m.conn.QueryRowCtx(ctx, &room, `SELECT `+liveRoomRows+` FROM live_room WHERE id=?`, id)
	return &room, err
}

func (m *liveRoomModel) FindLive(ctx context.Context, masterId string, limit int) ([]LiveRoom, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := `SELECT ` + liveRoomRows + ` FROM live_room WHERE status='live'`
	args := make([]interface{}, 0, 2)
	if masterId != "" {
		query += ` AND master_id=?`
		args = append(args, masterId)
	}
	query += ` ORDER BY started_at DESC LIMIT ?`
	args = append(args, limit)
	var rooms []LiveRoom
	if err := m.conn.QueryRowsCtx(ctx, &rooms, query, args...); err != nil {
		return nil, err
	}
	return rooms, nil
}

func (m *liveRoomModel) BindOpenIM(ctx context.Context, id int64, ownerId, groupId string) (*LiveRoom, error) {
	result, err := m.conn.ExecCtx(ctx, `UPDATE live_room SET openim_group_id=?,update_time=CURRENT_TIMESTAMP WHERE id=? AND owner_id=? AND status='created'`, groupId, id, ownerId)
	if err != nil {
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return nil, sqlx.ErrNotFound
	}
	return m.FindOne(ctx, id)
}

func (m *liveRoomModel) Start(ctx context.Context, id int64, ownerId, providerRoomId, pushURL, watchURL string) (*LiveRoom, error) {
	result, err := m.conn.ExecCtx(ctx, `UPDATE live_room SET provider_room_id=?,push_url=?,watch_url=?,status='live',started_at=CURRENT_TIMESTAMP,update_time=CURRENT_TIMESTAMP WHERE id=? AND owner_id=? AND status='created'`, providerRoomId, pushURL, watchURL, id, ownerId)
	if err != nil {
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return nil, sqlx.ErrNotFound
	}
	return m.FindOne(ctx, id)
}

func (m *liveRoomModel) Close(ctx context.Context, id int64, ownerId string) (*LiveRoom, error) {
	result, err := m.conn.ExecCtx(ctx, `UPDATE live_room SET status='ended',ended_at=CURRENT_TIMESTAMP,update_time=CURRENT_TIMESTAMP WHERE id=? AND owner_id=? AND status='live'`, id, ownerId)
	if err != nil {
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return nil, sqlx.ErrNotFound
	}
	return m.FindOne(ctx, id)
}
