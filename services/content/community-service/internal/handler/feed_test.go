package handler

import (
	"context"
	"encoding/json"
	"github.com/askxuan/community-service/internal/model"
	"github.com/askxuan/community-service/internal/svc"
	"github.com/askxuan/community-service/internal/types"
	"net/http/httptest"
	"testing"
)

type feedModel struct {
	model.CommunityModel
	viewer string
}

func (m *feedModel) ListFeed(_ context.Context, r *types.FeedReq, p, s int) (int64, []types.Post, error) {
	m.viewer = r.Viewer
	return 1, []types.Post{{Id: "P1", OwnerId: "private-owner", AuditRemark: "private-review"}}, nil
}
func (m *feedModel) EnrichMedia(_ context.Context, _ []types.Post, _ string, _ bool) error {
	return nil
}
func TestFeedUsesVerifiedHeaderAndHidesInternalFields(t *testing.T) {
	m := &feedModel{}
	req := httptest.NewRequest("GET", "/api/v1/community/feed?viewer=attacker&following=true", nil)
	req.Header.Set("X-User-Id", "real-viewer")
	w := httptest.NewRecorder()
	feed(&svc.ServiceContext{Model: m})(w, req)
	if m.viewer != "real-viewer" {
		t.Fatalf("viewer=%q body=%s", m.viewer, w.Body.String())
	}
	var res struct {
		Code int
		Data types.PostListResp
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Code != 0 || len(res.Data.List) != 1 || res.Data.List[0].OwnerId != "" || res.Data.List[0].AuditRemark != "" {
		t.Fatal(w.Body.String())
	}
}
