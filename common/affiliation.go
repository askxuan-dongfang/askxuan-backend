package common

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// CanonicalBelief validates new writes. Legacy display inference must never be
// used to manufacture a standardized affiliation for a new entity.
func CanonicalBelief(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "taoism" {
		code = "daoism"
	}
	switch code {
	case "han_buddhism", "tibetan_buddhism", "theravada_buddhism", "daoism", "folk":
		return code, nil
	}
	return "", NewBizError(40010, "请选择五类有效的信仰类型")
}

func BeliefDisplayName(code string) string {
	return map[string]string{"han_buddhism": "汉传佛教", "tibetan_buddhism": "藏传佛教", "theravada_buddhism": "南传佛教", "daoism": "道教", "taoism": "道教", "folk": "民间信仰"}[code]
}

var traditionCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)

// Drafts preserve unresolved choices; submission additionally checks the live
// catalog inside its transaction. nil means omitted on a legacy update.
func ValidateTraditionCodeShape(codes []string, required bool) error {
	if len(codes) > 12 || required && len(codes) == 0 {
		return NewBizError(40010, "请选择 1—12 项宗派或信俗标签")
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if !traditionCodePattern.MatchString(code) || seen[code] {
			return NewBizError(40010, "宗派或信俗编码无效或重复")
		}
		seen[code] = true
	}
	return nil
}

// ValidateTraditions locks the catalog to prevent an enable/parent change racing
// the write. Retained historical cross-parent or disabled relationships survive
// ordinary updates; additions must always be enabled and under the chosen type.
func ValidateTraditions(ctx context.Context, tx sqlx.Session, belief string, codes, existing []string, previousBelief string, required bool) error {
	canonical, err := CanonicalBelief(belief)
	if err != nil {
		return err
	}
	if err = ValidateTraditionCodeShape(codes, required); err != nil {
		return err
	}
	previous, _ := CanonicalBelief(previousBelief)
	old := map[string]bool{}
	for _, code := range existing {
		old[code] = true
	}
	ordered := append([]string(nil), codes...)
	sort.Strings(ordered)
	for _, code := range ordered {
		var tag struct {
			Group  string `db:"tag_group"`
			Parent string `db:"belief_code"`
			Status string `db:"status"`
		}
		// FOR SHARE prevents concurrent catalog changes without requiring the
		// authenticating/master service to gain catalog write privileges.
		err = tx.QueryRowCtx(ctx, &tag, "SELECT tag_group,belief_code,status FROM askxuan_temple.directory_tag WHERE code=? FOR SHARE", code)
		retained := old[code] && previous == canonical
		if errors.Is(err, sqlx.ErrNotFound) && retained {
			continue // Preserve an existing unresolved relationship for explicit review.
		}
		if errors.Is(err, sqlx.ErrNotFound) {
			return NewBizError(40010, "宗派或信俗标签不存在，请重新选择")
		}
		if err != nil {
			return err
		}
		if tag.Group != "tradition" || !retained && (tag.Status != "enabled" || tag.Parent != canonical) {
			return NewBizError(40010, "请选择当前信仰类型下已启用的宗派或信俗；历史关联变更需明确复核")
		}
	}
	return nil
}

// ProviderTraditions is called after locking the owning entity, serializing
// profile and relationship updates without dropping disabled historical rows.
func ProviderTraditions(ctx context.Context, tx sqlx.Session, kind, code string) ([]string, error) {
	rows := []string{}
	err := tx.QueryRowsCtx(ctx, &rows, "SELECT tag_code FROM askxuan_temple.provider_directory_tag WHERE provider_kind=? AND provider_code=? ORDER BY tag_code FOR UPDATE", kind, code)
	return rows, err
}

func InsertProviderTraditions(ctx context.Context, tx sqlx.Session, kind, code string, codes []string) error {
	for _, tag := range codes {
		if _, err := tx.ExecCtx(ctx, "INSERT INTO askxuan_temple.provider_directory_tag(provider_kind,provider_code,tag_code) VALUES(?,?,?)", kind, code, tag); err != nil {
			return err
		}
	}
	return nil
}
