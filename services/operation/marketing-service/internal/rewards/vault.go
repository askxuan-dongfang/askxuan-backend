package rewards

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"io"
	"strings"
	"time"
)

var ErrStock = errors.New("兑换库存不足或已失效，请检查奖品库存")
var ErrDuplicateStock = errors.New("库存包含重复的兑换码或链接，请核对后重新导入")
var ErrVault = errors.New("兑换库存暂不可用，请联系管理员")

type Redemption struct {
	Code         string `json:"code"`
	URL          string `json:"url"`
	Platform     string `json:"platform"`
	Instructions string `json:"instructions"`
	ExpiresAt    int64  `json:"expiresAt"`
}

func (s Store) vault() (cipher.AEAD, []byte, error) {
	key, err := base64.StdEncoding.DecodeString(s.RedemptionKey)
	if err != nil || len(key) != 32 {
		return nil, nil, ErrVault
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, ErrVault
	}
	a, err := cipher.NewGCM(block)
	return a, key, err
}
func (s Store) sealCredential(prizeID int64, raw string) ([]byte, string, error) {
	a, key, err := s.vault()
	if err != nil {
		return nil, "", err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, "", err
	}
	ciphertext := a.Seal(nonce, nonce, []byte(raw), []byte(fmt.Sprintf("reward-prize:%d", prizeID)))
	h := hmac.New(sha256.New, key)
	h.Write([]byte("reward-stock:" + raw))
	return ciphertext, hex.EncodeToString(h.Sum(nil)), nil
}
func (s Store) openCredential(prizeID int64, data []byte) (string, error) {
	a, _, err := s.vault()
	if err != nil {
		return "", err
	}
	if len(data) < a.NonceSize() {
		return "", ErrVault
	}
	plain, err := a.Open(nil, data[:a.NonceSize()], data[a.NonceSize():], []byte(fmt.Sprintf("reward-prize:%d", prizeID)))
	if err != nil {
		return "", ErrVault
	}
	return string(plain), nil
}

// ImportStock accepts one credential per item. Imports are append-only and never
// echoed back. Published inventory is immutable; retries cannot duplicate units.
func (s Store) ImportStock(ctx context.Context, cid, pid int64, items []string, actor string) error {
	if len(items) < 1 || len(items) > 1000 {
		return ErrInvalid
	}
	return s.transaction(ctx, func(tx *sql.Tx) error {
		c, err := lockedCampaign(ctx, tx, cid)
		if err != nil {
			return err
		}
		if c.Status != "draft" {
			return ErrConflict
		}
		var p *Prize
		for _, v := range c.Prizes {
			if v.ID == pid {
				v := v
				p = &v
				break
			}
		}
		if p == nil || p.Kind == "physical" {
			return ErrInvalid
		}
		if p.StockCount+len(items) > p.Quantity {
			return ErrStock
		}
		seen := map[string]bool{}
		for _, raw := range items {
			raw = strings.TrimSpace(raw)
			if !validText(raw, 1, 2000) || strings.ContainsAny(raw, "\r\n\x00") || (p.Kind == "link" && !safeHTTPS(raw)) {
				return ErrInvalid
			}
			if seen[raw] {
				return ErrDuplicateStock
			}
			seen[raw] = true
			ciphertext, digest, err := s.sealCredential(pid, raw)
			if err != nil {
				return err
			}
			// Unique index also protects concurrent imports into different campaigns.
			if _, err = tx.ExecContext(ctx, `INSERT INTO reward_stock(prize_id,credential,credential_digest,created_at) VALUES(?,?,?,?)`, pid, ciphertext, digest, time.Now().Unix()); err != nil {
				if isDuplicate(err) {
					return ErrDuplicateStock
				}
				return err
			}
		}
		return audit(ctx, tx, cid, 0, actor, "import_stock", fmt.Sprintf("奖品 %d 导入 %d 份兑换库存", pid, len(items)))
	})
}

// Redemption is an owner-only read. Neither public campaigns nor admin order
// lists return redeemable values. External redemption itself is not simulated.
func (s Store) Redemption(ctx context.Context, id int64, user string) (Redemption, error) {
	var out Redemption
	var pid int64
	var kind string
	var encrypted []byte
	err := s.DB.QueryRowContext(ctx, `SELECT o.prize_id,o.prize_kind,o.platform,o.instructions,o.expires_at,o.redemption_url,s.credential FROM reward_order o JOIN reward_stock s ON s.order_id=o.id WHERE o.id=? AND o.user_id=? AND o.status='delivered'`, id, user).Scan(&pid, &kind, &out.Platform, &out.Instructions, &out.ExpiresAt, &out.URL, &encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	raw, err := s.openCredential(pid, encrypted)
	if err != nil {
		return Redemption{}, err
	}
	if kind == "link" {
		out.URL = raw
	} else if kind == "code" {
		out.Code = raw
	} else {
		return Redemption{}, ErrInvalid
	}
	return out, nil
}

func isDuplicate(err error) bool {
	var e *mysql.MySQLError
	return errors.As(err, &e) && e.Number == 1062
}
