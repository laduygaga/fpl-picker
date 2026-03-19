package model

import (
	"math"
	"testing"

	"fpl-picker/api"
)

func intPtr(v int) *int { return &v }

func makePlayer(id, team, elementType, nowCost, minutes int, opts map[string]string) api.Player {
	p := api.Player{
		ID:          id,
		WebName:     opts["name"],
		Team:        team,
		ElementType: elementType,
		NowCost:     nowCost,
		Minutes:     minutes,
		Status:      "a",
	}
	if v, ok := opts["form"]; ok {
		p.Form = v
	}
	if v, ok := opts["ep_next"]; ok {
		p.EPNext = v
	}
	if v, ok := opts["ppg"]; ok {
		p.PointsPerGame = v
	}
	if v, ok := opts["xgi"]; ok {
		p.ExpectedGoalInvolvements = v
	}
	if v, ok := opts["ict"]; ok {
		p.ICTIndex = v
	}
	if v, ok := opts["xg"]; ok {
		p.ExpectedGoals = v
	}
	if v, ok := opts["xa"]; ok {
		p.ExpectedAssists = v
	}
	if v, ok := opts["xgc"]; ok {
		p.ExpectedGoalsConceded = v
	}
	if v, ok := opts["status"]; ok {
		p.Status = v
	}
	if v, ok := opts["selected"]; ok {
		p.SelectedByPercent = v
	}
	return p
}

func testFixture(event, teamH, teamA int) api.Fixture {
	e := event
	return api.Fixture{
		ID:    teamH*100 + teamA,
		Event: &e,
		TeamH: teamH,
		TeamA: teamA,
	}
}

func testTeam(id int, shortName string) api.Team {
	return api.Team{ID: id, ShortName: shortName, Name: shortName}
}

func TestIsEligible(t *testing.T) {
	tests := []struct {
		name     string
		player   api.Player
		eligible bool
	}{
		{"available with minutes", api.Player{Status: "a", Minutes: 200}, true},
		{"injured", api.Player{Status: "i", Minutes: 200}, false},
		{"suspended", api.Player{Status: "s", Minutes: 200}, false},
		{"unavailable", api.Player{Status: "u", Minutes: 200}, false},
		{"zero minutes", api.Player{Status: "a", Minutes: 0}, false},
		{"under 90 minutes", api.Player{Status: "a", Minutes: 45}, false},
		{"doubtful 75% chance", api.Player{Status: "d", Minutes: 200, ChanceOfPlayingNextRound: intPtr(75)}, true},
		{"doubtful 25% chance", api.Player{Status: "d", Minutes: 200, ChanceOfPlayingNextRound: intPtr(25)}, false},
		{"doubtful nil chance", api.Player{Status: "d", Minutes: 200}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEligible(tt.player)
			if got != tt.eligible {
				t.Errorf("isEligible() = %v, want %v", got, tt.eligible)
			}
		})
	}
}

func TestNormalizer(t *testing.T) {
	n := newNormalizer([]float64{0, 5, 10})
	if got := n.normalize(0); got != 0 {
		t.Errorf("normalize(0) = %f, want 0", got)
	}
	if got := n.normalize(10); got != 1 {
		t.Errorf("normalize(10) = %f, want 1", got)
	}
	if got := n.normalize(5); math.Abs(got-0.5) > 0.001 {
		t.Errorf("normalize(5) = %f, want 0.5", got)
	}
	if got := n.normalize(-1); got != 0 {
		t.Errorf("normalize(-1) = %f, want 0 (clamped)", got)
	}
	if got := n.normalize(15); got != 1 {
		t.Errorf("normalize(15) = %f, want 1 (clamped)", got)
	}

	flat := newNormalizer([]float64{3, 3, 3})
	if got := flat.normalize(3); got != 0 {
		t.Errorf("flat normalizer should return 0, got %f", got)
	}

	empty := newNormalizer([]float64{})
	if got := empty.normalize(5); got != 1 {
		t.Errorf("empty normalizer with default {0,1} should clamp 5 to 1, got %f", got)
	}
}

func TestPosName(t *testing.T) {
	cases := map[int]string{1: "GK", 2: "DEF", 3: "MID", 4: "FWD", 99: "???"}
	for et, want := range cases {
		if got := PosName(et); got != want {
			t.Errorf("PosName(%d) = %q, want %q", et, got, want)
		}
	}
}

func TestScorerNextEventID(t *testing.T) {
	events := []api.Event{
		{ID: 28, IsNext: false},
		{ID: 29, IsNext: false},
		{ID: 30, IsNext: true},
	}
	teams := []api.Team{testTeam(1, "ARS")}
	s := NewScorer(teams, nil, events, nil)
	if s.NextEventID() != 30 {
		t.Errorf("NextEventID() = %d, want 30", s.NextEventID())
	}
}

func TestScorerNoFixtureZeroScore(t *testing.T) {
	teams := []api.Team{testTeam(1, "ARS"), testTeam(2, "CHE")}
	events := []api.Event{{ID: 30, IsNext: true}}
	players := []api.Player{
		makePlayer(1, 1, PosFWD, 100, 900, map[string]string{
			"name": "Player1", "form": "5.0", "ep_next": "5.0", "ppg": "5.0",
			"xgi": "10.0", "ict": "100.0", "xg": "8.0",
		}),
	}
	gk := makePlayer(99, 1, PosGK, 40, 900, map[string]string{
		"name": "GK1", "xg": "0.5", "xgc": "10.0",
	})
	allPlayers := append(players, gk)

	s := NewScorer(teams, nil, events, allPlayers)
	scored := s.ScoreAll(players)

	if len(scored) != 1 {
		t.Fatalf("expected 1 scored player, got %d", len(scored))
	}
	if scored[0].Score != 0 {
		t.Errorf("player with no fixture should have score 0, got %f", scored[0].Score)
	}
	if scored[0].HasFixture {
		t.Error("HasFixture should be false")
	}
}

func TestScorerHomeAdvantage(t *testing.T) {
	teams := []api.Team{testTeam(1, "ARS"), testTeam(2, "CHE")}
	events := []api.Event{{ID: 30, IsNext: true}}
	fix := []api.Fixture{testFixture(30, 1, 2)}

	baseOpts := map[string]string{
		"form": "5.0", "ep_next": "5.0", "ppg": "5.0",
		"xgi": "10.0", "ict": "100.0", "xg": "8.0", "xgc": "10.0",
	}

	homePlayer := makePlayer(1, 1, PosFWD, 100, 900, mergeMaps(baseOpts, map[string]string{"name": "HomeGuy"}))
	awayPlayer := makePlayer(2, 2, PosFWD, 100, 900, mergeMaps(baseOpts, map[string]string{"name": "AwayGuy"}))

	gk1 := makePlayer(91, 1, PosGK, 40, 900, map[string]string{"name": "GK1", "xg": "0.5", "xgc": "10.0"})
	gk2 := makePlayer(92, 2, PosGK, 40, 900, map[string]string{"name": "GK2", "xg": "0.5", "xgc": "10.0"})

	allPlayers := []api.Player{homePlayer, awayPlayer, gk1, gk2}
	s := NewScorer(teams, fix, events, allPlayers)
	scored := s.ScoreAll([]api.Player{homePlayer, awayPlayer})

	if len(scored) != 2 {
		t.Fatalf("expected 2 scored players, got %d", len(scored))
	}

	var homeScore, awayScore float64
	for _, sp := range scored {
		if sp.Player.ID == 1 {
			homeScore = sp.Score
		} else {
			awayScore = sp.Score
		}
	}
	if homeScore <= awayScore {
		t.Errorf("home player (%.4f) should score higher than away player (%.4f) with identical stats", homeScore, awayScore)
	}
	if math.Abs(homeScore-awayScore-0.02) > 0.001 {
		t.Errorf("home advantage should be exactly 0.02, got diff %f", homeScore-awayScore)
	}
}

func TestFindPlayersByName(t *testing.T) {
	scored := []ScoredPlayer{
		{Player: api.Player{WebName: "Salah"}},
		{Player: api.Player{WebName: "Haaland"}},
		{Player: api.Player{WebName: "B.Fernandes"}},
		{Player: api.Player{WebName: "J.Timber"}},
	}

	t.Run("exact match", func(t *testing.T) {
		found := FindPlayersByName(scored, []string{"Salah"})
		if len(found) != 1 || found[0].Player.WebName != "Salah" {
			t.Errorf("expected Salah, got %v", found)
		}
	})

	t.Run("fuzzy match substring", func(t *testing.T) {
		found := FindPlayersByName(scored, []string{"Timber"})
		if len(found) != 1 || found[0].Player.WebName != "J.Timber" {
			t.Errorf("expected J.Timber via fuzzy, got %v", found)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		found := FindPlayersByName(scored, []string{"Salah", "Haaland"})
		if len(found) != 2 {
			t.Errorf("expected 2, got %d", len(found))
		}
	})

	t.Run("empty and whitespace", func(t *testing.T) {
		found := FindPlayersByName(scored, []string{"", "  "})
		if len(found) != 0 {
			t.Errorf("expected 0, got %d", len(found))
		}
	})

	t.Run("no match", func(t *testing.T) {
		found := FindPlayersByName(scored, []string{"Nonexistent"})
		if len(found) != 0 {
			t.Errorf("expected 0, got %d", len(found))
		}
	})
}

func TestBestXIFromSquad(t *testing.T) {
	squad := []ScoredPlayer{
		{Player: api.Player{ElementType: PosGK, ID: 1}, Score: 0.5},
		{Player: api.Player{ElementType: PosGK, ID: 2}, Score: 0.3},
		{Player: api.Player{ElementType: PosDEF, ID: 3}, Score: 0.8},
		{Player: api.Player{ElementType: PosDEF, ID: 4}, Score: 0.7},
		{Player: api.Player{ElementType: PosDEF, ID: 5}, Score: 0.6},
		{Player: api.Player{ElementType: PosDEF, ID: 6}, Score: 0.4},
		{Player: api.Player{ElementType: PosDEF, ID: 7}, Score: 0.3},
		{Player: api.Player{ElementType: PosMID, ID: 8}, Score: 0.9},
		{Player: api.Player{ElementType: PosMID, ID: 9}, Score: 0.85},
		{Player: api.Player{ElementType: PosMID, ID: 10}, Score: 0.7},
		{Player: api.Player{ElementType: PosMID, ID: 11}, Score: 0.5},
		{Player: api.Player{ElementType: PosMID, ID: 12}, Score: 0.4},
		{Player: api.Player{ElementType: PosFWD, ID: 13}, Score: 0.95},
		{Player: api.Player{ElementType: PosFWD, ID: 14}, Score: 0.6},
		{Player: api.Player{ElementType: PosFWD, ID: 15}, Score: 0.2},
	}

	starters, fm, score := BestXIFromSquad(squad)

	if len(starters) != 11 {
		t.Fatalf("expected 11 starters, got %d", len(starters))
	}
	if fm == "" {
		t.Error("formation should not be empty")
	}
	if score <= 0 {
		t.Error("total score should be positive")
	}

	prevPos := 0
	for _, s := range starters {
		if s.Player.ElementType < prevPos {
			t.Error("starters should be sorted by position")
			break
		}
		prevPos = s.Player.ElementType
	}
}

func mergeMaps(a, b map[string]string) map[string]string {
	m := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		m[k] = v
	}
	for k, v := range b {
		m[k] = v
	}
	return m
}
