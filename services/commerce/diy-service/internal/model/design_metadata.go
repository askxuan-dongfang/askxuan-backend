package model

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// DesignTags belongs to a published version, not the paid composition or draft.
type DesignTags struct {
	FiveElements []string `json:"fiveElements"`
	Meanings     []string `json:"meanings"`
}

type DesignListFilter struct {
	Keyword     string
	FiveElement string
	Meaning     string
}

func validFiveElement(code string) bool {
	switch code {
	case "metal", "wood", "water", "fire", "earth":
		return true
	}
	return false
}

func validMeaning(code string) bool {
	switch code {
	case "study", "social", "love", "wealth", "peace", "protection", "warding", "career", "purify", "calm", "sleep", "benefactor", "communication", "confidence", "beauty", "vitality":
		return true
	}
	return false
}

func normalizeDesignTags(tags *DesignTags) (*DesignTags, error) {
	if tags == nil {
		return nil, nil
	}
	if len(tags.FiveElements) > 5 || len(tags.Meanings) > 16 {
		return nil, common.ErrParam
	}
	out := &DesignTags{FiveElements: []string{}, Meanings: []string{}}
	seen := map[string]bool{}
	for _, code := range tags.FiveElements {
		if !validFiveElement(code) {
			return nil, common.ErrParam
		}
		if !seen[code] {
			out.FiveElements = append(out.FiveElements, code)
			seen[code] = true
		}
	}
	for _, code := range tags.Meanings {
		if !validMeaning(code) {
			return nil, common.ErrParam
		}
		if !seen[code] {
			out.Meanings = append(out.Meanings, code)
			seen[code] = true
		}
	}
	return out, nil
}

type DesignMetadata struct {
	AuthorName string
	UseCount   int64
	DesignTags
}

// ReadDesignMetadata enriches a whole page in one query. The author is the
// design's owner, never the financial beneficiary. Existing private copies count
// too; self-copies do not. No private user fields are read.
func ReadDesignMetadata(ctx context.Context, db sqlx.Session, designs []*DiyDesign) (map[int64]DesignMetadata, error) {
	metadata := make(map[int64]DesignMetadata, len(designs))
	if len(designs) == 0 {
		return metadata, nil
	}
	args := make([]interface{}, 0, len(designs))
	for _, design := range designs {
		args = append(args, design.Id)
	}
	var rows []struct {
		Id         int64  `db:"id"`
		AuthorName string `db:"author_name"`
		UseCount   int64  `db:"use_count"`
		Snapshot   string `db:"snapshot"`
	}
	err := db.QueryRowsCtx(ctx, &rows, `SELECT d.id,
 COALESCE(NULLIF(TRIM(u.nickname),''),'创作者') AS author_name,
 COUNT(c.id) AS use_count,COALESCE(p.design_snapshot,'') AS snapshot
 FROM askxuan_diy.diy_design d
 LEFT JOIN askxuan_user.user u ON u.id=d.user_id
 LEFT JOIN askxuan_diy.diy_design c ON c.source_design_id=d.id AND c.user_id<>d.user_id
 LEFT JOIN askxuan_diy.diy_design_publication p ON p.id=d.publication_id
 WHERE d.id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")+`)
 GROUP BY d.id,u.nickname,p.design_snapshot`, args...)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		tags := DesignTags{FiveElements: []string{}, Meanings: []string{}}
		if row.Snapshot != "" {
			if err := json.Unmarshal([]byte(row.Snapshot), &tags); err != nil {
				return nil, err
			}
		}
		if tags.FiveElements == nil {
			tags.FiveElements = []string{}
		}
		if tags.Meanings == nil {
			tags.Meanings = []string{}
		}
		metadata[row.Id] = DesignMetadata{AuthorName: row.AuthorName, UseCount: row.UseCount, DesignTags: tags}
	}
	return metadata, nil
}
