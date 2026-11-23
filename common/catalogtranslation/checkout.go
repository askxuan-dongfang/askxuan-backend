package catalogtranslation

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrCheckoutRevision = common.NewBizError(46010, "The English terms have changed. Reload and review them before continuing.")

type CheckoutItem struct {
	Kind string
	ID   int64
}

type CheckoutSnapshot map[string]Published

// ConfirmCheckout accepts the exact English that the customer reviewed.
// A nil map represents an existing Chinese checkout.
func ConfirmCheckout(ctx context.Context, tx sqlx.Session, expected map[string]string, items []CheckoutItem) (CheckoutSnapshot, error) {
	if expected == nil {
		return nil, nil
	}
	if len(items) == 0 || len(expected) == 0 || len(expected) > 100 {
		return nil, ErrCheckoutRevision
	}
	groups := make(map[string][]string)
	seen := make(map[CheckoutItem]struct{}, len(items))
	for _, item := range items {
		if _, duplicate := seen[item]; duplicate {
			continue
		}
		seen[item] = struct{}{}
		id := strconv.FormatInt(item.ID, 10)
		if expected[item.Kind+":"+id] == "" {
			return nil, ErrCheckoutRevision
		}
		groups[item.Kind] = append(groups[item.Kind], id)
	}
	if len(seen) != len(expected) {
		return nil, ErrCheckoutRevision
	}
	result := make(CheckoutSnapshot, len(seen))
	for kind, ids := range groups {
		published, err := LookupSession(ctx, tx, kind, ids)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			key := kind + ":" + id
			item := published[id]
			if !item.TransactionReady || item.Revision != expected[key] {
				return nil, ErrCheckoutRevision
			}
			result[key] = item
		}
	}
	return result, nil
}

func SaveCheckoutSnapshot(ctx context.Context, tx sqlx.Session, id int64, snapshot CheckoutSnapshot) error {
	if len(snapshot) == 0 {
		return nil
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = tx.ExecCtx(ctx, `INSERT INTO catalog_checkout_snapshot (id, payload) VALUES (?,?)`, id, data)
	return err
}

// LoadCheckoutSnapshots is called only with IDs selected by the order's owner or admin policy.
func LoadCheckoutSnapshots(ctx context.Context, db sqlx.Session, ids []int64) (map[int64]CheckoutSnapshot, error) {
	result := make(map[int64]CheckoutSnapshot)
	if len(ids) == 0 {
		return result, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	var rows []struct {
		ID      int64  `db:"id"`
		Payload []byte `db:"payload"`
	}
	query := `SELECT id,payload FROM catalog_checkout_snapshot WHERE id IN (?` + strings.Repeat(",?", len(ids)-1) + ")"
	if err := db.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	for _, row := range rows {
		var snapshot CheckoutSnapshot
		if err := json.Unmarshal(row.Payload, &snapshot); err != nil {
			return nil, err
		}
		result[row.ID] = snapshot
	}
	return result, nil
}
