package apply

import (
	"strings"
	"testing"

	"fpl-picker/api"
	"fpl-picker/model"
)

// makeScoredPlayer is a helper that builds a ScoredPlayer with explicit
// identity, team, position, cost, and score fields. Mirrors the helper in
// recommender_test.go without depending on it.
func makeScoredPlayer(id, team, pos, cost int, score float64) model.ScoredPlayer {
	return model.ScoredPlayer{
		Player: api.Player{
			ID:          id,
			Team:        team,
			ElementType: pos,
			NowCost:     cost,
			WebName:     "P" + string(rune('A'+id-1)),
		},
		Score: score,
	}
}

// buildSquadResult constructs a SquadResult for a 3-4-3 with captain + VC.
func buildSquadResult() model.SquadResult {
	starters := []model.ScoredPlayer{
		makeScoredPlayer(1, 1, model.PosGK, 50, 0.50),
		makeScoredPlayer(2, 1, model.PosDEF, 55, 0.65),
		makeScoredPlayer(3, 2, model.PosDEF, 60, 0.70),
		makeScoredPlayer(4, 2, model.PosDEF, 65, 0.75),
		makeScoredPlayer(5, 3, model.PosMID, 80, 0.80),
		makeScoredPlayer(6, 3, model.PosMID, 85, 0.85),
		makeScoredPlayer(7, 4, model.PosMID, 90, 0.90),
		makeScoredPlayer(8, 4, model.PosMID, 95, 0.95),
		makeScoredPlayer(9, 5, model.PosFWD, 100, 0.70),
		makeScoredPlayer(10, 5, model.PosFWD, 105, 0.85),
		makeScoredPlayer(11, 6, model.PosFWD, 110, 0.90),
	}
	bench := []model.ScoredPlayer{
		makeScoredPlayer(12, 6, model.PosGK, 40, 0.20),
		makeScoredPlayer(13, 7, model.PosDEF, 45, 0.30),
		makeScoredPlayer(14, 7, model.PosMID, 50, 0.35),
		makeScoredPlayer(15, 8, model.PosFWD, 55, 0.40),
	}
	return model.SquadResult{
		Formation:   "3-4-3",
		Starters:    starters,
		Bench:       bench,
		Captain:     starters[8], // id 9
		ViceCaptain: starters[9], // id 10
		TotalScore:  7.95,
		XICost:      88.5,
		TotalCost:   99.5,
		Budget:      100.0,
	}
}

func TestPlanLineupPacksFifteenSlots(t *testing.T) {
	plan := PlanLineup(buildSquadResult())

	if len(plan.Picks) != 15 {
		t.Fatalf("len(Picks) = %d, want 15", len(plan.Picks))
	}
	if plan.Formation != "3-4-3" {
		t.Errorf("Formation = %q, want 3-4-3", plan.Formation)
	}
	if plan.CaptainID != 9 {
		t.Errorf("CaptainID = %d, want 9", plan.CaptainID)
	}
	if plan.ViceCaptainID != 10 {
		t.Errorf("ViceCaptainID = %d, want 10", plan.ViceCaptainID)
	}
	if len(plan.BenchOrder) != 4 {
		t.Errorf("len(BenchOrder) = %d, want 4", len(plan.BenchOrder))
	}

	positions := map[int]int{}
	for _, p := range plan.Picks {
		positions[p.Position]++
	}
	for slot := 1; slot <= 15; slot++ {
		if positions[slot] != 1 {
			t.Errorf("slot %d has %d picks, want exactly 1", slot, positions[slot])
		}
	}

	captainCount, viceCount := 0, 0
	for _, p := range plan.Picks {
		if p.IsCaptain {
			captainCount++
		}
		if p.IsViceCaptain {
			viceCount++
		}
	}
	if captainCount != 1 || viceCount != 1 {
		t.Errorf("captain=%d vice=%d, want 1 each", captainCount, viceCount)
	}
}

func TestPlanLineupBenchOrdering(t *testing.T) {
	plan := PlanLineup(buildSquadResult())

	benchInPlan := []api.Pick{}
	for _, p := range plan.Picks {
		if p.Position >= 12 {
			benchInPlan = append(benchInPlan, p)
		}
	}
	if len(benchInPlan) != 4 {
		t.Fatalf("bench picks = %d, want 4", len(benchInPlan))
	}

	wantOrder := []int{12, 13, 14, 15}
	for i, p := range benchInPlan {
		if p.Position != 12+i {
			t.Errorf("bench slot %d position = %d, want %d", i, p.Position, 12+i)
		}
		if p.Element != wantOrder[i] {
			t.Errorf("bench slot %d element = %d, want %d", i, p.Element, wantOrder[i])
		}
	}
}

func TestDiffLineupCaptainSwap(t *testing.T) {
	current := &api.MyTeam{Picks: captainSwapTeam()}
	proposed := captainSwapPlan()

	diff := DiffLineup(current, proposed)
	if !strings.Contains(diff, "Captain:") {
		t.Errorf("diff should mention captain change, got %q", diff)
	}
	if !strings.Contains(diff, "Vice-captain:") {
		t.Errorf("diff should mention vice-captain change, got %q", diff)
	}
}

// captainSwapTeam returns a 15-pick team where element 5 is captain and
// element 7 is vice-captain.
func captainSwapTeam() []api.Pick {
	out := make([]api.Pick, 15)
	for i := range out {
		out[i] = api.Pick{Element: i + 1, Position: i + 1}
	}
	out[4].IsCaptain = true     // id 5
	out[6].IsViceCaptain = true // id 7
	out[4].ElementType = model.PosMID
	out[6].ElementType = model.PosMID
	for i := 0; i < 11; i++ {
		out[i].ElementType = model.PosMID
	}
	return out
}

// captainSwapPlan returns a 15-pick plan where element 8 is captain and
// element 6 is vice-captain.
func captainSwapPlan() LineupPlan {
	picks := make([]api.Pick, 15)
	for i := range picks {
		picks[i] = api.Pick{Element: i + 1, Position: i + 1, ElementType: model.PosMID}
	}
	picks[7].IsCaptain = true     // id 8
	picks[5].IsViceCaptain = true // id 6
	return LineupPlan{
		Picks:         picks,
		CaptainID:     8,
		ViceCaptainID: 6,
		Formation:     "0-0-11-0",
	}
}

func TestDiffLineupNoChanges(t *testing.T) {
	current := &api.MyTeam{Picks: identicalTeam()}
	plan := identicalPlan()

	diff := DiffLineup(current, plan)
	if !strings.Contains(diff, "no lineup changes") {
		t.Errorf("diff = %q, want no-changes sentinel", diff)
	}
}

func identicalTeam() []api.Pick {
	out := make([]api.Pick, 15)
	for i := range out {
		out[i] = api.Pick{Element: i + 1, Position: i + 1, ElementType: model.PosMID}
	}
	out[0].IsCaptain = true
	out[1].IsViceCaptain = true
	return out
}

func identicalPlan() LineupPlan {
	return LineupPlan{
		Picks:         identicalTeam(),
		CaptainID:     1,
		ViceCaptainID: 2,
		Formation:     "0-0-11-0",
		BenchOrder:    []int{12, 13, 14, 15},
	}
}

func TestDeriveFormationFromPicks(t *testing.T) {
	picks := []api.Pick{}
	for i := 1; i <= 1; i++ {
		picks = append(picks, api.Pick{Element: i, Position: i, ElementType: model.PosGK})
	}
	for i := 2; i <= 4; i++ {
		picks = append(picks, api.Pick{Element: i, Position: i, ElementType: model.PosDEF})
	}
	for i := 5; i <= 8; i++ {
		picks = append(picks, api.Pick{Element: i, Position: i, ElementType: model.PosMID})
	}
	for i := 9; i <= 11; i++ {
		picks = append(picks, api.Pick{Element: i, Position: i, ElementType: model.PosFWD})
	}
	for i := 12; i <= 15; i++ {
		picks = append(picks, api.Pick{Element: i, Position: i, ElementType: model.PosDEF})
	}
	got := deriveFormation(picks)
	want := "1-3-4-3"
	if got != want {
		t.Errorf("deriveFormation = %q, want %q", got, want)
	}
}

func TestDeriveFormationEmpty(t *testing.T) {
	if got := deriveFormation(nil); got != "" {
		t.Errorf("deriveFormation(nil) = %q, want empty", got)
	}
}

func TestBenchOrderEquality(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{1, 2, 3}
	c := []int{1, 3, 2}
	d := []int{1, 2}
	if !equalIntSlice(a, b) {
		t.Error("a == b should be true")
	}
	if equalIntSlice(a, c) {
		t.Error("a != c should be true")
	}
	if equalIntSlice(a, d) {
		t.Error("a != d (length differs)")
	}
}
