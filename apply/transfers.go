package apply

import (
	"context"
	"fmt"
	"sort"

	"fpl-picker/api"
	"fpl-picker/model"
)

// TransferSuggestion describes a single OUT → IN swap that moves the user's
// current team closer to the optimizer's optimal squad.
//
// SellingPrice is the OUT player's tenths-of-£m value (sourced from
// MyTeam.Picks[].SellingPrice). PurchasePrice is the IN player's current
// NowCost. ScoreUplift is the IN player's score minus the OUT player's score
// (raw, not multiplied by captain).
type TransferSuggestion struct {
	Out           model.ScoredPlayer
	In            model.ScoredPlayer
	SellingPrice  int
	PurchasePrice int
	ScoreUplift   float64
}

// PlanTransfers proposes transfers to take the user's current team toward the
// optimizer's optimal squad. Constraints enforced:
//
//   - Budget: Σ(selling prices) + bank ≥ Σ(purchase prices). The bank and
//     selling prices are in tenths of £m; the function tracks the running
//     budget as each suggestion is locked in.
//   - maxHits: stops proposing transfers when the resulting points-hit
//     estimate would exceed the cap. Each transfer beyond the user's free
//     quota costs 4 points.
//
// The 3-players-per-team constraint is NOT enforced here — the FPL server
// validates it on commit. The Pick struct returned by /api/my-team/ does not
// include team_id, so a local check would require an external lookup that
// complicates the call site; the server is the source of truth.
//
// Algorithm (greedy):
//  1. Identify IN candidates (optimal players not in current squad) sorted
//     by score desc.
//  2. Identify OUT candidates (current players not in optimal squad) sorted
//     by score asc (weakest first).
//  3. For each IN, pick the OUT that yields the largest score uplift subject
//     to budget. Lock in the swap, update local bank, and continue until
//     no IN fits or hits cap is reached.
func PlanTransfers(current *api.MyTeam, optimal model.SquadResult, maxHits int) []TransferSuggestion {
	if current == nil || len(current.Picks) == 0 {
		return nil
	}

	currentByID := map[int]api.Pick{}
	for _, p := range current.Picks {
		currentByID[p.Element] = p
	}

	optimalByID := map[int]model.ScoredPlayer{}
	for _, p := range optimal.Starters {
		optimalByID[p.Player.ID] = p
	}
	for _, p := range optimal.Bench {
		optimalByID[p.Player.ID] = p
	}

	var incoming []model.ScoredPlayer
	for id, sp := range optimalByID {
		if _, ok := currentByID[id]; ok {
			continue
		}
		incoming = append(incoming, sp)
	}
	sort.Slice(incoming, func(i, j int) bool { return incoming[i].Score > incoming[j].Score })

	type outCand struct {
		pick      api.Pick
		bestScore float64
	}
	var outgoing []outCand
	for id, pick := range currentByID {
		if _, ok := optimalByID[id]; ok {
			continue
		}
		oc := outCand{pick: pick}
		if sp, ok := optimalByID[id]; ok {
			oc.bestScore = sp.Score
		}
		outgoing = append(outgoing, oc)
	}
	sort.Slice(outgoing, func(i, j int) bool {
		return outgoing[i].bestScore < outgoing[j].bestScore
	})

	bank := current.Transfers.Bank
	usedFree := current.Transfers.Made
	freeLimit := current.Transfers.Limit

	var suggestions []TransferSuggestion
	used := make([]bool, len(outgoing))

	// Pre-compute how many free transfers remain. With maxHits=0 the caller
	// refuses to spend any points on hits, so we cap suggestions to that
	// many transfers and stop.
	freeRemaining := max(freeLimit-usedFree, 0)
	maxByHits := max(freeRemaining+maxHits/4, 0)

	for _, in := range incoming {
		if len(suggestions) >= maxByHits {
			break
		}

		bestIdx := -1
		bestUplift := -1e18

		for i := range outgoing {
			if used[i] {
				continue
			}
			outPick := outgoing[i].pick
			netCost := in.Player.NowCost - outPick.SellingPrice
			if netCost > bank {
				continue
			}
			uplift := in.Score - outgoing[i].bestScore
			if uplift > bestUplift {
				bestUplift = uplift
				bestIdx = i
			}
		}
		if bestIdx < 0 {
			continue
		}

		outPick := outgoing[bestIdx].pick
		outSP := makeOutScoredPlayer(outPick, outgoing[bestIdx].bestScore)
		suggestions = append(suggestions, TransferSuggestion{
			Out:           outSP,
			In:            in,
			SellingPrice:  outPick.SellingPrice,
			PurchasePrice: in.Player.NowCost,
			ScoreUplift:   bestUplift,
		})

		bank -= in.Player.NowCost - outPick.SellingPrice
		used[bestIdx] = true
	}

	return suggestions
}

// makeOutScoredPlayer wraps an api.Pick into a ScoredPlayer so the diff
// layer can render the OUT player's name + score without re-resolving it.
func makeOutScoredPlayer(p api.Pick, score float64) model.ScoredPlayer {
	return model.ScoredPlayer{
		Player: api.Player{
			ID:          p.Element,
			ElementType: p.ElementType,
		},
		Score: score,
	}
}

// hitsExceeded returns true when adding another transfer would push points
// hits beyond the configured cap. Each transfer beyond the free limit costs
// 4 points (per FPL rules).
//
// When maxHits <= 0, any positive transfer count is rejected — the cap is
// zero points so even a single transfer that costs points is over the cap.
// The caller is expected to drive this with proposed+1 once it has picked
// a candidate to commit.
func hitsExceeded(usedFree, freeLimit, proposed int, maxHits int) bool {
	if maxHits < 0 {
		return proposed > 0
	}
	total := usedFree + proposed
	hits := max((total-freeLimit)*4, 0)
	if maxHits == 0 {
		return hits > 0
	}
	return hits > maxHits
}

// BuildTransferRequest converts a list of suggestions into the API request
// shape that /api/transfers/ expects. eventID is the *target* gameweek
// (next, not current). chip is "" or one of "wildcard" / "freehit".
//
// Returns nil when there are no transfers to send AND no chip to activate.
func BuildTransferRequest(entryID, eventID int, suggestions []TransferSuggestion, chip string) *api.TransferRequest {
	if len(suggestions) == 0 && chip == "" {
		return nil
	}
	req := &api.TransferRequest{
		Confirmed: false,
		Entry:     entryID,
		Event:     eventID,
		Transfers: make([]api.Transfer, 0, len(suggestions)),
	}
	for _, s := range suggestions {
		req.Transfers = append(req.Transfers, api.Transfer{
			ElementIn:     s.In.Player.ID,
			ElementOut:    s.Out.Player.ID,
			PurchasePrice: s.PurchasePrice,
			SellingPrice:  s.SellingPrice,
		})
	}
	if chip != "" {
		req.Chip = api.ChipPtr(chip)
	}
	return req
}

// PreviewPointsHits calls the two-step transfer validation to surface the
// points hits the server would charge for the supplied request. The caller
// can use this to decide whether to commit.
//
// Returns the spent_points value from the validation response, or an error.
func PreviewPointsHits(ctx context.Context, client *api.AuthClient, req api.TransferRequest) (int, error) {
	if client == nil {
		return 0, fmt.Errorf("apply: nil auth client")
	}
	return client.ValidateTransfers(req)
}
