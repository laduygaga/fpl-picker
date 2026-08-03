package apply

import (
	"testing"

	"fpl-picker/model"
)

func TestPlanTransfersEmptyCurrent(t *testing.T) {
	optimal := model.SquadResult{
		Starters: []model.ScoredPlayer{makeScored(1, model.PosGK, 50, 0.5)},
	}
	if got := PlanTransfers(nil, optimal, 4, nil); got != nil {
		t.Errorf("nil current should return nil suggestions, got %v", got)
	}
}
