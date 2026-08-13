package model

import "testing"

func TestCalculateBookingSplitBalanced(t *testing.T) {
	split, err := CalculateBookingSplit(200, 1, 201, 0.12)
	if err != nil {
		t.Fatal(err)
	}
	if split.Commission != 24.12 || split.MasterCommission != 24 || split.TempleCommission != 0.12 {
		t.Fatalf("unexpected commission split: %+v", split)
	}
	if split.MasterNet != 176 || split.TempleNet != 0.88 {
		t.Fatalf("unexpected target split: %+v", split)
	}
	if !sameMoney(split.Commission+split.MasterNet+split.TempleNet, split.Total) {
		t.Fatalf("ledger does not balance: %+v", split)
	}
}

func TestCalculateBookingSplitRejectsTamperedSnapshot(t *testing.T) {
	if _, err := CalculateBookingSplit(200, 1, 999, 0.12); err == nil {
		t.Fatal("expected mismatched snapshot to fail")
	}
	if _, err := CalculateBookingSplit(200, 0, 200, 1.01); err == nil {
		t.Fatal("expected invalid commission rate to fail")
	}
}

func TestCalculateConsultationSplitBalanced(t *testing.T) {
	split, err := CalculateConsultationSplit(39, 0.20)
	if err != nil {
		t.Fatal(err)
	}
	if split.Total != 39 || split.Commission != 7.8 || split.MasterNet != 31.2 {
		t.Fatalf("unexpected consultation split: %+v", split)
	}
	if !sameMoney(split.Commission+split.MasterNet, split.Total) {
		t.Fatalf("consultation ledger does not balance: %+v", split)
	}
	if _, err := CalculateConsultationSplit(39, 1.01); err == nil {
		t.Fatal("expected invalid consultation commission rate to fail")
	}
}
