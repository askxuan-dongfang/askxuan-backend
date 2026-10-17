package rewards

import (
	"bytes"
	"encoding/base64"
	"testing"
	"time"
)

func bps(n int) *int { return &n }
func multiFixture() Campaign {
	c := fixture()
	c.Kind = "wheel"
	c.Capacity = 10
	c.Budget = 100000
	c.Prizes = []Prize{
		{Name: "香囊", Kind: "physical", Value: 1000, Quantity: 1, ProbabilityBps: bps(1250)},
		{Name: "平台礼品卡", Kind: "code", Value: 5000, Quantity: 1, ProbabilityBps: bps(250), Platform: "测试平台", Instructions: "前往平台兑换", ExpiresAt: c.EndsAt + 86400, RedemptionURL: "https://example.com/redeem"},
		{Name: "兑换券", Kind: "link", Value: 2000, Quantity: 1, ProbabilityBps: bps(500), Platform: "测试平台", Instructions: "打开专属兑换链接", ExpiresAt: c.EndsAt + 86400},
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
		"budget": func(c *Campaign) { c.Budget = 7999 }, "capacity": func(c *Campaign) { c.Capacity = 0 },
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
func TestLegacyTicketSelectsActualPrize(t *testing.T) {
	c := multiFixture()
	for i := range c.Prizes {
		c.Prizes[i].ProbabilityBps = nil
	}
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

func TestFixedProbabilityValidationAndBoundaries(t *testing.T) {
	c := multiFixture()
	// Inventory is independent from the probability and can exceed participation slots.
	c.Prizes[0].Quantity = 5000
	c.Budget = 10000000
	if err := Validate(c, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []int{0, -1, 10001} {
		v := multiFixture()
		v.Prizes[0].ProbabilityBps = bps(invalid)
		if Validate(v, time.Now().Unix()) == nil {
			t.Fatalf("accepted %d", invalid)
		}
	}
	v := multiFixture()
	v.Prizes[0].ProbabilityBps = nil
	if Validate(v, time.Now().Unix()) == nil {
		t.Fatal("mixed probability modes accepted")
	}
	v = multiFixture()
	v.Prizes[0].ProbabilityBps = bps(10000)
	if Validate(v, time.Now().Unix()) == nil {
		t.Fatal("total over 100% accepted")
	}
	counts := map[string]int{}
	for n := 0; n < 10000; n++ {
		if p := prizeForTicket(c, n); p != nil {
			counts[p.Name]++
		} else {
			counts["lost"]++
		}
	}
	if counts[c.Prizes[0].Name] != 1250 || counts[c.Prizes[1].Name] != 250 || counts[c.Prizes[2].Name] != 500 || counts["lost"] != 8000 {
		t.Fatal(counts)
	}
	// More participants and fewer prizes must not change the configured probability.
	c.ParticipantCount = 9
	c.Prizes[0].AwardedCount = 4999
	if p := prizeForTicket(c, 1249); p == nil || p.Kind != "physical" {
		t.Fatal("probability drift")
	}
	c.Prizes = c.Prizes[:1]
	c.Prizes[0].ProbabilityBps = bps(10000)
	if Validate(c, time.Now().Unix()) != nil || prizeForTicket(c, 9999) == nil || prizeForTicket(c, 10000) != nil {
		t.Fatal("100% boundary")
	}
}
