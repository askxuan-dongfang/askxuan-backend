package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	ReportStatusPending  = "pending"
	ReportStatusHandled  = "handled"
	ReportStatusRejected = "rejected"
)

var reportTransitions = map[string]map[string]bool{ReportStatusPending: {ReportStatusHandled: true, ReportStatusRejected: true}}

func CanTransitReport(from, to string) bool { return from != to && reportTransitions[from][to] }

type ReviewReport struct {
	Id           int64  `db:"id" json:"id"`
	ReviewId     int64  `db:"review_id" json:"reviewId"`
	ReporterId   string `db:"reporter_id" json:"reporterId"`
	Reason       string `db:"reason" json:"reason"`
	Status       string `db:"status" json:"status"`
	HandleResult string `db:"handle_result" json:"handleResult"`
	CreateTime   string `db:"create_time" json:"createTime"`
}

func ListReports(ctx context.Context, status string, page, size int) ([]ReviewReport, int64, error) {
	where := ""
	args := []any{}
	if status != "" {
		where = " WHERE status=?"
		args = append(args, status)
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	var total int64
	if err := db.QueryRowCtx(ctx, &total, "SELECT COUNT(*) FROM review_report"+where, args...); err != nil {
		return nil, 0, err
	}
	queryArgs := append(append([]any{}, args...), size, (page-1)*size)
	var list []ReviewReport
	err := db.QueryRowsCtx(ctx, &list, `SELECT id,review_id,reporter_id,reason,status,handle_result,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time FROM review_report`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, queryArgs...)
	return list, total, err
}

func CreateReport(ctx context.Context, report ReviewReport) (ReviewReport, error) {
	if report.Status == "" {
		report.Status = ReportStatusPending
	}
	result, err := db.ExecCtx(ctx, "INSERT INTO review_report(review_id,reporter_id,reason,status,handle_result) VALUES(?,?,?,?,?)", report.ReviewId, report.ReporterId, report.Reason, report.Status, report.HandleResult)
	if err != nil {
		return ReviewReport{}, err
	}
	report.Id, err = result.LastInsertId()
	if err != nil {
		return ReviewReport{}, err
	}
	return FindReportByID(ctx, report.Id)
}

func FindReportByID(ctx context.Context, id int64) (ReviewReport, error) {
	var report ReviewReport
	err := db.QueryRowCtx(ctx, &report, `SELECT id,review_id,reporter_id,reason,status,handle_result,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time FROM review_report WHERE id=?`, id)
	return report, err
}

func UpdateReportStatus(ctx context.Context, id int64, status, handleResult string) (bool, error) {
	result, err := db.ExecCtx(ctx, "UPDATE review_report SET status=?,handle_result=? WHERE id=? AND status=?", status, handleResult, id, ReportStatusPending)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

// HandleReport atomically resolves a pending report and applies its review visibility change.
func HandleReport(ctx context.Context, report ReviewReport, status, handleResult string) (bool, error) {
	updated := false
	err := db.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		result, err := session.ExecCtx(ctx, "UPDATE review_report SET status=?,handle_result=? WHERE id=? AND status=?", status, handleResult, report.Id, ReportStatusPending)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows != 1 {
			return nil
		}
		if status == ReportStatusHandled {
			result, err = session.ExecCtx(ctx, "UPDATE review SET status=? WHERE id=?", ReviewStatusHidden, report.ReviewId)
			if err != nil {
				return err
			}
			rows, err = result.RowsAffected()
			if err != nil {
				return err
			}
			if rows != 1 {
				return fmt.Errorf("review %d not found while handling report %d", report.ReviewId, report.Id)
			}
		}
		updated = true
		return nil
	})
	return updated, err
}
