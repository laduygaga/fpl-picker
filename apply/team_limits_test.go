package apply

import (
	"encoding/json"
	"fmt"
	"testing"

	"fpl-picker/api"
	"fpl-picker/model"
)

func TestIsTeamLimitValidationErrorMatchesStructuredCode(t *testing.T) {
	var transferErr api.TransferError
	if err := json.Unmarshal([]byte(`{"non_field_errors":[{"code":"transfer_team_limit_reached","message":"team limit"}]}`), &transferErr); err != nil {
		t.Fatalf("decode transfer error: %v", err)
	}

	if !isTeamLimitValidationError(fmt.Errorf("wrapped: %w", &transferErr)) {
		t.Error("structured transfer_team_limit_reached should be retryable through a wrapped error")
	}
}

func TestPlanTransfersAllowsFullTeamReplacement(t *testing.T) {
	currentPicks := make([]api.Pick, 0, 15)
	for i := 1; i <= 15; i++ {
		currentPicks = append(currentPicks, api.Pick{
			Element:      i,
			SellingPrice: 50,
			ElementType:  model.PosMID,
		})
	}
	current := &api.MyTeam{
		Picks:     currentPicks,
		Transfers: api.TransferStatus{Bank: 100, Limit: intPtr(1)},
	}
	optimal := model.SquadResult{Starters: []model.ScoredPlayer{
		makeScored(100, model.PosMID, 60, 1),
	}}
	for i := 2; i <= 15; i++ {
		optimal.Starters = append(optimal.Starters, makeScored(i, model.PosMID, 50, 0))
	}
	idToTeam := map[int]int{1: 1, 2: 1, 3: 1, 100: 1}

	got := PlanTransfers(current, optimal, 4, idToTeam)

	if len(got) != 1 || got[0].Out.Player.ID != 1 || got[0].In.Player.ID != 100 {
		t.Fatalf("suggestions = %+v, want 1→100", got)
	}
	counts := teamCount(current.Picks, idToTeam)
	counts[idToTeam[got[0].Out.Player.ID]]--
	counts[idToTeam[got[0].In.Player.ID]]++
	if counts[1] > 3 {
		t.Fatalf("team 1 count = %d, want at most 3", counts[1])
	}
}

func TestPlanTransfersNeverExceedsMappedTeamLimit(t *testing.T) {
	currentPicks := make([]api.Pick, 0, 15)
	for i := 1; i <= 15; i++ {
		currentPicks = append(currentPicks, api.Pick{
			Element:      i,
			SellingPrice: 50,
			ElementType:  model.PosMID,
		})
	}
	current := &api.MyTeam{
		Picks:     currentPicks,
		Transfers: api.TransferStatus{Bank: 1000, Limit: intPtr(2)},
	}
	optimal := model.SquadResult{Starters: []model.ScoredPlayer{
		makeScored(100, model.PosMID, 60, 1),
		makeScored(101, model.PosMID, 60, 0.9),
	}}
	idToTeam := map[int]int{1: 1, 2: 1, 100: 1, 101: 1}

	got := PlanTransfers(current, optimal, 8, idToTeam)

	counts := teamCount(current.Picks, idToTeam)
	for _, suggestion := range got {
		counts[idToTeam[suggestion.Out.Player.ID]]--
		counts[idToTeam[suggestion.In.Player.ID]]++
	}
	if counts[1] > 3 {
		t.Fatalf("team 1 count = %d, want at most 3; suggestions = %+v", counts[1], got)
	}
}

func TestIsTeamLimitValidationErrorMatchesExactCode(t *testing.T) {
	teamLimit := fmt.Errorf("wrapped: %w", &api.TransferError{NonFormErrors: []string{"transfer_team_limit_reached"}})
	unrelated := &api.TransferError{NonFormErrors: []string{"transfer_element_in_is_pick"}}
	nearMiss := &api.TransferError{NonFormErrors: []string{"transfer_team_limit_reached_extra"}}

	if !isTeamLimitValidationError(teamLimit) {
		t.Error("exact transfer_team_limit_reached should be retryable")
	}
	if isTeamLimitValidationError(unrelated) {
		t.Error("unrelated validation error should not be retryable")
	}
	if isTeamLimitValidationError(nearMiss) {
		errorMsg := nearMiss.NonFormErrors[0]
		t.Errorf("near-miss validation error %q should not be retryable", errorMsg)
	}
}
