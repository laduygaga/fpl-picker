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

func TestRenderDiffIncludesKeySections(t *testing.T) {
	current := &api.MyTeam{
		Picks: []api.Pick{
			{Element: 1, Position: 1, ElementType: model.PosGK, IsCaptain: true, SellingPrice: 50},
			{Element: 2, Position: 2, ElementType: model.PosDEF, IsViceCaptain: true, SellingPrice: 60},
		},
		Transfers: api.TransferStatus{Bank: 10},
	}
	plan := LineupPlan{
		Picks: []api.Pick{
			{Element: 2, Position: 1, ElementType: model.PosDEF, IsCaptain: true},
			{Element: 1, Position: 2, ElementType: model.PosGK, IsViceCaptain: true},
		},
		CaptainID:     2,
		ViceCaptainID: 1,
	}
	transfers := []TransferSuggestion{
		{
			Out:           model.ScoredPlayer{Player: api.Player{ID: 9, WebName: "Out", ElementType: model.PosFWD, NowCost: 50, Team: 1}, Score: 0.5},
			In:            model.ScoredPlayer{Player: api.Player{ID: 99, WebName: "In", ElementType: model.PosFWD, NowCost: 70, Team: 2}, Score: 0.9},
			SellingPrice:  55,
			PurchasePrice: 70,
			ScoreUplift:   0.4,
		},
	}

	diff := RenderDiff(current, plan, transfers, 4)
	for _, want := range []string{"Planned changes", "Lineup", "Transfers", "Score", "Net cost", "Points hits: 4"} {
		if !strings.Contains(diff, want) {
			t.Errorf("RenderDiff missing %q. Got:\n%s", want, diff)
		}
	}
}

func TestRenderDiffNoTransfers(t *testing.T) {
	current := &api.MyTeam{Picks: []api.Pick{
		{Element: 1, Position: 1, ElementType: model.PosGK},
	}}
	plan := LineupPlan{
		Picks: []api.Pick{
			{Element: 1, Position: 1, ElementType: model.PosGK, IsCaptain: true},
		},
		CaptainID: 1,
	}
	diff := RenderDiff(current, plan, nil, 0)
	if !strings.Contains(diff, "(none)") {
		t.Errorf("empty transfers should print '(none)'. Got:\n%s", diff)
	}
	if !strings.Contains(diff, "Points hits: 0") {
		t.Errorf("zero hits should appear. Got:\n%s", diff)
	}
}

func TestSortSuggestions(t *testing.T) {
	s := []TransferSuggestion{
		{In: model.ScoredPlayer{Player: api.Player{ID: 1}}, ScoreUplift: 0.1},
		{In: model.ScoredPlayer{Player: api.Player{ID: 2}}, ScoreUplift: 0.5},
		{In: model.ScoredPlayer{Player: api.Player{ID: 3}}, ScoreUplift: 0.3},
	}
	SortSuggestions(s)
	if s[0].In.Player.ID != 2 || s[1].In.Player.ID != 3 || s[2].In.Player.ID != 1 {
		t.Errorf("SortSuggestions did not order by uplift desc: %+v", s)
	}
}

func TestFormatPlayerWithEmptyWebName(t *testing.T) {
	p := api.Player{ID: 42, ElementType: model.PosMID, NowCost: 70}
	got := formatPlayer(p)
	if !strings.Contains(got, "#42") {
		t.Errorf("formatPlayer with empty WebName should fall back to ID, got %q", got)
	}
	if !strings.Contains(got, "£7.0M") {
		t.Errorf("formatPlayer should render cost, got %q", got)
	}
}

func TestRunRejectsNilClient(t *testing.T) {
	_, err := Run(context.Background(), nil, &api.MyTeam{Transfers: api.TransferStatus{Entry: 1}}, model.SquadResult{}, Options{})
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestRunRejectsNilCurrent(t *testing.T) {
	client := api.NewAuthClient(context.Background())
	_, err := Run(context.Background(), client, nil, model.SquadResult{}, Options{})
	if err == nil {
		t.Fatal("expected error for nil current team")
	}
}

func TestRunRejectsZeroEntry(t *testing.T) {
	client := api.NewAuthClient(context.Background())
	_, err := Run(context.Background(), client, &api.MyTeam{}, model.SquadResult{}, Options{})
	if err == nil {
		t.Fatal("expected error for zero entry id")
	}
}

// TestRunEndToEndDryRun exercises Run with a mock FPL server and verifies
// the pipeline populates Result without performing writes when Apply=false.
func TestRunEndToEndDryRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/me/":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"player":{"entry":99,"id":7,"name":"U","email":"u@e.com"}}`))
		case strings.HasPrefix(r.URL.Path, "/api/my-team/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"picks":[
				{"element":1,"position":1,"multiplier":1,"is_captain":true,"is_vice_captain":false,"selling_price":50,"purchase_price":50,"can_sub":false,"has_played":false,"is_sub":false,"element_type":1},
				{"element":2,"position":2,"multiplier":1,"is_captain":false,"is_vice_captain":true,"selling_price":50,"purchase_price":50,"can_sub":false,"has_played":false,"is_sub":false,"element_type":2}
			],"chips":[],"transfers":{"bank":100,"value":100,"limit":1,"made":0,"entry":99}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client := api.NewAuthClient(context.Background())
	parsed, _ := url.Parse(ts.URL)
	client.SetBaseURL(ts.URL)
	client.SetTransport(&hostRewrite{host: parsed.Host})

	current, err := client.GetMyTeam(99)
	if err != nil {
		t.Fatalf("GetMyTeam: %v", err)
	}

	res, err := Run(context.Background(), client, current, model.SquadResult{}, Options{
		Apply:         false,
		SkipTransfers: true,
		SkipLineup:    true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Bank != 100 || res.SquadValue != 100 {
		t.Errorf("Bank/SquadValue = %d/%d, want 100/100", res.Bank, res.SquadValue)
	}
	if res.TransfersMade != 0 {
		t.Errorf("dry-run TransfersMade = %d, want 0", res.TransfersMade)
	}
}
