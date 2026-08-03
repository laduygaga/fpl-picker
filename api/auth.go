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
	bearer  string // optional OAuth/JWT access_token (x-api-authorization: Bearer)
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

// SetBaseURL overrides the API base URL. Used by tests to point at a local
// httptest server; production callers should leave it at the default FPL host.
func (c *AuthClient) SetBaseURL(u string) { c.baseURL = u }

// SetTransport overrides the HTTP transport. Used by tests to install a
// host-rewriting transport; production callers should leave the default
// http.Transport in place.
func (c *AuthClient) SetTransport(t http.RoundTripper) { c.http.Transport = t }

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

// LoadCookies seeds the jar from a Netscape-style cookie header value
// ("name1=value1; name2=value2; ..."). All cookies are scoped to the
// fantasy.premierleague.com domain so subsequent API calls authenticate.
//
// Typical cookies to include (export from your browser after logging in at
// fantasy.premierleague.com):
//
//	pl_profile  — the gating auth cookie. Without this every private API
//	              call returns 403 "Authentication credentials were not
//	              provided."
//	csrftoken    — required by POSTs to /api/transfers/ and the lineup
//	              update endpoint.
//	sessionid    — Django session cookie (typically optional for read-only
//	              access; required for writes alongside csrftoken).
//
// Domain (.premierleague.com) matches all subdomains — fantasy, users,
// account. We attach them all to fantasy.premierleague.com as the single
// API origin.
//
// Use this when the FPL login flow is unreachable (DNS block, OAuth-only
// auth, etc.). The user logs into fantasy.premierleague.com in their
// browser, exports cookies via DevTools, and pastes them here. The cookies
// expire after ~30 days, just like a browser session.
//
// Returns the number of cookies successfully loaded.
func (c *AuthClient) LoadCookies(headerValue string) (int, error) {
	base, err := url.Parse(c.baseURL + "/")
	if err != nil {
		return 0, fmt.Errorf("parse base URL: %w", err)
	}

	now := time.Now()
	var cookies []*http.Cookie
	for _, part := range strings.Split(headerValue, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eq := strings.IndexByte(part, '=')
		if eq <= 0 {
			continue
		}
		name := strings.TrimSpace(part[:eq])
		value := strings.TrimSpace(part[eq+1:])
		cookies = append(cookies, &http.Cookie{
			Name:     name,
			Value:    value,
			Path:     "/",
			Domain:   ".premierleague.com",
			Secure:   true,
			HttpOnly: false,
			Expires:  now.Add(30 * 24 * time.Hour),
		})
	}
	if len(cookies) == 0 {
		return 0, errors.New("no cookies parsed from input")
	}
	c.http.Jar.SetCookies(base, cookies)
	return len(cookies), nil
}

// LoadBearer sets an OAuth/JWT bearer token used for all subsequent requests.
// The token is sent via the x-api-authorization header.
//
// The 2026-era FPL web app uses an OAuth flow through account.premierleague.com
// and stores the resulting access_token in localStorage (not as a cookie). The
// API at fantasy.premierleague.com accepts the token via
// `x-api-authorization: Bearer <jwt>` for read AND write endpoints.
//
// The input may be either:
//   - A raw JWT string (the long eyJ... dot-separated value copied from
//     localStorage), OR
//   - The full oidc.user JSON object from localStorage (which contains
//     `access_token` as a nested field). When the input starts with `{`,
//     this function extracts the access_token automatically.
//
// Extract the token from your browser:
//  1. Log into fantasy.premierleague.com in Chrome/Firefox.
//  2. DevTools → Application → Local Storage → https://fantasy.premierleague.com
//  3. Find a key whose value is JSON containing "access_token". The typical
//     name is `oidc.user:https://account.premierleague.com/as:<client_id>`.
//  4. Pass either the raw JWT value OR the full JSON via --bearer or --bearer-file.
//
// Returns an error if the token is empty or if JSON input lacks access_token.
func (c *AuthClient) LoadBearer(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("bearer token is empty")
	}

	// If the input looks like JSON (oidc.user shape), extract access_token.
	if strings.HasPrefix(token, "{") {
		var envelope struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal([]byte(token), &envelope); err != nil {
			return fmt.Errorf("bearer JSON parse: %w", err)
		}
		if envelope.AccessToken == "" {
			return errors.New("bearer JSON has no access_token field")
		}
		token = envelope.AccessToken
	}

	c.bearer = token
	return nil
}

// HasBearer reports whether a bearer token has been loaded.
func (c *AuthClient) HasBearer() bool { return c.bearer != "" }

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
	if c.bearer != "" {
		req.Header.Set("x-api-authorization", "Bearer "+c.bearer)
	}

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
	if c.bearer != "" {
		req.Header.Set("x-api-authorization", "Bearer "+c.bearer)
	}

	return c.http.Do(req)
}
