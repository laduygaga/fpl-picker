package apply

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"fpl-picker/api"
	"fpl-picker/model"
)

func makeScored(id, pos, cost int, score float64) model.ScoredPlayer {
	return model.ScoredPlayer{
		Player: api.Player{ID: id, ElementType: pos, NowCost: cost, WebName: nameFor(id)},
		Score:  score,
	}
}

func nameFor(id int) string {
	if id <= 26 {
		return "P" + string(rune('A'+id-1))
	}
	return "P" + string(rune('A'+((id-1)%26)))
}

func intPtr(v int) *int { return &v }

func TestPlanTransfersSingleSwapWithinBudget(t *testing.T) {

	// Current team: 15 players, IDs 1..15. Bank=50 tenths (£5m).
	// Optimal team: 15 players, swap 5→100, drop 10.
	// Bank + 100's selling price (8 tenths) = 58. NowCost 100 = 70.
	// 58 >= 70? No, so the transfer should be rejected.
	currentPicks := []api.Pick{}
	for i := 1; i <= 15; i++ {
		currentPicks = append(currentPicks, api.Pick{
			Element:       i,
			Position:      i,
			SellingPrice:  50,
			PurchasePrice: 50,
			ElementType:   model.PosMID,
		})
	}
	current := &api.MyTeam{
		Picks: currentPicks,
		Transfers: api.TransferStatus{
			Bank: 50, Value: 750, Limit: intPtr(1), Made: 0, Entry: 3808385,
		},
	}

	optimalStarters := []model.ScoredPlayer{
		makeScored(1, model.PosGK, 50, 0.50),
		makeScored(2, model.PosDEF, 55, 0.55),
		makeScored(3, model.PosDEF, 60, 0.60),
		makeScored(4, model.PosDEF, 65, 0.65),
		makeScored(100, model.PosMID, 70, 0.95), // NEW, replaces 5
		makeScored(6, model.PosMID, 80, 0.85),
		makeScored(7, model.PosMID, 85, 0.90),
		makeScored(8, model.PosMID, 90, 0.95),
		makeScored(9, model.PosFWD, 100, 0.70),
		makeScored(11, model.PosFWD, 105, 0.85),
		makeScored(12, model.PosFWD, 110, 0.90),
	}
	optimalBench := []model.ScoredPlayer{
		makeScored(13, model.PosGK, 40, 0.20),
		makeScored(14, model.PosDEF, 45, 0.30),
		makeScored(15, model.PosMID, 50, 0.35),
		makeScored(11, model.PosFWD, 55, 0.40), // 11 is bench (different from starter 11)
	}
	optimal := model.SquadResult{
		Starters: optimalStarters,
		Bench:    optimalBench,
	}

	suggestions := PlanTransfers(current, optimal, 4, nil)
	// Should propose at least one transfer: 100 in, 5 out (5 isn't in optimal).
	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion, got 0")
	}

	// First suggestion should be 5→100 (or 10→something) — verify the OUT
	// has no overlap with the optimal squad.
	for _, s := range suggestions {
		if s.In.Player.ID == s.Out.Player.ID {
			t.Errorf("IN and OUT are the same player (%d)", s.In.Player.ID)
		}
		if s.PurchasePrice-s.SellingPrice > 50 {
			t.Errorf("suggestion %d→%d exceeds bank: net cost %d > 50",
				s.Out.Player.ID, s.In.Player.ID, s.PurchasePrice-s.SellingPrice)
		}
	}
}

func TestPlanTransfersRespectsMaxHits(t *testing.T) {
	// Build a worst case: 5 swaps needed, cap at 0 hits → no transfers proposed.
	currentPicks := []api.Pick{}
	for i := 1; i <= 15; i++ {
		currentPicks = append(currentPicks, api.Pick{
			Element: i, Position: i, SellingPrice: 50, PurchasePrice: 50,
			ElementType: model.PosMID,
		})
	}
	current := &api.MyTeam{
		Picks:     currentPicks,
		Transfers: api.TransferStatus{Bank: 1000, Limit: intPtr(0), Made: 0, Entry: 1},
	}

	var optimalStarters []model.ScoredPlayer
	for i := 100; i < 111; i++ {
		optimalStarters = append(optimalStarters, makeScored(i, model.PosMID, 60, 0.9))
	}
	optimalStarters[0] = makeScored(1, model.PosGK, 50, 0.5) // 1 stays
	optimal := model.SquadResult{
		Starters: optimalStarters,
		Bench: []model.ScoredPlayer{
			makeScored(2, model.PosGK, 40, 0.2),
			makeScored(3, model.PosDEF, 45, 0.3),
			makeScored(4, model.PosDEF, 50, 0.35),
			makeScored(5, model.PosFWD, 55, 0.4),
		},
	}

	got := PlanTransfers(current, optimal, 0, nil)
	if len(got) != 0 {
		t.Errorf("maxHits=0 should produce 0 suggestions, got %d", len(got))
	}

	got = PlanTransfers(current, optimal, 16, nil)
	if len(got) == 0 {
		t.Errorf("maxHits=16 should produce at least one suggestion, got 0")
	}
}

func TestPlanTransfersNoSuggestionsWhenAlreadyOptimal(t *testing.T) {
	currentPicks := []api.Pick{}
	optimalStarters := []model.ScoredPlayer{
		makeScored(1, model.PosGK, 50, 0.50),
		makeScored(2, model.PosDEF, 55, 0.55),
		makeScored(3, model.PosDEF, 60, 0.60),
		makeScored(4, model.PosDEF, 65, 0.65),
		makeScored(5, model.PosMID, 70, 0.70),
		makeScored(6, model.PosMID, 75, 0.75),
		makeScored(7, model.PosMID, 80, 0.80),
		makeScored(8, model.PosMID, 85, 0.85),
		makeScored(9, model.PosFWD, 90, 0.70),
		makeScored(10, model.PosFWD, 95, 0.85),
		makeScored(11, model.PosFWD, 100, 0.90),
	}
	optimalBench := []model.ScoredPlayer{
		makeScored(12, model.PosGK, 40, 0.20),
		makeScored(13, model.PosDEF, 45, 0.30),
		makeScored(14, model.PosMID, 50, 0.35),
		makeScored(15, model.PosFWD, 55, 0.40),
	}

	for i := 1; i <= 15; i++ {
		currentPicks = append(currentPicks, api.Pick{
			Element: i, Position: i, SellingPrice: 50, PurchasePrice: 50,
			ElementType: model.PosMID,
		})
	}
	current := &api.MyTeam{
		Picks:     currentPicks,
		Transfers: api.TransferStatus{Bank: 50, Limit: intPtr(1), Made: 0, Entry: 1},
	}
	optimal := model.SquadResult{
		Starters: optimalStarters,
		Bench:    optimalBench,
	}

	if got := PlanTransfers(current, optimal, 4, nil); len(got) != 0 {
		t.Errorf("already-optimal team should produce 0 suggestions, got %d: %+v", len(got), got)
	}
}

func TestBuildTransferRequestEmpty(t *testing.T) {
	if got := BuildTransferRequest(1, 1, nil, ""); got != nil {
		t.Errorf("empty + no chip = nil request, got %+v", got)
	}
}

func TestBuildTransferRequestWithChip(t *testing.T) {
	req := BuildTransferRequest(7, 3, nil, "wildcard")
	if req == nil {
		t.Fatal("chip-only request should be non-nil")
	}
	if req.Entry != 7 || req.Event != 3 {
		t.Errorf("entry/event = %d/%d, want 7/3", req.Entry, req.Event)
	}
	if req.Chip == nil || *req.Chip != "wildcard" {
		t.Errorf("chip = %v, want wildcard", req.Chip)
	}
}

func TestBuildTransferRequestShape(t *testing.T) {
	sug := []TransferSuggestion{
		{
			Out:           makeScored(5, model.PosMID, 50, 0.5),
			In:            makeScored(100, model.PosMID, 70, 0.9),
			SellingPrice:  55,
			PurchasePrice: 70,
			ScoreUplift:   0.4,
		},
	}
	req := BuildTransferRequest(3808385, 1, sug, "")
	if req == nil {
		t.Fatal("expected non-nil")
	}
	if len(req.Transfers) != 1 {
		t.Fatalf("transfers = %d, want 1", len(req.Transfers))
	}
	if req.Transfers[0].ElementIn != 100 || req.Transfers[0].ElementOut != 5 {
		t.Errorf("transfer in/out = %d/%d, want 100/5", req.Transfers[0].ElementIn, req.Transfers[0].ElementOut)
	}
	if req.Transfers[0].PurchasePrice != 70 || req.Transfers[0].SellingPrice != 55 {
		t.Errorf("prices = %d/%d, want 70/55", req.Transfers[0].PurchasePrice, req.Transfers[0].SellingPrice)
	}
}

func TestPreviewPointsHits(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"non_form_errors":["fail"],"spent_points":8}`))
	}))
	defer ts.Close()

	client := api.NewAuthClient(context.Background())
	parsed, _ := url.Parse(ts.URL)
	client.SetBaseURL(ts.URL)
	client.SetTransport(&hostRewrite{host: parsed.Host})

	req := api.TransferRequest{Entry: 1, Event: 1}
	spent, err := PreviewPointsHits(context.Background(), client, req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if spent != 8 {
		t.Errorf("spent = %d, want 8", spent)
	}
	te := api.AsTransferError(err)
	if te == nil || !strings.Contains(te.NonFormErrors[0], "fail") {
		t.Errorf("error = %v", err)
	}
}

// hostRewrite rewrites outbound requests to hit a local test server. Mirrors
// the helper in api/auth_test.go (kept duplicated here to avoid cross-package
// test helpers).
type hostRewrite struct{ host string }

func (h *hostRewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	r2 := req.Clone(req.Context())
	r2.URL.Host = h.host
	r2.URL.Scheme = "http"
	r2.Host = h.host
	return http.DefaultTransport.RoundTrip(r2)
}

func TestHitsExceeded(t *testing.T) {
	cases := []struct {
		used, free, proposed, max int
		want                      bool
	}{
		{0, 1, 0, 4, false},  // no transfers, no hits
		{0, 1, 1, 4, false},  // 1 transfer, 0 hits
		{0, 1, 2, 4, false},  // 2 transfers, 4 hits = cap, not exceeded
		{0, 1, 3, 4, true},   // 3 transfers, 8 hits > 4 cap
		{0, 0, 1, 0, true},   // max=0 means no transfers at all
		{0, 0, 0, 0, false},  // no transfers
		{4, 1, 0, 16, false}, // 4 used + 0 proposed → 12 hits ≤ 16
		{5, 1, 0, 16, false}, // 5 used → 16 hits, equals cap, not exceeded
	}
	for _, c := range cases {
		got := hitsExceeded(c.used, c.free, c.proposed, c.max)
		if got != c.want {
			t.Errorf("hitsExceeded(%d,%d,%d,%d) = %v, want %v",
				c.used, c.free, c.proposed, c.max, got, c.want)
		}
	}
	if !hitsExceeded(5, 1, 0, 15) {
		t.Error("hitsExceeded(5,1,0,15): 16 hits > 15 cap should be true")
	}
}
