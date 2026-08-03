// Package apply contains the business logic that turns an optimizer
// SquadResult into a concrete set of changes the user can preview and apply
// to their live FPL team: lineup planning, transfer planning, diff rendering,
// and the orchestration entry point.
package apply

import (
	"fmt"
	"sort"
	"strings"

	"fpl-picker/api"
	"fpl-picker/model"
)

// LineupPlan is the proposed 15-man squad in FPL slot order (1..11 starters,
// 12..15 bench). It mirrors the JSON shape of /api/my-team/'s `picks` array
// but includes the read-only fields the server will compute on the response.
type LineupPlan struct {
	Picks         []api.Pick
	CaptainID     int
	ViceCaptainID int
	Formation     string
	BenchOrder    []int // player IDs in bench slot order (positions 12..15)
}

// formationName maps the formation string returned by the optimizer to a
// display label. Returns "" when the formation is unknown.
func formationName(r model.SquadResult) string {
	return r.Formation
}

// PlanLineup packs a SquadResult into a 15-position LineupPlan.
//
//   - Positions 1..11 are starters, sorted by (element_type, score desc).
//   - Positions 12..15 are bench, sorted by cost asc within the optimizer's
//     bench order (the optimizer already gave us a valid bench).
//   - The captain and vice-captain come from the optimizer's picks.
//   - The bench order is recorded as a separate slice so the diff layer can
//     show the bench-slot delta cleanly.
func PlanLineup(result model.SquadResult) LineupPlan {
	plan := LineupPlan{
		Formation:     formationName(result),
		CaptainID:     result.Captain.Player.ID,
		ViceCaptainID: result.ViceCaptain.Player.ID,
	}

	// Starters — already sorted by position then score by the optimizer, but
	// we re-sort defensively in case the caller passes an unsorted result.
	starters := append([]model.ScoredPlayer(nil), result.Starters...)
	model.SortByPosAndScore(starters)

	bench := append([]model.ScoredPlayer(nil), result.Bench...)
	// Bench slot order: optimizer returns bench in fill order (cheapest first
	// when budget is tight, best-first when budget is generous). We preserve
	// that order rather than re-sorting by cost.
	for _, b := range bench {
		plan.BenchOrder = append(plan.BenchOrder, b.Player.ID)
	}

	picks := make([]api.Pick, 0, 15)

	// Starters occupy slots 1..11. Captain + vice-captain are stamped here.
	pos := 1
	for _, sp := range starters {
		p := api.Pick{
			Element:  sp.Player.ID,
			Position: pos,
		}
		if sp.Player.ID == plan.CaptainID {
			p.IsCaptain = true
		}
		if sp.Player.ID == plan.ViceCaptainID {
			p.IsViceCaptain = true
		}
		picks = append(picks, p)
		pos++
	}

	// Bench occupies slots 12..15 in optimizer order.
	for i, sp := range bench {
		picks = append(picks, api.Pick{
			Element:  sp.Player.ID,
			Position: pos,
		})
		_ = i
		pos++
	}

	plan.Picks = picks
	return plan
}

// toLineupUpdate converts a LineupPlan into the body POST'd to /api/my-team/.
func toLineupUpdate(plan LineupPlan, chip string) api.LineupUpdate {
	upd := api.LineupUpdate{Picks: plan.Picks}
	if chip != "" {
		upd.Chip = api.ChipPtr(chip)
	}
	return upd
}

// DiffLineup returns a human-readable, line-by-line comparison between the
// current FPL team and the proposed lineup. Returns "" when nothing changed.
//
// Sections:
//   - Captain / vice-captain swap
//   - Formation change
//   - Per-position player swaps (starters + bench)
//   - Bench-order changes
func DiffLineup(current *api.MyTeam, proposed LineupPlan) string {
	if current == nil {
		return "no current team available"
	}

	curByElement := map[int]api.Pick{}
	for _, p := range current.Picks {
		curByElement[p.Element] = p
	}
	newByElement := map[int]api.Pick{}
	for _, p := range proposed.Picks {
		newByElement[p.Element] = p
	}

	var b strings.Builder

	// Captain swap.
	curCap := findByPredicate(current.Picks, func(p api.Pick) bool { return p.IsCaptain })
	newCap := findByPredicate(proposed.Picks, func(p api.Pick) bool { return p.IsCaptain })
	if curCap != nil && newCap != nil && curCap.Element != newCap.Element {
		fmt.Fprintf(&b, "  Captain: %d → %d\n", curCap.Element, newCap.Element)
	}
	curVC := findByPredicate(current.Picks, func(p api.Pick) bool { return p.IsViceCaptain })
	newVC := findByPredicate(proposed.Picks, func(p api.Pick) bool { return p.IsViceCaptain })
	if curVC != nil && newVC != nil && curVC.Element != newVC.Element {
		fmt.Fprintf(&b, "  Vice-captain: %d → %d\n", curVC.Element, newVC.Element)
	}

	// Formation change (we only have the proposed one; skip if unknown).
	if proposed.Formation != "" {
		curFormation := deriveFormation(current.Picks)
		if curFormation != "" && curFormation != proposed.Formation {
			fmt.Fprintf(&b, "  Formation: %s → %s\n", curFormation, proposed.Formation)
		}
	}

	// Per-position changes for slots 1..15.
	for slot := 1; slot <= 15; slot++ {
		cur := findByPredicate(current.Picks, func(p api.Pick) bool { return p.Position == slot })
		next := findByPredicate(proposed.Picks, func(p api.Pick) bool { return p.Position == slot })
		if cur == nil || next == nil {
			continue
		}
		if cur.Element != next.Element {
			label := fmt.Sprintf("  Slot %2d", slot)
			if slot >= 12 {
				label = fmt.Sprintf("  Bench %d", slot-11)
			}
			fmt.Fprintf(&b, "%s: %d → %d\n", label, cur.Element, next.Element)
		}
	}

	// Bench-order changes (track movement within positions 12..15).
	curBenchOrder := benchOrder(current.Picks)
	if !equalIntSlice(curBenchOrder, proposed.BenchOrder) {
		fmt.Fprintf(&b, "  Bench order: %v → %v\n", curBenchOrder, proposed.BenchOrder)
	}

	if b.Len() == 0 {
		return "  (no lineup changes)"
	}
	return strings.TrimRight(b.String(), "\n")
}

func findByPredicate(picks []api.Pick, pred func(api.Pick) bool) *api.Pick {
	for i := range picks {
		if pred(picks[i]) {
			return &picks[i]
		}
	}
	return nil
}

// deriveFormation reads the FPL slot order and returns "X-Y-Z" for starters.
// Returns "" when the squad is not a valid 15-man team.
func deriveFormation(picks []api.Pick) string {
	gk, def, mid, fwd := 0, 0, 0, 0
	for _, p := range picks {
		if p.Position < 1 || p.Position > 11 {
			continue
		}
		// ElementType is set on GET responses; missing when caller only
		// passed a LineupUpdate-shaped struct. Best-effort: skip unknown.
		switch p.ElementType {
		case model.PosGK:
			gk++
		case model.PosDEF:
			def++
		case model.PosMID:
			mid++
		case model.PosFWD:
			fwd++
		}
	}
	if gk == 0 && def == 0 && mid == 0 && fwd == 0 {
		return ""
	}
	return fmt.Sprintf("%d-%d-%d-%d", gk, def, mid, fwd)
}

// benchOrder returns the player IDs in bench slot order (positions 12..15).
func benchOrder(picks []api.Pick) []int {
	out := []int{}
	for _, p := range picks {
		if p.Position >= 12 && p.Position <= 15 {
			out = append(out, p.Element)
		}
	}
	// Sort by slot so two reads with the same content compare equal.
	sort.SliceStable(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
