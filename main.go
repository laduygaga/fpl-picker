package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"fpl-picker/api"
	"fpl-picker/display"
	"fpl-picker/model"
)

const teamFile = ".fpl-team.txt"

func main() {
	budget := flag.Float64("budget", 100.0, "Total budget in £M (default: 100.0)")
	topN := flag.Int("top", 5, "Show top N players per position")
	diffN := flag.Int("diff", 10, "Show top N differential picks (low ownership)")
	diffMax := flag.Float64("diff-max", 10.0, "Max ownership %% for differentials")
	fresh := flag.Bool("fresh", false, "Clear cache and fetch fresh data")
	myTeam := flag.String("my-team", "", "Comma-separated player web names for comparison")
	saveTeam := flag.Bool("save-team", false, "Save -my-team to .fpl-team.txt for future runs")
	excluded := flag.String("excluded", "", "Comma-separated player web names to exclude from picks")
	excludedTeams := flag.String("excluded-teams", "", "Comma-separated team short names to exclude (e.g. ARS,MCI)")
	flag.Parse()

	teamNames := resolveTeamNames(*myTeam, *saveTeam)

	client := api.NewClient()

	if *fresh {
		fmt.Println("Clearing cache...")
		_ = client.ClearCache()
	}

	fmt.Println("Fetching FPL data...")

	bootstrap, err := client.FetchBootstrap()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fixtures, err := client.FetchFixtures()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded %d players, %d teams, %d fixtures\n",
		len(bootstrap.Elements), len(bootstrap.Teams), len(fixtures))

	scorer := model.NewScorer(bootstrap.Teams, fixtures, bootstrap.Events, bootstrap.Elements)
	scored := scorer.ScoreAll(bootstrap.Elements)

	fmt.Printf("Scoring %d eligible players for GW%d...\n", len(scored), scorer.NextEventID())

	if *excluded != "" {
		excludeSet := parseNames(*excluded)
		scored = filterExcluded(scored, excludeSet)
		fmt.Printf("Excluded %d players: %s\n", len(excludeSet), strings.Join(excludeSet, ", "))
	}

	if *excludedTeams != "" {
		teams := parseNames(*excludedTeams)
		before := len(scored)
		scored = filterExcludedTeams(scored, teams)
		fmt.Printf("Excluded %d players from teams: %s\n", before-len(scored), strings.Join(teams, ", "))
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	budgetTenths := int(*budget * 10)
	result := model.FindBestSquad(scored, budgetTenths)

	display.PrintSquad(result, scorer.NextEventID())

	if len(teamNames) > 0 {
		myPlayers := model.FindPlayersByName(scored, teamNames)
		if len(myPlayers) > 0 {
			display.PrintMySquad(myPlayers, result)
		} else {
			fmt.Println("No matching players found for your team.")
		}
	}

	if *topN > 0 {
		display.PrintTopByPosition(scored, *topN)
	}

	if *diffN > 0 {
		display.PrintDifferentials(scored, *diffMax, *diffN)
	}
}

func parseNames(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func filterExcluded(scored []model.ScoredPlayer, names []string) []model.ScoredPlayer {
	excl := make(map[string]bool, len(names))
	for _, n := range names {
		excl[strings.ToLower(n)] = true
	}
	out := make([]model.ScoredPlayer, 0, len(scored))
	for _, sp := range scored {
		if !excl[strings.ToLower(sp.Player.WebName)] {
			out = append(out, sp)
		}
	}
	return out
}

func filterExcludedTeams(scored []model.ScoredPlayer, teams []string) []model.ScoredPlayer {
	excl := make(map[string]bool, len(teams))
	for _, t := range teams {
		excl[strings.ToUpper(strings.TrimSpace(t))] = true
	}
	out := make([]model.ScoredPlayer, 0, len(scored))
	for _, sp := range scored {
		if !excl[strings.ToUpper(sp.TeamName)] {
			out = append(out, sp)
		}
	}
	return out
}

func resolveTeamNames(flagVal string, save bool) []string {
	if flagVal != "" {
		names := strings.Split(flagVal, ",")
		for i := range names {
			names[i] = strings.TrimSpace(names[i])
		}
		if save {
			_ = os.WriteFile(teamFile, []byte(strings.Join(names, "\n")), 0o644)
			fmt.Printf("Saved %d players to %s\n", len(names), teamFile)
		}
		return names
	}

	data, err := os.ReadFile(teamFile)
	if err != nil {
		return nil
	}

	var names []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	if len(names) > 0 {
		fmt.Printf("Loaded %d players from %s\n", len(names), teamFile)
	}
	return names
}
