package model

import (
	"math"
	"testing"

	"fpl-picker/api"
)

func makeScoredPlayer(id, team, pos, cost int, score float64) ScoredPlayer {
	return ScoredPlayer{
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

func TestFindBestSquadBasic(t *testing.T) {
	var players []ScoredPlayer

	for i := 1; i <= 4; i++ {
		players = append(players, makeScoredPlayer(i, i, PosGK, 40, 0.3+float64(i)*0.01))
	}
	for i := 5; i <= 12; i++ {
		players = append(players, makeScoredPlayer(i, (i%10)+1, PosDEF, 45, 0.5+float64(i)*0.01))
	}
	for i := 13; i <= 20; i++ {
		players = append(players, makeScoredPlayer(i, (i%10)+1, PosMID, 50, 0.6+float64(i)*0.01))
	}
	for i := 21; i <= 25; i++ {
		players = append(players, makeScoredPlayer(i, (i%10)+1, PosFWD, 70, 0.7+float64(i)*0.01))
	}

	result := FindBestSquad(players, 1000, nil)

	if len(result.Starters) != 11 {
		t.Fatalf("expected 11 starters, got %d", len(result.Starters))
	}
	if len(result.Bench) != 4 {
		t.Fatalf("expected 4 bench, got %d", len(result.Bench))
	}
	if result.Formation == "" {
		t.Error("formation should not be empty")
	}
	if result.TotalScore <= 0 {
		t.Error("total score should be positive")
	}
	if result.TotalCost > result.Budget {
		t.Errorf("total cost £%.1fM exceeds budget £%.1fM", result.TotalCost, result.Budget)
	}
	if result.XICost <= 0 {
		t.Error("XI cost should be positive")
	}
	if result.Captain.Score < result.ViceCaptain.Score {
		t.Error("captain should have higher score than vice-captain")
	}
}

func TestFindBestSquadTeamLimit(t *testing.T) {
	var players []ScoredPlayer

	for i := 1; i <= 4; i++ {
		players = append(players, makeScoredPlayer(i, 1, PosGK, 40, 0.5))
	}
	for i := 5; i <= 9; i++ {
		players = append(players, makeScoredPlayer(i, 1, PosDEF, 45, 0.9))
	}
	for i := 10; i <= 14; i++ {
		players = append(players, makeScoredPlayer(i, 2, PosDEF, 45, 0.3))
	}
	for i := 15; i <= 22; i++ {
		players = append(players, makeScoredPlayer(i, (i%4)+3, PosMID, 50, 0.6))
	}
	for i := 23; i <= 27; i++ {
		players = append(players, makeScoredPlayer(i, (i%3)+7, PosFWD, 70, 0.7))
	}

	result := FindBestSquad(players, 1200, nil)

	teamCounts := map[int]int{}
	allPlayers := append(result.Starters, result.Bench...)
	for _, p := range allPlayers {
		teamCounts[p.Player.Team]++
	}
	for team, count := range teamCounts {
		if count > 3 {
			t.Errorf("team %d has %d players, max allowed is 3", team, count)
		}
	}
}

func TestFindBestSquadBudgetRespected(t *testing.T) {
	var players []ScoredPlayer

	for i := 1; i <= 4; i++ {
		players = append(players, makeScoredPlayer(i, i, PosGK, 40, 0.3))
	}
	for i := 5; i <= 12; i++ {
		players = append(players, makeScoredPlayer(i, (i%10)+1, PosDEF, 45, 0.5))
	}
	for i := 13; i <= 20; i++ {
		players = append(players, makeScoredPlayer(i, (i%10)+1, PosMID, 60, 0.6))
	}
	for i := 21; i <= 25; i++ {
		players = append(players, makeScoredPlayer(i, (i%10)+1, PosFWD, 80, 0.7))
	}

	budget := 800
	result := FindBestSquad(players, budget, nil)

	if result.TotalCost > float64(budget)/10.0 {
		t.Errorf("total cost £%.1fM exceeds budget £%.1fM", result.TotalCost, float64(budget)/10.0)
	}
}

func TestFindBestSquadImpossibleBudget(t *testing.T) {
	var players []ScoredPlayer

	for i := 1; i <= 4; i++ {
		players = append(players, makeScoredPlayer(i, i, PosGK, 100, 0.3))
	}
	for i := 5; i <= 12; i++ {
		players = append(players, makeScoredPlayer(i, (i%10)+1, PosDEF, 100, 0.5))
	}
	for i := 13; i <= 20; i++ {
		players = append(players, makeScoredPlayer(i, (i%10)+1, PosMID, 100, 0.6))
	}
	for i := 21; i <= 25; i++ {
		players = append(players, makeScoredPlayer(i, (i%10)+1, PosFWD, 100, 0.7))
	}

	result := FindBestSquad(players, 500, nil)

	if len(result.Starters) != 0 {
		t.Error("should return empty result for impossible budget")
	}
}

func TestFindBestSquadXIFirstOptimization(t *testing.T) {
	var players []ScoredPlayer

	players = append(players, makeScoredPlayer(1, 1, PosGK, 40, 0.3))
	players = append(players, makeScoredPlayer(2, 2, PosGK, 40, 0.2))

	for i := 3; i <= 7; i++ {
		players = append(players, makeScoredPlayer(i, i, PosDEF, 45, 0.5))
	}
	players = append(players, makeScoredPlayer(30, 11, PosDEF, 40, 0.1))
	players = append(players, makeScoredPlayer(31, 12, PosDEF, 40, 0.1))

	for i := 8; i <= 12; i++ {
		players = append(players, makeScoredPlayer(i, i, PosMID, 50, 0.6))
	}
	players = append(players, makeScoredPlayer(32, 13, PosMID, 40, 0.1))
	players = append(players, makeScoredPlayer(33, 14, PosMID, 40, 0.1))

	players = append(players, makeScoredPlayer(13, 8, PosFWD, 150, 0.95))
	players = append(players, makeScoredPlayer(14, 9, PosFWD, 45, 0.4))
	players = append(players, makeScoredPlayer(15, 10, PosFWD, 45, 0.3))
	players = append(players, makeScoredPlayer(34, 15, PosFWD, 40, 0.1))

	result := FindBestSquad(players, 1000, nil)

	if len(result.Starters) != 11 {
		t.Fatalf("expected 11 starters, got %d", len(result.Starters))
	}

	hasExpensiveFWD := false
	for _, s := range result.Starters {
		if s.Player.ID == 13 {
			hasExpensiveFWD = true
		}
	}
	if !hasExpensiveFWD {
		t.Error("XI-first optimization should include the expensive high-scoring FWD in starters")
	}

	if result.XICost >= result.TotalCost {
		t.Error("bench should add cost beyond XI")
	}
}

func TestPickCaptains(t *testing.T) {
	starters := []ScoredPlayer{
		{Player: api.Player{ID: 1}, Score: 0.5},
		{Player: api.Player{ID: 2}, Score: 0.9},
		{Player: api.Player{ID: 3}, Score: 0.7},
	}

	cap, vc := pickCaptains(starters)

	if cap.Player.ID != 2 {
		t.Errorf("captain should be player 2 (highest score), got %d", cap.Player.ID)
	}
	if vc.Player.ID != 3 {
		t.Errorf("vice-captain should be player 3 (second highest), got %d", vc.Player.ID)
	}
}

func TestPickCaptainsEmpty(t *testing.T) {
	cap, vc := pickCaptains(nil)
	if cap.Player.ID != 0 || vc.Player.ID != 0 {
		t.Error("empty starters should return zero-value captains")
	}
}

func TestBudgetUtilization(t *testing.T) {
	var players []ScoredPlayer
	id := 1

	for _, tc := range []struct {
		cost  int
		score float64
		team  int
	}{
		{55, 0.65, 1}, {50, 0.60, 2}, {40, 0.30, 3}, {40, 0.25, 4},
	} {
		players = append(players, makeScoredPlayer(id, tc.team, PosGK, tc.cost, tc.score))
		id++
	}

	for _, tc := range []struct {
		cost  int
		score float64
		team  int
	}{
		{70, 0.75, 1}, {65, 0.72, 2}, {60, 0.68, 3}, {55, 0.65, 5},
		{50, 0.55, 6}, {45, 0.45, 7}, {40, 0.30, 8}, {40, 0.25, 9},
	} {
		players = append(players, makeScoredPlayer(id, tc.team, PosDEF, tc.cost, tc.score))
		id++
	}

	for _, tc := range []struct {
		cost  int
		score float64
		team  int
	}{
		{120, 0.90, 1}, {100, 0.85, 2}, {90, 0.80, 3}, {80, 0.75, 5},
		{60, 0.55, 6}, {50, 0.45, 10}, {45, 0.35, 11}, {40, 0.28, 12},
	} {
		players = append(players, makeScoredPlayer(id, tc.team, PosMID, tc.cost, tc.score))
		id++
	}

	for _, tc := range []struct {
		cost  int
		score float64
		team  int
	}{
		{130, 0.95, 4}, {110, 0.88, 5}, {90, 0.78, 6},
		{60, 0.50, 13}, {45, 0.35, 14}, {40, 0.28, 15},
	} {
		players = append(players, makeScoredPlayer(id, tc.team, PosFWD, tc.cost, tc.score))
		id++
	}

	budget := 1020 // £102.0M in tenths
	result := FindBestSquad(players, budget, nil)

	if len(result.Starters) != 11 {
		t.Fatalf("expected 11 starters, got %d", len(result.Starters))
	}
	if len(result.Bench) != 4 {
		t.Fatalf("expected 4 bench, got %d", len(result.Bench))
	}

	budgetM := float64(budget) / 10.0
	utilization := result.TotalCost / budgetM

	t.Logf("Budget: £%.1fM, Spent: £%.1fM, Utilization: %.1f%%", budgetM, result.TotalCost, utilization*100)
	t.Logf("Formation: %s, XI Score: %.3f", result.Formation, result.TotalScore)

	if utilization < 0.85 {
		t.Errorf("budget utilization %.1f%% is too low (want >85%%); budget=£%.1fM, spent=£%.1fM",
			utilization*100, budgetM, result.TotalCost)
	}
}

func TestClashPenalty(t *testing.T) {
	t.Run("penalty_computation", func(t *testing.T) {
		pairings := map[int][]FixturePairing{
			1: {{OpponentID: 2, Difficulty: 4}},
			2: {{OpponentID: 1, Difficulty: 4}},
		}

		xi := []ScoredPlayer{
			makeScoredPlayer(1, 1, PosFWD, 100, 0.80),
			makeScoredPlayer(2, 2, PosDEF, 50, 0.60),
		}

		penalty := clashPenalty(xi, pairings)
		if penalty <= 0 {
			t.Error("expected positive clash penalty for FWD(team1) vs DEF(team2) when team1 plays team2")
		}
		t.Logf("Clash penalty: %.4f", penalty)

		expected := 0.15 * 0.7 * 0.60
		if math.Abs(penalty-expected) > 0.001 {
			t.Errorf("penalty=%.4f, want ~%.4f", penalty, expected)
		}
	})

	t.Run("no_penalty_non_opponents", func(t *testing.T) {
		pairings := map[int][]FixturePairing{
			1: {{OpponentID: 2, Difficulty: 3}},
			2: {{OpponentID: 1, Difficulty: 3}},
			3: {{OpponentID: 4, Difficulty: 3}},
			4: {{OpponentID: 3, Difficulty: 3}},
		}

		xi := []ScoredPlayer{
			makeScoredPlayer(1, 1, PosFWD, 100, 0.80),
			makeScoredPlayer(2, 3, PosDEF, 50, 0.60),
		}

		penalty := clashPenalty(xi, pairings)
		if penalty != 0 {
			t.Errorf("expected zero penalty for non-opposing teams, got %.4f", penalty)
		}
	})

	t.Run("gk_higher_weight", func(t *testing.T) {
		pairings := map[int][]FixturePairing{
			1: {{OpponentID: 2, Difficulty: 4}},
			2: {{OpponentID: 1, Difficulty: 4}},
		}

		xiWithDEF := []ScoredPlayer{
			makeScoredPlayer(1, 1, PosMID, 80, 0.70),
			makeScoredPlayer(2, 2, PosDEF, 50, 0.70),
		}
		penaltyDEF := clashPenalty(xiWithDEF, pairings)

		xiWithGK := []ScoredPlayer{
			makeScoredPlayer(1, 1, PosMID, 80, 0.70),
			makeScoredPlayer(3, 2, PosGK, 50, 0.70),
		}
		penaltyGK := clashPenalty(xiWithGK, pairings)

		if penaltyGK <= penaltyDEF {
			t.Errorf("GK penalty (%.4f) should be higher than DEF penalty (%.4f) due to higher weight",
				penaltyGK, penaltyDEF)
		}
		t.Logf("DEF penalty: %.4f, GK penalty: %.4f", penaltyDEF, penaltyGK)
	})

	t.Run("optimizer_prefers_non_clash", func(t *testing.T) {
		var players []ScoredPlayer
		id := 1

		// Team 1 plays team 2 (FDR 5 = hard).  Teams 3-8 are NOT paired,
		// so their GK/DEF won't clash with team 1's MID.
		pairings := map[int][]FixturePairing{
			1: {{OpponentID: 2, Difficulty: 5}},
			2: {{OpponentID: 1, Difficulty: 5}},
		}

		// GK: team2 (clashes with team1 MID), team3 (no fixture → no clash)
		players = append(players, makeScoredPlayer(id, 2, PosGK, 45, 0.55))
		id++
		players = append(players, makeScoredPlayer(id, 3, PosGK, 45, 0.54))
		id++
		players = append(players, makeScoredPlayer(id, 7, PosGK, 40, 0.20))
		id++

		for _, tm := range []int{3, 4, 5, 6, 7, 8} {
			players = append(players, makeScoredPlayer(id, tm, PosDEF, 45, 0.50+float64(id)*0.001))
			id++
		}
		players = append(players, makeScoredPlayer(id, 5, PosDEF, 40, 0.20))
		id++
		players = append(players, makeScoredPlayer(id, 8, PosDEF, 40, 0.20))
		id++

		// Team 1 MID clashes with team 2 GK/DEF
		players = append(players, makeScoredPlayer(id, 1, PosMID, 90, 0.85))
		id++
		for _, tm := range []int{3, 4, 5, 6, 7} {
			players = append(players, makeScoredPlayer(id, tm, PosMID, 60, 0.60))
			id++
		}
		players = append(players, makeScoredPlayer(id, 8, PosMID, 40, 0.20))
		id++

		for _, tm := range []int{3, 5, 7} {
			players = append(players, makeScoredPlayer(id, tm, PosFWD, 70, 0.65))
			id++
		}
		players = append(players, makeScoredPlayer(id, 8, PosFWD, 40, 0.25))
		id++

		result := FindBestSquad(players, 1000, pairings)
		if len(result.Starters) != 11 {
			t.Fatalf("expected 11 starters, got %d", len(result.Starters))
		}

		hasTeam1Atk := false
		hasTeam2Def := false
		for _, s := range result.Starters {
			if s.Player.Team == 1 && (s.Player.ElementType == PosMID || s.Player.ElementType == PosFWD) {
				hasTeam1Atk = true
			}
			if s.Player.Team == 2 && (s.Player.ElementType == PosGK || s.Player.ElementType == PosDEF) {
				hasTeam2Def = true
			}
		}

		if hasTeam1Atk && hasTeam2Def {
			t.Error("optimizer picked clashing ATK(team1) + DEF(team2) despite available non-clashing alternatives")
		} else {
			t.Log("optimizer successfully avoided head-to-head clash")
		}
	})
}

func TestIntegrationScorerToRecommender(t *testing.T) {
	teams := []api.Team{
		testTeam(1, "ARS"),
		testTeam(2, "CHE"),
		testTeam(3, "LIV"),
		testTeam(4, "MCI"),
		testTeam(5, "TOT"),
		testTeam(6, "MUN"),
	}

	events := []api.Event{{ID: 10, IsNext: true}}

	fixtures := []api.Fixture{
		testFixture(10, 1, 2),
		testFixture(10, 3, 4),
		testFixture(10, 5, 6),
	}

	baseOpts := map[string]string{
		"form": "5.0", "ep_next": "5.0", "ppg": "4.0",
		"xgi": "8.0", "ict": "80.0", "xg": "6.0", "xa": "2.0", "xgc": "8.0",
		"selected": "10.0",
	}

	var allPlayers []api.Player
	id := 1

	for tm := 1; tm <= 6; tm++ {
		allPlayers = append(allPlayers, makePlayer(id, tm, PosGK, 40, 900,
			mergeMaps(baseOpts, map[string]string{"name": "GK" + string(rune('A'+tm-1))})))
		id++
	}

	for tm := 1; tm <= 6; tm++ {
		for j := 0; j < 3; j++ {
			cost := 45 + j*5
			allPlayers = append(allPlayers, makePlayer(id, tm, PosDEF, cost, 900,
				mergeMaps(baseOpts, map[string]string{"name": "DEF" + string(rune('A'+id-1))})))
			id++
		}
	}

	for tm := 1; tm <= 6; tm++ {
		for j := 0; j < 3; j++ {
			cost := 60 + j*10
			allPlayers = append(allPlayers, makePlayer(id, tm, PosMID, cost, 900,
				mergeMaps(baseOpts, map[string]string{"name": "MID" + string(rune('A'+id-1))})))
			id++
		}
	}

	for tm := 1; tm <= 6; tm++ {
		for j := 0; j < 2; j++ {
			cost := 70 + j*20
			allPlayers = append(allPlayers, makePlayer(id, tm, PosFWD, cost, 900,
				mergeMaps(baseOpts, map[string]string{"name": "FWD" + string(rune('A'+id-1))})))
			id++
		}
	}

	scorer := NewScorer(teams, fixtures, events, allPlayers, "1")
	if scorer.NextEventID() != 10 {
		t.Fatalf("NextEventID() = %d, want 10", scorer.NextEventID())
	}

	scored := scorer.ScoreAll(allPlayers)
	if len(scored) == 0 {
		t.Fatal("ScoreAll returned no scored players")
	}

	for _, sp := range scored {
		if !sp.HasFixture {
			t.Errorf("player %s should have a fixture", sp.Player.WebName)
		}
	}

	budget := 1500
	result := FindBestSquad(scored, budget, nil)

	if len(result.Starters) != 11 {
		t.Fatalf("expected 11 starters, got %d", len(result.Starters))
	}
	if len(result.Bench) != 4 {
		t.Fatalf("expected 4 bench, got %d", len(result.Bench))
	}
	if result.Formation == "" {
		t.Error("formation should not be empty")
	}

	if result.TotalCost > float64(budget)/10.0 {
		t.Errorf("total cost £%.1fM exceeds budget £%.1fM", result.TotalCost, float64(budget)/10.0)
	}

	teamCounts := map[int]int{}
	for _, sp := range result.Starters {
		teamCounts[sp.Player.Team]++
	}
	for _, sp := range result.Bench {
		teamCounts[sp.Player.Team]++
	}
	for tm, cnt := range teamCounts {
		if cnt > 3 {
			t.Errorf("team %d has %d players in squad, max is 3", tm, cnt)
		}
	}

	if result.Captain.Player.ID == 0 {
		t.Error("captain should be assigned")
	}
	if result.ViceCaptain.Player.ID == 0 {
		t.Error("vice-captain should be assigned")
	}
	if result.Captain.Player.ID == result.ViceCaptain.Player.ID {
		t.Error("captain and vice-captain should be different players")
	}
	if result.Captain.Score < result.ViceCaptain.Score {
		t.Error("captain should have score >= vice-captain")
	}

	if result.TotalScore <= 0 {
		t.Error("total score should be positive")
	}
	if result.XICost <= 0 {
		t.Error("XI cost should be positive")
	}
	if result.XICost > result.TotalCost {
		t.Error("XI cost should be <= total cost")
	}

	posCounts := map[int]int{}
	for _, sp := range result.Starters {
		posCounts[sp.Player.ElementType]++
	}
	if posCounts[PosGK] != 1 {
		t.Errorf("starters should have exactly 1 GK, got %d", posCounts[PosGK])
	}
	if posCounts[PosDEF] < 3 || posCounts[PosDEF] > 5 {
		t.Errorf("starters should have 3-5 DEF, got %d", posCounts[PosDEF])
	}
	if posCounts[PosMID] < 3 || posCounts[PosMID] > 5 {
		t.Errorf("starters should have 3-5 MID, got %d", posCounts[PosMID])
	}
	if posCounts[PosFWD] < 1 || posCounts[PosFWD] > 3 {
		t.Errorf("starters should have 1-3 FWD, got %d", posCounts[PosFWD])
	}
}

func BenchmarkFindBestSquad(b *testing.B) {
	var players []ScoredPlayer
	id := 1
	numTeams := 20

	for tm := 1; tm <= numTeams; tm++ {
		for j := 0; j < 2; j++ {
			players = append(players, makeScoredPlayer(id, tm, PosGK, 40+j*10, 0.3+float64(j)*0.1))
			id++
		}
	}
	for tm := 1; tm <= numTeams; tm++ {
		for j := 0; j < 5; j++ {
			players = append(players, makeScoredPlayer(id, tm, PosDEF, 40+j*8, 0.4+float64(j)*0.05))
			id++
		}
	}
	for tm := 1; tm <= numTeams; tm++ {
		for j := 0; j < 5; j++ {
			players = append(players, makeScoredPlayer(id, tm, PosMID, 50+j*15, 0.5+float64(j)*0.06))
			id++
		}
	}
	for tm := 1; tm <= numTeams; tm++ {
		for j := 0; j < 3; j++ {
			players = append(players, makeScoredPlayer(id, tm, PosFWD, 60+j*25, 0.55+float64(j)*0.08))
			id++
		}
	}

	b.Logf("Pool size: %d players across %d teams", len(players), numTeams)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		FindBestSquad(players, 1000, nil)
	}
}
