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

	// Transfers must commit BEFORE the lineup update — the lineup update
	// rewrites every player's slot/captain/VC, and FPL rejects picks that
	// reference players who aren't on the team yet. So we plan transfers,
	// commit them, then plan+post lineup against the post-transfer world.

	var suggestions []TransferSuggestion
	if !opts.SkipTransfers {
		suggestions = PlanTransfers(current, optimal, opts.MaxHits)
		res.TransfersPlanned = len(suggestions)

		if len(suggestions) > 0 || opts.Chip != "" {
			req := BuildTransferRequest(entryID, 1, suggestions, opts.Chip)
			if req != nil {
				spent, verr := PreviewPointsHits(ctx, client, *req)
				if verr != nil {
					// Validation failed — re-fetch + re-plan once.
					// FPL's validation is lenient; stale `current` can
					// pass validate then fail at commit time.
					if opts.Apply {
						freshCurrent, ferr := client.GetMyTeam(entryID)
						if ferr == nil {
							resuggested := PlanTransfers(freshCurrent, optimal, opts.MaxHits)
							suggestions = resuggested
							res.TransfersPlanned = len(suggestions)
							req = BuildTransferRequest(entryID, 1, suggestions, opts.Chip)
							if req != nil {
								spent, verr = PreviewPointsHits(ctx, client, *req)
							}
						}
					}
					if verr != nil {
						if te := api.AsTransferError(verr); te != nil {
							return res, fmt.Errorf("apply: transfer validation failed: %w", verr)
						}
						return res, fmt.Errorf("apply: transfer validation error: %w", verr)
					}
				}
				res.PointsHits = spent
				if opts.Apply {
					cerr := client.CommitTransfers(*req)
					if cerr != nil {
						// Commit failed — FPL's commit is stricter than its
						// validation. Re-fetch, re-plan, re-validate, and
						// commit once more.
						freshCurrent, ferr := client.GetMyTeam(entryID)
						if ferr == nil {
							resuggested := PlanTransfers(freshCurrent, optimal, opts.MaxHits)
							suggestions = resuggested
							res.TransfersPlanned = len(suggestions)
							req = BuildTransferRequest(entryID, 1, suggestions, opts.Chip)
							if req != nil {
								if _, verr2 := PreviewPointsHits(ctx, client, *req); verr2 == nil {
									cerr = client.CommitTransfers(*req)
									if cerr == nil {
										res.TransfersMade = len(suggestions)
									}
								}
							}
						}
						if cerr != nil {
							return res, fmt.Errorf("apply: commit transfers failed: %w", cerr)
						}
					} else {
						res.TransfersMade = len(suggestions)
					}
				}
			}
		}
	}

	if !opts.SkipLineup {
		// Plan lineup from the optimal squad. After transfers above, the
		// players in optimal.Starters/Bench are now on the user's team, so
		// posting lineup picks that reference them succeeds.
		lineup := PlanLineup(optimal)
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

	if !opts.Apply {
		SortSuggestions(suggestions)
		_, _ = fmt.Fprintln(os.Stderr, RenderDiff(current, PlanLineup(optimal), suggestions, res.PointsHits))
	}

	return res, nil
}

// samePlayerSet reports whether two MyTeam snapshots reference the same 15
// player IDs (ignoring position/captain/etc). Used to detect FPL's partial-
// commit behaviour: the planner thinks transfers are needed but the server
// has already applied some of them between our fetch and our POST.
func samePlayerSet(a, b *api.MyTeam) bool {
	if len(a.Picks) != len(b.Picks) {
		return false
	}
	seen := make(map[int]bool, len(a.Picks))
	for _, p := range a.Picks {
		seen[p.Element] = true
	}
	for _, p := range b.Picks {
		if !seen[p.Element] {
			return false
		}
	}
	return true
}
