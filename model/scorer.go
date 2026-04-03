package model

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"

	"fpl-picker/api"
)

// Position constants matching FPL element_type values.
const (
	PosGK  = 1
	PosDEF = 2
	PosMID = 3
	PosFWD = 4
)

// Scoring weights for the base score (sum = 1.0).
const (
	wFDR      = 0.30
	wTotalPts = 0.20
	wForm     = 0.15
	wEP       = 0.05
	wPPG      = 0.05
	wXGI90    = 0.03
	wICT90    = 0.02
	// Opponent quality and position-specific bonuses are 20% total,
	// split per position inside scorePositionBonus.
)

const (
	wOpponentQuality = 0.20 // shared across all positions
	wHomeAdvantage   = 0.02
	wCleanSheetBonus = 0.04
)

// Formula defines a scoring weight preset.
type Formula struct {
	Name  string
	FDR   float64
	Pts   float64
	Form  float64
	EP    float64
	PPG   float64
	XGI   float64
	ICT   float64
}

// Preset formulas.
var Formulas = map[string]Formula{
	"1": {
		Name:  "Balanced",
		FDR:   0.30,
		Pts:   0.20,
		Form:  0.15,
		EP:    0.05,
		PPG:   0.05,
		XGI:   0.03,
		ICT:   0.02,
	},
	"2": {
		Name:  "Attacker",
		FDR:   0.15,
		Pts:   0.10,
		Form:  0.25,
		EP:    0.10,
		PPG:   0.10,
		XGI:   0.15,
		ICT:   0.15,
	},
	"3": {
		Name:  "Defender",
		FDR:   0.35,
		Pts:   0.25,
		Form:  0.10,
		EP:    0.05,
		PPG:   0.10,
		XGI:   0.05,
		ICT:   0.10,
	},
}

func GetFormula(id string) Formula {
	if f, ok := Formulas[id]; ok {
		return f
	}
	return Formulas["1"] // default
}

// ScoredPlayer holds a player with their computed recommendation score.
//
// Field groups:
//   - Identity: Player, TeamName, PositionName
//   - Scoring metrics: Score, EPNextVal, FormVal, PPGVal, XGIP90, ICTP90
//   - Fixture context: OppScore, OppDesc, HasFixture, IsHome, UpcomingFixtures
//   - Display helpers: ValueRating, OwnershipPct
type ScoredPlayer struct {
	Player       api.Player
	Score        float64
	TeamName     string
	PositionName string

	EPNextVal float64
	FormVal   float64
	PPGVal    float64
	XGIP90    float64
	ICTP90    float64

	OppScore   float64 // 0-1, higher = easier fixture
	OppDesc    string  // e.g. "ARS(H) [Strong Atk, Avg Def]"
	HasFixture bool
	IsHome     bool
	FDRVal     float64 // easyFDR: 6 - difficulty (higher = easier)

	UpcomingFixtures string
	ValueRating      float64
	OwnershipPct     float64
}

// gwContext holds next-GW fixture context for a team.
type gwContext struct {
	oppID          int
	isHome         bool
	oppDefWeakness float64 // higher = weaker defence = better for our attackers
	oppAttWeakness float64 // higher = weaker attack = better for our defenders
	easyFDR        float64 // 6 - FDR: higher = easier fixture (FDR 1→5, FDR 5→1)
}

// Scorer computes recommendation scores for FPL players using
// opponent-conditioned, per-game metrics optimized for a single gameweek.
type Scorer struct {
	teams       map[int]api.Team
	fixtures    []api.Fixture
	nextEventID int

	teamAttackP90  map[int]float64 // team's xG per 90 (higher = stronger attack)
	teamDefenceP90 map[int]float64 // team's xGA per 90 (higher = weaker defence)
	gwCtx          map[int][]gwContext

	formula Formula // scoring weights
}

// NewScorer creates a scorer from FPL data. It pre-computes team-level
// attacking/defensive strength from player aggregate stats.
func NewScorer(teams []api.Team, fixtures []api.Fixture, events []api.Event, players []api.Player, formulaID string) *Scorer {
	teamMap := make(map[int]api.Team, len(teams))
	for _, t := range teams {
		teamMap[t.ID] = t
	}

	nextGW := 1
	for _, e := range events {
		if e.IsNext {
			nextGW = e.ID
			break
		}
	}

	s := &Scorer{
		teams:       teamMap,
		fixtures:    fixtures,
		nextEventID: nextGW,
		formula:     GetFormula(formulaID),
	}
	s.computeTeamStats(players)
	s.buildGWContext()

	return s
}

// NextEventID returns the upcoming gameweek number.
func (s *Scorer) NextEventID() int {
	return s.nextEventID
}

// FixturePairing holds an opponent team and the FPL difficulty rating (1–5)
// for that fixture from this team's perspective.
type FixturePairing struct {
	OpponentID int
	Difficulty int
}

// FixturePairings returns a map of team ID → list of opponent pairings
// for the next gameweek. Used by the optimizer for head-to-head clash detection.
func (s *Scorer) FixturePairings() map[int][]FixturePairing {
	pairings := make(map[int][]FixturePairing)
	for _, f := range s.fixtures {
		if f.Event == nil || *f.Event != s.nextEventID {
			continue
		}
		pairings[f.TeamH] = append(pairings[f.TeamH], FixturePairing{
			OpponentID: f.TeamA,
			Difficulty: f.TeamHDifficulty,
		})
		pairings[f.TeamA] = append(pairings[f.TeamA], FixturePairing{
			OpponentID: f.TeamH,
			Difficulty: f.TeamADifficulty,
		})
	}
	return pairings
}

// computeTeamStats derives team-level attacking and defensive strength
// from player-level xG and GK xGA data, normalized to per-90 rates.
func (s *Scorer) computeTeamStats(players []api.Player) {
	teamXG := map[int]float64{}
	teamXGA := map[int]float64{}
	teamGKMinutes := map[int]float64{}

	for _, p := range players {
		if p.Minutes == 0 {
			continue
		}
		teamXG[p.Team] += parseFloat(p.ExpectedGoals)
		// Only GKs with significant minutes carry the full team xGC
		if p.ElementType == PosGK && p.Minutes > 450 {
			teamXGA[p.Team] += parseFloat(p.ExpectedGoalsConceded)
		}
		if p.ElementType == PosGK {
			teamGKMinutes[p.Team] += float64(p.Minutes)
		}
	}

	s.teamAttackP90 = make(map[int]float64, len(s.teams))
	s.teamDefenceP90 = make(map[int]float64, len(s.teams))
	for id := range s.teams {
		mins := teamGKMinutes[id]
		if mins < 90 {
			mins = 90
		}
		gp := mins / 90.0
		s.teamAttackP90[id] = teamXG[id] / gp
		s.teamDefenceP90[id] = teamXGA[id] / gp
	}
}

// buildGWContext builds per-team fixture context for the next gameweek.
// Supports double gameweeks (DGW) where a team has multiple fixtures.
func (s *Scorer) buildGWContext() {
	s.gwCtx = make(map[int][]gwContext)

	for _, f := range s.fixtures {
		if f.Event == nil || *f.Event != s.nextEventID {
			continue
		}

		s.gwCtx[f.TeamH] = append(s.gwCtx[f.TeamH], gwContext{
			oppID:          f.TeamA,
			isHome:         true,
			oppDefWeakness: s.teamDefenceP90[f.TeamA],
			oppAttWeakness: 1.0 / math.Max(0.5, s.teamAttackP90[f.TeamA]),
			easyFDR:        float64(6 - f.TeamHDifficulty),
		})

		s.gwCtx[f.TeamA] = append(s.gwCtx[f.TeamA], gwContext{
			oppID:          f.TeamH,
			isHome:         false,
			oppDefWeakness: s.teamDefenceP90[f.TeamH],
			oppAttWeakness: 1.0 / math.Max(0.5, s.teamAttackP90[f.TeamH]),
			easyFDR:        float64(6 - f.TeamADifficulty),
		})
	}
}

type rawMetrics struct {
	ppg, ep, form, xgi90, ict90, oppAtk, oppDef, totalPts, easyFDR float64
}

type scoreNormals struct {
	ppg, ep, form, xgi90, ict90, oppAtk, oppDef, totalPts, easyFDR float64
}

func extractRawVectors(raws []rawMetrics) (ppg, ep, form, xgi90, ict90, oppAtk, oppDef, totalPts, easyFDR []float64) {
	n := len(raws)
	ppg = make([]float64, n)
	ep = make([]float64, n)
	form = make([]float64, n)
	xgi90 = make([]float64, n)
	ict90 = make([]float64, n)
	oppAtk = make([]float64, n)
	oppDef = make([]float64, n)
	totalPts = make([]float64, n)
	easyFDR = make([]float64, n)

	for i, r := range raws {
		ppg[i] = r.ppg
		ep[i] = r.ep
		form[i] = r.form
		xgi90[i] = r.xgi90
		ict90[i] = r.ict90
		oppAtk[i] = r.oppAtk
		oppDef[i] = r.oppDef
		totalPts[i] = r.totalPts
		easyFDR[i] = r.easyFDR
	}
	return
}

func buildNormalizers(ppg, ep, form, xgi90, ict90, oppAtk, oppDef, totalPts, easyFDR []float64) (
	nPPG, nEP, nForm, nXGI, nICT, nOA, nOD, nTP, nFDR normalizer,
) {
	return newNormalizer(ppg), newNormalizer(ep), newNormalizer(form),
		newNormalizer(xgi90), newNormalizer(ict90), newNormalizer(oppAtk),
		newNormalizer(oppDef), newNormalizer(totalPts), newNormalizer(easyFDR)
}

func normalizeAll(r rawMetrics, nPPG, nEP, nForm, nXGI, nICT, nOA, nOD, nTP, nFDR normalizer) scoreNormals {
	return scoreNormals{
		ppg:      nPPG.normalize(r.ppg),
		ep:       nEP.normalize(r.ep),
		form:     nForm.normalize(r.form),
		xgi90:    nXGI.normalize(r.xgi90),
		ict90:    nICT.normalize(r.ict90),
		oppAtk:   nOA.normalize(r.oppAtk),
		oppDef:   nOD.normalize(r.oppDef),
		totalPts: nTP.normalize(r.totalPts),
		easyFDR:  nFDR.normalize(r.easyFDR),
	}
}

func baseScore(f Formula, n scoreNormals) float64 {
	return f.FDR*n.easyFDR +
		f.Pts*n.totalPts +
		f.Form*n.form +
		f.EP*n.ep +
		f.PPG*n.ppg +
		f.XGI*n.xgi90 +
		f.ICT*n.ict90
}

func scorePositionBonus(p api.Player, f Formula, n scoreNormals, gp float64) (bonus float64, oppScore float64) {
	oppWeight := wOpponentQuality
	cleanSheetBonus := wCleanSheetBonus

	switch p.ElementType {
	case PosGK:
		opp := n.oppAtk
		bonus = oppWeight * opp
		if xgc := parseFloat(p.ExpectedGoalsConceded) / gp; xgc < 1.0 {
			bonus += cleanSheetBonus * (1 - xgc)
		}
		return bonus, opp

	case PosDEF:
		opp := 0.70*n.oppAtk + 0.30*n.oppDef
		bonus = oppWeight * opp
		if xgc := parseFloat(p.ExpectedGoalsConceded) / gp; xgc < 1.0 {
			bonus += cleanSheetBonus * (1 - xgc)
		}
		bonus += 0.03 * (parseFloat(p.ExpectedAssists) / gp)
		return bonus, opp

	case PosMID:
		opp := 0.20*n.oppAtk + 0.80*n.oppDef
		bonus = oppWeight * opp
		bonus += 0.04 * (parseFloat(p.ExpectedGoals)/gp + parseFloat(p.ExpectedAssists)/gp)
		return bonus, opp

	case PosFWD:
		opp := 0.10*n.oppAtk + 0.90*n.oppDef
		bonus = oppWeight * opp
		bonus += 0.07 * (parseFloat(p.ExpectedGoals) / gp)
		return bonus, opp

	default:
		return 0, 0
	}
}

// ScoreAll computes opponent-conditioned, per-game scores for all eligible players.
func (s *Scorer) ScoreAll(players []api.Player) []ScoredPlayer {
	var scored []ScoredPlayer
	var raws []rawMetrics

	for _, p := range players {
		if !isEligible(p) {
			continue
		}

		gp := math.Max(1.0, float64(p.Minutes)/90.0)
		ctxList, hasFix := s.gwCtx[p.Team]

		var oppAtk, oppDef, easyFDR float64
		oppDesc := "BLANK"
		isHome := false

		if hasFix && len(ctxList) > 0 {
			for _, ctx := range ctxList {
				oppAtk += ctx.oppAttWeakness
				oppDef += ctx.oppDefWeakness
				easyFDR += ctx.easyFDR
			}
			n := float64(len(ctxList))
			oppAtk /= n
			oppDef /= n
			easyFDR /= n
			isHome = ctxList[0].isHome
			oppDesc = s.describeOpponents(ctxList)
		}

		raws = append(raws, rawMetrics{
			ppg:      parseFloat(p.PointsPerGame),
			ep:       parseFloat(p.EPNext),
			form:     parseFloat(p.Form),
			xgi90:    parseFloat(p.ExpectedGoalInvolvements) / gp,
			ict90:    parseFloat(p.ICTIndex) / gp,
			oppAtk:   oppAtk,
			oppDef:   oppDef,
			totalPts: float64(p.TotalPoints),
			easyFDR:  easyFDR,
		})

		scored = append(scored, ScoredPlayer{
			Player:           p,
			TeamName:         s.teams[p.Team].ShortName,
			PositionName:     PosName(p.ElementType),
			EPNextVal:        raws[len(raws)-1].ep,
			FormVal:          raws[len(raws)-1].form,
			PPGVal:           raws[len(raws)-1].ppg,
			XGIP90:           raws[len(raws)-1].xgi90,
			ICTP90:           raws[len(raws)-1].ict90,
			HasFixture:       hasFix,
			IsHome:           isHome,
			OppDesc:          oppDesc,
			FDRVal:           raws[len(raws)-1].easyFDR,
			UpcomingFixtures: s.describeFixtures(p.Team),
			OwnershipPct:     parseFloat(p.SelectedByPercent),
		})
	}

	if len(raws) == 0 {
		return scored
	}

	ppgV, epV, formV, xgiV, ictV, oaV, odV, tpV, fdrV := extractRawVectors(raws)
	nPPG, nEP, nForm, nXGI, nICT, nOA, nOD, nTP, nFDR := buildNormalizers(ppgV, epV, formV, xgiV, ictV, oaV, odV, tpV, fdrV)

	for i := range scored {
		if !scored[i].HasFixture {
			scored[i].Score = 0
			continue
		}

		n := normalizeAll(raws[i], nPPG, nEP, nForm, nXGI, nICT, nOA, nOD, nTP, nFDR)
		score := baseScore(s.formula, n)

		gp := math.Max(1.0, float64(scored[i].Player.Minutes)/90.0)
		bonus, oppScore := scorePositionBonus(scored[i].Player, s.formula, n, gp)
		score += bonus
		scored[i].OppScore = oppScore

		if scored[i].IsHome {
			score += wHomeAdvantage
		}

		scored[i].Score = score

		costM := float64(scored[i].Player.NowCost) / 10.0
		if costM > 0 {
			scored[i].ValueRating = score / costM
		}
	}

	return scored
}

// FindPlayersByName resolves player web names (with fuzzy matching) to ScoredPlayers.
func FindPlayersByName(scored []ScoredPlayer, names []string) []ScoredPlayer {
	byName := make(map[string]ScoredPlayer, len(scored))
	for _, s := range scored {
		byName[s.Player.WebName] = s
	}

	var found []ScoredPlayer
	for _, nm := range names {
		nm = strings.TrimSpace(nm)
		if nm == "" {
			continue
		}
		s, ok := byName[nm]
		if !ok {
			for k, v := range byName {
				if strings.Contains(k, nm) || strings.Contains(nm, k) {
					s = v
					ok = true
					break
				}
			}
		}
		if ok {
			found = append(found, s)
		}
	}
	return found
}

// BestXIFromSquad picks the highest-scoring valid formation from a set of players.
func BestXIFromSquad(squad []ScoredPlayer) (starters []ScoredPlayer, formation string, totalScore float64) {
	byPos := map[int][]ScoredPlayer{}
	for _, p := range squad {
		byPos[p.Player.ElementType] = append(byPos[p.Player.ElementType], p)
	}
	for pos := range byPos {
		pool := byPos[pos]
		sort.Slice(pool, func(i, j int) bool { return pool[i].Score > pool[j].Score })
	}

	for _, fm := range ValidFormations {
		needs := map[int]int{PosGK: fm.GK, PosDEF: fm.DEF, PosMID: fm.MID, PosFWD: fm.FWD}

		valid := true
		for pos, n := range needs {
			if len(byPos[pos]) < n {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}

		var trial []ScoredPlayer
		score := 0.0
		for pos, n := range needs {
			for j := range n {
				trial = append(trial, byPos[pos][j])
				score += byPos[pos][j].Score
			}
		}

		if score > totalScore {
			totalScore = score
			starters = trial
			formation = fm.Name
		}
	}

	SortByPosAndScore(starters)
	return
}

// describeOpponents builds a human-readable opponent profile string.
// For DGW, joins multiple opponent descriptions.
func (s *Scorer) describeOpponents(ctxList []gwContext) string {
	descs := make([]string, len(ctxList))
	for i, ctx := range ctxList {
		descs[i] = s.describeOpponent(ctx)
	}
	return strings.Join(descs, " + ")
}

func (s *Scorer) describeOpponent(ctx gwContext) string {
	oppTeam := s.teams[ctx.oppID]
	atkRating := s.teamAttackP90[ctx.oppID]
	defRating := s.teamDefenceP90[ctx.oppID]

	atkLabel := "Weak Atk"
	if atkRating > 1.8 {
		atkLabel = "Strong Atk"
	} else if atkRating > 1.3 {
		atkLabel = "Avg Atk"
	}

	defLabel := "Solid Def"
	if defRating > 1.5 {
		defLabel = "Leaky Def"
	} else if defRating > 1.0 {
		defLabel = "Avg Def"
	}

	ha := "H"
	if !ctx.isHome {
		ha = "A"
	}
	return fmt.Sprintf("%s(%s) [%s, %s]", oppTeam.ShortName, ha, atkLabel, defLabel)
}

// describeFixtures returns upcoming opponents for the next 3 gameweeks.
func (s *Scorer) describeFixtures(teamID int) string {
	type fixtureInfo struct {
		gw   int
		desc string
	}
	var upcoming []fixtureInfo

	for _, f := range s.fixtures {
		if f.Event == nil || f.Finished {
			continue
		}
		gw := *f.Event
		if gw < s.nextEventID || gw >= s.nextEventID+3 {
			continue
		}
		if f.TeamH == teamID {
			opp := s.teams[f.TeamA]
			upcoming = append(upcoming, fixtureInfo{gw, fmt.Sprintf("%s(H)", opp.ShortName)})
		} else if f.TeamA == teamID {
			opp := s.teams[f.TeamH]
			upcoming = append(upcoming, fixtureInfo{gw, fmt.Sprintf("%s(A)", opp.ShortName)})
		}
	}

	if len(upcoming) == 0 {
		return "BLANK"
	}

	descs := make([]string, len(upcoming))
	for i, u := range upcoming {
		descs[i] = u.desc
	}
	return strings.Join(descs, ", ")
}

func isEligible(p api.Player) bool {
	if p.Status == "i" || p.Status == "s" || p.Status == "u" {
		return false
	}
	if p.Minutes < 90 {
		return false
	}
	if p.Status == "d" && p.ChanceOfPlayingNextRound != nil && *p.ChanceOfPlayingNextRound < 50 {
		return false
	}
	return true
}

// PosName returns the short position label for an element type.
func PosName(elementType int) string {
	switch elementType {
	case PosGK:
		return "GK"
	case PosDEF:
		return "DEF"
	case PosMID:
		return "MID"
	case PosFWD:
		return "FWD"
	default:
		return "???"
	}
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("warning: parseFloat(%q): %v", s, err)
		return 0
	}
	return v
}

// SortByPosAndScore sorts players by position (ascending) then score (descending).
func SortByPosAndScore(players []ScoredPlayer) {
	sort.Slice(players, func(i, j int) bool {
		pi, pj := players[i].Player.ElementType, players[j].Player.ElementType
		if pi != pj {
			return pi < pj
		}
		return players[i].Score > players[j].Score
	})
}

type normalizer struct {
	min, max float64
}

func newNormalizer(vals []float64) normalizer {
	if len(vals) == 0 {
		return normalizer{0, 1}
	}
	lo, hi := vals[0], vals[0]
	for _, v := range vals[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return normalizer{lo, hi}
}

func (n normalizer) normalize(v float64) float64 {
	r := n.max - n.min
	if r == 0 {
		return 0
	}
	return math.Max(0, math.Min(1, (v-n.min)/r))
}
