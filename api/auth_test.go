package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// loginTestServer simulates users.premierleague.com + fantasy.premierleague.com
// in a single httptest server by rewriting the host header. It records the
// request it received so tests can assert on the form body, headers, and method.
type loginTestServer struct {
	loginCalls int

	// loginForm captures the parsed POST form *inside* the handler before
	// the body is closed by the server. Reading r.PostForm after the handler
	// returns yields an empty value because http.Server discards the body
	// once the response is written.
	loginForm    url.Values
	loginHeaders http.Header

	loginState  string // "success" | "fail" | "" (no redirect)
	loginReason string

	meHandler func(w http.ResponseWriter, r *http.Request)
}

func newLoginServer(t *testing.T, state, reason string) (*httptest.Server, *loginTestServer) {
	t.Helper()
	ls := &loginTestServer{loginState: state, loginReason: reason}

	mux := http.NewServeMux()
	mux.HandleFunc("/accounts/login/", func(w http.ResponseWriter, r *http.Request) {
		ls.loginCalls++
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err == nil {
			ls.loginForm = r.PostForm
		}
		ls.loginHeaders = r.Header.Clone()
		if ls.loginState == "" {
			http.Error(w, "no state configured", http.StatusInternalServerError)
			return
		}
		q := url.Values{"state": {ls.loginState}}
		if ls.loginReason != "" {
			q.Set("reason", ls.loginReason)
		}
		http.Redirect(w, r, "https://fantasy.premierleague.com/a/login?"+q.Encode(),
			http.StatusFound)
	})

	mux.HandleFunc("/a/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/me/", func(w http.ResponseWriter, r *http.Request) {
		if ls.meHandler != nil {
			ls.meHandler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"player":{"entry":3808385,"id":12345,"name":"U","email":"u@e.com"}}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, ls
}

// newAuthClientForTest returns an AuthClient pointed at the supplied httptest
// server URL. Both baseURL and loginURL are overridden so requests stay inside
// the test process.
func newAuthClientForTest(t *testing.T, srvURL string) *AuthClient {
	t.Helper()
	c := NewAuthClient(context.Background())
	parsed, _ := url.Parse(srvURL)
	c.http.Jar = nil
	c.baseURL = srvURL
	// http.Client.Transport rewrites every host → 127.0.0.1:port of srvURL.
	c.http.Transport = &rewriteHostTransport{host: parsed.Host}
	return c
}

func TestLoginSuccess(t *testing.T) {
	srv, ls := newLoginServer(t, "success", "")
	client := newAuthClientForTest(t, srv.URL)

	if err := client.Login("u@e.com", "hunter2"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if ls.loginCalls != 1 {
		t.Errorf("login calls = %d, want 1", ls.loginCalls)
	}

	// Verify form fields.
	if got := ls.loginForm.Get("login"); got != "u@e.com" {
		t.Errorf("login form = %q, want u@e.com", got)
	}
	if got := ls.loginForm.Get("password"); got != "hunter2" {
		t.Errorf("password form = %q, want hunter2", got)
	}
	if got := ls.loginForm.Get("app"); got != "plfpl-web" {
		t.Errorf("app form = %q, want plfpl-web", got)
	}
	if got := ls.loginForm.Get("redirect_uri"); got != loginRedirectURI {
		t.Errorf("redirect_uri = %q, want %q", got, loginRedirectURI)
	}
	if ct := ls.loginHeaders.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want form-encoded", ct)
	}
	if ua := ls.loginHeaders.Get("User-Agent"); ua != userAgent {
		t.Errorf("User-Agent = %q, want %q", ua, userAgent)
	}
}

func TestLoginFailState(t *testing.T) {
	srv, _ := newLoginServer(t, "fail", "InvalidLogin")
	client := newAuthClientForTest(t, srv.URL)

	err := client.Login("u@e.com", "wrong")
	if !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("err = %v, want ErrAuthFailed", err)
	}
	if !strings.Contains(err.Error(), "InvalidLogin") {
		t.Errorf("err %q should include reason", err.Error())
	}
}

func TestLoginForbidden(t *testing.T) {
	srv, _ := newLoginServer(t, "success", "")
	client := newAuthClientForTest(t, srv.URL)

	// Re-route /accounts/login/ to 403 instead.
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/accounts/login/") {
			http.Error(w, "blocked", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	err := client.Login("u@e.com", "pw")
	if !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("err = %v, want ErrAuthFailed", err)
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("err %q should mention 403", err.Error())
	}
}

func TestIsLoggedInTrue(t *testing.T) {
	srv, _ := newLoginServer(t, "success", "")
	client := newAuthClientForTest(t, srv.URL)

	ok, err := client.IsLoggedIn()
	if err != nil {
		t.Fatalf("IsLoggedIn: %v", err)
	}
	if !ok {
		t.Error("IsLoggedIn = false, want true")
	}
}

func TestIsLoggedInFalseWhenPlayerNull(t *testing.T) {
	srv, ls := newLoginServer(t, "success", "")
	ls.meHandler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"player":null}`))
	}
	client := newAuthClientForTest(t, srv.URL)

	ok, err := client.IsLoggedIn()
	if err != nil {
		t.Fatalf("IsLoggedIn: %v", err)
	}
	if ok {
		t.Error("IsLoggedIn = true, want false when player is null")
	}
}

func TestMeReturnsTeamID(t *testing.T) {
	srv, _ := newLoginServer(t, "success", "")
	client := newAuthClientForTest(t, srv.URL)

	me, err := client.Me()
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.Entry.String() != "3808385" {
		t.Errorf("Entry = %s, want 3808385", me.Entry.String())
	}
	if me.Email != "u@e.com" {
		t.Errorf("Email = %q, want u@e.com", me.Email)
	}
}

func TestMeRequiresAuth(t *testing.T) {
	srv, ls := newLoginServer(t, "success", "")
	ls.meHandler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"Authentication credentials were not provided."}`))
	}
	client := newAuthClientForTest(t, srv.URL)

	_, err := client.Me()
	if !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("err = %v, want ErrAuthFailed", err)
	}
}

// rewriteHostTransport rewrites every outbound request to hit host:port.
// Returns nil if the original transport is fine — but we need to swap host.
type rewriteHostTransport struct {
	host string
}

func (r *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Host = r.host
	req2.URL.Scheme = "http"
	req2.Host = r.host
	return http.DefaultTransport.RoundTrip(req2)
}

var _ http.RoundTripper = (*rewriteHostTransport)(nil)

// Ensure Me + json.Number round-tripping works as expected (large team IDs).
func TestMeHandlesLargeTeamID(t *testing.T) {
	srv, ls := newLoginServer(t, "success", "")
	ls.meHandler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"player":{"entry":99999999,"id":7,"name":"Big","email":"b@e.com"}}`))
	}
	client := newAuthClientForTest(t, srv.URL)

	me, err := client.Me()
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	var n int64
	if err := json.Unmarshal([]byte(me.Entry), &n); err != nil {
		t.Fatalf("Entry not parseable as int: %v", err)
	}
	if n != 99999999 {
		t.Errorf("Entry = %d, want 99999999", n)
	}
}
