package logic

import (
	"github.com/askxuan/user-service/internal/model"
	"github.com/askxuan/user-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProfileClearAndOmit(t *testing.T) {
	original := &model.User{Nickname: "原昵称", Avatar: "https://example.test/avatar.jpg", Bio: "简介", Region: "上海", Birthday: "2000-01-02"}
	for _, tt := range []struct {
		name, body  string
		avatar, bio string
		fail        bool
	}{
		{"omit preserves", `{}`, original.Avatar, original.Bio, false},
		{"clear avatar", `{"avatar":""}`, "", original.Bio, false},
		{"clear text", `{"bio":""}`, original.Avatar, "", false},
		{"replace avatar", `{"avatar":"https://example.test/new.jpg"}`, "https://example.test/new.jpg", original.Bio, false},
		{"empty name rejected", `{"nickname":"  "}`, "", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("PUT", "/users/profile", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			var input types.UpdateProfileReq
			if err := httpx.Parse(req, &input); err != nil {
				t.Fatal(err)
			}
			got, err := mergeProfileUpdate(original, &input)
			if tt.fail {
				if err == nil {
					t.Fatal("expected invalid nickname")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Avatar != tt.avatar || got.Bio != tt.bio || got.Region != original.Region || got.Birthday != original.Birthday {
				t.Fatalf("partial update mismatch: %+v", got)
			}
			if original.Avatar != "https://example.test/avatar.jpg" {
				t.Fatal("mutated source record")
			}
		})
	}
}
