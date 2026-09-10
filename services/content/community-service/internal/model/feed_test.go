package model

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/community-service/internal/types"
	"reflect"
	"strings"
	"testing"
)

func TestFeedFiltersAreAppliedBeforePagination(t *testing.T) {
	req := &types.FeedReq{Type: "article", BeliefCode: "daoism", Following: true, Viewer: "viewer", Keyword: "100%_!"}
	where, args := feedWhere(req)
	if !strings.Contains(where, "p.status='approved'") || !strings.Contains(where, "EXISTS (SELECT 1 FROM master_follow") {
		t.Fatal(where)
	}
	if !reflect.DeepEqual(args, []interface{}{"article", "daoism", "viewer", "%100!%!_!!%", "%100!%!_!!%"}) {
		t.Fatalf("query arguments: %#v", args)
	}
}
func TestFeedHydratesViewerStateAndAssetsInBatches(t *testing.T) {
	m, mock := newMockModel(t)
	req := &types.FeedReq{Viewer: "u1", Following: true, Sort: "popular"}
	mock.ExpectQuery("SELECT COUNT.*EXISTS").WithArgs("u1").WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(13))
	mock.ExpectQuery("SELECT p.post_no.*ORDER BY p.like_count DESC,p.comment_count DESC,p.create_time DESC,p.post_no DESC LIMIT").WithArgs("u1", 12, 12).WillReturnRows(postRowsResult("approved"))
	mock.ExpectQuery("SELECT post_no,id,media_id,asset_type,sort FROM post_asset WHERE post_no IN").WithArgs("P1").WillReturnRows(sqlmock.NewRows([]string{"post_no", "id", "media_id", "asset_type", "sort"}).AddRow("P1", 1, 5, "video", 0))
	mock.ExpectQuery("SELECT post_no FROM post_like").WithArgs("u1", "P1").WillReturnRows(sqlmock.NewRows([]string{"post_no"}).AddRow("P1"))
	mock.ExpectQuery("SELECT DISTINCT f.master_id").WithArgs("u1", "P1").WillReturnRows(sqlmock.NewRows([]string{"master_id"}).AddRow("M1"))
	total, posts, err := m.ListFeed(context.Background(), req, 2, 12)
	if err != nil {
		t.Fatal(err)
	}
	if total != 13 || len(posts) != 1 || !posts[0].Liked || !posts[0].Following || len(posts[0].Assets) != 1 {
		t.Fatalf("bad feed: %+v", posts)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestMediaVisibilityAnonymousAndLoggedIn(t *testing.T) {
	m, mock := newMockModel(t)
	posts := []types.Post{{Id: "P1", CoverMediaId: 2, Assets: []types.Asset{{MediaId: 5}}}}
	if err := m.EnrichMedia(context.Background(), posts, "", false); err != nil {
		t.Fatal(err)
	}
	if posts[0].CoverUrl != "" {
		t.Fatal("anonymous media leaked")
	}
	mock.ExpectQuery("SELECT id,playback_url,cover_url,duration.*status='ready'.*audit_status='approved' OR owner_id").WithArgs(int64(2), int64(5), "u1").WillReturnRows(sqlmock.NewRows([]string{"id", "playback_url", "cover_url", "duration"}).AddRow(2, "/objects/cover.jpg", "", 0))
	if err := m.EnrichMedia(context.Background(), posts, "u1", false); err != nil {
		t.Fatal(err)
	}
	if posts[0].CoverUrl != "/objects/cover.jpg" || posts[0].Assets[0].Url != "" {
		t.Fatal("unapproved asset was exposed")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestPostApprovalRollsBackIfMediaPublicationFails(t *testing.T) {
	m, mock := newMockModel(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status,audit_id FROM post").WithArgs("P1").WillReturnRows(sqlmock.NewRows([]string{"status", "audit_id"}).AddRow("pending", 5))
	mock.ExpectExec("UPDATE post SET status=").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE askxuan_audit.audit_queue").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO askxuan_audit.audit_log").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE askxuan_media.media_asset").WillReturnError(context.DeadlineExceeded)
	mock.ExpectRollback()
	if _, err := m.ReviewPost(context.Background(), "P1", "admin", "approved", ""); err == nil {
		t.Fatal("partial publication accepted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
