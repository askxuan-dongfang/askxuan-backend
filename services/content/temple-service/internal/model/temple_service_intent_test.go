package model

import "testing"

func TestValidIntentTags(t *testing.T) {
	if !ValidIntentTags([]string{"peace", "rite"}) {
		t.Fatal("valid intention tags rejected")
	}
	if ValidIntentTags([]string{"not-a-tag"}) {
		t.Fatal("invalid intention tag accepted")
	}
}
