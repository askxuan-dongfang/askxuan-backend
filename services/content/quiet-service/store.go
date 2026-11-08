package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

var errIdempotency = errors.New("同一个保存请求对应了不同练习内容")
var redisNamespace = regexp.MustCompile(`^quiet-[a-z0-9-]+(?::[a-z0-9-]+)*:$`)

type practiceRecord struct {
	ID      string `json:"id,omitempty"`
	Date    string `json:"date"`
	Seconds int64  `json:"seconds"`
	Hits    int64  `json:"hits"`
	Beads   int64  `json:"beads"`
}

type roomSnapshot struct {
	ID        string          `json:"id"`
	Owner     string          `json:"owner"`
	Count     int             `json:"count"`
	Snapshot  json.RawMessage `json:"snapshot"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type storage struct {
	db          *sql.DB
	redis       *redis.Client
	prefix      string
	mu          sync.Mutex
	guests      map[string]time.Time
	records     map[string][]practiceRecord
	idempotency map[string]practiceRecord
	serial      uint64
}

func memoryStorage() *storage {
	return &storage{guests: map[string]time.Time{}, records: map[string][]practiceRecord{}, idempotency: map[string]practiceRecord{}}
}

func persistentStorage(ctx context.Context, mysqlAddr, mysqlPassword, redisAddr, redisPassword, prefix string) (*storage, error) {
	if mysqlAddr == "" || redisAddr == "" || mysqlPassword == "" || redisPassword == "" || !redisNamespace.MatchString(prefix) {
		return nil, errors.New("静心独立存储配置不完整")
	}
	cfg := mysql.NewConfig()
	cfg.User, cfg.Passwd, cfg.Net, cfg.Addr, cfg.DBName = "quiet", mysqlPassword, "tcp", mysqlAddr, "quiet_local"
	cfg.ParseTime, cfg.Loc, cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = true, time.UTC, 5*time.Second, 5*time.Second, 5*time.Second
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, errors.New("无法初始化静心独立数据库")
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	s := memoryStorage()
	s.db, s.redis, s.prefix = db, redis.NewClient(&redis.Options{Addr: redisAddr, Password: redisPassword, DialTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second}), prefix
	if err = db.PingContext(ctx); err != nil {
		s.close()
		return nil, errors.New("静心独立MySQL连接失败")
	}
	if err = s.redis.Ping(ctx).Err(); err != nil {
		s.close()
		return nil, errors.New("静心独立Redis连接失败")
	}
	for _, ddl := range []string{
		// guest_hash also stores server-derived customer principal hashes; no client identity is accepted.
		`CREATE TABLE IF NOT EXISTS quiet_guests (token_hash CHAR(64) CHARACTER SET ascii PRIMARY KEY, expires_at DATETIME(3) NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS quiet_practices (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, guest_hash CHAR(64) CHARACTER SET ascii NOT NULL, idempotency_key VARCHAR(80) CHARACTER SET ascii NOT NULL, practiced_at DATETIME(3) NOT NULL, seconds INT UNSIGNED NOT NULL, hits INT UNSIGNED NOT NULL, beads INT UNSIGNED NOT NULL, UNIQUE KEY guest_request (guest_hash,idempotency_key), KEY guest_records (guest_hash,id))`,
	} {
		if _, err = db.ExecContext(ctx, ddl); err != nil {
			s.close()
			return nil, errors.New("静心独立表初始化失败")
		}
	}
	return s, nil
}

func (s *storage) close() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.redis != nil {
		_ = s.redis.Close()
	}
}

func (s *storage) guest(ctx context.Context, hash string) (bool, error) {
	if s.db != nil {
		var valid bool
		err := s.db.QueryRowContext(ctx, `SELECT expires_at > UTC_TIMESTAMP(3) FROM quiet_guests WHERE token_hash=?`, hash).Scan(&valid)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return valid, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.guests[hash].After(time.Now()), nil
}

func (s *storage) createGuest(ctx context.Context, hash string, expiry time.Time) error {
	if s.db != nil {
		_, err := s.db.ExecContext(ctx, `INSERT INTO quiet_guests (token_hash,expires_at) VALUES (?,?)`, hash, expiry.UTC())
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guests[hash] = expiry
	return nil
}

func (s *storage) save(ctx context.Context, guest, key string, record practiceRecord) (practiceRecord, error) {
	when, err := time.Parse(time.RFC3339Nano, record.Date)
	if err != nil {
		return record, err
	}
	record.Date = when.UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano)
	if s.db != nil {
		result, err := s.db.ExecContext(ctx, `INSERT INTO quiet_practices (guest_hash,idempotency_key,practiced_at,seconds,hits,beads) VALUES (?,?,?,?,?,?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`, guest, key, when.UTC().Truncate(time.Millisecond), record.Seconds, record.Hits, record.Beads)
		if err != nil {
			return record, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return record, err
		}
		var stored practiceRecord
		var stamp time.Time
		err = s.db.QueryRowContext(ctx, `SELECT id,practiced_at,seconds,hits,beads FROM quiet_practices WHERE id=? AND guest_hash=?`, id, guest).Scan(&stored.ID, &stamp, &stored.Seconds, &stored.Hits, &stored.Beads)
		stored.Date = stamp.UTC().Format(time.RFC3339Nano)
		if err == nil && (stored.Date != record.Date || stored.Seconds != record.Seconds || stored.Hits != record.Hits || stored.Beads != record.Beads) {
			err = errIdempotency
		}
		return stored, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	request := guest + ":" + key
	if stored, exists := s.idempotency[request]; exists {
		if stored.Date != record.Date || stored.Seconds != record.Seconds || stored.Hits != record.Hits || stored.Beads != record.Beads {
			return record, errIdempotency
		}
		return stored, nil
	}
	s.serial++
	record.ID = strconv.FormatUint(s.serial, 10)
	s.records[guest] = append(s.records[guest], record)
	s.idempotency[request] = record
	return record, nil
}

func (s *storage) list(ctx context.Context, guest string) ([]practiceRecord, error) {
	records := make([]practiceRecord, 0)
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, `SELECT id,practiced_at,seconds,hits,beads FROM quiet_practices WHERE guest_hash=? ORDER BY id DESC LIMIT 50`, guest)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var record practiceRecord
			var stamp time.Time
			if err := rows.Scan(&record.ID, &stamp, &record.Seconds, &record.Hits, &record.Beads); err != nil {
				return nil, err
			}
			record.Date = stamp.UTC().Format(time.RFC3339Nano)
			records = append(records, record)
		}
		return records, rows.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.records[guest]
	for i := len(stored) - 1; i >= 0 && len(records) < 50; i-- {
		records = append(records, stored[i])
	}
	return records, nil
}

func (s *storage) clear(ctx context.Context, guest string) error {
	if s.db != nil {
		_, err := s.db.ExecContext(ctx, `DELETE FROM quiet_practices WHERE guest_hash=?`, guest)
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, guest)
	for request := range s.idempotency {
		if strings.HasPrefix(request, guest+":") {
			delete(s.idempotency, request)
		}
	}
	return nil
}

func (s *storage) syncRoom(ctx context.Context, owner string, snapshot roomSnapshot) error {
	if s.redis == nil {
		return nil
	}
	const script = `local owner=redis.call('HGET',KEYS[1],'owner'); if owner and owner~=ARGV[1] then return 0 end; redis.call('HSET',KEYS[1],'owner',ARGV[1],'count',ARGV[2],'snapshot',ARGV[3],'updatedAt',ARGV[4]); redis.call('EXPIRE',KEYS[1],60); return 1`
	result, err := s.redis.Eval(ctx, script, []string{s.prefix + "room:" + snapshot.ID}, owner, snapshot.Count, string(snapshot.Snapshot), snapshot.UpdatedAt.UTC().Format(time.RFC3339Nano)).Int()
	if err == nil && result != 1 {
		return errors.New("房间归属已变更")
	}
	return err
}

func (s *storage) roomSnapshots(ctx context.Context, owner string) ([]roomSnapshot, error) {
	result := make([]roomSnapshot, 0)
	if s.redis == nil {
		return result, nil
	}
	var cursor uint64
	for {
		keys, next, err := s.redis.Scan(ctx, cursor, s.prefix+"room:*", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			fields, err := s.redis.HGetAll(ctx, key).Result()
			if err != nil {
				return nil, err
			}
			if fields["owner"] != owner {
				continue
			}
			count, err := strconv.Atoi(fields["count"])
			if err != nil || !json.Valid([]byte(fields["snapshot"])) {
				return nil, fmt.Errorf("房间状态异常")
			}
			stamp, err := time.Parse(time.RFC3339Nano, fields["updatedAt"])
			if err != nil {
				return nil, err
			}
			result = append(result, roomSnapshot{ID: strings.TrimPrefix(key, s.prefix+"room:"), Owner: owner, Count: count, Snapshot: json.RawMessage(fields["snapshot"]), UpdatedAt: stamp})
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return result, nil
}
