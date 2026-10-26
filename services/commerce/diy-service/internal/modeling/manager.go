package modeling

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrUnavailable = common.NewBizError(50220, "请先配置图生 3D 服务")
var ErrTask = common.NewBizError(40020, "建模任务不存在或不可访问")
var ErrConflict = common.NewBizError(40920, "请求标识已用于其他素材，请重新发起")
var requestID = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

type Manager struct {
	DownloadModel   func(context.Context, string) ([]byte, error)
	DB              sqlx.SqlConn
	Secret, FileURL string
	Client          *http.Client
	ProviderURL     string
}

func New(db sqlx.SqlConn, secret, fileURL string) *Manager {
	if fileURL == "" {
		fileURL = "http://file-service:8097"
	}
	return &Manager{DB: db, Secret: secret, FileURL: strings.TrimRight(fileURL, "/"), ProviderURL: "https://api.meshy.ai", DownloadModel: downloadModel, Client: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

type Config struct {
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Configured   bool   `json:"configured"`
	HasAPIKey    bool   `json:"hasApiKey"`
	CanConfigure bool   `json:"canConfigure"`
	Revision     int64  `json:"revision"`
}
type ConfigUpdate struct {
	APIKey      string `json:"apiKey"`
	ClearAPIKey bool   `json:"clearApiKey"`
	Model       string `json:"model"`
}
type configRow struct {
	Cipher   string `db:"api_key_cipher"`
	Model    string `db:"model"`
	Revision int64  `db:"revision"`
}

func (m *Manager) cipher() (cipher.AEAD, error) {
	if len(m.Secret) < 32 {
		return nil, ErrUnavailable
	}
	key := sha256.Sum256([]byte("askxuan-diy-modeling-v1:" + m.Secret))
	block, e := aes.NewCipher(key[:])
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(block)
}
func (m *Manager) seal(value string) (string, error) {
	c, e := m.cipher()
	if e != nil {
		return "", e
	}
	n := make([]byte, c.NonceSize())
	if _, e = rand.Read(n); e != nil {
		return "", e
	}
	return base64.RawStdEncoding.EncodeToString(c.Seal(n, n, []byte(value), []byte("meshy"))), nil
}
func (m *Manager) open(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	c, e := m.cipher()
	if e != nil {
		return "", e
	}
	b, e := base64.RawStdEncoding.DecodeString(value)
	if e != nil || len(b) < c.NonceSize() {
		return "", ErrUnavailable
	}
	plain, e := c.Open(nil, b[:c.NonceSize()], b[c.NonceSize():], []byte("meshy"))
	return string(plain), e
}
func (m *Manager) readConfig(ctx context.Context) (configRow, error) {
	var r configRow
	e := m.DB.QueryRowPartialCtx(ctx, &r, "SELECT api_key_cipher,model,revision FROM diy_modeling_config WHERE id=1")
	if errors.Is(e, sqlx.ErrNotFound) {
		return configRow{Model: "meshy-7.1"}, nil
	}
	return r, e
}
func (m *Manager) PublicConfig(ctx context.Context, canConfigure bool) (Config, error) {
	r, e := m.readConfig(ctx)
	if e != nil {
		return Config{}, e
	}
	key, e := m.open(r.Cipher)
	return Config{Provider: "meshy", Model: r.Model, Configured: e == nil && key != "", HasAPIKey: r.Cipher != "", CanConfigure: canConfigure, Revision: r.Revision}, nil
}
func (m *Manager) UpdateConfig(ctx context.Context, actor int64, u ConfigUpdate) (Config, error) {
	if actor <= 0 || len(u.APIKey) > 1024 || strings.ContainsAny(u.APIKey, "\r\n") {
		return Config{}, common.ErrParam
	}
	r, e := m.readConfig(ctx)
	if e != nil {
		return Config{}, e
	}
	if u.Model == "" {
		u.Model = r.Model
	}
	if u.Model != "meshy-7.1" && u.Model != "meshy-6" && u.Model != "latest" {
		return Config{}, common.ErrParam
	}
	if u.ClearAPIKey {
		r.Cipher = ""
	} else if strings.TrimSpace(u.APIKey) != "" {
		r.Cipher, e = m.seal(strings.TrimSpace(u.APIKey))
		if e != nil {
			return Config{}, ErrUnavailable
		}
	}
	var result sql.Result
	if r.Revision == 0 {
		result, e = m.DB.ExecCtx(ctx, `INSERT IGNORE INTO diy_modeling_config(id,api_key_cipher,model,revision,updated_by) VALUES(1,?,?,1,?)`, r.Cipher, u.Model, actor)
	} else {
		result, e = m.DB.ExecCtx(ctx, `UPDATE diy_modeling_config SET api_key_cipher=?,model=?,revision=revision+1,updated_by=? WHERE id=1 AND revision=?`, r.Cipher, u.Model, actor, r.Revision)
	}
	if e != nil {
		return Config{}, e
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return Config{}, common.NewBizError(40921, "建模配置已被其他管理员更新，请刷新后重试")
	}
	return m.PublicConfig(ctx, true)
}

type CreateTask struct {
	RequestID string   `json:"requestId"`
	PhotoURLs []string `json:"photoUrls"`
}
type Task struct {
	ID           string                        `db:"id" json:"id"`
	OwnerID      int64                         `db:"owner_id" json:"-"`
	RequestHash  string                        `db:"request_hash" json:"-"`
	Photos       string                        `db:"photos" json:"-"`
	ModelName    string                        `db:"model" json:"-"`
	Status       string                        `db:"status" json:"status"`
	Progress     int                           `db:"progress" json:"progress"`
	ProviderID   string                        `db:"provider_id" json:"-"`
	GLBURL       string                        `db:"glb_url" json:"-"`
	USDZURL      string                        `db:"usdz_url" json:"-"`
	ErrorMessage string                        `db:"error_message" json:"errorMessage,omitempty"`
	Attempts     int                           `db:"attempts" json:"-"`
	Model        *model.GeneratedMaterialModel `json:"model,omitempty"`
	PhotoURLs    []string                      `json:"photoUrls"`
}

const taskFields = "id,owner_id,request_hash,photos,model,status,progress,provider_id,glb_url,usdz_url,error_message,attempts"

func expose(t *Task) *Task {
	_ = json.Unmarshal([]byte(t.Photos), &t.PhotoURLs)
	if t.Status == "succeeded" {
		t.Model = &model.GeneratedMaterialModel{JobID: t.ID, GLBURL: t.GLBURL, USDZURL: t.USDZURL}
	}
	return t
}
func (m *Manager) Task(ctx context.Context, id string, owner int64, super bool) (*Task, error) {
	if !requestID.MatchString(id) {
		return nil, ErrTask
	}
	var t Task
	e := m.DB.QueryRowPartialCtx(ctx, &t, "SELECT "+taskFields+" FROM diy_modeling_task WHERE id=?", id)
	if e != nil || (!super && t.OwnerID != owner) {
		return nil, ErrTask
	}
	return expose(&t), nil
}
func (m *Manager) Create(ctx context.Context, owner int64, u CreateTask) (*Task, error) {
	if owner <= 0 || !requestID.MatchString(u.RequestID) || len(u.PhotoURLs) < 1 || len(u.PhotoURLs) > 4 {
		return nil, common.ErrParam
	}
	for _, p := range u.PhotoURLs {
		if !model.ValidAssetURL(p) {
			return nil, common.ErrParam
		}
	}
	raw, _ := json.Marshal(u.PhotoURLs)
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	var existing Task
	e := m.DB.QueryRowPartialCtx(ctx, &existing, "SELECT "+taskFields+" FROM diy_modeling_task WHERE owner_id=? AND request_id=?", owner, u.RequestID)
	if e == nil {
		if existing.RequestHash != hash {
			return nil, ErrConflict
		}
		return expose(&existing), nil
	}
	if !errors.Is(e, sqlx.ErrNotFound) {
		return nil, e
	}
	cfg, e := m.readConfig(ctx)
	if e != nil {
		return nil, e
	}
	key, e := m.open(cfg.Cipher)
	if e != nil || key == "" {
		return nil, ErrUnavailable
	}
	var id [16]byte
	if _, e = rand.Read(id[:]); e != nil {
		return nil, e
	}
	taskID := hex.EncodeToString(id[:])
	_, e = m.DB.ExecCtx(ctx, `INSERT IGNORE INTO diy_modeling_task(id,owner_id,request_id,request_hash,photos,model) VALUES(?,?,?,?,?,?)`, taskID, owner, u.RequestID, hash, string(raw), cfg.Model)
	if e != nil {
		return nil, e
	}
	e = m.DB.QueryRowPartialCtx(ctx, &existing, "SELECT "+taskFields+" FROM diy_modeling_task WHERE owner_id=? AND request_id=?", owner, u.RequestID)
	if e != nil {
		return nil, e
	}
	if existing.RequestHash != hash {
		return nil, ErrConflict
	}
	return expose(&existing), nil
}
func (m *Manager) ValidateModel(ctx context.Context, s *model.MaterialStudio, actor int64, super bool) error {
	if s == nil || s.Source != "generated" {
		return nil
	}
	if s.Model == nil {
		return common.NewBizError(40021, "请先完成模型生成")
	}
	t, e := m.Task(ctx, s.Model.JobID, actor, super)
	if e != nil {
		return e
	}
	if t.Status != "succeeded" || t.GLBURL != s.Model.GLBURL || t.USDZURL != s.Model.USDZURL {
		return common.NewBizError(40021, "生成模型尚未就绪或素材不匹配")
	}
	return nil
}
