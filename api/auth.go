package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// AuthClient is an HTTP client with cookie jar support for FPL auth.
// It tracks session cookies across the two FPL hosts (users.premierleague.com
// for the login form, fantasy.premierleague.com for the API).
type AuthClient struct {
	http    *http.Client
	baseURL string // "https://fantasy.premierleague.com"
	ctx     context.Context
}

// loginURL is the form-encoded POST endpoint on the accounts host.
const loginURL = "https://users.premierleague.com/accounts/login/"

// loginRedirectURI is the redirect target FPL inspects for state=success|fail.
const loginRedirectURI = "https://fantasy.premierleague.com/a/login"

// userAgent matches the FPL Android app's UA — avoids 403 bot detection at the
// Fastly edge. Verified in fpl-api.md §3.4.
const userAgent = "Dalvik/2.1.0 (Linux; U; Android 5.1; PRO 5 Build/LMY47D)"

// ErrAuthFailed is returned when the login redirect carries state=fail or the
// response status indicates a blocked/forbidden outcome.
var ErrAuthFailed = errors.New("fpl: authentication failed")

// NewAuthClient creates an authenticated FPL client with a cookie jar that
// accepts all cookies (no Public Suffix List dependency).
func NewAuthClient(ctx context.Context) *AuthClient {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
	}
	client.CheckRedirect = captureLoginRedirect(client.CheckRedirect)
	return &AuthClient{
		http:    client,
		baseURL: "https://fantasy.premierleague.com",
		ctx:     ctx,
	}
}

// loginState carries the redirect target captured by captureLoginRedirect.
// It is set during the redirect chain and read by Login immediately after
// the final response is received.
type loginState struct {
	state  string
	reason string
}

// captureLoginRedirect wraps the default CheckRedirect to extract the
// state=success|fail query param from the post-login Location header. We use
// this instead of resp.Location() because the final response after following
// redirects has no Location header — only the redirect chain does.
func captureLoginRedirect(prev func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if loc := req.Response.Header.Get("Location"); loc != "" {
			if u, err := url.Parse(loc); err == nil {
				state := u.Query().Get("state")
				if state != "" {
					if ctx := req.Context().Value(loginCtxKey{}); ctx != nil {
						ls := ctx.(*loginState)
						ls.state = state
						ls.reason = u.Query().Get("reason")
					}
				}
			}
		}
		if prev != nil {
			return prev(req, via)
		}
		return nil
	}
}

type loginCtxKey struct{}

// httpClient exposes the underlying client for tests; callers should not use it.
func (c *AuthClient) httpClient() *http.Client { return c.http }

// Login authenticates with email + password against the FPL accounts host.
//
// FPL returns HTTP 302 with Location: ...?state=success on success and
// ...?state=fail&reason=... on failure. We check the state param rather than
// the HTTP status, mirroring amosbastian/fpl's approach.
//
// The body is form-encoded (NOT JSON) and includes the app identifier +
// redirect_uri FPL uses to validate the request. The Android User-Agent avoids
// 403 bot blocks at the Fastly edge.
func (c *AuthClient) Login(email, password string) error {
	form := url.Values{
		"login":        {email},
		"password":     {password},
		"app":          {"plfpl-web"},
		"redirect_uri": {loginRedirectURI},
	}

	ctx := context.WithValue(c.ctx, loginCtxKey{}, &loginState{})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", c.baseURL+"/")
	req.Header.Set("Origin", c.baseURL)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%w: blocked by edge (403), try a different network", ErrAuthFailed)
	}

	ls, _ := ctx.Value(loginCtxKey{}).(*loginState)
	if ls == nil {
		return fmt.Errorf("%w: login state not captured", ErrAuthFailed)
	}
	switch ls.state {
	case "success":
		return nil
	case "fail":
		reason := ls.reason
		if reason == "" {
			reason = "unknown"
		}
		return fmt.Errorf("%w: %s", ErrAuthFailed, reason)
	default:
		return fmt.Errorf("%w: no state=success in redirect chain", ErrAuthFailed)
	}
}

// Me is the parsed /api/me/ response. The "entry" field is the user's team_id.
type Me struct {
	Entry json.Number `json:"entry"` // team_id, parsed to int
	ID    json.Number `json:"id"`    // user_id
	Name  string      `json:"name"`
	Email string      `json:"email"`
}

// PlayerEntry mirrors the inner "player" object on /api/me/. When the user is
// not logged in, "player" is JSON null — see IsLoggedIn.
type PlayerEntry struct {
	Entry json.Number `json:"entry"`
	ID    json.Number `json:"id"`
	Name  string      `json:"name"`
	Email string      `json:"email"`
}

// meResponse wraps the raw /api/me/ payload.
type meResponse struct {
	Player *PlayerEntry `json:"player"`
}

// Me fetches the logged-in user's profile. Entry is the team_id; it is
// returned as a json.Number so callers can parse to int without losing the
// integer on 32-bit platforms (the FPL team IDs are large but fit in int64).
func (c *AuthClient) Me() (*Me, error) {
	var raw meResponse
	if err := c.getJSON("/api/me/", &raw); err != nil {
		return nil, err
	}
	if raw.Player == nil {
		return nil, ErrAuthFailed
	}
	return &Me{
		Entry: raw.Player.Entry,
		ID:    raw.Player.ID,
		Name:  raw.Player.Name,
		Email: raw.Player.Email,
	}, nil
}

// IsLoggedIn is a cheap probe. It returns true when /api/me/ returns a
// non-null "player" object, false otherwise (including on auth failure).
func (c *AuthClient) IsLoggedIn() (bool, error) {
	var raw meResponse
	if err := c.getJSON("/api/me/", &raw); err != nil {
		return false, err
	}
	return raw.Player != nil, nil
}

// getJSON performs an authenticated GET against the FPL API and decodes the
// JSON body into target. Trailing slash is mandatory — Fastly 301s otherwise.
func (c *AuthClient) getJSON(path string, target any) error {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%w: 403 from %s — session invalid", ErrAuthFailed, path)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s: status %d: %s", path, resp.StatusCode, body)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// doPOST is a thin wrapper for authenticated POST requests that return JSON.
// Trailing-slash enforcement matches getJSON.
func (c *AuthClient) doPOST(path string, body any) (*http.Response, error) {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, c.baseURL+path,
		strings.NewReader(string(buf)))
	if err != nil {
		return nil, fmt.Errorf("build POST %s: %w", path, err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", c.baseURL)
	req.Header.Set("Referer", c.baseURL+"/")

	return c.http.Do(req)
}
