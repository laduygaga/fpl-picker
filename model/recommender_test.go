package model

import (
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

	result := FindBestSquad(players, 1000)

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

	result := FindBestSquad(players, 1200)

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
	result := FindBestSquad(players, budget)

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

	result := FindBestSquad(players, 500)

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

	result := FindBestSquad(players, 1000)

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

func TestIsAlreadyPicked(t *testing.T) {
	picked := []ScoredPlayer{
		{Player: api.Player{ID: 1}},
		{Player: api.Player{ID: 5}},
	}

	if !isAlreadyPicked(picked, 1) {
		t.Error("should find player 1")
	}
	if !isAlreadyPicked(picked, 5) {
		t.Error("should find player 5")
	}
	if isAlreadyPicked(picked, 99) {
		t.Error("should not find player 99")
	}
}
