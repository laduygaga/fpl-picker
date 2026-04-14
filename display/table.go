package display

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"fpl-picker/model"
)

// PrintSquad renders the optimized squad with opponent-conditioned scoring.
func PrintSquad(result model.SquadResult, nextGW int) {
	if len(result.Starters) == 0 {
		fmt.Println("No valid squad could be assembled with the given budget.")
		return
	}

	fmt.Println()
	fmt.Println(strings.Repeat("═", 120))
	fmt.Printf("  GW%d — XI-FIRST | OPPONENT-CONDITIONED SCORING\n", nextGW)
	fmt.Printf("  FDR 30%% | TotalPts 20%% | Opponent Quality 20%% | Form 15%% | EP 5%% | PPG 5%% | xGI/90 3%% | ICT/90 2%%\n")
	fmt.Printf("  Budget: £%.1fM  |  XI Cost: £%.1fM  |  Squad Cost: £%.1fM  |  Formation: %s\n",
		result.Budget, result.XICost, result.TotalCost, result.Formation)
	fmt.Printf("  CAPTAIN: %s (%.3f)  |  VICE: %s (%.3f)\n",
		result.Captain.Player.WebName, result.Captain.Score,
		result.ViceCaptain.Player.WebName, result.ViceCaptain.Score)
	fmt.Println(strings.Repeat("─", 120))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		"POS", "PLAYER", "TEAM", "COST", "SCORE", "EP", "FORM", "OPP-Q", "OPPONENT PROFILE")
	fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		"───", "──────────────────", "────", "────", "─────", "────", "────", "─────", "─────────────────────────")

	currentPos := 0
	for _, sp := range result.Starters {
		if sp.Player.ElementType != currentPos {
			if currentPos != 0 {
				fmt.Fprintln(w)
			}
			currentPos = sp.Player.ElementType
		}

		badge := " "
		if sp.Player.ID == result.Captain.Player.ID {
			badge = "©"
		} else if sp.Player.ID == result.ViceCaptain.Player.ID {
			badge = "V"
		} else if sp.IsDGW {
			badge = "*"
		}

		oq := fmt.Sprintf("%.0f%%", sp.OppScore*100)
		fmt.Fprintf(w, "  %s\t%s %s\t%s\t£%.1fM\t%.3f\t%.1f\t%.1f\t%s\t%s\n",
			sp.PositionName,
			sp.Player.WebName, badge,
			sp.TeamName,
			float64(sp.Player.NowCost)/10.0,
			sp.Score,
			sp.EPNextVal,
			sp.FormVal,
			oq,
			sp.OppDesc,
		)
	}
	w.Flush()

	fmt.Println(strings.Repeat("─", 120))
	fmt.Printf("  Starting XI Score: %.3f\n", result.TotalScore)
	fmt.Println()

	fmt.Println("  BENCH (best within budget)")
	fmt.Println("  " + strings.Repeat("─", 70))
	wb := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for i, sp := range result.Bench {
		fmt.Fprintf(wb, "    %d\t%s\t%s\t%s\t£%.1fM\t%.3f\t%s\n",
			i+1,
			sp.PositionName,
			sp.Player.WebName,
			sp.TeamName,
			float64(sp.Player.NowCost)/10.0,
			sp.Score,
			sp.OppDesc,
		)
	}
	wb.Flush()
	fmt.Println()

	var warnings []string
	for _, sp := range result.Starters {
		if sp.Player.Status == "d" {
			pct := ""
			if sp.Player.ChanceOfPlayingNextRound != nil {
				pct = fmt.Sprintf(" (%d%% chance)", *sp.Player.ChanceOfPlayingNextRound)
			}
			news := sp.Player.News
			if news == "" {
				news = "Doubtful"
			}
			warnings = append(warnings, fmt.Sprintf("  ⚠ %s%s — %s", sp.Player.WebName, pct, news))
		}
	}
	if len(warnings) > 0 {
		fmt.Println("  AVAILABILITY WARNINGS")
		fmt.Println("  " + strings.Repeat("─", 70))
		for _, w := range warnings {
			fmt.Println(w)
		}
		fmt.Println()
	}
}

// PrintMySquad renders the user's current squad with scores and suggests transfers.
func PrintMySquad(myPlayers []model.ScoredPlayer, optimal model.SquadResult) {
	if len(myPlayers) == 0 {
		return
	}

	myXI, myFm, myScore := model.BestXIFromSquad(myPlayers)
	model.SortByPosAndScore(myXI)

	fmt.Printf("\n%s\n  YOUR XI (%s) — Score: %.3f  |  OPTIMAL — Score: %.3f  |  GAP: %.3f\n%s\n",
		strings.Repeat("═", 120), myFm, myScore, optimal.TotalScore, optimal.TotalScore-myScore, strings.Repeat("─", 120))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	currentPos := 0
	for _, sp := range myXI {
		if sp.Player.ElementType != currentPos {
			if currentPos != 0 {
				fmt.Fprintln(w)
			}
			currentPos = sp.Player.ElementType
		}
		oq := fmt.Sprintf("%.0f%%", sp.OppScore*100)
		fmt.Fprintf(w, "  %s\t%s\t%s\t£%.1fM\t%.3f\tEP:%.1f\tForm:%.1f\tOPP-Q:%s\t%s\n",
			sp.PositionName, sp.Player.WebName, sp.TeamName,
			float64(sp.Player.NowCost)/10.0, sp.Score, sp.EPNextVal, sp.FormVal, oq, sp.OppDesc)
	}
	w.Flush()

	// Transfer suggestions: weakest 3 players → best affordable replacements
	sort.Slice(myPlayers, func(i, j int) bool { return myPlayers[i].Score < myPlayers[j].Score })
	fmt.Printf("\n%s\n  TRANSFER TARGETS (opponent-aware)\n%s\n",
		strings.Repeat("═", 120), strings.Repeat("─", 120))

	printTransferTargets(myPlayers, optimal.Starters)
}

func printTransferTargets(weakest []model.ScoredPlayer, pool []model.ScoredPlayer) {
	allScored := pool
	limit := min(3, len(weakest))

	for _, weak := range weakest[:limit] {
		pos := weak.Player.ElementType
		costLim := float64(weak.Player.NowCost) + 50

		fmt.Printf("\n  OUT: %-14s %-5s £%.1fM  Score:%.3f  EP:%.1f  OPP-Q:%.0f%%  %s\n",
			weak.Player.WebName, weak.TeamName, float64(weak.Player.NowCost)/10,
			weak.Score, weak.EPNextVal, weak.OppScore*100, weak.OppDesc)

		cnt := 0
		for _, c := range allScored {
			if c.Player.ElementType != pos || float64(c.Player.NowCost) > costLim {
				continue
			}
			if c.Player.ID == weak.Player.ID || c.Score <= weak.Score {
				continue
			}
			fmt.Printf("   IN: %-14s %-5s £%.1fM  Score:%.3f  EP:%.1f  OPP-Q:%.0f%%  (+%.3f)  %s\n",
				c.Player.WebName, c.TeamName, float64(c.Player.NowCost)/10,
				c.Score, c.EPNextVal, c.OppScore*100, c.Score-weak.Score, c.OppDesc)
			cnt++
			if cnt >= 2 {
				break
			}
		}
	}
	fmt.Println()
}

// PrintTopByPosition shows the top N players for each position.
func PrintTopByPosition(players []model.ScoredPlayer, n int) {
	fmt.Println()
	fmt.Println(headerLine("TOP PICKS BY POSITION"))

	byPos := map[int][]model.ScoredPlayer{}
	for _, p := range players {
		byPos[p.Player.ElementType] = append(byPos[p.Player.ElementType], p)
	}

	posOrder := []int{model.PosGK, model.PosDEF, model.PosMID, model.PosFWD}

	for _, pos := range posOrder {
		pool := byPos[pos]
		if len(pool) == 0 {
			continue
		}

		limit := min(n, len(pool))

		posLabel := posFullName(pos)
		fmt.Printf("\n  %s\n", posLabel)
		fmt.Printf("  %s\n", strings.Repeat("─", len(posLabel)+4))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "    %s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			"#", "PLAYER", "TEAM", "COST", "SCORE", "OPP-Q", "VALUE", "OPPONENT")

		for i := 0; i < limit; i++ {
			sp := pool[i]
			oq := fmt.Sprintf("%.0f%%", sp.OppScore*100)
			fmt.Fprintf(w, "    %d\t%s\t%s\t£%.1fM\t%.3f\t%s\t%.3f\t%s\n",
				i+1,
				sp.Player.WebName,
				sp.TeamName,
				float64(sp.Player.NowCost)/10.0,
				sp.Score,
				oq,
				sp.ValueRating,
				sp.OppDesc,
			)
		}
		w.Flush()
	}

	fmt.Println()
}

// PrintDifferentials shows high-scoring players with low ownership.
func PrintDifferentials(players []model.ScoredPlayer, maxOwnership float64, n int) {
	fmt.Println()
	fmt.Println(headerLine("DIFFERENTIALS (Low Ownership, High Score)"))

	var diffs []model.ScoredPlayer
	for _, p := range players {
		if p.OwnershipPct <= maxOwnership && p.OwnershipPct > 0 {
			diffs = append(diffs, p)
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Score > diffs[j].Score
	})

	if len(diffs) > n {
		diffs = diffs[:n]
	}

	if len(diffs) == 0 {
		fmt.Printf("  No differentials found under %.0f%% ownership.\n\n", maxOwnership)
		return
	}

	fmt.Printf("  Players under %.0f%% ownership, ranked by score:\n", maxOwnership)
	fmt.Println("  " + strings.Repeat("─", 90))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "    %s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		"#", "POS", "PLAYER", "TEAM", "COST", "SCORE", "OWN%", "OPPONENT")

	for i, sp := range diffs {
		fmt.Fprintf(w, "    %d\t%s\t%s\t%s\t£%.1fM\t%.3f\t%.1f%%\t%s\n",
			i+1,
			sp.PositionName,
			sp.Player.WebName,
			sp.TeamName,
			float64(sp.Player.NowCost)/10.0,
			sp.Score,
			sp.OwnershipPct,
			sp.OppDesc,
		)
	}
	w.Flush()
	fmt.Println()
}

func headerLine(title string) string {
	width := 120
	pad := max(0, (width-len(title)-4)/2)
	return fmt.Sprintf("%s  %s  %s",
		strings.Repeat("═", pad), title, strings.Repeat("═", width-pad-len(title)-4))
}

func posFullName(pos int) string {
	switch pos {
	case model.PosGK:
		return "GOALKEEPERS"
	case model.PosDEF:
		return "DEFENDERS"
	case model.PosMID:
		return "MIDFIELDERS"
	case model.PosFWD:
		return "FORWARDS"
	default:
		return "UNKNOWN"
	}
}
