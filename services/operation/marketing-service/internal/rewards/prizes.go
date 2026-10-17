package rewards

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Prize contains public configuration only. Redemption credentials live in a separate vault.
type Prize struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Image         string `json:"image"`
	Value         int64  `json:"value"`
	Quantity      int    `json:"quantity"`
	AwardedCount  int    `json:"awardedCount"`
	Platform      string `json:"platform"`
	Instructions  string `json:"instructions"`
	ExpiresAt     int64  `json:"expiresAt"`
	RedemptionURL string `json:"redemptionUrl"`
	StockCount    int    `json:"stockCount,omitempty"`
	// Nil preserves the published rules of campaigns created before fixed odds.
	ProbabilityBps *int `json:"probabilityBps,omitempty"`
}

func safeHTTPS(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && len(raw) <= 2000 && !strings.ContainsAny(raw, "\r\n\t")
}
func validatePrizes(c Campaign) error {
	if len(c.Prizes) > 10 {
		return ErrInvalid
	}
	if len(c.Prizes) == 0 {
		if len(c.WheelSlots) > 0 {
			return ErrInvalid
		}
		return nil
	}
	if c.Kind != "wheel" {
		return ErrInvalid
	}
	total, quantity, odds, configured := int64(0), 0, 0, 0
	ids := map[int64]bool{}
	for _, p := range c.Prizes {
		if !validText(p.Name, 1, 120) || p.Value < 1 || p.Value > 100000000 || p.Quantity < 1 || p.Quantity > 100000 || p.ID < 0 || (p.ID > 0 && ids[p.ID]) {
			return ErrInvalid
		}
		if p.ProbabilityBps != nil {
			configured++
			if *p.ProbabilityBps < 1 || *p.ProbabilityBps > 10000 {
				return ErrInvalid
			}
			odds += *p.ProbabilityBps
		}
		ids[p.ID] = true
		if p.Image != "" && !safeHTTPS(p.Image) {
			return ErrInvalid
		}
		if p.Kind != "physical" && p.Kind != "code" && p.Kind != "link" {
			return ErrInvalid
		}
		if p.Kind != "physical" {
			if !validText(p.Platform, 1, 80) || !validText(p.Instructions, 1, 2000) || p.ExpiresAt <= c.EndsAt {
				return ErrInvalid
			}
			if p.RedemptionURL != "" && (p.Kind != "code" || !safeHTTPS(p.RedemptionURL)) {
				return ErrInvalid
			}
		} else if p.RedemptionURL != "" || p.ExpiresAt != 0 || p.Platform != "" || p.Instructions != "" {
			return ErrInvalid
		}
		total += p.Value * int64(p.Quantity)
		quantity += p.Quantity
	}
	if (configured != 0 && configured != len(c.Prizes)) || odds > 10000 || c.Budget < total || (configured == 0 && (quantity > 1000 || quantity > c.Capacity)) {
		return ErrInvalid
	}
	return validateWheelSlots(c, odds)
}

// Slots refer to frozen prize positions, independent of their actual odds.
func validateWheelSlots(c Campaign, odds int) error {
	if len(c.WheelSlots) == 0 {
		if len(c.Prizes) == 10 && odds < 10000 {
			return ErrInvalid
		}
		return nil
	}
	if c.Kind != "wheel" || len(c.WheelSlots) != 10 {
		return ErrInvalid
	}
	seen := map[int]bool{}
	for _, slot := range c.WheelSlots {
		if slot < -1 || slot >= len(c.Prizes) {
			return ErrInvalid
		}
		seen[slot] = true
	}
	for i := range c.Prizes {
		if !seen[i] {
			return ErrInvalid
		}
	}
	if (odds < 10000) != seen[-1] {
		return ErrInvalid
	}
	return nil
}

// Top-level fields remain as a legacy summary, never as the selected award.
func normalizeCampaign(c *Campaign) {
	if len(c.Prizes) == 0 {
		return
	}
	c.PrizeQuantity = 0
	for _, p := range c.Prizes {
		c.PrizeQuantity += p.Quantity
	}
	c.PrizeName = c.Prizes[0].Name
	if len(c.Prizes) > 1 {
		c.PrizeName = fmt.Sprintf("%d 种好礼", len(c.Prizes))
	}
	c.Image = c.Prizes[0].Image
	c.PrizeValue = c.Prizes[0].Value
}

type prizeQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadPrizes(ctx context.Context, q prizeQuerier, c *Campaign, admin bool) error {
	rows, err := q.QueryContext(ctx, `SELECT p.id,p.name,p.kind,p.image,p.value,p.quantity,p.awarded_count,p.platform,p.instructions,p.expires_at,p.redemption_url,p.probability_bps,(SELECT COUNT(*) FROM reward_stock s WHERE s.prize_id=p.id AND s.order_id IS NULL) FROM reward_prize p WHERE p.campaign_id=? ORDER BY p.position,p.id`, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	c.Prizes = []Prize{}
	for rows.Next() {
		var p Prize
		if err = rows.Scan(&p.ID, &p.Name, &p.Kind, &p.Image, &p.Value, &p.Quantity, &p.AwardedCount, &p.Platform, &p.Instructions, &p.ExpiresAt, &p.RedemptionURL, &p.ProbabilityBps, &p.StockCount); err != nil {
			return err
		}
		if !admin {
			p.StockCount = 0
		}
		c.Prizes = append(c.Prizes, p)
	}
	c.Phase = c.phase(time.Now().Unix())
	if c.fixedOdds() {
		c.Algorithm = "crypto-rand-fixed-bps-v1"
	}
	return rows.Err()
}
func savePrizes(ctx context.Context, tx *sql.Tx, c Campaign, old []Prize) error {
	known := map[int64]Prize{}
	for _, p := range old {
		known[p.ID] = p
	}
	for i, p := range c.Prizes {
		if p.ID == 0 {
			_, err := tx.ExecContext(ctx, `INSERT INTO reward_prize(campaign_id,position,name,kind,image,value,quantity,platform,instructions,expires_at,redemption_url,probability_bps) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, c.ID, i, p.Name, p.Kind, p.Image, p.Value, p.Quantity, p.Platform, p.Instructions, p.ExpiresAt, p.RedemptionURL, p.ProbabilityBps)
			if err != nil {
				return err
			}
		} else {
			before, ok := known[p.ID]
			if !ok {
				return ErrInvalid
			}
			// Imported stock cannot silently change its meaning or expiry.
			if before.StockCount > 0 && (before.Kind != p.Kind || before.Platform != p.Platform || before.ExpiresAt != p.ExpiresAt) {
				return ErrConflict
			}
			_, err := tx.ExecContext(ctx, `UPDATE reward_prize SET position=?,name=?,kind=?,image=?,value=?,quantity=?,platform=?,instructions=?,expires_at=?,redemption_url=?,probability_bps=? WHERE id=? AND campaign_id=?`, i, p.Name, p.Kind, p.Image, p.Value, p.Quantity, p.Platform, p.Instructions, p.ExpiresAt, p.RedemptionURL, p.ProbabilityBps, p.ID, c.ID)
			if err != nil {
				return err
			}
			delete(known, p.ID)
		}
	}
	for id, p := range known {
		if p.StockCount > 0 {
			return ErrConflict
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM reward_prize WHERE id=? AND campaign_id=?`, id, c.ID); err != nil {
			return err
		}
	}
	return nil
}
func prizeForTicket(c Campaign, n int) *Prize {
	if n < 0 {
		return nil
	}
	for _, p := range c.Prizes {
		if c.fixedOdds() {
			n -= *p.ProbabilityBps
		} else {
			n -= p.Quantity - p.AwardedCount
		}
		if n < 0 {
			return &p
		}
	}
	return nil
}

func (c Campaign) fixedOdds() bool {
	if c.Kind != "wheel" || len(c.Prizes) == 0 {
		return false
	}
	for _, p := range c.Prizes {
		if p.ProbabilityBps == nil {
			return false
		}
	}
	return true
}
func (s Store) award(ctx context.Context, tx *sql.Tx, c Campaign, e Entry, p *Prize) error {
	if p == nil {
		return newOrder(ctx, tx, c, e)
	}
	if p.Kind != "physical" && p.ExpiresAt <= time.Now().Unix() {
		return ErrStock
	}
	status := "awaiting_address"
	if p.Kind != "physical" {
		status = "delivered"
	}
	r, err := tx.ExecContext(ctx, `INSERT INTO reward_order(campaign_id,entry_id,user_id,prize_name,code,created_at,prize_id,prize_kind,platform,instructions,expires_at,redemption_url,status) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.ID, e.ID, e.UserID, p.Name, e.Code, time.Now().Unix(), p.ID, p.Kind, p.Platform, p.Instructions, p.ExpiresAt, p.RedemptionURL, status)
	if err != nil {
		return err
	}
	oid, err := r.LastInsertId()
	if err != nil {
		return err
	}
	if p.Kind != "physical" {
		var stockID int64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM reward_stock WHERE prize_id=? AND order_id IS NULL ORDER BY id LIMIT 1 FOR UPDATE`, p.ID).Scan(&stockID); err != nil {
			if err == sql.ErrNoRows {
				return ErrStock
			}
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE reward_stock SET order_id=? WHERE id=? AND order_id IS NULL`, oid, stockID); err != nil {
			return err
		}
	}
	r, err = tx.ExecContext(ctx, `UPDATE reward_prize SET awarded_count=awarded_count+1 WHERE id=? AND awarded_count<quantity`, p.ID)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrStock
	}
	return audit(ctx, tx, c.ID, oid, "system", "award", fmt.Sprintf("%s:prize:%d", e.Code, p.ID))
}
