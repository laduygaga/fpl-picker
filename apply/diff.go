package apply

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"fpl-picker/api"
	"fpl-picker/model"
)

// RenderDiff produces a human-readable summary of every change the planner is
// about to apply: lineup swap (captain, formation, bench order), the
// transfer bundle, and a totals line.
//
// Output is a single string suitable for printing to a TTY or piping. The
// caller chooses the destination (stdout vs stderr) — this package does not
// write to either directly.
func RenderDiff(current *api.MyTeam, lineup LineupPlan, transfers []TransferSuggestion, pointsHits int) string {
	var b strings.Builder

	b.WriteString("Planned changes\n")
	b.WriteString("================\n\n")

	b.WriteString("Lineup\n")
	b.WriteString("------\n")
	b.WriteString(DiffLineup(current, lineup))
	b.WriteString("\n\n")

	b.WriteString("Transfers\n")
	b.WriteString("---------\n")
	if len(transfers) == 0 {
		b.WriteString("  (none)\n")
	} else {
		b.WriteString(renderTransfers(transfers))
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "Points hits: %d\n", pointsHits)
	if lineup.Formation != "" {
		fmt.Fprintf(&b, "Formation:   %s\n", lineup.Formation)
	}

	return b.String()
}

// renderTransfers formats the transfer list as a tab-aligned table:
//
//	OUT                  IN                  Score Δ   Net cost
//	Saka (ARS, MID, 9.5M) → Palmer (CHE, MID, 10.5M)  +0.12    +1.0M
func renderTransfers(transfers []TransferSuggestion) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "  OUT\tIN\tScore Δ\tNet cost")
	fmt.Fprintln(w, "  ---\t--\t-------\t--------")
	for _, t := range transfers {
		outStr := formatPlayer(t.Out.Player)
		inStr := formatPlayer(t.In.Player)
		net := float64(t.PurchasePrice-t.SellingPrice) / 10.0
		netStr := fmt.Sprintf("%+.1fM", net)
		if net == 0 {
			netStr = "0.0M"
		}
		fmt.Fprintf(w, "  %s\t%s\t%+0.3f\t%s\n",
			outStr, inStr, t.ScoreUplift, netStr)
	}
	_ = w.Flush()
	return b.String()
}

// formatPlayer returns "WebName (TEAM, POS, X.XM)" — same shape the display
// package uses elsewhere. WebName + TeamName are best-effort from the player
// struct alone; an empty WebName is replaced with the element ID so the
// diff is still readable.
func formatPlayer(p api.Player) string {
	name := p.WebName
	if name == "" {
		name = fmt.Sprintf("#%d", p.ID)
	}
	team := teamShort(p.Team)
	pos := model.PosName(p.ElementType)
	cost := float64(p.NowCost) / 10.0
	return fmt.Sprintf("%s (%s, %s, £%.1fM)", name, team, pos, cost)
}

// teamShort is a placeholder for team short-name lookup. The actual team
// short name lives in bootstrap.Teams; this helper is overridden in tests
// via package-level var to keep the surface area small.
var teamShort = func(teamID int) string {
	if teamID == 0 {
		return "??"
	}
	return fmt.Sprintf("T%d", teamID)
}

// SortSuggestions orders transfers by descending score uplift so the highest-
// impact swaps appear first in the diff.
func SortSuggestions(s []TransferSuggestion) {
	sort.SliceStable(s, func(i, j int) bool {
		return s[i].ScoreUplift > s[j].ScoreUplift
	})
}
