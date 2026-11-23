package catalogjob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/askxuan/common"
	catalog "github.com/askxuan/common/catalogtranslation"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var (
	errNotFound = common.NewBizError(40417, "找不到可翻译的原始内容")
	errConflict = common.NewBizError(40932, "中文内容或英文稿已更新，请刷新后重试")
	errContent  = common.NewBizError(40016, "请根据当前中文内容填写所有英文栏位")
)

type Store struct{ DB sqlx.SqlConn }

type Draft struct {
	Revision       string            `json:"revision"`
	Status         string            `json:"status"`
	Source         map[string]string `json:"source"`
	Fields         map[string]string `json:"fields"`
	ReviewRequired bool              `json:"reviewRequired"`
	PublicEligible bool              `json:"publicEligible"`
}

type Edit struct {
	Revision string            `json:"revision"`
	Fields   map[string]string `json:"fields"`
	Publish  bool              `json:"publish"`
}

type ledgerRow struct {
	SourceRevision string `db:"source_revision"`
	Status         string `db:"status"`
	Version        int64  `db:"version"`
	Fields         string `db:"fields"`
}

func draftRow(ctx context.Context, db sqlx.Session, kind, id string, lock bool) (ledgerRow, error) {
	query := `SELECT source_revision,status,version,COALESCE(fields,'{}') AS fields
		FROM catalog_translation WHERE kind=? AND resource_id=?`
	if lock {
		query += " FOR UPDATE"
	}
	row := ledgerRow{}
	err := db.QueryRowCtx(ctx, &row, query, kind, id)
	if errors.Is(err, sqlx.ErrNotFound) {
		err = nil
	}
	return row, err
}

func makeDraft(src catalog.Source, row ledgerRow) (Draft, error) {
	out := Draft{
		Revision: src.Revision + ":" + strconv.FormatInt(row.Version, 10),
		Status:   "missing", Source: src.Fields, Fields: map[string]string{},
		ReviewRequired: true, PublicEligible: src.PublicEligible,
	}
	if row.SourceRevision == src.Revision {
		out.Status = row.Status
		out.ReviewRequired = row.Status != "published"
		if err := json.Unmarshal([]byte(row.Fields), &out.Fields); err != nil {
			return Draft{}, fmt.Errorf("decode catalog translation draft: %w", err)
		}
	}
	return out, nil
}

// Manage accepts edits only to the current authoritative Chinese projection.
// For a temple session, templeCode comes from verified gateway claims.
func (s Store) Manage(ctx context.Context, kind, id, templeCode string, actorID int64, enqueue bool, edit *Edit) (Draft, error) {
	src, err := loadManagedSource(ctx, s.DB, kind, id, templeCode)
	if err != nil {
		return Draft{}, err
	}
	if !enqueue && edit == nil {
		row, err := draftRow(ctx, s.DB, kind, src.ID, false)
		if err != nil {
			return Draft{}, err
		}
		return makeDraft(src, row)
	}
	var draft Draft
	err = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		// Serialize new drafts, worker writes, and human approvals for this ID.
		if _, err := tx.ExecCtx(ctx, `INSERT INTO catalog_translation(kind,resource_id,source_revision)
			VALUES(?,?,'') ON DUPLICATE KEY UPDATE resource_id=VALUES(resource_id)`, kind, src.ID); err != nil {
			return err
		}
		row, err := draftRow(ctx, tx, kind, src.ID, true)
		if err != nil {
			return err
		}
		lockedSource, err := loadManagedSource(ctx, tx, kind, src.ID, templeCode)
		if err != nil {
			return err
		}
		draft, err = makeDraft(lockedSource, row)
		if err != nil {
			return err
		}
		if enqueue {
			if draft.Status != "missing" && draft.Status != "failed" {
				return nil
			}
			_, err = tx.ExecCtx(ctx, `UPDATE catalog_translation SET source_revision=?,status='queued',version=version+1,
				fields=NULL,editor_id=NULL,lease_until=NULL
				WHERE kind=? AND resource_id=?`, lockedSource.Revision, kind, lockedSource.ID)
			if err == nil {
				draft.Status = "queued"
				draft.Fields = map[string]string{}
			}
		} else {
			if edit.Revision != draft.Revision {
				return errConflict
			}
			fields, err := validateFields(draft.Source, edit.Fields, edit.Publish)
			if err != nil {
				return err
			}
			raw, err := json.Marshal(fields)
			if err != nil {
				return err
			}
			if edit.Publish {
				fingerprints := make(map[string]string, len(lockedSource.Fields))
				for key, text := range lockedSource.Fields {
					fingerprints[key] = fieldFingerprint(text)
				}
				fingerprintJSON, err := json.Marshal(fingerprints)
				if err != nil {
					return err
				}
				_, err = tx.ExecCtx(ctx, `UPDATE catalog_translation SET source_revision=?,status='published',version=version+1,
					fields=?,editor_id=?,published_fields=?,published_source_revision=?,published_source_fingerprints=?,
					published_by=?,published_at=UTC_TIMESTAMP(6),lease_until=NULL WHERE kind=? AND resource_id=?`,
					lockedSource.Revision, raw, actorID, raw, lockedSource.Revision, fingerprintJSON, actorID, kind, lockedSource.ID)
				if err != nil {
					return err
				}
				draft.Status = "published"
			} else {
				_, err = tx.ExecCtx(ctx, `UPDATE catalog_translation SET source_revision=?,status='review',version=version+1,
					fields=?,editor_id=?,lease_until=NULL WHERE kind=? AND resource_id=?`, lockedSource.Revision, raw, actorID, kind, lockedSource.ID)
				draft.Status = "review"
			}
			if err != nil {
				return err
			}
			draft.Fields = fields
			draft.ReviewRequired = !edit.Publish
		}
		if err != nil {
			return err
		}
		draft.Revision = lockedSource.Revision + ":" + strconv.FormatInt(row.Version+1, 10)
		return nil
	})
	return draft, err
}

func fieldFingerprint(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func loadManagedSource(ctx context.Context, db sqlx.Session, kind, id, templeCode string) (catalog.Source, error) {
	src, err := catalog.Load(ctx, db, kind, id)
	switch {
	case errors.Is(err, catalog.ErrLookup):
		return catalog.Source{}, common.ErrParamInvalid
	case errors.Is(err, sqlx.ErrNotFound):
		return catalog.Source{}, errNotFound
	case err != nil:
		return catalog.Source{}, err
	case templeCode != "" && src.OwnerTemple != templeCode:
		return catalog.Source{}, common.ErrTempleIsolation
	case src.Fields["title"] == "":
		return catalog.Source{}, errNotFound
	}
	return src, nil
}

func validateFields(source, fields map[string]string, complete bool) (map[string]string, error) {
	result := make(map[string]string)
	for name, text := range fields {
		if source[name] == "" || len(text) > 192<<10 {
			return nil, common.ErrParamInvalid
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if complete && strings.ContainsFunc(text, func(r rune) bool {
			return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
		}) {
			return nil, errContent
		}
		result[name] = text
	}
	if len(result) == 0 || (complete && len(result) != len(source)) {
		return nil, errContent
	}
	return result, nil
}
