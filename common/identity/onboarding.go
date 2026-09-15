package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrApplicationConflict = errors.New("申请已更新，请刷新后重试")
var ErrOnboardingDenied = errors.New("无权操作此申请或账号")

func ApplicantRole(kind string) string {
	switch kind {
	case "master":
		return "master_applicant"
	case "temple":
		return "temple_applicant"
	}
	return ""
}
func IsApplicant(role string) bool { return role == "master_applicant" || role == "temple_applicant" }

type WorkProfile struct {
	Name               string   `json:"name"`
	LegalName          string   `json:"legalName"`
	Contact            string   `json:"contact"`
	Region             string   `json:"region"`
	Address            string   `json:"address"`
	Belief             string   `json:"belief"`
	Sect               string   `json:"sect"`
	Position           string   `json:"position"`
	RegistrationNumber string   `json:"registrationNumber"`
	Description        string   `json:"description"`
	EvidenceIDs        []string `json:"evidenceIds"`
}
type Application struct {
	ID          int64  `db:"id" json:"id"`
	AccountID   int64  `db:"account_id" json:"accountId"`
	Kind        string `db:"kind" json:"kind"`
	Status      string `db:"status" json:"status"`
	ProfileJSON string `db:"profile_json" json:"-"`
	Revision    int    `db:"revision" json:"revision"`
	ReviewNote  string `db:"review_note" json:"reviewNote"`
	EntityCode  string `db:"entity_code" json:"entityCode"`
	UpdatedAt   string `db:"updated_at" json:"updatedAt"`
}

const applicationColumns = "id,account_id,kind,status,profile_json,revision,review_note,entity_code,DATE_FORMAT(updated_at,'%Y-%m-%d %H:%i:%s') updated_at"

func (a *Application) decode() error {
	var p WorkProfile
	return json.Unmarshal([]byte(a.ProfileJSON), &p)
}
func (a Application) MarshalJSON() ([]byte, error) {
	type row Application
	var p WorkProfile
	if e := json.Unmarshal([]byte(a.ProfileJSON), &p); e != nil {
		return nil, e
	}
	if p.EvidenceIDs == nil {
		p.EvidenceIDs = []string{}
	}
	return json.Marshal(struct {
		row
		Profile WorkProfile `json:"profile"`
	}{row(a), p})
}
func (s *Accounts) WorkApplication(ctx context.Context, account int64) (*Application, error) {
	var a Application
	if e := s.DB.QueryRowCtx(ctx, &a, "SELECT "+applicationColumns+" FROM onboarding_application WHERE account_id=?", account); e != nil {
		return nil, e
	}
	return &a, a.decode()
}
func (s *Accounts) WorkApplications(ctx context.Context, status string, page int) ([]Application, error) {
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		return nil, ErrOnboardingDenied
	}
	if status != "" && status != "draft" && status != "submitted" && status != "rejected" && status != "approved" {
		return nil, fmt.Errorf("申请状态无效")
	}
	rows := []Application{}
	e := s.DB.QueryRowsCtx(ctx, &rows, "SELECT "+applicationColumns+" FROM onboarding_application WHERE (?='' OR status=?) ORDER BY id DESC LIMIT 50 OFFSET ?", status, status, (page-1)*50)
	for i := range rows {
		if err := rows[i].decode(); err != nil {
			return nil, err
		}
	}
	return rows, e
}
func (s *Accounts) RegisterWork(ctx context.Context, kind, email, username, password, code, agreement string) (int64, error) {
	role := ApplicantRole(kind)
	if role == "" {
		return 0, fmt.Errorf("注册入口无效")
	}
	email, e := NormalizeEmail(email)
	if e != nil {
		return 0, e
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernamePattern.MatchString(username) {
		return 0, fmt.Errorf("用户名需以字母开头，包含 4–32 位小写字母、数字或下划线")
	}
	if agreement != AgreementVersion {
		return 0, fmt.Errorf("请阅读并同意当前用户协议和隐私政策")
	}
	hash, e := HashPassword(password)
	if e != nil {
		return 0, e
	}
	if e = s.Challenges.Consume(ctx, "email", "admin:"+kind+"_register:"+email, code, 5); e != nil {
		return 0, e
	}
	var id int64
	e = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var reserved int64
		if e := tx.QueryRowCtx(ctx, &reserved, "SELECT COUNT(*) FROM master_invitation WHERE email=?", email); e != nil {
			return e
		}
		if reserved > 0 {
			return fmt.Errorf("该邮箱已有寺院分配账号，请从大师端激活")
		}
		var roleID int64
		if e := tx.QueryRowCtx(ctx, &roleID, "SELECT id FROM role WHERE code=?", role); e != nil {
			return e
		}
		r, e := tx.ExecCtx(ctx, "INSERT INTO admin_account(account,password,name,role_id,status) VALUES (?,?,?,?,'enabled')", username, hash, username, roleID)
		if e != nil {
			return e
		}
		id, e = r.LastInsertId()
		if e != nil {
			return e
		}
		if _, e = tx.ExecCtx(ctx, "INSERT INTO auth_identity(domain,user_id,username,email,password_hash,agreement_version,verified_at) VALUES ('admin',?,?,?,?,?,NOW())", id, username, email, hash, agreement); e != nil {
			return e
		}
		_, e = tx.ExecCtx(ctx, "INSERT INTO onboarding_application(account_id,kind,profile_json) VALUES (?,?,'{}')", id, kind)
		return e
	})
	if e != nil {
		return 0, fmt.Errorf("注册未完成，邮箱或用户名可能已使用；寺院分配账号请使用激活入口")
	}
	return id, nil
}
func validateWorkProfile(p WorkProfile, kind string, submit bool) error {
	for _, v := range []struct {
		s string
		n int
	}{{p.Name, 64}, {p.LegalName, 64}, {p.Contact, 64}, {p.Region, 64}, {p.Address, 255}, {p.Belief, 32}, {p.Sect, 32}, {p.Position, 32}, {p.RegistrationNumber, 100}, {p.Description, 512}} {
		if utf8.RuneCountInString(v.s) > v.n {
			return fmt.Errorf("资料字段超出长度限制")
		}
	}
	if len(p.EvidenceIDs) > 8 {
		return fmt.Errorf("最多提交 8 份证明材料")
	}
	if submit {
		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.LegalName) == "" || strings.TrimSpace(p.Contact) == "" || strings.TrimSpace(p.Region) == "" || strings.TrimSpace(p.Sect) == "" || len(p.EvidenceIDs) == 0 {
			return fmt.Errorf("请补齐名称、本人／经办人姓名、联系方式、地区、宗派和证明材料")
		}
		if kind == "temple" && (strings.TrimSpace(p.Address) == "" || strings.TrimSpace(p.RegistrationNumber) == "") {
			return fmt.Errorf("请补齐寺院地址和登记证号，并上传机构证明、管理授权")
		}
		if kind == "master" && strings.TrimSpace(p.Position) == "" {
			return fmt.Errorf("请填写身份／职务")
		}
		switch p.Belief {
		case "han_buddhism", "tibetan_buddhism", "taoism", "folk":
		default:
			return fmt.Errorf("请选择信仰流派")
		}
	}
	return nil
}
func (s *Accounts) SaveWorkApplication(ctx context.Context, account int64, p WorkProfile, revision int, submit bool) (*Application, error) {
	e := s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var a Application
		if e := tx.QueryRowCtx(ctx, &a, "SELECT "+applicationColumns+" FROM onboarding_application WHERE account_id=? FOR UPDATE", account); e != nil {
			return e
		}
		if a.Revision != revision {
			return ErrApplicationConflict
		}
		if a.Status != "draft" && a.Status != "rejected" {
			return fmt.Errorf("当前申请不可修改")
		}
		if e := validateWorkProfile(p, a.Kind, submit); e != nil {
			return e
		}
		for _, id := range p.EvidenceIDs {
			var n int64
			if e := tx.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM onboarding_evidence WHERE id=? AND account_id=?", id, account); e != nil {
				return e
			}
			if n != 1 {
				return fmt.Errorf("证明材料不存在或不属于当前账号")
			}
		}
		raw, _ := json.Marshal(p)
		status := a.Status
		action := "save"
		if submit {
			status = "submitted"
			action = "submit"
		}
		if _, e := tx.ExecCtx(ctx, "UPDATE onboarding_application SET profile_json=?,status=?,revision=revision+1 WHERE id=?", string(raw), status, a.ID); e != nil {
			return e
		}
		_, e := tx.ExecCtx(ctx, "INSERT INTO onboarding_event(application_id,actor_id,action,revision,profile_json) VALUES (?,?,?,?,?)", a.ID, account, action, a.Revision+1, string(raw))
		return e
	})
	if e != nil {
		return nil, e
	}
	return s.WorkApplication(ctx, account)
}

// Review locks the immutable submitted revision and provisions the business entity
// and role in the same transaction. Caller must verify the reviewer's current role.
func (s *Accounts) ReviewWorkApplication(ctx context.Context, reviewer, id int64, revision int, approve bool, note string) error {
	if len([]rune(note)) > 1000 || (!approve && strings.TrimSpace(note) == "") {
		return fmt.Errorf("退回时请填写原因（最多 1000 字）")
	}
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var a Application
		if e := tx.QueryRowCtx(ctx, &a, "SELECT "+applicationColumns+" FROM onboarding_application WHERE id=? FOR UPDATE", id); e != nil {
			return e
		}
		if a.Status != "submitted" || a.Revision != revision {
			return ErrApplicationConflict
		}
		if a.AccountID == reviewer {
			return ErrOnboardingDenied
		}
		if e := a.decode(); e != nil {
			return e
		}
		status := "rejected"
		entity := ""
		if approve {
			var profile WorkProfile
			if e := json.Unmarshal([]byte(a.ProfileJSON), &profile); e != nil {
				return e
			}
			if e := validateWorkProfile(profile, a.Kind, true); e != nil {
				return e
			}
			role := a.Kind
			if a.Kind == "temple" {
				role = "temple_admin"
			}
			var roleID int64
			if e := tx.QueryRowCtx(ctx, &roleID, "SELECT id FROM role WHERE code=?", role); e != nil {
				return e
			}
			var currentRole string
			if e := tx.QueryRowCtx(ctx, &currentRole, "SELECT r.code FROM admin_account a JOIN role r ON r.id=a.role_id WHERE a.id=? AND a.status='enabled' FOR UPDATE", a.AccountID); e != nil {
				return e
			}
			if currentRole != ApplicantRole(a.Kind) {
				return ErrOnboardingDenied
			}
			p := profile
			beliefName := map[string]string{"han_buddhism": "汉传佛教", "tibetan_buddhism": "藏传佛教", "taoism": "道教", "folk": "民间信仰"}[p.Belief]
			entity = "N" + strings.ToUpper(RandomID()[:14])
			if a.Kind == "master" {
				_, e := tx.ExecCtx(ctx, `INSERT INTO askxuan_master.master(code,dharma_name,lay_name,temple_code,position,belief_code,sect,type,auth_status,shelf_status,platform_status,manage_by,specialties,avatar,rating,consult_enabled,consult_fee,consult_valid_hours,consult_response_minutes) VALUES (?,?,?,'',?,?,?,?,'已认证','off_shelf','normal','platform',?,'',0,0,0,72,30)`, entity, p.Name, p.LegalName, p.Position, p.Belief, p.Sect, beliefName, "")
				if e != nil {
					return e
				}
				if _, e = tx.ExecCtx(ctx, "INSERT INTO askxuan_master.master_profile_ext(master_code,bio,pricing) VALUES (?,?,'')", entity, p.Description); e != nil {
					return e
				}
				if _, e = tx.ExecCtx(ctx, "UPDATE admin_account SET role_id=?,name=?,master_id=?,temple_id='' WHERE id=?", roleID, p.Name, entity, a.AccountID); e != nil {
					return e
				}
			} else if a.Kind == "temple" {
				_, e := tx.ExecCtx(ctx, `INSERT INTO askxuan_temple.temple(code,name,region,type,belief_code,sect,status,address,description) VALUES (?,?,?,?,?,?,'正常',?,?)`, entity, p.Name, p.Region, beliefName, p.Belief, p.Sect, p.Address, p.Description)
				if e != nil {
					return e
				}
				if _, e = tx.ExecCtx(ctx, "UPDATE admin_account SET role_id=?,name=?,temple_id=?,master_id='' WHERE id=?", roleID, p.LegalName, entity, a.AccountID); e != nil {
					return e
				}
				if _, e = tx.ExecCtx(ctx, "INSERT INTO askxuan_temple.temple_admin(temple_code,account_id,role) VALUES (?,?,'admin')", entity, a.AccountID); e != nil {
					return e
				}
			} else {
				return ErrOnboardingDenied
			}
			// Existing applicant sessions cannot gain workbench privileges by refresh.
			if _, e := s.Challenges.Redis.IncrCtx(ctx, EpochKey("admin", a.AccountID)); e != nil {
				return e
			}
			status = "approved"
		}
		if _, e := tx.ExecCtx(ctx, "UPDATE onboarding_application SET status=?,review_note=?,entity_code=?,revision=revision+1 WHERE id=?", status, note, entity, id); e != nil {
			return e
		}
		_, e := tx.ExecCtx(ctx, "INSERT INTO onboarding_event(application_id,actor_id,action,revision,note,profile_json) VALUES (?,?,?,?,?,?)", id, reviewer, status, a.Revision+1, note, a.ProfileJSON)
		return e
	})
}
