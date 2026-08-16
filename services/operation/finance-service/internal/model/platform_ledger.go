package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	LedgerEventPaymentReceipt         = "payment_receipt"
	LedgerEventBookingSettlement      = "booking_settlement"
	LedgerEventConsultationSettlement = "consultation_settlement"
	LedgerAccountPlatformCash         = "platform_cash"
	LedgerAccountCustomerFunds        = "customer_funds_held"
	LedgerAccountCommission           = "platform_commission"
	LedgerAccountMasterPayable        = "master_payable"
	LedgerAccountTemplePayable        = "temple_payable"
)

type PaymentReceipt struct {
	PaymentNo  string
	SourceType string
	SourceNo   string
	Amount     float64
}

type BookingSettlement struct {
	BookingID   string
	UserID      string
	TempleID    string
	TempleName  string
	MasterID    string
	MasterName  string
	ServiceName string
	BookingDate string
	ServiceFee  float64
	MeritMoney  float64
	TotalFee    float64
}

type BookingSplit struct {
	Rate               float64
	Total              float64
	Commission         float64
	MasterGross        float64
	MasterCommission   float64
	MasterNet          float64
	TempleGross        float64
	TempleCommission   float64
	TempleNet          float64
	MasterSettlementID int64
	TempleSettlementID int64
	Created            bool
}

type ConsultationSettlement struct {
	ConsultationID string
	UserID         string
	MasterID       string
	MasterName     string
	PaidAt         string
	ConsultFee     float64
}

type ConsultationSplit struct {
	Rate               float64
	Total              float64
	Commission         float64
	MasterNet          float64
	MasterSettlementID int64
	Created            bool
}

func CalculateConsultationSplit(total, rate float64) (ConsultationSplit, error) {
	total, rate = money(total), round4(rate)
	if total <= 0 || rate < 0 || rate > 1 {
		return ConsultationSplit{}, fmt.Errorf("咨询金额或平台抽成比例无效")
	}
	commission := money(total * rate)
	masterNet := money(total - commission)
	if !sameMoney(commission+masterNet, total) {
		return ConsultationSplit{}, fmt.Errorf("咨询分账结果不平衡")
	}
	return ConsultationSplit{Rate: rate, Total: total, Commission: commission, MasterNet: masterNet}, nil
}

type ledgerTransaction struct {
	ID          int64   `db:"id"`
	PaymentNo   string  `db:"payment_no"`
	TotalAmount float64 `db:"total_amount"`
}

func CalculateBookingSplit(serviceFee, meritMoney, totalFee, rate float64) (BookingSplit, error) {
	serviceFee, meritMoney, totalFee, rate = money(serviceFee), money(meritMoney), money(totalFee), round4(rate)
	if serviceFee < 0 || meritMoney < 0 || totalFee <= 0 {
		return BookingSplit{}, fmt.Errorf("预约计价必须为非负数且总额大于零")
	}
	if rate < 0 || rate > 1 {
		return BookingSplit{}, fmt.Errorf("平台抽成比例超出范围")
	}
	if !sameMoney(serviceFee+meritMoney, totalFee) {
		return BookingSplit{}, fmt.Errorf("预约价格快照不平衡: service=%.2f merit=%.2f total=%.2f", serviceFee, meritMoney, totalFee)
	}
	totalCommission := money(totalFee * rate)
	masterCommission := money(serviceFee * rate)
	templeCommission := money(totalCommission - masterCommission)
	split := BookingSplit{
		Rate: rate, Total: totalFee, Commission: totalCommission,
		MasterGross: serviceFee, MasterCommission: masterCommission, MasterNet: money(serviceFee - masterCommission),
		TempleGross: meritMoney, TempleCommission: templeCommission, TempleNet: money(meritMoney - templeCommission),
	}
	if split.MasterNet < 0 || split.TempleNet < 0 || !sameMoney(split.Commission+split.MasterNet+split.TempleNet, totalFee) {
		return BookingSplit{}, fmt.Errorf("预约分账结果不平衡")
	}
	return split, nil
}

func RecordPlatformReceipt(ctx context.Context, receipt PaymentReceipt) error {
	if receipt.PaymentNo == "" || receipt.SourceType == "" || receipt.SourceNo == "" || receipt.Amount <= 0 {
		return fmt.Errorf("平台收款事件字段不完整")
	}
	receipt.Amount = money(receipt.Amount)
	return db.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		transactionNo := deterministicNo("PAY", receipt.PaymentNo, 64)
		res, err := session.ExecCtx(ctx, `INSERT IGNORE INTO finance_transaction
			(transaction_no,source_type,source_no,payment_no,event_type,total_amount,status)
			VALUES(?,?,?,?,?,?,?)`, transactionNo, receipt.SourceType, receipt.SourceNo,
			receipt.PaymentNo, LedgerEventPaymentReceipt, receipt.Amount, "posted")
		if err != nil {
			return err
		}
		created, _ := res.RowsAffected()
		var tx ledgerTransaction
		if err := session.QueryRowCtx(ctx, &tx, `SELECT id,payment_no,total_amount FROM finance_transaction
			WHERE source_type=? AND source_no=? AND event_type=? FOR UPDATE`,
			receipt.SourceType, receipt.SourceNo, LedgerEventPaymentReceipt); err != nil {
			return err
		}
		if tx.PaymentNo != receipt.PaymentNo || !sameMoney(tx.TotalAmount, receipt.Amount) {
			return fmt.Errorf("重复收款事件与原总账记录不一致")
		}
		if created == 0 {
			return nil
		}
		if err := insertLedgerEntry(ctx, session, tx.ID, LedgerAccountPlatformCash, "", "debit", receipt.Amount); err != nil {
			return err
		}
		if err := insertLedgerEntry(ctx, session, tx.ID, LedgerAccountCustomerFunds, "", "credit", receipt.Amount); err != nil {
			return err
		}
		_, err = session.ExecCtx(ctx, `INSERT INTO finance_log(settlement_id,amount,type,description)
			VALUES(0,?,'income',?)`, receipt.Amount,
			fmt.Sprintf("平台总账收款:%s:%s:%s", receipt.SourceType, receipt.SourceNo, receipt.PaymentNo))
		return err
	})
}

func AccrueBookingSettlement(ctx context.Context, booking BookingSettlement) (BookingSplit, error) {
	// MasterID 允许为空（全寺执行预约：服务费与功德全归寺院，大师分成为 0）
	if booking.BookingID == "" || booking.BookingDate == "" {
		return BookingSplit{}, fmt.Errorf("预约分账字段不完整")
	}
	// 野生大师（大师直约，无寺庙）使用独立费率；寺庙为空时 TempleNet 恒为 0
	bizType := BizTypeBooking
	if booking.TempleID == "" {
		bizType = BizTypeWildMaster
	}
	var rate float64
	if err := db.QueryRowCtx(ctx, &rate, `SELECT rate FROM commission_config WHERE biz_type=?`, bizType); err != nil {
		return BookingSplit{}, fmt.Errorf("读取预约抽成配置失败: %w", err)
	}
	split, err := CalculateBookingSplit(booking.ServiceFee, booking.MeritMoney, booking.TotalFee, rate)
	if err != nil {
		return BookingSplit{}, err
	}
	err = db.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var receipt ledgerTransaction
		if err := session.QueryRowCtx(ctx, &receipt, `SELECT id,payment_no,total_amount FROM finance_transaction
			WHERE source_type=? AND source_no=? AND event_type=? FOR UPDATE`,
			BizTypeBooking, booking.BookingID, LedgerEventPaymentReceipt); err != nil {
			return fmt.Errorf("预约尚未进入平台总账: %w", err)
		}
		if !sameMoney(receipt.TotalAmount, split.Total) {
			return fmt.Errorf("平台收款与预约快照金额不一致")
		}

		res, err := session.ExecCtx(ctx, `INSERT IGNORE INTO finance_transaction
			(transaction_no,source_type,source_no,payment_no,event_type,total_amount,status)
			VALUES(?,?,?,?,?,?,?)`, deterministicNo("BKS", booking.BookingID, 64), BizTypeBooking,
			booking.BookingID, receipt.PaymentNo, LedgerEventBookingSettlement, split.Total, "posted")
		if err != nil {
			return err
		}
		created, _ := res.RowsAffected()
		var settlementTx ledgerTransaction
		if err := session.QueryRowCtx(ctx, &settlementTx, `SELECT id,payment_no,total_amount FROM finance_transaction
			WHERE source_type=? AND source_no=? AND event_type=? FOR UPDATE`,
			BizTypeBooking, booking.BookingID, LedgerEventBookingSettlement); err != nil {
			return err
		}
		if !sameMoney(settlementTx.TotalAmount, split.Total) {
			return fmt.Errorf("重复预约分账与原总账记录不一致")
		}
		if created == 0 {
			split.MasterSettlementID = findSettlementID(ctx, session, BizTypeBooking, booking.BookingID, SettleTypeMaster, booking.MasterID)
			split.TempleSettlementID = findSettlementID(ctx, session, BizTypeBooking, booking.BookingID, SettleTypeTemple, booking.TempleID)
			return nil
		}

		if err := insertLedgerEntry(ctx, session, settlementTx.ID, LedgerAccountCustomerFunds, "", "debit", split.Total); err != nil {
			return err
		}
		for _, entry := range []struct {
			account, target string
			amount          float64
		}{
			{LedgerAccountCommission, "", split.Commission},
			{LedgerAccountMasterPayable, booking.MasterID, split.MasterNet},
			{LedgerAccountTemplePayable, booking.TempleID, split.TempleNet},
		} {
			if entry.amount > 0 {
				if err := insertLedgerEntry(ctx, session, settlementTx.ID, entry.account, entry.target, "credit", entry.amount); err != nil {
					return err
				}
			}
		}

		periodStart, periodEnd, err := settlementPeriod(booking.BookingDate)
		if err != nil {
			return err
		}
		if split.MasterGross > 0 {
			split.MasterSettlementID, err = insertSettlement(ctx, session, deterministicNo("SETB-M", booking.BookingID, 32), BizTypeBooking, SettleTypeMaster,
				booking.MasterID, booking.MasterName, booking.BookingID, periodStart, periodEnd,
				split.MasterGross, split.Rate, split.MasterCommission, split.MasterNet)
			if err != nil {
				return err
			}
		}
		if split.TempleGross > 0 {
			split.TempleSettlementID, err = insertSettlement(ctx, session, deterministicNo("SETB-T", booking.BookingID, 32), BizTypeBooking, SettleTypeTemple,
				booking.TempleID, booking.TempleName, booking.BookingID, periodStart, periodEnd,
				split.TempleGross, split.Rate, split.TempleCommission, split.TempleNet)
			if err != nil {
				return err
			}
		}
		_, err = session.ExecCtx(ctx, `INSERT INTO finance_log(settlement_id,amount,type,description)
			VALUES(0,?,'allocation',?)`, split.Commission, fmt.Sprintf("预约平台分账:%s", booking.BookingID))
		if err == nil {
			split.Created = true
		}
		return err
	})
	return split, err
}

func AccrueConsultationSettlement(ctx context.Context, consultation ConsultationSettlement) (ConsultationSplit, error) {
	if consultation.ConsultationID == "" || consultation.MasterID == "" || consultation.PaidAt == "" {
		return ConsultationSplit{}, fmt.Errorf("咨询分账字段不完整")
	}
	var rate float64
	if err := db.QueryRowCtx(ctx, &rate, `SELECT rate FROM commission_config WHERE biz_type=?`, BizTypeConsultation); err != nil {
		return ConsultationSplit{}, fmt.Errorf("读取咨询抽成配置失败: %w", err)
	}
	split, err := CalculateConsultationSplit(consultation.ConsultFee, rate)
	if err != nil {
		return ConsultationSplit{}, err
	}
	err = db.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var receipt ledgerTransaction
		if err := session.QueryRowCtx(ctx, &receipt, `SELECT id,payment_no,total_amount FROM finance_transaction WHERE source_type=? AND source_no=? AND event_type=? FOR UPDATE`, BizTypeConsultation, consultation.ConsultationID, LedgerEventPaymentReceipt); err != nil {
			return fmt.Errorf("咨询款尚未进入平台总账: %w", err)
		}
		if !sameMoney(receipt.TotalAmount, split.Total) {
			return fmt.Errorf("平台收款与咨询价格快照不一致")
		}
		res, err := session.ExecCtx(ctx, `INSERT IGNORE INTO finance_transaction (transaction_no,source_type,source_no,payment_no,event_type,total_amount,status) VALUES(?,?,?,?,?,?,?)`,
			deterministicNo("CST", consultation.ConsultationID, 64), BizTypeConsultation, consultation.ConsultationID, receipt.PaymentNo, LedgerEventConsultationSettlement, split.Total, "posted")
		if err != nil {
			return err
		}
		created, _ := res.RowsAffected()
		var settlementTx ledgerTransaction
		if err := session.QueryRowCtx(ctx, &settlementTx, `SELECT id,payment_no,total_amount FROM finance_transaction WHERE source_type=? AND source_no=? AND event_type=? FOR UPDATE`, BizTypeConsultation, consultation.ConsultationID, LedgerEventConsultationSettlement); err != nil {
			return err
		}
		if created == 0 {
			split.MasterSettlementID = findSettlementID(ctx, session, BizTypeConsultation, consultation.ConsultationID, SettleTypeMaster, consultation.MasterID)
			return nil
		}
		if err := insertLedgerEntry(ctx, session, settlementTx.ID, LedgerAccountCustomerFunds, "", "debit", split.Total); err != nil {
			return err
		}
		if split.Commission > 0 {
			if err := insertLedgerEntry(ctx, session, settlementTx.ID, LedgerAccountCommission, "", "credit", split.Commission); err != nil {
				return err
			}
		}
		if split.MasterNet > 0 {
			if err := insertLedgerEntry(ctx, session, settlementTx.ID, LedgerAccountMasterPayable, consultation.MasterID, "credit", split.MasterNet); err != nil {
				return err
			}
		}
		paidDate := consultation.PaidAt
		if len(paidDate) >= 10 {
			paidDate = paidDate[:10]
		}
		periodStart, periodEnd, err := settlementPeriod(paidDate)
		if err != nil {
			return err
		}
		split.MasterSettlementID, err = insertSettlement(ctx, session, deterministicNo("SETC-M", consultation.ConsultationID, 32), BizTypeConsultation, SettleTypeMaster,
			consultation.MasterID, consultation.MasterName, consultation.ConsultationID, periodStart, periodEnd,
			split.Total, split.Rate, split.Commission, split.MasterNet)
		if err != nil {
			return err
		}
		_, err = session.ExecCtx(ctx, `INSERT INTO finance_log(settlement_id,amount,type,description) VALUES(0,?,'allocation',?)`, split.Commission, fmt.Sprintf("即时咨询平台分账:%s", consultation.ConsultationID))
		if err == nil {
			split.Created = true
		}
		return err
	})
	return split, err
}

func insertLedgerEntry(ctx context.Context, session sqlx.Session, transactionID int64, account, target, direction string, amount float64) error {
	_, err := session.ExecCtx(ctx, `INSERT INTO finance_ledger_entry(transaction_id,account_code,target_id,direction,amount)
		VALUES(?,?,?,?,?)`, transactionID, account, target, direction, money(amount))
	return err
}

func insertSettlement(ctx context.Context, session sqlx.Session, settlementNo, sourceType, settleType, targetID, targetName, sourceNo, periodStart, periodEnd string, gross, rate, commission, net float64) (int64, error) {
	res, err := session.ExecCtx(ctx, `INSERT INTO settlement
		(settlement_no,settle_type,target_id,target_name,period_start,period_end,order_count,total_amount,commission_rate,commission_amount,settle_amount,status,source_type,source_no)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, settlementNo, settleType, targetID, targetName,
		periodStart, periodEnd, 1, gross, rate, commission, net, SettlementPending, sourceType, sourceNo)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func findSettlementID(ctx context.Context, session sqlx.Session, sourceType, sourceNo, settleType, targetID string) int64 {
	var id int64
	_ = session.QueryRowCtx(ctx, &id, `SELECT id FROM settlement WHERE source_type=? AND source_no=? AND settle_type=? AND target_id=?`, sourceType, sourceNo, settleType, targetID)
	return id
}

func settlementPeriod(bookingDate string) (string, string, error) {
	date, err := time.ParseInLocation("2006-01-02", bookingDate, time.Local)
	if err != nil {
		return "", "", fmt.Errorf("预约日期无效: %w", err)
	}
	start := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	end := start.AddDate(0, 1, 0).Add(-time.Second)
	return start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), nil
}

func deterministicNo(prefix, value string, maxLength int) string {
	sum := sha256.Sum256([]byte(value))
	no := prefix + "-" + hex.EncodeToString(sum[:12])
	if len(no) > maxLength {
		return no[:maxLength]
	}
	return no
}

func money(value float64) float64        { return math.Round(value*100) / 100 }
func round4(value float64) float64       { return math.Round(value*10000) / 10000 }
func sameMoney(left, right float64) bool { return math.Abs(money(left)-money(right)) < 0.001 }
