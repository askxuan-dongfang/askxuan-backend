package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const accountTestOrigin = "http://localhost:5186"

func accountFixture(t *testing.T, authority http.HandlerFunc) (*app, *httptest.Server) {
	t.Helper()
	service, err := newApp(memoryStorage(), "account-fixture-csrf-only")
	if err != nil {
		t.Fatal(err)
	}
	if authority != nil {
		auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/api/v1/auth/access" {
				t.Errorf("unexpected authority request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			authority(w, r)
		}))
		t.Cleanup(auth.Close)
		service.authURL, service.authClient = auth.URL, auth.Client()
	}
	t.Cleanup(service.close)
	server := httptest.NewServer(service.handler())
	t.Cleanup(server.Close)
	return service, server
}

func accountCustomers(w http.ResponseWriter, r *http.Request) {
	var id int64
	switch r.Header.Get("Authorization") {
	case "Bearer opaque-test-a":
		id = 101
	case "Bearer opaque-test-b":
		id = 202
	case "Bearer revoked":
		writeJSON(w, 200, map[string]any{"code": 40105})
		return
	default:
		writeJSON(w, 200, map[string]any{"code": 40102})
		return
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"userId": id, "roles": []string{"customer"}, "clientId": "customer"}})
}

func accountCall(t *testing.T, b *browser, method, path, bearer string, body any, headers http.Header) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequest(method, b.base+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", accountTestOrigin)
	request.Header.Set("X-Quiet-CSRF", b.csrf)
	request.Header.Set("Idempotency-Key", "account-request-0001")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	for name, values := range headers {
		request.Header[name] = values
	}
	response, err := b.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, data
}

func accountWantStatus(t *testing.T, status int, data []byte, wantStatus, wantCode int) {
	t.Helper()
	if status != wantStatus {
		t.Fatalf("status = %d, want %d; response=%s", status, wantStatus, data)
	}
	if wantCode != 0 {
		var envelope struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil || envelope.Code != wantCode {
			t.Fatalf("business code = %d, want %d; response=%s", envelope.Code, wantCode, data)
		}
	}
}

func accountList(t *testing.T, b *browser, bearer, path string, headers http.Header) []practiceRecord {
	t.Helper()
	status, data := accountCall(t, b, http.MethodGet, path, bearer, nil, headers)
	accountWantStatus(t, status, data, 200, 0)
	var rows []practiceRecord
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func accountRecord(seconds int64) practiceRecord {
	return practiceRecord{Date: time.Now().UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano), Seconds: seconds, Hits: 2, Beads: 108}
}

func accountOwnerHash(authority string, id int64) string {
	return tokenHash("quiet:customer:" + authority + ":" + strconv.FormatInt(id, 10))
}

func accountRandomID(t *testing.T) int64 {
	t.Helper()
	raw, err := randomHex(6)
	if err != nil {
		t.Fatal(err)
	}
	value, err := strconv.ParseUint(raw, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	return int64(value) + 1<<48
}

func TestAccountSessionAndCrossCookieHistory(t *testing.T) {
	service, server := accountFixture(t, accountCustomers)
	a, anotherCookie, otherAccount := newBrowser(t, server.URL, false), newBrowser(t, server.URL, false), newBrowser(t, server.URL, false)
	if guestHashFor(t, a) == guestHashFor(t, anotherCookie) {
		t.Fatal("separate browsers reused the same guest")
	}
	status, data := accountCall(t, a, http.MethodGet, "/quiet-api/session", "opaque-test-a", nil, nil)
	accountWantStatus(t, status, data, 200, 0)
	var session struct {
		Mode      string `json:"mode"`
		Available *bool  `json:"accountHistoryAvailable"`
		CSRF      string `json:"csrfToken"`
	}
	if json.Unmarshal(data, &session) != nil || session.Mode != "account-local" || session.Available == nil || !*session.Available || session.CSRF != a.csrf {
		t.Fatalf("account session capability or guest CSRF changed: %s", data)
	}
	status, data = accountCall(t, a, http.MethodPost, "/quiet-api/records", "opaque-test-a", accountRecord(31), nil)
	accountWantStatus(t, status, data, 200, 0)
	rows := accountList(t, anotherCookie, "opaque-test-a", "/quiet-api/records", nil)
	if len(rows) != 1 || rows[0].Seconds != 31 || rows[0].ID == "" {
		t.Fatalf("same account could not read across cookies: %#v", rows)
	}
	if len(accountList(t, otherAccount, "opaque-test-b", "/quiet-api/records", nil)) != 0 || len(a.list(t)) != 0 {
		t.Fatal("account history crossed to another account or guest")
	}
	service.store.mu.Lock()
	ownerCount := len(service.store.records[accountOwnerHash(service.authURL, 101)])
	service.store.mu.Unlock()
	if ownerCount != 1 {
		t.Fatal("record owner was not the verified customer identity hash")
	}
	status, data = accountCall(t, otherAccount, http.MethodPost, "/quiet-api/records", "opaque-test-b", accountRecord(62), nil)
	accountWantStatus(t, status, data, 200, 0)
	if rows = accountList(t, a, "opaque-test-a", "/quiet-api/records", nil); len(rows) != 1 || rows[0].Seconds != 31 {
		t.Fatal("same idempotency key across accounts mixed their records")
	}
	status, data = accountCall(t, anotherCookie, http.MethodDelete, "/quiet-api/records", "opaque-test-a", nil, nil)
	accountWantStatus(t, status, data, 200, 0)
	if len(accountList(t, a, "opaque-test-a", "/quiet-api/records", nil)) != 0 || len(accountList(t, otherAccount, "opaque-test-b", "/quiet-api/records", nil)) != 1 {
		t.Fatal("cross-cookie account delete affected another owner")
	}
}

func TestAccountDoesNotAdoptGuestHistory(t *testing.T) {
	_, server := accountFixture(t, accountCustomers)
	a, b := newBrowser(t, server.URL, false), newBrowser(t, server.URL, false)
	status, data := accountCall(t, a, http.MethodPost, "/quiet-api/records", "", accountRecord(17), nil)
	accountWantStatus(t, status, data, 200, 0)
	if len(accountList(t, a, "opaque-test-a", "/quiet-api/records", nil)) != 0 {
		t.Fatal("logging in adopted visitor history")
	}
	status, data = accountCall(t, a, http.MethodPost, "/quiet-api/records", "opaque-test-a", accountRecord(43), nil)
	accountWantStatus(t, status, data, 200, 0)
	if rows := a.list(t); len(rows) != 1 || rows[0].Seconds != 17 || len(b.list(t)) != 0 {
		t.Fatal("guest history changed on account save or leaked to another guest")
	}
	if rows := accountList(t, b, "opaque-test-a", "/quiet-api/records", nil); len(rows) != 1 || rows[0].Seconds != 43 {
		t.Fatal("account history still depended on the guest cookie")
	}
	status, data = accountCall(t, b, http.MethodDelete, "/quiet-api/records", "opaque-test-a", nil, nil)
	accountWantStatus(t, status, data, 200, 0)
	if len(a.list(t)) != 1 {
		t.Fatal("account deletion removed the earlier anonymous record")
	}
}

func TestAccountRejectsCallerSelectedOwners(t *testing.T) {
	service, server := accountFixture(t, accountCustomers)
	a, b := newBrowser(t, server.URL, false), newBrowser(t, server.URL, false)
	status, data := accountCall(t, b, http.MethodPost, "/quiet-api/records", "opaque-test-b", accountRecord(72), nil)
	accountWantStatus(t, status, data, 200, 0)
	forged := http.Header{"X-User-Id": {"202"}, "X-Account-Id": {"202"}, "X-Guest-Id": {guestHashFor(t, b)}, "X-Quiet-Owner": {accountOwnerHash(service.authURL, 202)}}
	path := "/quiet-api/records?userId=202&accountId=202&owner=" + accountOwnerHash(service.authURL, 202) + "&guest=" + guestHashFor(t, b)
	if len(accountList(t, a, "opaque-test-a", path, forged)) != 0 || len(accountList(t, a, "", path, forged)) != 0 {
		t.Fatal("query or identity headers selected another record owner")
	}
	status, data = accountCall(t, a, http.MethodPost, path, "opaque-test-a", accountRecord(19), forged)
	accountWantStatus(t, status, data, 200, 0)
	for _, field := range []string{"userId", "accountId", "guestID", "owner", "id"} {
		body := map[string]any{"date": time.Now().UTC().Format(time.RFC3339Nano), "seconds": 1, "hits": 1, "beads": 1, field: "202"}
		status, data = accountCall(t, a, http.MethodPost, path, "opaque-test-a", body, forged)
		accountWantStatus(t, status, data, 400, 0)
	}
	status, data = accountCall(t, a, http.MethodDelete, path, "opaque-test-a", nil, forged)
	accountWantStatus(t, status, data, 200, 0)
	if rows := accountList(t, b, "opaque-test-b", "/quiet-api/records", nil); len(rows) != 1 || rows[0].Seconds != 72 {
		t.Fatal("forged owner on delete affected another account")
	}
}

func TestAccountStillRequiresGuestCookieAndCSRF(t *testing.T) {
	_, server := accountFixture(t, accountCustomers)
	a, b := newBrowser(t, server.URL, false), newBrowser(t, server.URL, false)
	withoutCookie := &browser{base: server.URL, client: &http.Client{Timeout: time.Second}}
	status, data := accountCall(t, withoutCookie, http.MethodGet, "/quiet-api/records", "opaque-test-a", nil, nil)
	accountWantStatus(t, status, data, 403, 40310)
	for _, headers := range []http.Header{{"X-Quiet-CSRF": {""}}, {"X-Quiet-CSRF": {b.csrf}}, {"Origin": {"https://untrusted.invalid"}}} {
		for _, method := range []string{http.MethodPost, http.MethodDelete} {
			status, data = accountCall(t, a, method, "/quiet-api/records", "opaque-test-a", accountRecord(1), headers)
			accountWantStatus(t, status, data, 403, 0)
		}
	}
	if len(accountList(t, a, "opaque-test-a", "/quiet-api/records", nil)) != 0 {
		t.Fatal("an unsafe account write reached storage")
	}
}

func TestAccountAuthorityFailuresNeverFallBackToGuest(t *testing.T) {
	for _, test := range []struct {
		name, body string
		httpStatus int
		wantStatus int
		wantCode   int
	}{
		{"invalid", `{"code":40101}`, 200, 401, 40101},
		{"invalid-token", `{"code":40102}`, 200, 401, 40102},
		{"expired", `{"code":40103}`, 200, 401, 40103},
		{"revoked", `{"code":40105}`, 200, 401, 40105},
		{"forbidden", `{"code":40301}`, 200, 403, 40301},
		{"admin", `{"code":0,"data":{"userId":101,"roles":["platform_super"],"clientId":"platform-admin"}}`, 200, 403, 40301},
		{"mixed-roles", `{"code":0,"data":{"userId":101,"roles":["customer","master"],"clientId":"customer"}}`, 200, 403, 40301},
		{"wrong-client", `{"code":0,"data":{"userId":101,"roles":["customer"],"clientId":"master"}}`, 200, 403, 40301},
		{"missing-identity", `{"code":0}`, 200, 403, 40301},
		{"non-positive-id", `{"code":0,"data":{"userId":0,"roles":["customer"],"clientId":"customer"}}`, 200, 403, 40301},
		{"outage", `{"code":50205}`, 503, 503, 50205},
		// The auth API's business contract is HTTP 200; proxy HTTP failures are dependencies, not logout instructions.
		{"http-401-not-authoritative", `{"code":40103}`, 401, 503, 50205},
		{"malformed-json", `{`, 200, 503, 50205},
		{"missing-code", `{"data":{"userId":101,"roles":["customer"],"clientId":"customer"}}`, 200, 503, 50205},
		{"wrong-id-type", `{"code":0,"data":{"userId":"101","roles":["customer"],"clientId":"customer"}}`, 200, 503, 50205},
		{"unknown-business-error", `{"code":59999}`, 200, 503, 50205},
		{"oversized", `{"code":0,"padding":"` + strings.Repeat("x", 16400) + `"}`, 200, 503, 50205},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, server := accountFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer invalid" {
					t.Error("bearer was not forwarded to the authority")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.httpStatus)
				_, _ = io.WriteString(w, test.body)
			})
			b := newBrowser(t, server.URL, false)
			status, data := accountCall(t, b, http.MethodPost, "/quiet-api/records", "", accountRecord(12), nil)
			accountWantStatus(t, status, data, 200, 0)
			for _, request := range []struct{ method, path string }{{"GET", "/quiet-api/session"}, {"GET", "/quiet-api/records"}, {"POST", "/quiet-api/records"}, {"DELETE", "/quiet-api/records"}} {
				status, data = accountCall(t, b, request.method, request.path, "invalid", accountRecord(25), nil)
				accountWantStatus(t, status, data, test.wantStatus, test.wantCode)
			}
			if rows := b.list(t); len(rows) != 1 || rows[0].Seconds != 12 {
				t.Fatal("failed account request mutated or adopted guest history")
			}
		})
	}
}

type accountRoundTrip func(*http.Request) (*http.Response, error)

func (run accountRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) { return run(req) }

func TestAccountRechecksRevocationAndTransportFailure(t *testing.T) {
	var revoked atomic.Bool
	service, server := accountFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if revoked.Load() {
			writeJSON(w, 200, map[string]any{"code": 40105})
			return
		}
		accountCustomers(w, r)
	})
	b := newBrowser(t, server.URL, false)
	status, data := accountCall(t, b, http.MethodPost, "/quiet-api/records", "opaque-test-a", accountRecord(33), nil)
	accountWantStatus(t, status, data, 200, 0)
	revoked.Store(true)
	status, data = accountCall(t, b, http.MethodDelete, "/quiet-api/records", "opaque-test-a", nil, nil)
	accountWantStatus(t, status, data, 401, 40105)
	service.authClient = &http.Client{Transport: accountRoundTrip(func(*http.Request) (*http.Response, error) { return nil, errors.New("fixture authority unavailable") })}
	status, data = accountCall(t, b, http.MethodGet, "/quiet-api/records", "opaque-test-a", nil, nil)
	accountWantStatus(t, status, data, 503, 50205)
	rows, err := service.store.list(context.Background(), accountOwnerHash(service.authURL, 101))
	if err != nil || len(rows) != 1 || rows[0].Seconds != 33 || len(b.list(t)) != 0 {
		t.Fatal("revocation or dependency failure changed record ownership")
	}
}

func TestAccountUnconfiguredKeepsExplicitGuestBoundary(t *testing.T) {
	_, server := accountFixture(t, nil)
	b := newBrowser(t, server.URL, false)
	for _, bearer := range []string{"", "opaque-test-a"} {
		status, data := accountCall(t, b, http.MethodGet, "/quiet-api/session", bearer, nil, nil)
		accountWantStatus(t, status, data, 200, 0)
		var session struct {
			Mode      string `json:"mode"`
			Available *bool  `json:"accountHistoryAvailable"`
		}
		if json.Unmarshal(data, &session) != nil || session.Mode != "guest-local" || session.Available == nil || *session.Available {
			t.Fatalf("unconfigured service claimed account history: %s", data)
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		status, data := accountCall(t, b, method, "/quiet-api/records", "opaque-test-a", accountRecord(9), nil)
		accountWantStatus(t, status, data, 503, 50205)
	}
	status, data := accountCall(t, b, http.MethodPost, "/quiet-api/records", "", accountRecord(8), nil)
	accountWantStatus(t, status, data, 200, 0)
	if rows := b.list(t); len(rows) != 1 || rows[0].Seconds != 8 {
		t.Fatal("guest-only mode stopped preserving explicit guest records")
	}
}

func TestAccountAuthEndpointRejectsAmbiguousConfiguration(t *testing.T) {
	for _, test := range []struct{ value, want string }{
		{"http://localhost:9000", "http://localhost:9000"},
		{"http://localhost:9000/", "http://localhost:9000"},
		{"https://auth.fixture.invalid", "https://auth.fixture.invalid"},
		{"HTTP://AUTH.FIXTURE.INVALID:80/", "http://auth.fixture.invalid"},
		{"HTTPS://AUTH.FIXTURE.INVALID:443/", "https://auth.fixture.invalid"},
		{"http://AUTH.FIXTURE.INVALID:9000/", "http://auth.fixture.invalid:9000"},
		{"http://[::1]:80/", "http://[::1]"},
	} {
		got, err := authEndpoint(test.value)
		if err != nil || got != test.want {
			t.Errorf("canonical authority = %q, want %q (error %v)", got, test.want, err)
		}
	}
	if got, err := authEndpoint(""); err != nil || got != "" {
		t.Fatal("empty configuration did not preserve guest-only mode")
	}
	for _, value := range []string{"localhost:9000", "ftp://auth.fixture.invalid", "http://user:fixture@auth.fixture.invalid", "http://auth.fixture.invalid/api/v1", "http://auth.fixture.invalid?target=other", "http://auth.fixture.invalid#fragment"} {
		if _, err := authEndpoint(value); err == nil {
			t.Errorf("ambiguous authority address accepted: %q", value)
		}
	}
}

func TestMySQLAccountHistoryIsolationAndReopen(t *testing.T) {
	runID, err := randomHex(12)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "quiet-account-reopen:" + runID + ":"
	store := integrationStore(t, prefix)
	var service *app
	var server *httptest.Server
	var accountHashes, guestHashes []string
	stop := func() {
		if server != nil {
			server.Close()
			server = nil
		}
		if service != nil {
			service.close()
			service = nil
		}
		if store != nil {
			store.close()
			store = nil
		}
	}
	t.Cleanup(func() {
		stop()
		cleanup := integrationStore(t, prefix)
		defer cleanup.close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, owner := range append(append([]string{}, accountHashes...), guestHashes...) {
			if _, err := cleanup.db.ExecContext(ctx, "DELETE FROM quiet_practices WHERE guest_hash=?", owner); err != nil {
				t.Error("could not remove this run's practice fixture", err)
			}
		}
		for _, guest := range guestHashes {
			if _, err := cleanup.db.ExecContext(ctx, "DELETE FROM quiet_guests WHERE token_hash=?", guest); err != nil {
				t.Error("could not remove this run's guest fixture", err)
			}
		}
		var cursor uint64
		for {
			keys, next, err := cleanup.redis.Scan(ctx, cursor, prefix+"*", 100).Result()
			if err != nil {
				t.Error("could not scan this run's Redis prefix", err)
				break
			}
			if len(keys) > 0 {
				if err := cleanup.redis.Del(ctx, keys...).Err(); err != nil {
					t.Error("could not remove this run's Redis fixtures", err)
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	})

	var ids [2]int64
	authority := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/api/v1/auth/access" {
			t.Error("unexpected mock authority request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		id := ids[0]
		switch req.Header.Get("Authorization") {
		case "Bearer opaque-test-a":
		case "Bearer opaque-test-b":
			id = ids[1]
		default:
			writeJSON(w, 200, map[string]any{"code": 40102})
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"userId": id, "roles": []string{"customer"}, "clientId": "customer"}})
	}))
	t.Cleanup(authority.Close)
	// Large random IDs are mock identities; refuse any pre-existing owner before registering cleanup.
	for attempt := 0; attempt < 8; attempt++ {
		for i := range ids {
			ids[i] = accountRandomID(t)
		}
		if ids[0] == ids[1] {
			continue
		}
		a, b := accountOwnerHash(authority.URL, ids[0]), accountOwnerHash(authority.URL, ids[1])
		var existing int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM quiet_practices WHERE guest_hash IN (?,?)", a, b).Scan(&existing); err != nil {
			t.Fatal(err)
		}
		if existing == 0 {
			accountHashes = []string{a, b}
			break
		}
	}
	if len(accountHashes) != 2 {
		t.Fatal("could not allocate unused mock customer owners")
	}
	start := func() {
		t.Helper()
		var err error
		service, err = newApp(store, "stable-account-reopen-fixture")
		if err != nil {
			t.Fatal(err)
		}
		service.authURL, service.authClient = authority.URL, authority.Client()
		server = httptest.NewServer(service.handler())
	}
	start()
	newPersistentBrowser := func() *browser {
		t.Helper()
		browser := newBrowser(t, server.URL, true)
		guestHashes = append(guestHashes, guestHashFor(t, browser))
		return browser
	}
	a, anotherCookie, b := newPersistentBrowser(), newPersistentBrowser(), newPersistentBrowser()
	for _, save := range []struct {
		browser *browser
		bearer  string
		seconds int64
	}{{a, "", 21}, {a, "opaque-test-a", 31}, {b, "opaque-test-b", 47}} {
		status, data := accountCall(t, save.browser, http.MethodPost, "/quiet-api/records", save.bearer, accountRecord(save.seconds), nil)
		accountWantStatus(t, status, data, 200, 0)
	}
	assertOwners := func() {
		t.Helper()
		for _, read := range []struct {
			browser *browser
			bearer  string
			seconds int64
		}{{a, "opaque-test-a", 31}, {anotherCookie, "opaque-test-a", 31}, {b, "opaque-test-b", 47}, {a, "", 21}} {
			rows := accountList(t, read.browser, read.bearer, "/quiet-api/records", nil)
			if len(rows) != 1 || rows[0].Seconds != read.seconds {
				t.Fatalf("persistent account or guest ownership changed: %#v", rows)
			}
		}
		if len(anotherCookie.list(t)) != 0 || len(b.list(t)) != 0 {
			t.Fatal("an account record was copied into anonymous history")
		}
	}
	assertOwners()
	if err := store.redis.Set(context.Background(), prefix+"reopen-marker", "account-fixture", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	stop()
	store = integrationStore(t, prefix)
	start()
	for _, browser := range []*browser{a, anotherCookie, b} {
		browser.base = server.URL
		browser.session(t, true)
		guestHashes = append(guestHashes, guestHashFor(t, browser))
	}
	assertOwners()
	if got, err := store.redis.Get(context.Background(), prefix+"reopen-marker").Result(); err != nil || got != "account-fixture" {
		t.Fatal("isolated Redis state did not survive storage reopen", err)
	}
	status, data := accountCall(t, anotherCookie, http.MethodDelete, "/quiet-api/records?userId="+strconv.FormatInt(ids[1], 10), "opaque-test-a", nil, http.Header{"X-User-Id": {strconv.FormatInt(ids[1], 10)}})
	accountWantStatus(t, status, data, 200, 0)
	if len(accountList(t, a, "opaque-test-a", "/quiet-api/records", nil)) != 0 || len(accountList(t, b, "opaque-test-b", "/quiet-api/records", nil)) != 1 || len(a.list(t)) != 1 {
		t.Fatal("persistent account deletion crossed an account or guest boundary")
	}
	status, data = accountCall(t, a, http.MethodDelete, "/quiet-api/records", "", nil, nil)
	accountWantStatus(t, status, data, 200, 0)
	if len(a.list(t)) != 0 || len(accountList(t, b, "opaque-test-b", "/quiet-api/records", nil)) != 1 {
		t.Fatal("anonymous deletion crossed into persisted account history")
	}
	t.Log("isolated MySQL reopen, Redis prefix persistence, account/cookie/guest separation and scoped deletion verified using mock authority only")
}

func TestMySQLAccountAuthorityNamespacesDoNotShareHistory(t *testing.T) {
	runID, err := randomHex(12)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "quiet-account-authority:" + runID + ":"
	store := integrationStore(t, prefix)
	t.Cleanup(store.close)
	var owners, guests []string
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, owner := range append(append([]string{}, owners...), guests...) {
			if _, err := store.db.ExecContext(ctx, "DELETE FROM quiet_practices WHERE guest_hash=?", owner); err != nil {
				t.Error("could not remove this run's practice fixture", err)
			}
		}
		for _, guest := range guests {
			if _, err := store.db.ExecContext(ctx, "DELETE FROM quiet_guests WHERE token_hash=?", guest); err != nil {
				t.Error("could not remove this run's guest fixture", err)
			}
		}
		var cursor uint64
		for {
			keys, next, err := store.redis.Scan(ctx, cursor, prefix+"*", 100).Result()
			if err != nil {
				t.Error("could not scan this run's Redis prefix", err)
				break
			}
			if len(keys) > 0 {
				if err := store.redis.Del(ctx, keys...).Err(); err != nil {
					t.Error("could not remove this run's Redis fixtures", err)
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	})
	var id int64
	respond := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/api/v1/auth/access" || req.Header.Get("Authorization") != "Bearer opaque-test-a" {
			t.Error("unexpected mock authority request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"userId": id, "roles": []string{"customer"}, "clientId": "customer"}})
	})
	authorityA, authorityB := httptest.NewServer(respond), httptest.NewServer(respond)
	t.Cleanup(authorityA.Close)
	t.Cleanup(authorityB.Close)
	for attempt := 0; attempt < 8; attempt++ {
		id = accountRandomID(t)
		a, b := accountOwnerHash(authorityA.URL, id), accountOwnerHash(authorityB.URL, id)
		var existing int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM quiet_practices WHERE guest_hash IN (?,?)", a, b).Scan(&existing); err != nil {
			t.Fatal(err)
		}
		if existing == 0 {
			owners = []string{a, b}
			break
		}
	}
	if len(owners) != 2 || owners[0] == owners[1] {
		t.Fatal("could not allocate distinct unused authority owners")
	}
	service, err := newApp(store, "authority-namespace-csrf-fixture")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.close)
	service.authURL, service.authClient = authorityA.URL, authorityA.Client()
	server := httptest.NewServer(service.handler())
	t.Cleanup(server.Close)
	browser := newBrowser(t, server.URL, true)
	guests = append(guests, guestHashFor(t, browser))
	status, data := accountCall(t, browser, http.MethodPost, "/quiet-api/records", "", accountRecord(24), nil)
	accountWantStatus(t, status, data, 200, 0)
	status, data = accountCall(t, browser, http.MethodPost, "/quiet-api/records", "opaque-test-a", accountRecord(61), nil)
	accountWantStatus(t, status, data, 200, 0)
	service.authURL = authorityB.URL
	if len(accountList(t, browser, "opaque-test-a", "/quiet-api/records", nil)) != 0 {
		t.Fatal("a different authority reused the first authority's customer history")
	}
	status, data = accountCall(t, browser, http.MethodPost, "/quiet-api/records", "opaque-test-a", accountRecord(82), nil)
	accountWantStatus(t, status, data, 200, 0)
	if rows := accountList(t, browser, "opaque-test-a", "/quiet-api/records", nil); len(rows) != 1 || rows[0].Seconds != 82 {
		t.Fatal("the second authority could not save independent history")
	}
	status, data = accountCall(t, browser, http.MethodDelete, "/quiet-api/records", "opaque-test-a", nil, nil)
	accountWantStatus(t, status, data, 200, 0)
	if len(accountList(t, browser, "opaque-test-a", "/quiet-api/records", nil)) != 0 {
		t.Fatal("second authority's own deletion failed")
	}
	service.authURL = authorityA.URL
	if rows := accountList(t, browser, "opaque-test-a", "/quiet-api/records", nil); len(rows) != 1 || rows[0].Seconds != 61 {
		t.Fatal("switching back lost the first authority's original history")
	}
	status, data = accountCall(t, browser, http.MethodDelete, "/quiet-api/records", "opaque-test-a", nil, nil)
	accountWantStatus(t, status, data, 200, 0)
	if len(accountList(t, browser, "opaque-test-a", "/quiet-api/records", nil)) != 0 {
		t.Fatal("first authority's own deletion failed")
	}
	if rows := browser.list(t); len(rows) != 1 || rows[0].Seconds != 24 {
		t.Fatal("authority account writes or deletion changed anonymous history")
	}
	t.Log("same random customer ID remains isolated across two mock authority namespaces in one real SQL database")
}
