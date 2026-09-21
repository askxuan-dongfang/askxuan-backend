package logic

import (
	"context"
	"fmt"
	"github.com/askxuan/ai-service/internal/experience"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"sync"
	"testing"
	"time"
)

func TestExperienceMySQLPrivateHistory(t *testing.T) {
	dsn := os.Getenv("AI_EXPERIENCE_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated AI_EXPERIENCE_TEST_DSN not set")
	}
	s := &svc.ServiceContext{DB: sqlx.NewMysql(dsn)}
	ctx := context.Background()
	user := fmt.Sprint("test-", time.Now().UnixNano())
	defer s.DB.ExecCtx(ctx, `DELETE FROM ai_experience_note WHERE user_id=?`, user)
	req := ExperienceSave{RequestKey: "naming-same-key", Input: experience.Input{Skill: "naming", Naming: &experience.NamingInput{Purpose: "pen", Style: "nature"}}, Favorites: []string{"云舒"}, Note: "喜欢舒展的意象"}
	first, err := ExperienceSaveNote(ctx, s, user, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Data.Result.Version != experience.Version || len(first.Data.Result.Candidates) != 4 {
		t.Fatal("missing server output")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, e := ExperienceSaveNote(ctx, s, user, req)
			if e != nil || n.ID != first.ID {
				t.Errorf("retry changed note: %v", e)
			}
		}()
	}
	wg.Wait()
	if _, e := ExperienceGetNote(ctx, s, "other-user", first.ID); e == nil {
		t.Fatal("cross-account read")
	}
	if e := ExperienceDeleteNote(ctx, s, "other-user", first.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := ExperienceGetNote(ctx, s, user, first.ID); e != nil {
		t.Fatal("cross-account delete")
	}
	other, e := ExperienceListNotes(ctx, s, "other-user", 1)
	if e != nil || len(other) != 0 {
		t.Fatal("history leaked")
	}
	rows, e := ExperienceListNotes(ctx, s, user, 1)
	if e != nil || len(rows) != 1 {
		t.Fatal("duplicate rows")
	}
	req.Note = "不同内容"
	if _, e := ExperienceSaveNote(ctx, s, user, req); e == nil {
		t.Fatal("idempotency collision accepted")
	}
	req.RequestKey = "forged-favorite"
	req.Favorites = []string{"不是候选"}
	if _, e := ExperienceSaveNote(ctx, s, user, req); e == nil {
		t.Fatal("forged favorite saved")
	}
	if e := ExperienceDeleteNote(ctx, s, user, first.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := ExperienceGetNote(ctx, s, user, first.ID); e == nil {
		t.Fatal("deleted note readable")
	}
}
