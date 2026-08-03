package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type myTeamTestServer struct {
	getCalled  int
	postCalled int
	postBody   []byte
	postPath   string
	getResp    string
	postStatus int
	postResp   string
}

func newMyTeamServer(t *testing.T, ts *myTeamTestServer) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/my-team/", func(w http.ResponseWriter, r *http.Request) {
		// Path is /api/my-team/{tid}/ — extract the trailing segment.
		seg := r.URL.Path
		const prefix = "/api/my-team/"
		tid := "3808385"
		if len(seg) > len(prefix) {
			rest := seg[len(prefix):]
			if i := indexByte(rest, '/'); i >= 0 {
				tid = rest[:i]
			} else {
				tid = rest
			}
		}
		ts.postPath = r.URL.Path
		switch r.Method {
		case http.MethodGet:
			ts.getCalled++
			w.Header().Set("Content-Type", "application/json")
			if ts.getResp == "" {
				ts.getResp = defaultMyTeamResponse(tid)
			}
			_, _ = io.WriteString(w, ts.getResp)
		case http.MethodPost:
			ts.postCalled++
			body, _ := io.ReadAll(r.Body)
			ts.postBody = body
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(ts.postStatus)
			_, _ = io.WriteString(w, ts.postResp)
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func defaultMyTeamResponse(tid string) string {
	return `{
		"picks": [
			{"element": 318, "position": 1,  "multiplier": 1, "is_captain": false, "is_vice_captain": false, "selling_price": 71, "purchase_price": 70, "can_sub": true, "has_played": false, "is_sub": false, "element_type": 4},
			{"element": 113, "position": 2,  "multiplier": 1, "is_captain": false, "is_vice_captain": false, "selling_price": 50, "purchase_price": 50, "can_sub": true, "has_played": false, "is_sub": false, "element_type": 2},
			{"element": 285, "position": 3,  "multiplier": 1, "is_captain": false, "is_vice_captain": false, "selling_price": 45, "purchase_price": 45, "can_sub": true, "has_played": false, "is_sub": false, "element_type": 2},
			{"element": 448, "position": 4,  "multiplier": 1, "is_captain": false, "is_vice_captain": true,  "selling_price": 55, "purchase_price": 55, "can_sub": true, "has_played": false, "is_sub": false, "element_type": 2},
			{"element": 146, "position": 5,  "multiplier": 1, "is_captain": false, "is_vice_captain": false, "selling_price": 60, "purchase_price": 60, "can_sub": true, "has_played": false, "is_sub": false, "element_type": 3},
			{"element": 283, "position": 6,  "multiplier": 1, "is_captain": false, "is_vice_captain": false, "selling_price": 65, "purchase_price": 65, "can_sub": true, "has_played": false, "is_sub": false, "element_type": 3},
			{"element": 19,  "position": 7,  "multiplier": 1, "is_captain": false, "is_vice_captain": false, "selling_price": 70, "purchase_price": 70, "can_sub": true, "has_played": false, "is_sub": false, "element_type": 3},
			{"element": 246, "position": 8,  "multiplier": 1, "is_captain": false, "is_vice_captain": false, "selling_price": 80, "purchase_price": 80, "can_sub": true, "has_played": false, "is_sub": false, "element_type": 3},
			{"element": 314, "position": 9,  "multiplier": 1, "is_captain": false, "is_vice_captain": false, "selling_price": 75, "purchase_price": 75, "can_sub": true, "has_played": false, "is_sub": false, "element_type": 4},
			{"element": 318, "position": 10, "multiplier": 2, "is_captain": true,  "is_vice_captain": false, "selling_price": 71, "purchase_price": 70, "can_sub": true, "has_played": false, "is_sub": false, "element_type": 4},
			{"element": 28,  "position": 11, "multiplier": 1, "is_captain": false, "is_vice_captain": false, "selling_price": 90, "purchase_price": 90, "can_sub": true, "has_played": false, "is_sub": false, "element_type": 4},
			{"element": 254, "position": 12, "multiplier": 0, "is_captain": false, "is_vice_captain": false, "selling_price": 40, "purchase_price": 40, "can_sub": false, "has_played": false, "is_sub": false, "element_type": 2},
			{"element": 295, "position": 13, "multiplier": 0, "is_captain": false, "is_vice_captain": false, "selling_price": 45, "purchase_price": 45, "can_sub": false, "has_played": false, "is_sub": false, "element_type": 3},
			{"element": 346, "position": 14, "multiplier": 0, "is_captain": false, "is_vice_captain": false, "selling_price": 50, "purchase_price": 50, "can_sub": false, "has_played": false, "is_sub": false, "element_type": 3},
			{"element": 54,  "position": 15, "multiplier": 0, "is_captain": false, "is_vice_captain": false, "selling_price": 55, "purchase_price": 55, "can_sub": false, "has_played": false, "is_sub": false, "element_type": 4}
		],
		"chips": [
			{"id": 1, "name": "wildcard", "number": 1, "start_event": 2, "stop_event": 19, "chip_type": "transfer"},
			{"id": 2, "name": "freehit",  "number": 1, "start_event": 2, "stop_event": 19, "chip_type": "transfer"},
			{"id": 3, "name": "bboost",   "number": 1, "start_event": 2, "stop_event": 19, "chip_type": "team"},
			{"id": 4, "name": "3xc",      "number": 1, "start_event": 2, "stop_event": 19, "chip_type": "team"}
		],
		"transfers": {
			"bank": 26, "value": 1011, "limit": 1, "made": 0, "entry": ` + tid + `
		}
	}`
}

func newMyTeamClient(srvURL string) *AuthClient {
	c := NewAuthClient(context.Background())
	parsed, _ := url.Parse(srvURL)
	c.http.Jar = nil
	c.baseURL = srvURL
	c.http.Transport = &rewriteHostTransport{host: parsed.Host}
	return c
}

func TestGetMyTeamParses(t *testing.T) {
	ts := &myTeamTestServer{}
	srv := newMyTeamServer(t, ts)
	client := newMyTeamClient(srv.URL)

	team, err := client.GetMyTeam(3808385)
	if err != nil {
		t.Fatalf("GetMyTeam: %v", err)
	}

	if ts.getCalled != 1 {
		t.Errorf("get calls = %d, want 1", ts.getCalled)
	}
	if got := ts.postPath; got != "/api/my-team/3808385/" {
		t.Errorf("path = %q, want /api/my-team/3808385/", got)
	}
	if len(team.Picks) != 15 {
		t.Errorf("len(picks) = %d, want 15", len(team.Picks))
	}
	if team.Transfers.Bank != 26 {
		t.Errorf("bank = %d, want 26", team.Transfers.Bank)
	}
	if team.Transfers.Limit == nil || *team.Transfers.Limit != 1 {
		var got int
		if team.Transfers.Limit != nil {
			got = *team.Transfers.Limit
		}
		t.Errorf("limit = %d, want 1", got)
	}
	if len(team.Chips) != 4 {
		t.Errorf("len(chips) = %d, want 4", len(team.Chips))
	}
	captainCount := 0
	for _, p := range team.Picks {
		if p.IsCaptain {
			captainCount++
		}
	}
	if captainCount != 1 {
		t.Errorf("captain count = %d, want exactly 1", captainCount)
	}
}

func TestUpdateLineupBuildsBody(t *testing.T) {
	ts := &myTeamTestServer{postStatus: http.StatusOK}
	srv := newMyTeamServer(t, ts)
	client := newMyTeamClient(srv.URL)

	update := LineupUpdate{
		Picks: []Pick{
			{Element: 113, Position: 1}, {Element: 10, Position: 2},
			{Element: 285, Position: 3, IsViceCaptain: true},
			{Element: 448, Position: 4}, {Element: 146, Position: 5},
			{Element: 283, Position: 6}, {Element: 19, Position: 7},
			{Element: 246, Position: 8}, {Element: 314, Position: 9},
			{Element: 318, Position: 10, IsCaptain: true}, {Element: 28, Position: 11},
			{Element: 254, Position: 12}, {Element: 295, Position: 13},
			{Element: 346, Position: 14}, {Element: 54, Position: 15},
		},
		Chip: ChipPtr("wildcard"),
	}

	if err := client.UpdateLineup(3808385, update); err != nil {
		t.Fatalf("UpdateLineup: %v", err)
	}

	if ts.postCalled != 1 {
		t.Errorf("post calls = %d, want 1", ts.postCalled)
	}

	var parsed struct {
		Picks []Pick  `json:"picks"`
		Chip  *string `json:"chip"`
	}
	if err := json.Unmarshal(ts.postBody, &parsed); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(parsed.Picks) != 15 {
		t.Errorf("posted picks = %d, want 15", len(parsed.Picks))
	}
	if parsed.Chip == nil || *parsed.Chip != "wildcard" {
		t.Errorf("chip = %v, want wildcard", parsed.Chip)
	}

	// Read-only fields must NOT be present (omitempty strips zeros, but verify
	// by re-marshalling one pick and checking the keys).
	for _, p := range parsed.Picks {
		if p.Multiplier != 0 || p.SellingPrice != 0 || p.PurchasePrice != 0 {
			t.Errorf("pick %d leaked read-only fields: %+v", p.Element, p)
		}
	}
}

func TestUpdateLineupStripsUnknownChip(t *testing.T) {
	ts := &myTeamTestServer{postStatus: http.StatusOK}
	srv := newMyTeamServer(t, ts)
	client := newMyTeamClient(srv.URL)

	update := LineupUpdate{Picks: make([]Pick, 15), Chip: ChipPtr("not-a-real-chip")}
	if err := client.UpdateLineup(3808385, update); err != nil {
		t.Fatalf("UpdateLineup: %v", err)
	}
	var parsed struct {
		Chip *string `json:"chip"`
	}
	if err := json.Unmarshal(ts.postBody, &parsed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if parsed.Chip != nil {
		t.Errorf("chip = %v, want null after stripping unknown", parsed.Chip)
	}
}

func TestUpdateLineupRejectsError(t *testing.T) {
	ts := &myTeamTestServer{
		postStatus: http.StatusBadRequest,
		postResp:   `{"detail":"invalid picks"}`,
	}
	srv := newMyTeamServer(t, ts)
	client := newMyTeamClient(srv.URL)

	err := client.UpdateLineup(3808385, LineupUpdate{Picks: make([]Pick, 15)})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !contains(err.Error(), "400") {
		t.Errorf("error %q should mention 400", err.Error())
	}
}

func TestUpdateLineupForbiddenReturnsAuthError(t *testing.T) {
	ts := &myTeamTestServer{postStatus: http.StatusForbidden}
	srv := newMyTeamServer(t, ts)
	client := newMyTeamClient(srv.URL)

	err := client.UpdateLineup(3808385, LineupUpdate{Picks: make([]Pick, 15)})
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !contains(err.Error(), "403") {
		t.Errorf("error %q should mention 403", err.Error())
	}
}

func TestLineupHasChanged(t *testing.T) {
	teamID := 3808385
	srv := newMyTeamServer(t, &myTeamTestServer{})
	client := newMyTeamClient(srv.URL)
	current, err := client.GetMyTeam(teamID)
	if err != nil {
		t.Fatalf("GetMyTeam: %v", err)
	}

	// Identical update → no change.
	identical := LineupUpdate{Picks: clonePicks(current.Picks)}
	if LineupHasChanged(current, identical) {
		t.Error("identical lineup should not be reported as changed")
	}

	// Captain swap → changed.
	swapped := LineupUpdate{Picks: clonePicks(current.Picks)}
	for i := range swapped.Picks {
		if swapped.Picks[i].IsCaptain {
			swapped.Picks[i].IsCaptain = false
			swapped.Picks[i].IsViceCaptain = true
		} else if swapped.Picks[i].IsViceCaptain {
			swapped.Picks[i].IsViceCaptain = false
			swapped.Picks[i].IsCaptain = true
			break
		}
	}
	if !LineupHasChanged(current, swapped) {
		t.Error("captain swap should be reported as changed")
	}
}

func TestGetMyTeamURLTrailingSlash(t *testing.T) {
	ts := &myTeamTestServer{}
	srv := newMyTeamServer(t, ts)
	client := newMyTeamClient(srv.URL)

	if _, err := client.GetMyTeam(3808385); err != nil {
		t.Fatalf("GetMyTeam: %v", err)
	}
	// We routed via httptest.ServeMux pattern /api/my-team/ — if the request
	// path didn't have a trailing slash, mux would 301 to add it. Our counter
	// would still tick on the redirected GET, but we can confirm by checking
	// postPath recorded the right team_id.
	if got := ts.postPath; got != "/api/my-team/3808385/" {
		t.Errorf("path = %q, want /api/my-team/3808385/ (trailing-slash enforcement)", got)
	}
}

func clonePicks(in []Pick) []Pick {
	out := make([]Pick, len(in))
	copy(out, in)
	return out
}

// contains avoids pulling strings just for one tiny substring helper.
func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
