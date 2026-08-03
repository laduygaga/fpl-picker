package apply

import (
	"context"
	"fmt"
	"os"

	"fpl-picker/api"
	"fpl-picker/model"
)

// Options controls the Run pipeline.
//
//   - Apply=false (default) runs a dry-run: it reads the live squad, plans
//     lineup + transfers, and prints a diff to stderr. No writes happen.
//   - Apply=true additionally POSTs the lineup and commits transfers.
//
// SkipTransfers / SkipLineup let the caller iterate on a single phase.
type Options struct {
	Apply         bool   // --apply; default dry-run
	Chip          string // "" or "wildcard" | "freehit" | "bboost" | "3xc"
	MaxHits       int    // default 4
	SkipTransfers bool
	SkipLineup    bool
}

// Result is the summary returned by Run. Bank and SquadValue are in tenths
// of £m to match the /api/my-team/ response shape.
type Result struct {
	LineupChanged    bool
	TransfersPlanned int
	TransfersMade    int
	PointsHits       int
	Bank             int
	SquadValue       int
}

// Run executes the full apply pipeline using the supplied current + optimal
// squads:
//
//  1. Plan the lineup from the supplied optimal squad.
//  2. Plan transfers within budget + max-hits cap.
//  3. Render a dry-run diff to stderr (always; the CLI also prints its
//     own summary).
//  4. If opts.Apply, POST the lineup and commit the transfers.
//
// The caller is expected to have authenticated the client and fetched both
// squads. Run does not re-fetch — that lives in the CLI driver so the user
// sees the optimizer output before being prompted to commit.
//
// Errors are returned for unrecoverable conditions (auth failure, network
// failure, validation failure). Validation failures from /api/transfers/
// surface as wrapped *api.TransferError so the caller can match on errors.As.
func Run(ctx context.Context, client *api.AuthClient, entryID int, current *api.MyTeam, optimal model.SquadResult, opts Options) (*Result, error) {
	if client == nil {
		return nil, fmt.Errorf("apply: nil auth client")
	}
	if current == nil {
		return nil, fmt.Errorf("apply: nil current team")
	}
	if entryID == 0 {
		return nil, fmt.Errorf("apply: entry id is required (pass it explicitly; the 2026-era FPL API no longer echoes it inside the transfers block)")
	}
	if opts.MaxHits == 0 {
		opts.MaxHits = 4
	}

	res := &Result{
		Bank:       current.Transfers.Bank,
		SquadValue: current.Transfers.Value,
	}

	lineup := PlanLineup(optimal)
	if !opts.SkipLineup {
		update := api.LineupUpdate{Picks: lineup.Picks}
		if api.LineupHasChanged(current, update) {
			res.LineupChanged = true
			if opts.Apply {
				if opts.Chip != "" {
					update.Chip = api.ChipPtr(opts.Chip)
				}
				if err := client.UpdateLineup(entryID, update); err != nil {
					return res, fmt.Errorf("apply: lineup update failed: %w", err)
				}
			}
		}
	}

	var suggestions []TransferSuggestion
	if !opts.SkipTransfers {
		suggestions = PlanTransfers(current, optimal, opts.MaxHits)
		res.TransfersPlanned = len(suggestions)

		if len(suggestions) > 0 || opts.Chip != "" {
			req := BuildTransferRequest(entryID, 1, suggestions, opts.Chip)
			if req != nil {
				spent, verr := PreviewPointsHits(ctx, client, *req)
				if verr != nil {
					if te := api.AsTransferError(verr); te != nil {
						return res, fmt.Errorf("apply: transfer validation failed: %w", verr)
					}
					return res, fmt.Errorf("apply: transfer validation error: %w", verr)
				}
				res.PointsHits = spent
				if opts.Apply {
					if cerr := client.CommitTransfers(*req); cerr != nil {
						return res, fmt.Errorf("apply: commit transfers failed: %w", cerr)
					}
					res.TransfersMade = len(suggestions)
				}
			}
		}
	}

	if !opts.Apply {
		SortSuggestions(suggestions)
		_, _ = fmt.Fprintln(os.Stderr, RenderDiff(current, lineup, suggestions, res.PointsHits))
	}

	return res, nil
}
