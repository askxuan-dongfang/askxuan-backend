package model

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func newMockModel(t *testing.T) (*communityModel, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &communityModel{conn: sqlx.NewSqlConnFromDB(db)}, mock
}

func postRowsResult(status string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"post_no", "master_id", "owner_id", "type", "title", "content", "cover_media_id", "belief_code", "status", "audit_id", "audit_remark", "like_count", "comment_count", "create_time", "update_time",
	}).AddRow("P1", "M1", "owner", "video", "标题", "正文", 2, "han_buddhism", status, 5, "", 0, 0, "2026-07-13", "2026-07-13")
}

func TestReviewPostCommitsBusinessAndAuditStateTogether(t *testing.T) {
	m, mock := newMockModel(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status,audit_id FROM post").WithArgs("P1").WillReturnRows(
		sqlmock.NewRows([]string{"status", "audit_id"}).AddRow("pending", 5),
	)
	mock.ExpectExec("UPDATE post SET status=").WithArgs("approved", "", "P1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE askxuan_audit.audit_queue SET status=").WithArgs("approved", "auditor", "", int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO askxuan_audit.audit_log").WithArgs(int64(5), "approved", "auditor", "").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT " + postRows + " FROM post").WithArgs("P1").WillReturnRows(postRowsResult("approved"))
	mock.ExpectQuery("SELECT id,media_id,asset_type,sort FROM post_asset").WithArgs("P1").WillReturnRows(sqlmock.NewRows([]string{"id", "media_id", "asset_type", "sort"}))

	post, err := m.ReviewPost(context.Background(), "P1", "auditor", "approved", "")
	if err != nil {
		t.Fatal(err)
	}
	if post.Status != "approved" {
		t.Fatalf("unexpected post status: %s", post.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReviewPostRollsBackWhenAuditQueueAlreadyChanged(t *testing.T) {
	m, mock := newMockModel(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status,audit_id FROM post").WillReturnRows(sqlmock.NewRows([]string{"status", "audit_id"}).AddRow("pending", 5))
	mock.ExpectExec("UPDATE post SET status=").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE askxuan_audit.audit_queue SET status=").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	if _, err := m.ReviewPost(context.Background(), "P1", "auditor", "approved", ""); err == nil {
		t.Fatal("expected review rollback")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSetLikeIsIdempotent(t *testing.T) {
	m, mock := newMockModel(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE INTO post_like").WithArgs("user", "P1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE post SET like_count").WithArgs(1, "P1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT like_count FROM post").WithArgs("P1").WillReturnRows(sqlmock.NewRows([]string{"like_count"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE INTO post_like").WithArgs("user", "P1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT like_count FROM post").WithArgs("P1").WillReturnRows(sqlmock.NewRows([]string{"like_count"}).AddRow(1))

	for i := 0; i < 2; i++ {
		result, err := m.SetLike(context.Background(), "P1", "user", true)
		if err != nil {
			t.Fatal(err)
		}
		if result.LikeCount != 1 {
			t.Fatalf("duplicate like changed count: %d", result.LikeCount)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReviewCommentIncrementsCountOnlyOnce(t *testing.T) {
	m, mock := newMockModel(t)
	commentRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"comment_no", "post_no", "user_id", "content", "status", "audit_id", "audit_remark", "create_time"}).
			AddRow("C1", "P1", "user", "评论", "approved", 7, "", "2026-07-13")
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status,audit_id,post_no FROM post_comment").WithArgs("C1").WillReturnRows(
		sqlmock.NewRows([]string{"status", "audit_id", "post_no"}).AddRow("pending", 7, "P1"),
	)
	mock.ExpectExec("UPDATE post_comment SET status=").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE askxuan_audit.audit_queue SET status=").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO askxuan_audit.audit_log").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE post SET comment_count=comment_count\\+1").WithArgs("P1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT comment_no,post_no,user_id,content,status,audit_id,audit_remark,create_time FROM post_comment").WithArgs("C1").WillReturnRows(commentRows())

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status,audit_id,post_no FROM post_comment").WithArgs("C1").WillReturnRows(
		sqlmock.NewRows([]string{"status", "audit_id", "post_no"}).AddRow("approved", 7, "P1"),
	)
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT comment_no,post_no,user_id,content,status,audit_id,audit_remark,create_time FROM post_comment").WithArgs("C1").WillReturnRows(commentRows())

	for i := 0; i < 2; i++ {
		comment, err := m.ReviewComment(context.Background(), "C1", "auditor", "approved", "")
		if err != nil {
			t.Fatal(err)
		}
		if comment.Status != "approved" {
			t.Fatalf("unexpected comment status: %s", comment.Status)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
