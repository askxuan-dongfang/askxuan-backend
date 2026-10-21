package model

import (
	"context"
	"strings"

	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// InsertWithAffiliation keeps the new master, its qualification queue entry and
// all standard affiliations in one transaction. A failed association cannot
// leave a visible or assignable half-created profile behind.
func (m *masterModel) InsertWithAffiliation(ctx context.Context, data *Master, codes []string) (int64, error) {
	belief, err := common.CanonicalBelief(data.BeliefCode)
	if err != nil {
		return 0, err
	}
	if err := common.ValidateTraditionCodeShape(codes, true); err != nil {
		return 0, err
	}
	data.BeliefCode = belief
	var id int64
	err = m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if data.TempleCode != "" {
			var templeCode string
			if err := tx.QueryRowCtx(ctx, &templeCode, "SELECT code FROM askxuan_temple.temple WHERE code=? FOR SHARE", data.TempleCode); err != nil {
				return err
			}
		}
		if err := common.ValidateTraditions(ctx, tx, belief, codes, nil, "", true); err != nil {
			return err
		}
		var err error
		if id, err = insertMaster(ctx, tx, data); err != nil {
			return err
		}
		if _, err = tx.ExecCtx(ctx, "INSERT INTO master_audit(master_code,temple_code,credential_urls,status) VALUES (?,?,?,?)", data.Code, data.TempleCode, "", MasterAuditStatusPending); err != nil {
			return err
		}
		return common.InsertProviderTraditions(ctx, tx, "master", data.Code, codes)
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

// codes==nil preserves legacy and disabled relationships. Callers must enforce
// the existing platform_super privilege before supplying a replacement set.
func (m *masterModel) UpdateWithAffiliation(ctx context.Context, data *Master, codes []string) error {
	return m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var previousBelief string
		if err := tx.QueryRowCtx(ctx, &previousBelief, "SELECT belief_code FROM master WHERE id=? AND code=? FOR UPDATE", data.Id, data.Code); err != nil {
			return err
		}
		belief, err := common.CanonicalBelief(data.BeliefCode)
		if err != nil && (previousBelief != data.BeliefCode || codes != nil) {
			return err
		}
		if err == nil {
			data.BeliefCode = belief
		}
		existing, err := common.ProviderTraditions(ctx, tx, "master", data.Code)
		if err != nil {
			return err
		}
		previous, _ := common.CanonicalBelief(previousBelief)
		selected := codes
		if selected == nil {
			selected = existing
		}
		if codes != nil || belief != previous {
			if err := common.ValidateTraditions(ctx, tx, belief, selected, existing, previousBelief, false); err != nil {
				return err
			}
		}
		if err := updateMaster(ctx, tx, data); err != nil {
			return err
		}
		if codes == nil {
			data.TraditionCodes = strings.Join(existing, ",")
			return nil
		}
		if _, err := tx.ExecCtx(ctx, "DELETE FROM askxuan_temple.provider_directory_tag WHERE provider_kind='master' AND provider_code=?", data.Code); err != nil {
			return err
		}
		if err := common.InsertProviderTraditions(ctx, tx, "master", data.Code, codes); err != nil {
			return err
		}
		data.TraditionCodes = strings.Join(codes, ",")
		return nil
	})
}
