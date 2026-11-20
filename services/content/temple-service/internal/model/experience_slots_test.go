package model

import (
	"context"
	"os"
	"testing"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestExperienceSlotModesMySQL(t *testing.T) {
	dsn := os.Getenv("WISH_TEMPLE_TEST_DSN")
	if dsn == "" || os.Getenv("WISH_CHECKOUT_TEST_ISOLATED") != "1" {
		t.Skip("requires an isolated database loaded with the experience fixtures")
	}
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	services := NewTempleServiceModel(db)
	read := func() *TempleServiceRecord {
		t.Helper()
		result, err := services.FindByCodes(ctx, "TEXP1120", "S001")
		if err != nil || len(result.Slots) != 2 {
			t.Fatalf("experience slots = %+v, %v", result, err)
		}
		return result
	}
	slotModes := func(slots []TempleServiceSlot) (string, string) {
		var first, second string
		if slots[0].ParticipationMode != nil {
			first = *slots[0].ParticipationMode
		}
		if slots[1].ParticipationMode != nil {
			second = *slots[1].ParticipationMode
		}
		return first, second
	}
	initial := read()
	t.Cleanup(func() { _ = services.ReplaceSlots(ctx, initial.Id, initial.Slots) })
	if a, b := slotModes(initial.Slots); a != "onsite" || b != "proxy" {
		t.Fatalf("unexpected initial modes %q %q", a, b)
	}
	edit := append([]TempleServiceSlot(nil), initial.Slots...)
	edit[0].ParticipationMode = nil
	clear := ""
	edit[1].ParticipationMode = &clear
	if err := services.ReplaceSlots(ctx, initial.Id, edit); err != nil {
		t.Fatal(err)
	}
	if a, b := slotModes(read().Slots); a != "onsite" || b != "" {
		t.Fatalf("old client lost a mode or an explicit clear failed %q %q", a, b)
	}
}
