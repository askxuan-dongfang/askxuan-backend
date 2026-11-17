package rewards

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

func (p Prize) phase(c Campaign, now int64) string {
	if p.DrawnAt > 0 {
		return "drawn"
	}
	if c.Status != "published" {
		return c.Status
	}
	if now < c.StartsAt {
		return "scheduled"
	}
	if now >= c.EndsAt || p.ParticipantCount >= p.Capacity {
		return "awaiting_draw"
	}
	return "open"
}

// The campaign lock serializes joins and both full-capacity and scheduled draws.
func (s Store) drawPrizes(ctx context.Context, tx *sql.Tx, c Campaign, prizeID int64) error {
	now := time.Now().Unix()
	changed, complete := false, true
	for i := range c.Prizes {
		p := &c.Prizes[i]
		if p.DrawnAt > 0 {
			continue
		}
		if p.ID != prizeID || p.phase(c, now) != "awaiting_draw" {
			complete = false
			continue
		}
		rows, err := tx.QueryContext(ctx, "SELECT "+entryCols+" FROM reward_entry WHERE campaign_id=? AND prize_id=? ORDER BY id", c.ID, p.ID)
		if err != nil {
			return err
		}
		entries := []Entry{}
		h := sha256.New()
		for rows.Next() {
			entry, err := scanEntry(rows)
			if err != nil {
				rows.Close()
				return err
			}
			entries = append(entries, entry)
			fmt.Fprintln(h, entry.Code)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(entries) != p.ParticipantCount {
			return errors.New("reward prize participant count inconsistent")
		}
		winners := min(p.Quantity, len(entries))
		for j := 0; j < winners; j++ {
			offset, err := s.uniform(len(entries) - j)
			if err != nil {
				return err
			}
			entries[j], entries[j+offset] = entries[j+offset], entries[j]
		}
		if _, err = tx.ExecContext(ctx, "UPDATE reward_entry SET outcome='lost' WHERE campaign_id=? AND prize_id=?", c.ID, p.ID); err != nil {
			return err
		}
		for _, entry := range entries[:winners] {
			if _, err = tx.ExecContext(ctx, "UPDATE reward_entry SET outcome='won' WHERE id=?", entry.ID); err != nil {
				return err
			}
			// shortcut: expired inventory stays pending; add operator recovery when needed.
			if err = s.award(ctx, tx, c, entry, p); err != nil {
				return err
			}
		}
		p.DrawnAt, p.AwardedCount = now, winners
		p.PoolDigest = hex.EncodeToString(h.Sum(nil))
		p.Announcement = fmt.Sprintf("%s已开奖。有效参与 %d 人，实际中奖 %d 人。", p.Name, len(entries), winners)
		if _, err = tx.ExecContext(ctx, "UPDATE reward_prize SET drawn_at=?,pool_digest=?,announcement=? WHERE id=?", now, p.PoolDigest, p.Announcement, p.ID); err != nil {
			return err
		}
		if err = audit(ctx, tx, c.ID, 0, "system", "draw_prize", fmt.Sprintf("prize:%d；%s", p.ID, p.Announcement)); err != nil {
			return err
		}
		c.AwardedCount += winners
		changed = true
	}
	if !changed {
		return nil
	}
	if !complete {
		_, err := tx.ExecContext(ctx, "UPDATE reward_campaign SET awarded_count=?,version=version+1 WHERE id=?", c.AwardedCount, c.ID)
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT code FROM reward_entry WHERE campaign_id=? ORDER BY id", c.ID)
	if err != nil {
		return err
	}
	h := sha256.New()
	for rows.Next() {
		var code string
		if err = rows.Scan(&code); err != nil {
			rows.Close()
			return err
		}
		fmt.Fprintln(h, code)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	announcement := fmt.Sprintf("本期全部奖品已开奖。有效参与 %d 人次，实际中奖 %d 人次。", c.ParticipantCount, c.AwardedCount)
	if _, err = tx.ExecContext(ctx, "UPDATE reward_campaign SET status='drawn',awarded_count=?,drawn_at=?,pool_digest=?,announcement=?,version=version+1 WHERE id=?", c.AwardedCount, now, hex.EncodeToString(h.Sum(nil)), announcement, c.ID); err != nil {
		return err
	}
	return audit(ctx, tx, c.ID, 0, "system", "draw", announcement)
}
