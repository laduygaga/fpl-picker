package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"

	"fpl-picker/api"
	"fpl-picker/apply"
	"fpl-picker/credentials"
	"fpl-picker/display"
	"fpl-picker/model"
)

const teamFile = ".fpl-team.txt"

func main() {
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		switch os.Args[1] {
		case "apply":
			applyMain(os.Args[2:])
			return
		}
	}
	pickMain(os.Args[1:])
}

func pickMain(args []string) {
	budget := flag.Float64("budget", 100.0, "Total budget in £M (default: 100.0)")
	topN := flag.Int("top", 5, "Show top N players per position")
	diffN := flag.Int("diff", 10, "Show top N differential picks (low ownership)")
	diffMax := flag.Float64("diff-max", 10.0, "Max ownership %% for differentials")
	fresh := flag.Bool("fresh", false, "Clear cache and fetch fresh data")
	myTeam := flag.String("my-team", "", "Comma-separated player web names for comparison")
	saveTeam := flag.Bool("save-team", false, "Save -my-team to .fpl-team.txt for future runs")
	excluded := flag.String("excluded", "", "Comma-separated player web names to exclude from picks")
	excludedTeams := flag.String("excluded-teams", "", "Comma-separated team short names to exclude (e.g. ARS,MCI)")
	formula := flag.String("formula", "1", "Scoring formula: 1/balanced, 2/attacker, 3/defender")
	_ = flag.CommandLine.Parse(args)

	teamNames := resolveTeamNames(*myTeam, *saveTeam)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := api.NewClient(ctx)

	if *fresh {
		fmt.Println("Clearing cache...")
		_ = client.ClearCache()
	}

	fmt.Println("Fetching FPL data...")

	bootstrap, fixtures, err := fetchData(client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded %d players, %d teams, %d fixtures\n",
		len(bootstrap.Elements), len(bootstrap.Teams), len(fixtures))

	scorer := model.NewScorer(bootstrap.Teams, fixtures, bootstrap.Events, bootstrap.Elements, *formula)
	f := model.GetFormula(*formula)
	fmt.Printf("Using formula: %s (FDR=%.0f%%, Pts=%.0f%%, Form=%.0f%%, XGI=%.0f%%, ICT=%.0f%%)\n",
		f.Name, f.FDR*100, f.Pts*100, f.Form*100, f.XGI*100, f.ICT*100)
	scored := scorer.ScoreAll(bootstrap.Elements)

	fmt.Printf("Scoring %d eligible players for GW%d...\n", len(scored), scorer.NextEventID())

	if *excluded != "" {
		excludeSet := parseNames(*excluded)
		scored = filterPlayers(scored, excludeSet, func(sp model.ScoredPlayer) string { return sp.Player.WebName }, strings.ToLower)
		fmt.Printf("Excluded %d players: %s\n", len(excludeSet), strings.Join(excludeSet, ", "))
	}

	if *excludedTeams != "" {
		teams := parseNames(*excludedTeams)
		before := len(scored)
		scored = filterPlayers(scored, teams, func(sp model.ScoredPlayer) string { return sp.TeamName }, strings.ToUpper)
		fmt.Printf("Excluded %d players from teams: %s\n", before-len(scored), strings.Join(teams, ", "))
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	budgetTenths := int(*budget * 10)
	fmt.Fprintln(os.Stderr, "Optimizing squad across 7 formations...")
	result := model.FindBestSquad(scored, budgetTenths, scorer.FixturePairings())
	fmt.Fprintln(os.Stderr, "Squad optimization complete.")

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

func applyMain(args []string) {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	emailFlag := fs.String("email", "", "FPL login email")
	passwordFlag := fs.String("password", "", "FPL login password")
	saveCacheFlag := fs.Bool("save-cache", false, "Save credentials encrypted to the credential cache")
	passphraseFlag := fs.String("passphrase", "", "Passphrase for the credential cache")
	clearCacheFlag := fs.Bool("clear-cache", false, "Delete the credential cache and exit")
	applyFlag := fs.Bool("apply", false, "Actually post changes; default is dry-run")
	chipFlag := fs.String("chip", "", "Activate chip: wildcard, freehit, bboost, or 3xc")
	noTransfersFlag := fs.Bool("no-transfers", false, "Skip transfer planning and posting")
	noLineupFlag := fs.Bool("no-lineup", false, "Skip lineup posting")
	maxHitsFlag := fs.Int("max-hits", 4, "Maximum points hits to accept")
	gwFlag := fs.Int("gw", 0, "Gameweek; default is the next gameweek")
	formulaFlag := fs.String("formula", "1", "Scoring formula passed through to the scorer")
	freshFlag := fs.Bool("fresh", false, "Bypass the FPL API cache")
	cookiesFlag := fs.String("cookies", "", "Cookie header value from a logged-in browser session (e.g. 'pl_profile=...; csrftoken=...'). Skips the login flow.")
	cookiesFileFlag := fs.String("cookies-file", "", "Path to a file containing cookies (same format as --cookies)")
	bearerFlag := fs.String("bearer", "", "OAuth/JWT access token from localStorage (sent as Authorization: Bearer). Skips the login flow.")
	bearerFileFlag := fs.String("bearer-file", "", "Path to a file containing the bearer token (whitespace-trimmed)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	if *clearCacheFlag {
		if err := credentials.Clear(); err != nil {
			fmt.Fprintf(os.Stderr, "Could not clear credential cache: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "Credential cache cleared.")
		return
	}

	if *maxHitsFlag < 0 {
		fmt.Fprintln(os.Stderr, "Validation failed: --max-hits must be non-negative.")
		os.Exit(1)
	}
	if *chipFlag != "" {
		validChips := map[string]bool{"wildcard": true, "freehit": true, "bboost": true, "3xc": true}
		if !validChips[*chipFlag] {
			fmt.Fprintf(os.Stderr, "Validation failed: unsupported chip %q.\n", *chipFlag)
			os.Exit(1)
		}
	}

	mode := "dry-run"
	if *applyFlag {
		mode = "APPLYING"
	}
	fmt.Fprintf(os.Stderr, "=== APPLY: %s ===\n", mode)

	scanner := bufio.NewScanner(os.Stdin)
	email := strings.TrimSpace(*emailFlag)
	password := *passwordFlag

	hasAltAuth := *bearerFlag != "" || *bearerFileFlag != "" ||
		*cookiesFlag != "" || *cookiesFileFlag != ""

	if email != "" && password != "" && *saveCacheFlag {
		passphrase := *passphraseFlag
		if passphrase == "" {
			// TODO: use golang.org/x/term in production for no-echo prompts.
			passphrase = readApplyPrompt(scanner, "Cache passphrase")
		}
		if err := credentials.Save(email, password, passphrase); err != nil {
			fmt.Fprintf(os.Stderr, "Could not save credentials: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "Credentials saved to encrypted cache.")
	} else if *saveCacheFlag && credentials.Exists() {
		passphrase := *passphraseFlag
		if passphrase == "" {
			// TODO: use golang.org/x/term in production for no-echo prompts.
			passphrase = readApplyPrompt(scanner, "Cache passphrase")
		}
		var err error
		email, password, err = credentials.Load(passphrase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not load credentials: %v\n", err)
			os.Exit(1)
		}
	} else if !hasAltAuth {
		if email == "" {
			email = strings.TrimSpace(readApplyPrompt(scanner, "FPL email"))
		}
		if password == "" {
			// TODO: use golang.org/x/term in production for no-echo prompts.
			password = readApplyPrompt(scanner, "FPL password")
		}
	}
	if !hasAltAuth && (email == "" || password == "") {
		fmt.Fprintln(os.Stderr, "Validation failed: email and password are required.")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := api.NewAuthClient(ctx)

	bearerSource := *bearerFlag
	if bearerSource == "" && *bearerFileFlag != "" {
		data, err := os.ReadFile(*bearerFileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not read bearer file: %v\n", err)
			os.Exit(1)
		}
		bearerSource = string(data)
	}
	if bearerSource != "" {
		if err := client.LoadBearer(bearerSource); err != nil {
			fmt.Fprintf(os.Stderr, "Could not load bearer token: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Loaded bearer token from %s.\n", cookieLabel(*bearerFlag, *bearerFileFlag))
	}

	cookieSource := *cookiesFlag
	if cookieSource == "" && *cookiesFileFlag != "" {
		data, err := os.ReadFile(*cookiesFileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not read cookies file: %v\n", err)
			os.Exit(1)
		}
		cookieSource = string(data)
	}
	if cookieSource != "" {
		n, err := client.LoadCookies(cookieSource)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not load cookies: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Loaded %d cookie(s) from %s.\n", n, cookieLabel(*cookiesFlag, *cookiesFileFlag))
	}

	if client.HasBearer() || cookieSource != "" {
		ok, err := client.IsLoggedIn()
		if err != nil || !ok {
			fmt.Fprintf(os.Stderr, "Auth invalid or expired (IsLoggedIn=%v err=%v).\n", ok, err)
			os.Exit(1)
		}
	} else {
		if err := client.Login(email, password); err != nil {
			fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Logged in as %s\n", email)
	}

	me, err := client.Me()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not discover team ID: %v\n", err)
		os.Exit(1)
	}
	teamID, err := strconv.Atoi(me.Entry.String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: invalid team ID %q: %v\n", me.Entry, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Team ID: %d\n", teamID)

	myTeam, err := client.GetMyTeam(teamID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not fetch current team: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Loaded current team with %d picks.\n", len(myTeam.Picks))

	apiClient := api.NewClient(ctx)
	if *freshFlag {
		fmt.Fprintln(os.Stderr, "Clearing API cache...")
		if err := apiClient.ClearCache(); err != nil {
			fmt.Fprintf(os.Stderr, "Could not clear API cache: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Fprintln(os.Stderr, "Fetching FPL data...")
	bootstrap, fixtures, err := fetchData(apiClient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	scorer := model.NewScorer(bootstrap.Teams, fixtures, bootstrap.Events, bootstrap.Elements, *formulaFlag)
	f := model.GetFormula(*formulaFlag)
	fmt.Fprintf(os.Stderr, "Using formula: %s (FDR=%.0f%%, Pts=%.0f%%, Form=%.0f%%, XGI=%.0f%%, ICT=%.0f%%)\n",
		f.Name, f.FDR*100, f.Pts*100, f.Form*100, f.XGI*100, f.ICT*100)
	scored := scorer.ScoreAll(bootstrap.Elements)
	fmt.Fprintf(os.Stderr, "Scoring %d eligible players for GW%d...\n", len(scored), scorer.NextEventID())
	sort.Slice(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
	optimized := model.FindBestSquad(scored, 1000, scorer.FixturePairings())
	if *gwFlag > 0 {
		fmt.Fprintf(os.Stderr, "Using requested gameweek %d for display; apply backend targets its detected next gameweek.\n", *gwFlag)
	}
	fmt.Fprintln(os.Stderr, "Squad optimization complete.")
	display.PrintSquad(optimized, scorer.NextEventID())

	options := apply.Options{
		Apply:         false,
		Chip:          *chipFlag,
		MaxHits:       *maxHitsFlag,
		SkipTransfers: *noTransfersFlag,
		SkipLineup:    *noLineupFlag,
	}
	preview, err := apply.Run(ctx, client, myTeam, optimized, options)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Apply planning failed: %v\n", err)
		os.Exit(1)
	}
	printApplyDiff(applyDiffOptions{
		team:          myTeam,
		optimized:     optimized,
		players:       bootstrap.Elements,
		maxHits:       *maxHitsFlag,
		chip:          *chipFlag,
		skipTransfers: *noTransfersFlag,
		skipLineup:    *noLineupFlag,
	})
	printApplyResult(preview, false)

	if !*applyFlag {
		return
	}
	if !confirmApply(scanner) {
		fmt.Fprintln(os.Stderr, "Apply cancelled; no changes were posted.")
		return
	}

	options.Apply = true
	result, err := apply.Run(ctx, client, myTeam, optimized, options)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Apply failed: %v\n", err)
		os.Exit(1)
	}
	printApplyResult(result, true)
}

func readApplyPrompt(scanner *bufio.Scanner, label string) string {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSuffix(scanner.Text(), "\r")
}

func cookieLabel(flagVal, fileFlagVal string) string {
	if fileFlagVal != "" {
		return fileFlagVal
	}
	return "--cookies"
}

func confirmApply(scanner *bufio.Scanner) bool {
	answer := strings.ToLower(strings.TrimSpace(readApplyPrompt(scanner, "Post these changes? [y/N]")))
	return answer == "y" || answer == "yes"
}

type applyDiffOptions struct {
	team          *api.MyTeam
	optimized     model.SquadResult
	players       []api.Player
	maxHits       int
	chip          string
	skipTransfers bool
	skipLineup    bool
}

func printApplyDiff(options applyDiffOptions) {
	team := options.team
	optimized := options.optimized
	players := options.players
	maxHits := options.maxHits
	chip := options.chip
	skipTransfers := options.skipTransfers
	skipLineup := options.skipLineup
	nameByID := make(map[int]string, len(players))
	for _, player := range players {
		nameByID[player.ID] = player.WebName
	}
	currentIDs := make(map[int]bool, len(team.Picks))
	for _, pick := range team.Picks {
		currentIDs[pick.Element] = true
	}
	targetIDs := make(map[int]bool, len(optimized.Starters)+len(optimized.Bench))
	for _, player := range optimized.Starters {
		targetIDs[player.Player.ID] = true
	}
	for _, player := range optimized.Bench {
		targetIDs[player.Player.ID] = true
	}

	var outgoing, incoming []string
	for id := range currentIDs {
		if !targetIDs[id] {
			outgoing = append(outgoing, fmt.Sprintf("%s (%d)", nameByID[id], id))
		}
	}
	for id := range targetIDs {
		if !currentIDs[id] {
			incoming = append(incoming, fmt.Sprintf("%s (%d)", nameByID[id], id))
		}
	}
	sort.Strings(outgoing)
	sort.Strings(incoming)

	currentCaptain := "none"
	currentVice := "none"
	for _, pick := range team.Picks {
		if pick.IsCaptain {
			currentCaptain = nameByID[pick.Element]
		}
		if pick.IsViceCaptain {
			currentVice = nameByID[pick.Element]
		}
	}
	fmt.Fprintln(os.Stderr, "\nPlanned changes (shown before any POST):")
	if skipLineup {
		fmt.Fprintln(os.Stderr, "  Lineup: skipped")
	} else {
		fmt.Fprintf(os.Stderr, "  Captain: %s -> %s\n", currentCaptain, optimized.Captain.Player.WebName)
		fmt.Fprintf(os.Stderr, "  Vice-captain: %s -> %s\n", currentVice, optimized.ViceCaptain.Player.WebName)
		lineupChange := "would change"
		if len(outgoing) == 0 && len(incoming) == 0 && currentCaptain == optimized.Captain.Player.WebName {
			lineupChange = "no changes detected"
		}
		fmt.Fprintf(os.Stderr, "  Lineup: %s\n", lineupChange)
	}
	if skipTransfers {
		fmt.Fprintln(os.Stderr, "  Transfers: skipped")
	} else {
		fmt.Fprintf(os.Stderr, "  Transfers out: %s\n", strings.Join(outgoing, ", "))
		fmt.Fprintf(os.Stderr, "  Transfers in: %s\n", strings.Join(incoming, ", "))
		fmt.Fprintf(os.Stderr, "  Points hits: capped at %d\n", maxHits)
	}
	if chip == "" {
		fmt.Fprintln(os.Stderr, "  Chip: none")
	} else {
		fmt.Fprintf(os.Stderr, "  Chip: %s\n", chip)
	}
}

func printApplyResult(result *apply.Result, committed bool) {
	mode := "dry-run"
	if committed {
		mode = "applied"
	}
	fmt.Fprintf(os.Stdout, "\nApply summary (%s)\n", mode)
	fmt.Fprintln(os.Stdout, "-------------------")
	fmt.Fprintf(os.Stdout, "Lineup changed:    %t\n", result.LineupChanged)
	fmt.Fprintf(os.Stdout, "Transfers planned: %d\n", result.TransfersPlanned)
	fmt.Fprintf(os.Stdout, "Transfers made:    %d\n", result.TransfersMade)
	fmt.Fprintf(os.Stdout, "Points hits:       %d\n", result.PointsHits)
	fmt.Fprintf(os.Stdout, "Bank:              %d\n", result.Bank)
	fmt.Fprintf(os.Stdout, "Squad value:       %d\n", result.SquadValue)
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

func filterPlayers(scored []model.ScoredPlayer, excluded []string, key func(model.ScoredPlayer) string, norm func(string) string) []model.ScoredPlayer {
	excl := make(map[string]bool, len(excluded))
	for _, e := range excluded {
		excl[norm(e)] = true
	}
	out := make([]model.ScoredPlayer, 0, len(scored))
	for _, sp := range scored {
		if !excl[norm(key(sp))] {
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
	for line := range strings.SplitSeq(string(data), "\n") {
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

// fetchData loads bootstrap-static and fixtures concurrently.
// errCh is buffered (cap 2) so a fast-failing goroutine never blocks on send
// while the other fetch is still in flight.
func fetchData(client *api.Client) (*api.BootstrapResponse, []api.Fixture, error) {
	var (
		bootstrap *api.BootstrapResponse
		fixtures  []api.Fixture
		wg        sync.WaitGroup
		errs      = make(chan error, 2)
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		b, err := client.FetchBootstrap()
		if err != nil {
			errs <- err
			return
		}
		bootstrap = b
	}()
	go func() {
		defer wg.Done()
		f, err := client.FetchFixtures()
		if err != nil {
			errs <- err
			return
		}
		fixtures = f
	}()
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return nil, nil, err
		}
	}
	return bootstrap, fixtures, nil
}
