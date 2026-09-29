package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/reportdoc"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/types"
	"github.com/askxuan/common"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
	"time"
)

type ReportProduct struct {
	ToolConfig    string   `json:"-" db:"tool_config"`
	Ready         bool     `json:"ready" db:"-"`
	ExecutionNote string   `json:"executionNote" db:"-"`
	Code          string   `json:"code" db:"code"`
	Title         string   `json:"title" db:"title"`
	Subtitle      string   `json:"subtitle" db:"subtitle"`
	PriceCents    int64    `json:"priceCents" db:"price_cents"`
	PointsPrice   int64    `json:"pointsPrice" db:"points_price"`
	ChaptersJSON  string   `json:"-" db:"chapters_json"`
	Chapters      []string `json:"chapters" db:"-"`
	Version       string   `json:"version" db:"version"`
}
type Report struct {
	DocumentJSON    string              `json:"-" db:"document_json"`
	Document        *reportdoc.Document `json:"document,omitempty" db:"-"`
	GenerationStage string              `json:"generationStage" db:"generation_stage"`
	ID              int64               `json:"id" db:"id"`
	ReportNo        string              `json:"reportNo" db:"report_no"`
	UserID          string              `json:"-" db:"user_id"`
	SkillCode       string              `json:"skillCode" db:"skill_code"`
	Title           string              `json:"title" db:"title"`
	Version         string              `json:"version" db:"version"`
	Question        string              `json:"question" db:"question"`
	InputsJSON      string              `json:"-" db:"inputs_json"`
	ChaptersJSON    string              `json:"-" db:"chapters_json"`
	Chapters        []string            `json:"chapters" db:"-"`
	PriceCents      int64               `json:"priceCents" db:"price_cents"`
	PointsPrice     int64               `json:"pointsPrice" db:"points_price"`
	Status          string              `json:"status" db:"status"`
	Summary         string              `json:"summary" db:"summary"`
	Content         string              `json:"content" db:"content"`
	ErrorMessage    string              `json:"errorMessage" db:"error_message"`
	PointsPaid      int64               `json:"-" db:"points_paid"`
	Unlocked        bool                `json:"unlocked" db:"-"`
	CreatedAt       string              `json:"createdAt" db:"created_at"`
}

const reportCols = `document_json,generation_stage,id,report_no,user_id,skill_code,title,version,question,inputs_json,chapters_json,price_cents,points_price,status,summary,content,error_message,points_paid,created_at`

var reportSelectCols = strings.Replace(reportCols, "document_json", "COALESCE(document_json,'') document_json", 1)

type ReportRequest struct {
	Attachments []types.AIImageAttachment `json:"attachments,omitempty"`
	SkillCode   string                    `json:"skillCode"`
	Question    string                    `json:"question"`
	Inputs      map[string]interface{}    `json:"inputs"`
	RequestKey  string                    `json:"requestKey"`
}

func ReportProducts(ctx context.Context, s *svc.ServiceContext) ([]ReportProduct, error) {
	list := make([]ReportProduct, 0)
	err := s.DB.QueryRowsPartialCtx(ctx, &list, `SELECT p.code,p.title,p.subtitle,p.price_cents,p.points_price,p.chapters_json,p.version,COALESCE(CAST(s.tool_config AS CHAR),'{}') tool_config FROM ai_report_product p JOIN ai_skill s ON s.code=p.code WHERE p.enabled=1 AND s.status='enabled' ORDER BY s.sort_order`)
	if err != nil {
		return nil, err
	}
	if s.AgentVersion > 0 {
		filtered := make([]ReportProduct, 0, len(list))
		for _, product := range list {
			skill, e := s.SkillModel.FindByCode(ctx, product.Code)
			if e != nil || skill.Status != model.SkillStatusEnabled {
				continue
			}
			product.ToolConfig = skill.ToolConfig
			product.Version = reportAgentVersion(product.Version, s.AgentVersion)
			filtered = append(filtered, product)
		}
		list = filtered
	}
	for i := range list {
		_ = json.Unmarshal([]byte(list[i].ChaptersJSON), &list[i].Chapters)
		cfg, e := agent.ParseToolConfig(list[i].ToolConfig)
		list[i].Ready = e == nil
		list[i].ExecutionNote = "AI 文化参考 · 不含计算图盘"
		if e != nil {
			list[i].ExecutionNote = "配置暂不可用"
		}
		if cfg.Enabled {
			list[i].Ready = s.MCP.Configured(cfg)
			list[i].ExecutionNote = "解读前调用计算工具"
			if !list[i].Ready {
				list[i].ExecutionNote = "计算工具尚未配置，请稍后再试"
			}
		}
	}
	for i := range list {
		for _, code := range reportTools(list[i].Code) {
			if s.SkillModel == nil {
				list[i].Ready = false
				list[i].ExecutionNote = "计算能力暂不可用"
				break
			}
			skill, e := s.SkillModel.FindByCode(ctx, code)
			if e != nil || skill.Status != model.SkillStatusEnabled {
				list[i].Ready = false
				list[i].ExecutionNote = "计算能力暂未开放"
				break
			}
			cfg, e := agent.ParseToolConfig(skill.ToolConfig)
			if e != nil || !cfg.Enabled || !s.MCP.Configured(cfg) {
				list[i].Ready = false
				list[i].ExecutionNote = "计算能力暂不可用"
				break
			}
		}
		if list[i].Ready && len(reportTools(list[i].Code)) > 0 {
			list[i].ExecutionNote = "排盘图表 + AI 专题解读"
		}
		if list[i].Code == "face_palm" {
			if _, e := reportVisionModel(ctx, s); e != nil {
				list[i].Ready = false
				list[i].ExecutionNote = "图片分析模型暂未开放"
			}
		}
	}

	return list, err
}
func ReportGet(ctx context.Context, s *svc.ServiceContext, user string, id int64) (*Report, error) {
	var r Report
	if err := s.DB.QueryRowPartialCtx(ctx, &r, `SELECT `+reportSelectCols+` FROM ai_report WHERE id=? AND user_id=?`, id, user); err != nil {
		return nil, common.ErrForbidden
	}
	// A refund revokes cash access immediately. Client claims never grant access.
	var paid int64
	if err := s.DB.QueryRowCtx(ctx, &paid, `SELECT COUNT(*) FROM askxuan_payment.payment WHERE order_type='ai_report' AND order_no=? AND user_id=? AND status='success' AND CAST(ROUND(amount*100) AS SIGNED)=?`, r.ReportNo, user, r.PriceCents); err != nil {
		return nil, err
	}
	r.Unlocked = r.PointsPaid == 1 || paid > 0
	_ = json.Unmarshal([]byte(r.ChaptersJSON), &r.Chapters)
	if !r.Unlocked {
		r.Content = ""
	} else if r.DocumentJSON != "" {
		var doc reportdoc.Document
		if json.Unmarshal([]byte(r.DocumentJSON), &doc) == nil {
			// Raw evidence stays private; the reader receives normalized chart blocks.
			doc.Evidence = nil
			r.Document = &doc
		}
	}
	return &r, nil
}
func ReportList(ctx context.Context, s *svc.ServiceContext, user string, page int) ([]Report, error) {
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		return nil, common.ErrParam
	}
	rows := make([]Report, 0)
	// List never contains paid content or personal structured inputs.
	err := s.DB.QueryRowsPartialCtx(ctx, &rows, `SELECT id,report_no,user_id,skill_code,title,version,question,'{}' inputs_json,chapters_json,price_cents,points_price,status,summary,'' content,error_message,points_paid,created_at FROM ai_report WHERE user_id=? ORDER BY id DESC LIMIT 20 OFFSET ?`, user, (page-1)*20)
	for i := range rows {
		_ = json.Unmarshal([]byte(rows[i].ChaptersJSON), &rows[i].Chapters)
	}
	return rows, err
}
func ReportCreate(ctx context.Context, s *svc.ServiceContext, user string, req ReportRequest) (*Report, error) {
	if len(req.RequestKey) < 8 || len(req.RequestKey) > 64 || strings.TrimSpace(req.Question) == "" {
		return nil, common.ErrParam
	}
	var existing int64
	err := s.DB.QueryRowCtx(ctx, &existing, `SELECT id FROM ai_report WHERE user_id=? AND request_key=?`, user, req.RequestKey)
	if err == nil {
		return ReportGet(ctx, s, user, existing)
	}
	if !errors.Is(err, sqlx.ErrNotFound) {
		return nil, err
	}
	products, err := ReportProducts(ctx, s)
	if err != nil {
		return nil, err
	}
	var p *ReportProduct
	for i := range products {
		if products[i].Code == req.SkillCode {
			p = &products[i]
		}
	}
	if p == nil {
		return nil, common.ErrParam
	}
	if !p.Ready {
		return nil, common.NewBizError(50301, p.ExecutionNote)
	}
	skill, err := s.SkillModel.FindByCode(ctx, req.SkillCode)
	if err != nil {
		return nil, err
	}
	inputs, err := s.Guard.Validate(agent.GuidedInputSchema(skill.Code, skill.InputSchema), req.Question, req.Inputs)
	if err != nil {
		return nil, common.ErrParamInvalid
	}
	if _, err := agent.BuildToolArguments(skill.Code, req.Question, inputs, time.Now()); err != nil {
		return nil, common.ErrParamInvalid
	}
	if req.SkillCode == "marriage" {
		for _, facts := range []map[string]any{req.Inputs, reportPartnerFacts(req.Inputs)} {
			raw, _ := json.Marshal(facts)
			args, e := agent.BuildToolArguments("bazi", req.Question, string(raw), time.Now())
			if e != nil || args == "" {
				return nil, common.ErrParamInvalid
			}
		}
	}
	if len(req.Attachments) > 0 || req.SkillCode == "face_palm" {
		if len(req.Attachments) != 1 || req.SkillCode != "face_palm" {
			return nil, common.ErrParamInvalid
		}
		if _, e := encodeAttachments(req.Attachments); e != nil {
			return nil, common.ErrParamInvalid
		}
		if _, e := reportVisionModel(ctx, s); e != nil {
			return nil, common.NewBizError(50301, "图片分析模型暂不可用，请稍后再试")
		}
		stored := map[string]any{}
		_ = json.Unmarshal([]byte(inputs), &stored)
		stored["_reportImages"] = req.Attachments
		raw, _ := json.Marshal(stored)
		inputs = string(raw)
	}
	if err = s.UsageModel.Acquire(ctx, user, s.AIConfig.MinuteRequestLimit, s.AIConfig.DailyRequestLimit); err != nil {
		return nil, common.ErrTooManyRequest
	}
	result, err := s.DB.ExecCtx(ctx, `INSERT INTO ai_report(report_no,user_id,request_key,skill_code,title,version,question,inputs_json,chapters_json,price_cents,points_price,summary,content) VALUES(?,?,?,?,?,?,?,?,?,?,?,'','')`, "AR"+strings.ReplaceAll(uuid.NewString(), "-", ""), user, req.RequestKey, p.Code, p.Title, p.Version, req.Question, inputs, p.ChaptersJSON, p.PriceCents, p.PointsPrice)
	if err != nil {
		if e := s.DB.QueryRowCtx(ctx, &existing, `SELECT id FROM ai_report WHERE user_id=? AND request_key=?`, user, req.RequestKey); e == nil {
			return ReportGet(ctx, s, user, existing)
		}
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	go generateReport(s, id)
	return ReportGet(ctx, s, user, id)
}
func ReportRetry(ctx context.Context, s *svc.ServiceContext, user string, id int64) (*Report, error) {
	// Expired jobs can be retried after process termination; CAS prevents concurrent retries.
	r, err := ReportGet(ctx, s, user, id)
	if err != nil {
		return nil, err
	}
	if r.Status == "ready" {
		return r, nil
	}
	if err = s.UsageModel.Acquire(ctx, user, s.AIConfig.MinuteRequestLimit, s.AIConfig.DailyRequestLimit); err != nil {
		return nil, common.ErrTooManyRequest
	}
	res, err := s.DB.ExecCtx(ctx, `UPDATE ai_report SET status='generating',version=?,error_message='',updated_at=CURRENT_TIMESTAMP WHERE id=? AND user_id=? AND (status='failed' OR (status='generating' AND updated_at<DATE_SUB(NOW(),INTERVAL 5 MINUTE)))`, reportAgentVersion(r.Version, s.AgentVersion), id, user)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		go generateReport(s, id)
	}
	return ReportGet(ctx, s, user, id)
}
func generateReport(s *svc.ServiceContext, id int64) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(max(60, s.AIConfig.TaskTimeoutSeconds))*time.Second)
	defer cancel()
	var r Report
	if err := s.DB.QueryRowPartialCtx(ctx, &r, `SELECT `+reportSelectCols+` FROM ai_report WHERE id=?`, id); err != nil {
		return
	}
	fail := func() {
		_, _ = s.DB.ExecCtx(context.Background(), `UPDATE ai_report SET status='failed',generation_stage='failed',error_message='报告暂未生成成功，请免费重试；尚未扣款' WHERE id=? AND status='generating'`, id)
	}
	body, doc, usage, err := executeReport(ctx, s, &r)
	if err != nil || !validReportBody(body) || s.Guard.ValidateOutput(body.Content+body.Summary) != nil {
		fail()
		return
	}
	raw, _ := json.Marshal(doc)
	_, _ = s.DB.ExecCtx(ctx, `UPDATE ai_report SET status='ready',generation_stage='complete',summary=?,content=?,document_json=?,provider=?,model=?,prompt_tokens=?,completion_tokens=?,cost_micros=?,error_message='' WHERE id=? AND status='generating'`, body.Summary, body.Content, string(raw), s.Provider.Name(), usage.Model, usage.PromptTokens, usage.CompletionTokens, calculateCostMicros(usage, s.AIConfig, time.Now()), id)
}

type reportBody struct {
	Summary string `json:"summary"`
	Content string `json:"content"`
}

func validReportBody(b reportBody) bool {
	return len([]rune(strings.TrimSpace(b.Content))) >= 300 && len([]rune(strings.TrimSpace(b.Summary))) >= 20 && len([]rune(b.Summary)) <= 500 && strings.Count(b.Content, "## ") >= 3
}

// A report-linked conversation is created only after server-side entitlement verification.
func ReportConversation(ctx context.Context, s *svc.ServiceContext, user string, id int64) (map[string]int64, error) {
	r, err := ReportGet(ctx, s, user, id)
	if err != nil {
		return nil, err
	}
	if !r.Unlocked || r.Status != "ready" {
		return nil, common.ErrForbidden
	}
	var sid int64
	err = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if err := tx.QueryRowCtx(ctx, &sid, `SELECT chat_session_id FROM ai_report WHERE id=? AND user_id=? FOR UPDATE`, id, user); err != nil {
			return err
		}
		if sid > 0 {
			var status string
			err := tx.QueryRowCtx(ctx, &status, `SELECT status FROM ai_session WHERE id=? AND user_id=? FOR UPDATE`, sid, user)
			if err == nil && status == model.SessionStatusActive {
				return nil
			}
			if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
				return err
			}
			// A deleted follow-up must never be resurrected. The purchased report remains available.
		}
		result, err := tx.ExecCtx(ctx, `INSERT INTO ai_session(session_no,user_id,skill_code,selection_mode,skill_version,title,status) VALUES(?,?,'general','explicit','1.0',?,'active')`, strings.ReplaceAll(uuid.NewString(), "-", ""), user, "报告追问 · "+r.Title)
		if err != nil {
			return err
		}
		sid, err = result.LastInsertId()
		if err != nil {
			return err
		}
		_, err = tx.ExecCtx(ctx, `INSERT INTO ai_message(session_id,role,content,input_json,attachments_json,status) VALUES(?,'assistant',?,'{}','[]','completed')`, sid, "已关联《"+r.Title+"》报告及原始资料。请直接提出你想深入了解的问题；需要新增计算时，我会选择相应工具。")
		if err != nil {
			return err
		}
		_, err = tx.ExecCtx(ctx, `UPDATE ai_report SET chat_session_id=? WHERE id=?`, sid, id)
		return err
	})
	return map[string]int64{"sessionId": sid}, err
}

// ai_report.version is VARCHAR(20); retain the product prefix when it fits.
func reportAgentVersion(base string, version int64) string {
	base = strings.Split(base, "@a")[0]
	if version <= 0 {
		return base
	}
	suffix := fmt.Sprintf("@a%d", version)
	if len(suffix) >= 20 {
		return suffix[1:]
	}
	runes := []rune(base)
	if len(runes) > 20-len(suffix) {
		runes = runes[:20-len(suffix)]
	}
	return string(runes) + suffix
}

func reportVisionModel(ctx context.Context, s *svc.ServiceContext) (string, error) {
	if s.Models == nil {
		return "", provider.ErrModelNeedsVision
	}
	models, e := s.Models.List(ctx)
	if e != nil {
		return "", e
	}
	for _, m := range models.List {
		if m.SupportsVision {
			return m.ID, nil
		}
	}
	return "", provider.ErrModelNeedsVision
}
