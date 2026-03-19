package model

import (
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

// FindBestSquad builds an optimal 15-man FPL squad using XI-first optimization:
// 1. Reserve budget for cheapest possible bench (1 GK + 3 cheapest outfield)
// 2. Try every formation, filling XI with cheapest then upgrading within XI budget
// 3. Pick the formation that maximizes starting XI score
// 4. Fill bench slots with cheapest remaining eligible players
func FindBestSquad(players []ScoredPlayer, budgetTenths int) SquadResult {
	byPos := map[int][]ScoredPlayer{
		PosGK:  {},
		PosDEF: {},
		PosMID: {},
		PosFWD: {},
	}
	for _, p := range players {
		byPos[p.Player.ElementType] = append(byPos[p.Player.ElementType], p)
	}
	for pos := range byPos {
		sort.Slice(byPos[pos], func(i, j int) bool {
			return byPos[pos][i].Score > byPos[pos][j].Score
		})
	}

	budget := float64(budgetTenths)

	// Calculate minimum bench cost: cheapest GK + 3 cheapest outfield players
	cheapestByPos := map[int]int{}
	for pos, pool := range byPos {
		if len(pool) == 0 {
			continue
		}
		m := pool[0].Player.NowCost
		for _, p := range pool {
			if p.Player.NowCost < m {
				m = p.Player.NowCost
			}
		}
		cheapestByPos[pos] = m
	}

	var outfieldCosts []int
	for pos := PosDEF; pos <= PosFWD; pos++ {
		for _, p := range byPos[pos] {
			outfieldCosts = append(outfieldCosts, p.Player.NowCost)
		}
	}
	sort.Ints(outfieldCosts)

	benchReserve := float64(cheapestByPos[PosGK])
	for i := 0; i < 3 && i < len(outfieldCosts); i++ {
		benchReserve += float64(outfieldCosts[i])
	}
	xiBudget := budget - benchReserve

	// Try every formation and pick the one with highest XI score
	type result struct {
		fm    string
		xi    []ScoredPlayer
		score float64
		cost  float64
	}
	var best result

	for _, fm := range ValidFormations {
		r := tryFormation(fm, byPos, xiBudget)
		if r.score > best.score {
			best = r
		}
	}

	if len(best.xi) == 0 {
		return SquadResult{}
	}

	sortByPosAndScore(best.xi)

	// Fill bench: cheapest GK not in XI + 3 cheapest outfield not in XI
	xiIDs := map[int]bool{}
	teamCount := map[int]int{}
	for _, p := range best.xi {
		xiIDs[p.Player.ID] = true
		teamCount[p.Player.Team]++
	}

	xiPosCounts := map[int]int{}
	for _, p := range best.xi {
		xiPosCounts[p.Player.ElementType]++
	}

	bench := fillBench(byPos, xiIDs, teamCount, xiPosCounts, budget-best.cost)
	captain, viceCaptain := pickCaptains(best.xi)

	totalCost := best.cost
	for _, p := range bench {
		totalCost += float64(p.Player.NowCost)
	}

	return SquadResult{
		Formation:   best.fm,
		Starters:    best.xi,
		Bench:       bench,
		Captain:     captain,
		ViceCaptain: viceCaptain,
		TotalScore:  best.score,
		XICost:      best.cost / 10.0,
		TotalCost:   totalCost / 10.0,
		Budget:      float64(budgetTenths) / 10.0,
	}
}

func tryFormation(fm Formation, byPos map[int][]ScoredPlayer, xiBudget float64) struct {
	fm    string
	xi    []ScoredPlayer
	score float64
	cost  float64
} {
	type result = struct {
		fm    string
		xi    []ScoredPlayer
		score float64
		cost  float64
	}

	needs := map[int]int{PosGK: fm.GK, PosDEF: fm.DEF, PosMID: fm.MID, PosFWD: fm.FWD}

	cheapByPos := map[int][]ScoredPlayer{}
	for pos, pool := range byPos {
		s := make([]ScoredPlayer, len(pool))
		copy(s, pool)
		sort.Slice(s, func(i, j int) bool {
			return s[i].Player.NowCost < s[j].Player.NowCost
		})
		cheapByPos[pos] = s
	}

	var xi []ScoredPlayer
	tc := map[int]int{}
	rem := xiBudget

	for _, pos := range []int{PosGK, PosDEF, PosMID, PosFWD} {
		nd := needs[pos]
		cnt := 0
		for _, p := range cheapByPos[pos] {
			if cnt >= nd {
				break
			}
			if float64(p.Player.NowCost) > rem || tc[p.Player.Team] >= 3 {
				continue
			}
			xi = append(xi, p)
			tc[p.Player.Team]++
			rem -= float64(p.Player.NowCost)
			cnt++
		}
		if cnt < nd {
			return result{}
		}
	}

	// Iterative upgrade: swap each slot to highest-scoring affordable alternative
	upgraded := true
	for upgraded {
		upgraded = false
		for i := range xi {
			cur := xi[i]
			pos := cur.Player.ElementType
			xiCost := 0.0
			for _, s := range xi {
				xiCost += float64(s.Player.NowCost)
			}
			canSpend := xiBudget - xiCost + float64(cur.Player.NowCost)

			for _, c := range byPos[pos] {
				if c.Player.ID == cur.Player.ID || c.Score <= cur.Score || float64(c.Player.NowCost) > canSpend {
					continue
				}
				if isAlreadyPicked(xi, c.Player.ID) {
					continue
				}
				if c.Player.Team != cur.Player.Team && tc[c.Player.Team] >= 3 {
					continue
				}

				tc[cur.Player.Team]--
				if c.Player.Team != cur.Player.Team {
					tc[c.Player.Team]++
				}
				xi[i] = c
				upgraded = true
				break
			}
		}
	}

	sc := 0.0
	co := 0.0
	for _, s := range xi {
		sc += s.Score
		co += float64(s.Player.NowCost)
	}
	return result{fm.Name, xi, sc, co}
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
