package model

import (
	"math"
	"sort"
)

// Formation defines a valid starting XI shape.
type Formation struct {
	Name string
	GK   int
	DEF  int
	MID  int
	FWD  int
}

// ValidFormations lists all legal FPL starting formations.
var ValidFormations = []Formation{
	{"3-4-3", 1, 3, 4, 3},
	{"3-5-2", 1, 3, 5, 2},
	{"4-3-3", 1, 4, 3, 3},
	{"4-4-2", 1, 4, 4, 2},
	{"4-5-1", 1, 4, 5, 1},
	{"5-3-2", 1, 5, 3, 2},
	{"5-4-1", 1, 5, 4, 1},
}

var squadSlots = map[int]int{
	PosGK:  2,
	PosDEF: 5,
	PosMID: 5,
	PosFWD: 3,
}

// SquadResult holds the optimizer output.
type SquadResult struct {
	Formation   string
	Starters    []ScoredPlayer
	Bench       []ScoredPlayer
	Captain     ScoredPlayer
	ViceCaptain ScoredPlayer
	TotalScore  float64
	XICost      float64 // starting XI cost in £M
	TotalCost   float64 // full 15-man squad in £M
	Budget      float64 // in £M
}

// posCounts packs position counts for starters and bench into a single uint64.
// Layout: sGK(3) | sDEF(3) | sMID(3) | sFWD(3) | bGK(3) | bDEF(3) | bMID(3) | bFWD(3) = 24 bits
type posCounts uint32

func newPosCounts(sGK, sDEF, sMID, sFWD, bGK, bDEF, bMID, bFWD int) posCounts {
	return posCounts(sGK&7 | (sDEF&7)<<3 | (sMID&7)<<6 | (sFWD&7)<<9 |
		(bGK&7)<<12 | (bDEF&7)<<15 | (bMID&7)<<18 | (bFWD&7)<<21)
}

func (pc posCounts) sGK() int  { return int(pc) & 7 }
func (pc posCounts) sDEF() int { return int(pc>>3) & 7 }
func (pc posCounts) sMID() int { return int(pc>>6) & 7 }
func (pc posCounts) sFWD() int { return int(pc>>9) & 7 }
func (pc posCounts) bGK() int  { return int(pc>>12) & 7 }
func (pc posCounts) bDEF() int { return int(pc>>15) & 7 }
func (pc posCounts) bMID() int { return int(pc>>18) & 7 }
func (pc posCounts) bFWD() int { return int(pc>>21) & 7 }

func (pc posCounts) add(other posCounts) posCounts {
	return newPosCounts(
		pc.sGK()+other.sGK(), pc.sDEF()+other.sDEF(),
		pc.sMID()+other.sMID(), pc.sFWD()+other.sFWD(),
		pc.bGK()+other.bGK(), pc.bDEF()+other.bDEF(),
		pc.bMID()+other.bMID(), pc.bFWD()+other.bFWD(),
	)
}

func (pc posCounts) fitsWithin(target posCounts) bool {
	return pc.sGK() <= target.sGK() && pc.sDEF() <= target.sDEF() &&
		pc.sMID() <= target.sMID() && pc.sFWD() <= target.sFWD() &&
		pc.bGK() <= target.bGK() && pc.bDEF() <= target.bDEF() &&
		pc.bMID() <= target.bMID() && pc.bFWD() <= target.bFWD()
}

func maxPosCounts(a, b posCounts) posCounts {
	return newPosCounts(
		max(a.sGK(), b.sGK()), max(a.sDEF(), b.sDEF()),
		max(a.sMID(), b.sMID()), max(a.sFWD(), b.sFWD()),
		0, 0, 0, 0,
	)
}

func canReachTarget(current, remaining, target posCounts) bool {
	return current.sGK()+remaining.sGK() >= target.sGK() &&
		current.sDEF()+remaining.sDEF() >= target.sDEF() &&
		current.sMID()+remaining.sMID() >= target.sMID() &&
		current.sFWD()+remaining.sFWD() >= target.sFWD()
}

// teamOption represents one way to select 0–3 players from a single team.
type teamOption struct {
	counts  posCounts
	cost    int
	score   float64
	players []ScoredPlayer
}

type dpNode struct {
	cost  int
	score float64
}

func solveDP(teamIDs []int, byTeam map[int][]ScoredPlayer, target posCounts, budget int, fixturePairings map[int][]FixturePairing) (
	starters []ScoredPlayer, score float64, cost int, ok bool,
) {
	teamIdx := map[int]int{}
	for i, id := range teamIDs {
		teamIdx[id] = i
	}

	allOpts := make([][]teamOption, len(teamIDs))
	for i, teamID := range teamIDs {
		allOpts[i] = generateTeamOptions(byTeam[teamID], target)
	}

	if fixturePairings != nil {
		applyClashDiscounts(teamIDs, allOpts, byTeam, fixturePairings)
	}

	// Precompute max position contribution from teams[ti+1:] for reachability pruning.
	maxAfter := make([]posCounts, len(teamIDs)+1)
	for ti := len(teamIDs) - 1; ti >= 0; ti-- {
		best := maxAfter[ti+1]
		for _, opt := range allOpts[ti] {
			best = maxPosCounts(best, maxAfter[ti+1].add(opt.counts))
		}
		maxAfter[ti] = best
	}

	stages := make([]map[posCounts][]dpNode, len(teamIDs)+1)
	stages[0] = map[posCounts][]dpNode{
		0: {{cost: 0, score: 0}},
	}

	for ti := range teamIDs {
		opts := allOpts[ti]
		prev := stages[ti]
		next := map[posCounts][]dpNode{}
		reachable := maxAfter[ti+1]

		// Sort map keys for deterministic iteration order.
		sortedStates := make([]posCounts, 0, len(prev))
		for s := range prev {
			sortedStates = append(sortedStates, s)
		}
		sort.Slice(sortedStates, func(i, j int) bool { return sortedStates[i] < sortedStates[j] })

		for _, state := range sortedStates {
			nodes := prev[state]
			if !canReachTarget(state, reachable, target) {
				continue
			}

			// Try picking players from this team.
			pickedAny := false
			for _, opt := range opts {
				ns := state.add(opt.counts)
				if !ns.fitsWithin(target) {
					continue
				}
				for _, n := range nodes {
					nc := n.cost + opt.cost
					if nc > budget {
						continue
					}
					addToFrontier(next, ns, dpNode{cost: nc, score: n.score + opt.score})
					pickedAny = true
				}
			}

			// Skip transition: propagate state unchanged if we didn't pick any players.
			// This allows DP to skip teams entirely.
			if !pickedAny {
				for _, n := range nodes {
					addToFrontier(next, state, dpNode{cost: n.cost, score: n.score})
				}
			}
		}

		stages[ti+1] = next
	}

	finalFrontier, exists := stages[len(teamIDs)][target]
	if !exists {
		return nil, 0, 0, false
	}

	bestScore := -1.0
	bestCost := 0
	for _, n := range finalFrontier {
		if n.score > bestScore {
			bestScore = n.score
			bestCost = n.cost
		}
	}

	// Backtrack through saved stages to reconstruct the chosen players.
	remainScore := bestScore
	remainCost := bestCost
	remainState := target

	for ti := len(teamIDs) - 1; ti >= 0; ti-- {
		opts := allOpts[ti]
		prevStage := stages[ti]
		found := false

		for oi, opt := range opts {
			prevState := remainState
			prevState = subtractPosCounts(prevState, opt.counts)
			if prevState == posCounts(math.MaxUint32) {
				continue
			}

			prevNodes, ok := prevStage[prevState]
			if !ok {
				continue
			}

			wantCost := remainCost - opt.cost
			wantScore := remainScore - opt.score

			for _, pn := range prevNodes {
				if pn.cost == wantCost && math.Abs(pn.score-wantScore) < 1e-9 {
					starters = append(starters, allOpts[ti][oi].players...)
					remainScore = wantScore
					remainCost = wantCost
					remainState = prevState
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		// Fallback: team was skipped (0 players, state/cost/score unchanged).
		if !found {
			if prevNodes, has := prevStage[remainState]; has {
				for _, pn := range prevNodes {
					if pn.cost == remainCost && math.Abs(pn.score-remainScore) < 1e-9 {
						found = true
						break
					}
				}
			}
		}
	}

	return starters, bestScore, bestCost, true
}

// applyClashDiscounts discounts GK/DEF options from teams whose GW opponent
// has viable MID/FWD in the pool. Penalty = clashWeight × (FDR/5) × player.Score,
// where FDR (1–5) is the fixture difficulty from the FPL API. Harder opponents
// (higher FDR) produce a larger penalty. MID/FWD are NOT penalised — in FPL,
// attackers still earn points regardless of who they face.
func applyClashDiscounts(teamIDs []int, allOpts [][]teamOption, byTeam map[int][]ScoredPlayer, pairings map[int][]FixturePairing) {
	const clashWeight = 0.50

	type clashInfo struct {
		hasOppAtk bool
		maxFDR    int
	}
	teamClash := map[int]clashInfo{}

	for _, teamID := range teamIDs {
		ci := clashInfo{}
		for _, fp := range pairings[teamID] {
			for _, p := range byTeam[fp.OpponentID] {
				if p.Player.ElementType == PosMID || p.Player.ElementType == PosFWD {
					ci.hasOppAtk = true
					if fp.Difficulty > ci.maxFDR {
						ci.maxFDR = fp.Difficulty
					}
					break
				}
			}
		}
		if ci.hasOppAtk {
			teamClash[teamID] = ci
		}
	}

	for i, teamID := range teamIDs {
		ci, ok := teamClash[teamID]
		if !ok {
			continue
		}
		fdrScale := float64(ci.maxFDR) / 5.0
		for j := range allOpts[i] {
			opt := &allOpts[i][j]
			penalty := 0.0
			for _, p := range opt.players {
				if p.Player.ElementType == PosGK || p.Player.ElementType == PosDEF {
					penalty += clashWeight * fdrScale * p.Score
				}
			}
			opt.score -= penalty
		}
	}
}

func subtractPosCounts(a, b posCounts) posCounts {
	sGK := a.sGK() - b.sGK()
	sDEF := a.sDEF() - b.sDEF()
	sMID := a.sMID() - b.sMID()
	sFWD := a.sFWD() - b.sFWD()
	if sGK < 0 || sDEF < 0 || sMID < 0 || sFWD < 0 {
		return posCounts(math.MaxUint32)
	}
	return newPosCounts(sGK, sDEF, sMID, sFWD, 0, 0, 0, 0)
}

// FindBestSquad builds an optimal 15-man FPL squad using team-grouped DP.
//
// The optimizer enumerates valid player selections (0–3) per real-world team,
// then DPs across all teams to find the highest-scoring XI that fits the budget,
// position requirements, and team limits. A soft clash penalty discourages
// picking attackers from one team and defenders from the opposing team in the
// same GW fixture.
//
// fixturePairings maps teamID → []FixturePairing for the current GW.
// Pass nil to disable clash penalties.
func FindBestSquad(players []ScoredPlayer, budgetTenths int, fixturePairings map[int][]FixturePairing) SquadResult {
	byTeam := map[int][]ScoredPlayer{}
	for _, p := range players {
		byTeam[p.Player.Team] = append(byTeam[p.Player.Team], p)
	}

	teamIDs := make([]int, 0, len(byTeam))
	for id := range byTeam {
		teamIDs = append(teamIDs, id)
	}
	sort.Ints(teamIDs)

	byPos := map[int][]ScoredPlayer{
		PosGK: {}, PosDEF: {}, PosMID: {}, PosFWD: {},
	}
	for _, p := range players {
		byPos[p.Player.ElementType] = append(byPos[p.Player.ElementType], p)
	}

	var bestResult SquadResult
	bestObj := -math.MaxFloat64

	for _, fm := range ValidFormations {
		targetStarters := newPosCounts(fm.GK, fm.DEF, fm.MID, fm.FWD, 0, 0, 0, 0)

		benchReserve := estimateBenchCost(byPos, fm)

		dpBudget := budgetTenths - benchReserve
		if dpBudget <= 0 {
			continue
		}

		xi, xiScore, xiCost, ok := solveDP(teamIDs, byTeam, targetStarters, dpBudget, fixturePairings)
		if !ok {
			continue
		}

		obj := xiScore

		if obj > bestObj {
			bestObj = obj

			SortByPosAndScore(xi)
			captain, viceCaptain := pickCaptains(xi)

			xiIDs := map[int]bool{}
			teamCount := map[int]int{}
			for _, p := range xi {
				xiIDs[p.Player.ID] = true
				teamCount[p.Player.Team]++
			}
			xiPosCounts := map[int]int{}
			for _, p := range xi {
				xiPosCounts[p.Player.ElementType]++
			}
			bench := fillBench(byPos, xiIDs, teamCount, xiPosCounts, float64(budgetTenths-xiCost))

			totalCost := xiCost
			for _, p := range bench {
				totalCost += p.Player.NowCost
			}

			// Recompute raw XI score (xiScore may include clash discounts).
			rawScore := 0.0
			for _, p := range xi {
				rawScore += p.Score
			}

			bestResult = SquadResult{
				Formation:   fm.Name,
				Starters:    xi,
				Bench:       bench,
				Captain:     captain,
				ViceCaptain: viceCaptain,
				TotalScore:  rawScore,
				XICost:      float64(xiCost) / 10.0,
				TotalCost:   float64(totalCost) / 10.0,
				Budget:      float64(budgetTenths) / 10.0,
			}
		}
	}

	return bestResult
}

// estimateBenchCost estimates the minimum cost to fill bench slots for a formation
// by summing the cheapest available player per position needed.
func estimateBenchCost(byPos map[int][]ScoredPlayer, fm Formation) int {
	benchNeeds := map[int]int{
		PosGK:  squadSlots[PosGK] - fm.GK,
		PosDEF: squadSlots[PosDEF] - fm.DEF,
		PosMID: squadSlots[PosMID] - fm.MID,
		PosFWD: squadSlots[PosFWD] - fm.FWD,
	}

	total := 0
	for _, pos := range []int{PosGK, PosDEF, PosMID, PosFWD} {
		need := benchNeeds[pos]
		sorted := sortedByCostAsc(byPos[pos])
		for i := 0; i < need && i < len(sorted); i++ {
			total += sorted[i].Player.NowCost
		}
	}
	return total
}

// addToFrontier adds a node to the Pareto frontier for a given state.
// Dominance: node A dominates B if A.cost <= B.cost && A.score >= B.score (and at least one strict).
func addToFrontier(dp map[posCounts][]dpNode, state posCounts, node dpNode) {
	existing := dp[state]

	for _, e := range existing {
		if e.cost <= node.cost && e.score >= node.score {
			return
		}
	}

	kept := existing[:0]
	for _, e := range existing {
		if !(node.cost <= e.cost && node.score >= e.score) {
			kept = append(kept, e)
		}
	}

	kept = append(kept, node)
	dp[state] = kept
}

// generateTeamOptions enumerates all valid ways to pick 0–3 players from one team.
// Each player can be assigned as starter or bench.
// Only options that don't exceed target position counts are generated.
func generateTeamOptions(pool []ScoredPlayer, target posCounts) []teamOption {
	if len(pool) == 0 {
		return nil
	}

	posPool := map[int][]ScoredPlayer{}
	for _, p := range pool {
		posPool[p.Player.ElementType] = append(posPool[p.Player.ElementType], p)
	}
	var limited []ScoredPlayer
	for _, pos := range []int{PosGK, PosDEF, PosMID, PosFWD} {
		pp := posPool[pos]
		sort.Slice(pp, func(i, j int) bool { return pp[i].Score > pp[j].Score })
		limit := 3
		if len(pp) < limit {
			limit = len(pp)
		}
		limited = append(limited, pp[:limit]...)
	}

	var opts []teamOption

	n := len(limited)
	for size := 1; size <= 3 && size <= n; size++ {
		enumerateStarterSubsets(limited, size, 0, nil, target, &opts)
	}

	return pruneTeamOptions(opts)
}

func enumerateStarterSubsets(pool []ScoredPlayer, size, start int, current []int, target posCounts, opts *[]teamOption) {
	if len(current) == size {
		var players []ScoredPlayer
		sGK, sDEF, sMID, sFWD := 0, 0, 0, 0
		totalCost := 0
		totalScore := 0.0

		for _, idx := range current {
			p := pool[idx]
			players = append(players, p)
			totalCost += p.Player.NowCost
			totalScore += p.Score

			switch p.Player.ElementType {
			case PosGK:
				sGK++
			case PosDEF:
				sDEF++
			case PosMID:
				sMID++
			case PosFWD:
				sFWD++
			}
		}

		counts := newPosCounts(sGK, sDEF, sMID, sFWD, 0, 0, 0, 0)
		if !counts.fitsWithin(target) {
			return
		}

		*opts = append(*opts, teamOption{
			counts:  counts,
			cost:    totalCost,
			score:   totalScore,
			players: players,
		})
		return
	}
	for i := start; i < len(pool); i++ {
		enumerateStarterSubsets(pool, size, i+1, append(current, i), target, opts)
	}
}

// pruneTeamOptions removes dominated options within the same position count signature.
func pruneTeamOptions(opts []teamOption) []teamOption {
	groups := map[posCounts][]teamOption{}
	for _, o := range opts {
		groups[o.counts] = append(groups[o.counts], o)
	}

	var pruned []teamOption
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			if group[i].cost != group[j].cost {
				return group[i].cost < group[j].cost
			}
			return group[i].score > group[j].score
		})

		bestScore := -1.0
		for _, o := range group {
			if o.score > bestScore {
				pruned = append(pruned, o)
				bestScore = o.score
			}
		}
	}
	return pruned
}

// clashPenalty computes the soft penalty for head-to-head conflicts in the XI.
// Penalizes picking MID/FWD from team A and GK/DEF from team B when A plays B.
func clashPenalty(xi []ScoredPlayer, pairings map[int][]FixturePairing) float64 {
	if pairings == nil {
		return 0
	}

	penalty := 0.0
	for _, atk := range xi {
		if atk.Player.ElementType != PosMID && atk.Player.ElementType != PosFWD {
			continue
		}
		fps := pairings[atk.Player.Team]
		if len(fps) == 0 {
			continue
		}

		for _, def := range xi {
			if def.Player.ID == atk.Player.ID {
				continue
			}
			if def.Player.ElementType != PosGK && def.Player.ElementType != PosDEF {
				continue
			}

			for _, fp := range fps {
				if def.Player.Team == fp.OpponentID {
					defWeight := 0.7
					if def.Player.ElementType == PosGK {
						defWeight = 1.0
					}
					penalty += 0.15 * defWeight * math.Min(atk.Score, def.Score)
					break
				}
			}
		}
	}
	return penalty
}

func fillBench(byPos map[int][]ScoredPlayer, xiIDs map[int]bool, teamCount map[int]int, xiPosCounts map[int]int, remainingBudget float64) []ScoredPlayer {
	var bench []ScoredPlayer
	rem := remainingBudget

	benchNeeds := map[int]int{}
	for pos, total := range squadSlots {
		benchNeeds[pos] = total - xiPosCounts[pos]
	}

	for _, pos := range []int{PosGK, PosDEF, PosMID, PosFWD} {
		need := benchNeeds[pos]
		cnt := 0
		for _, p := range sortedByCostAsc(byPos[pos]) {
			if cnt >= need {
				break
			}
			if xiIDs[p.Player.ID] || teamCount[p.Player.Team] >= 3 {
				continue
			}
			if float64(p.Player.NowCost) > rem {
				continue
			}
			bench = append(bench, p)
			xiIDs[p.Player.ID] = true
			teamCount[p.Player.Team]++
			rem -= float64(p.Player.NowCost)
			cnt++
		}
	}

	return bench
}

func sortedByCostAsc(pool []ScoredPlayer) []ScoredPlayer {
	s := make([]ScoredPlayer, len(pool))
	copy(s, pool)
	sort.Slice(s, func(i, j int) bool {
		return s[i].Player.NowCost < s[j].Player.NowCost
	})
	return s
}

func pickCaptains(starters []ScoredPlayer) (captain, viceCaptain ScoredPlayer) {
	if len(starters) == 0 {
		return
	}
	sorted := make([]ScoredPlayer, len(starters))
	copy(sorted, starters)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})
	captain = sorted[0]
	if len(sorted) > 1 {
		viceCaptain = sorted[1]
	}
	return
}

func isAlreadyPicked(picked []ScoredPlayer, playerID int) bool {
	for _, p := range picked {
		if p.Player.ID == playerID {
			return true
		}
	}
	return false
}
