package rewards

import (
	"bytes"
	"encoding/base64"
	"testing"
	"time"
)

func multiFixture() Campaign {
	c := fixture()
	c.Kind = "wheel"
	c.Capacity = 10
	c.Budget = 100000
	c.Prizes = []Prize{
		{Name: "香囊", Kind: "physical", Value: 1000, Quantity: 1},
		{Name: "平台礼品卡", Kind: "code", Value: 5000, Quantity: 1, Platform: "测试平台", Instructions: "前往平台兑换", ExpiresAt: c.EndsAt + 86400, RedemptionURL: "https://example.com/redeem"},
		{Name: "兑换券", Kind: "link", Value: 2000, Quantity: 1, Platform: "测试平台", Instructions: "打开专属兑换链接", ExpiresAt: c.EndsAt + 86400},
	}
	normalizeCampaign(&c)
	return c
}
func TestMultiPrizeValidation(t *testing.T) {
	c := multiFixture()
	if err := Validate(c, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Campaign){
		"budget": func(c *Campaign) { c.Budget = 7999 }, "capacity": func(c *Campaign) { c.Capacity = 2 },
		"expires": func(c *Campaign) { c.Prizes[1].ExpiresAt = c.EndsAt }, "unsafe": func(c *Campaign) { c.Prizes[1].RedemptionURL = "javascript:alert(1)" },
		"credentialsInURL": func(c *Campaign) { c.Prizes[1].RedemptionURL = "https://user:password@example.com" },
		"type":             func(c *Campaign) { c.Prizes[1].Kind = "cash" }, "pool": func(c *Campaign) { c.Kind = "pool" },
		"duplicates": func(c *Campaign) { c.Prizes[0].ID = 1; c.Prizes[1].ID = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			v := multiFixture()
			change(&v)
			if Validate(v, time.Now().Unix()) == nil {
				t.Fatal("invalid prize accepted")
			}
		})
	}
}
func TestTicketSelectsActualPrize(t *testing.T) {
	c := multiFixture()
	c.Prizes[0].ID = 10
	c.Prizes[1].ID = 20
	c.Prizes[2].ID = 30
	c.Prizes[1].Quantity = 2
	for n, want := range []int64{10, 20, 20, 30} {
		p := prizeForTicket(c, n)
		if p == nil || p.ID != want {
			t.Fatalf("ticket %d", n)
		}
	}
	if prizeForTicket(c, 4) != nil {
		t.Fatal("losing ticket awarded")
	}
	c.Prizes[0].AwardedCount = 1
	if p := prizeForTicket(c, 0); p == nil || p.ID != 20 {
		t.Fatal("exhausted prize selected")
	}
}
func TestCredentialVault(t *testing.T) {
	s := Store{RedemptionKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))}
	cipher, digest, err := s.sealCredential(21, "TEST-ONLY-CODE")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(cipher, []byte("TEST-ONLY-CODE")) {
		t.Fatal("plaintext stored")
	}
	raw, err := s.openCredential(21, cipher)
	if err != nil || raw != "TEST-ONLY-CODE" {
		t.Fatal("cannot decrypt")
	}
	if _, err = s.openCredential(22, cipher); err == nil {
		t.Fatal("credential moved to another prize")
	}
	other, again, err := s.sealCredential(21, raw)
	if err != nil || again != digest || bytes.Equal(cipher, other) {
		t.Fatal("nonce or duplicate digest")
	}
	cipher[len(cipher)-1] ^= 1
	if _, err = s.openCredential(21, cipher); err == nil {
		t.Fatal("tamper accepted")
	}
	if _, _, err = (Store{}).sealCredential(1, raw); err == nil {
		t.Fatal("unconfigured vault accepted")
	}
}
