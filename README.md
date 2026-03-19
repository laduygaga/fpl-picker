# fpl-picker

CLI tool that picks the best Fantasy Premier League squad for the next gameweek. Optimizes the starting XI first, then fills the bench with the cheapest eligible players.

## How It Works

Scores every available player using a weighted model conditioned on opponent quality:

| Weight | Factor | Description |
|--------|--------|-------------|
| 35% | Expected Points (EP) | FPL's own next-GW projection |
| 25% | Opponent Quality | Position-specific — attackers score higher vs leaky defences, defenders score higher vs weak attacks |
| 20% | Form | Recent points per game |
| 10% | PPG | Season points per game |
| 5% | xGI/90 | Expected goal involvement per 90 minutes |
| 5% | ICT/90 | Influence + Creativity + Threat index per 90 |

All metrics are **per-game / per-90** — no season totals.

### Opponent Conditioning

Each position weights opponent attack weakness and defence weakness differently:

| Position | Opp Attack Weakness | Opp Defence Weakness |
|----------|--------------------:|---------------------:|
| GK | 100% | 0% |
| DEF | 70% | 30% |
| MID | 20% | 80% |
| FWD | 10% | 90% |

- **Opp Attack Weakness** = `1 / team_xG_per90` — higher means the opponent scores fewer goals (good for GK/DEF)
- **Opp Defence Weakness** = `team_xGA_per90` — higher means the opponent concedes more (good for MID/FWD)
- Home advantage adds a +0.02 bonus
- Double gameweeks are supported — opponent quality is averaged across fixtures

### XI-First Optimization

The optimizer tries all 7 valid formations (3-4-3, 3-5-2, 4-3-3, 4-4-2, 4-5-1, 5-3-2, 5-4-1), maximizing the starting XI score while reserving budget for the cheapest possible bench. The bench composition is derived from formation — e.g., a 3-4-3 XI needs 2 extra DEF, 1 extra MID, and 0 extra FWD on the bench.

Constraints: max 3 players per team, 15-man squad (1 GK + formation XI + bench).

## Install

```
go install fpl-picker@latest
```

Or build from source:

```
git clone <repo-url>
cd fpl-picker
go build -o fpl-picker .
```

Requires Go 1.25+.

## Usage

```
fpl-picker [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-budget` | `100.0` | Total squad budget in £M |
| `-top` | `5` | Show top N players per position |
| `-diff` | `10` | Show top N differential picks (low ownership) |
| `-diff-max` | `10.0` | Max ownership % for differentials |
| `-fresh` | `false` | Clear cache and fetch fresh data |
| `-my-team` | | Comma-separated player web names for squad comparison |
| `-save-team` | `false` | Save `-my-team` names to `.fpl-team.txt` for future runs |

### Examples

Pick the best squad with a £102.1M budget:

```
fpl-picker -budget 102.1
```

Compare your current squad against the optimal pick:

```
fpl-picker -budget 102.1 -my-team "Kelleher,Gabriel,Rice,Haaland,Mbeumo,Semenyo,Ekitiké" -save-team
```

On subsequent runs, your team auto-loads from `.fpl-team.txt` — no need to pass `-my-team` again:

```
fpl-picker -budget 102.1
```

Force fresh data (bypasses the 1-hour cache):

```
fpl-picker -budget 102.1 -fresh
```

### Sample Output

```
════════════════════════════════════════════════════════════════════════════════
  GW30 — XI-FIRST | OPPONENT-CONDITIONED SCORING
  Budget: £102.1M  |  XI Cost: £76.2M  |  Squad Cost: £91.6M  |  Formation: 3-5-2
  CAPTAIN: João Pedro (0.810)  |  VICE: B.Fernandes (0.725)
────────────────────────────────────────────────────────────────────────────────
  POS  PLAYER              TEAM  COST   SCORE  EP    FORM  OPP-Q  OPPONENT PROFILE
  ───  ──────────────────  ────  ────   ─────  ────  ────  ─────  ─────────────────────────
  GK   Petrović            BOU   £4.5M  0.629  6.5   6.0   91%    BUR(A) [Weak Atk, Leaky Def]

  DEF  Senesi              BOU   £5.0M  0.708  7.2   6.7   94%    BUR(A) [Weak Atk, Leaky Def]
  DEF  O'Reilly            MCI   £5.1M  0.660  8.2   7.7   53%    WHU(A) [Weak Atk, Leaky Def]
  DEF  Truffert            BOU   £4.6M  0.651  6.5   6.0   94%    BUR(A) [Weak Atk, Leaky Def]

  MID  B.Fernandes    V    MUN   £10.1M 0.725  8.2   7.7   38%    AVL(H) [Weak Atk, Avg Def]
  MID  Mac Allister        LIV   £6.3M  0.702  8.5   8.0   62%    TOT(H) [Weak Atk, Avg Def]
  ...

  FWD  João Pedro     ©    CHE   £7.7M  0.810  9.8   9.3   41%    NEW(H) [Avg Atk, Avg Def]
  FWD  Ekitiké              LIV   £9.2M  0.685  6.8   6.3   62%    TOT(H) [Weak Atk, Avg Def]
────────────────────────────────────────────────────────────────────────────────
  Starting XI Score: 7.518
```

When `-my-team` is provided (or auto-loaded), you also get:

- **Your XI vs Optimal** — side-by-side score comparison with gap
- **Transfer Targets** — weakest players in your squad with top replacements ranked by score uplift
- **Top Picks by Position** — best GK/DEF/MID/FWD with value ratings
- **Differentials** — high-scoring players under the ownership threshold

## Data Source

All data comes from the official FPL API — no auth required:

- `https://fantasy.premierleague.com/api/bootstrap-static/` — players, teams, events
- `https://fantasy.premierleague.com/api/fixtures/` — all matches

Responses are cached locally in `.fpl-cache/` for 1 hour. Use `-fresh` to bypass.

## Project Structure

```
fpl-picker/
├── main.go              # CLI entrypoint, flag parsing
├── api/
│   ├── client.go        # HTTP client with file-based caching
│   └── types.go         # FPL API response structs
├── model/
│   ├── scorer.go        # Opponent-conditioned scoring engine
│   ├── scorer_test.go   # Scorer unit tests
│   ├── recommender.go   # XI-first squad optimizer
│   └── recommender_test.go  # Recommender unit tests
├── display/
│   └── table.go         # CLI table rendering
├── go.mod
├── .gitignore
└── .fpl-team.txt        # Auto-saved team (gitignored)
```

## Testing

```
go test ./... -v
```

16 tests covering scoring logic, eligibility filtering, squad optimization, budget constraints, formation-aware bench composition, captain selection, and player name matching.
