# fpl-picker

CLI tool that picks the best Fantasy Premier League squad for the next gameweek. Optimizes the starting XI first, then fills the bench with the cheapest eligible players.

## How It Works

Scores every available player using a weighted model conditioned on opponent quality:

| Weight | Factor | Description |
|--------|--------|-------------|
| 30% | Fixture Difficulty (FDR) | FPL's 1–5 difficulty rating, inverted (easier = higher score) |
| 20% | Total Points | Season total points — proven performers |
| 20% | Opponent Quality | Position-specific — attackers score higher vs leaky defences, defenders score higher vs weak attacks |
| 15% | Form | Recent points per game |
| 5% | Expected Points (EP) | FPL's own next-GW projection |
| 5% | PPG | Season points per game |
| 3% | xGI/90 | Expected goal involvement per 90 minutes |
| 2% | ICT/90 | Influence + Creativity + Threat index per 90 |

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
- Double gameweeks get +0.05 bonus (players marked with `*` in output)

### Scoring Formulas

Choose a formula that matches your strategy:

| Formula | FDR | Pts | Form | EP | PPG | xGI | ICT | DGW | Best For |
|---------|-----|-----|------|-----|-----|-----|-----|-----|----------|
| `1` Balanced | 30% | 20% | 15% | 5% | 5% | 3% | 2% | +5% | General use |
| `2` Attacker | 15% | 10% | 25% | 10% | 10% | 15% | 15% | +5% | High-scoring leagues |
| `3` Defender | 35% | 25% | 10% | 5% | 10% | 5% | 10% | +5% | Clean sheet heavy |

Use `-formula 2` or `-formula 3` to switch. The default is `1` (Balanced).

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
| `-formula` | `1` | Scoring formula: `1`/`balanced`, `2`/`attacker`, `3`/`defender` (case-insensitive aliases accepted) |
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

Pick with Attacker formula (favors form, xG, ICT):

```bash
fpl-picker -budget 102.1 -formula 2
```

Pick with Defender formula (favors FDR, total points):

```bash
fpl-picker -budget 102.1 -formula 3
```

Compare your current squad against the optimal pick:

```
fpl-picker -budget 102.1 -my-team "Kelleher,Gabriel,Rice,Haaland,Mbeumo,Semenyo,Ekitiké" -save-team
```

On subsequent runs, your team auto-loads from `.fpl-team.txt` — no need to pass `-my-team` again:

```
fpl-picker -budget 102.1
```

Pick with Attacker formula (favors form, xG, ICT):

```bash
fpl-picker -budget 102.1 -formula 2
```

Pick with Defender formula (favors FDR, total points):

```bash
fpl-picker -budget 102.1 -formula 3
```

Force fresh data (bypasses the 1-hour cache):

```
fpl-picker -budget 102.1 -fresh
```

## Apply Command (live FPL integration)

The `apply` subcommand picks a squad AND posts it to your live FPL team.

⚠️ **Security**: This authenticates with your FPL account. The default is dry-run
(diff only, no posts). Real posts require `--apply`. Credentials are prompted
each run unless cached.

### Usage

```
fpl-picker apply [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--email` | (prompt) | FPL login email |
| `--password` | (prompt) | FPL login password |
| `--passphrase` | (prompt) | Passphrase for the encrypted credential cache |
| `--save-cache` | false | Save credentials encrypted to `~/.config/fpl-picker/.cached` (passphrase-protected) |
| `--clear-cache` | false | Delete the credential cache and exit |
| `--cookies` | "" | Cookie header from a logged-in browser session. Skips the login form. See [Cookie-paste auth](#cookie-paste-auth) below. |
| `--cookies-file` | "" | Path to a file containing cookies in the same format as `--cookies` |
| `--bearer` | "" | OAuth access token from localStorage (`x-api-authorization: Bearer`). The full oidc.user JSON also works — the token is extracted automatically. See [Bearer token auth](#bearer-token-auth) below. |
| `--bearer-file` | "" | Path to a file containing the bearer token (or full oidc.user JSON) |
| `--apply` | false | Actually post changes (default: dry-run) |
| `--chip` | "" | Activate chip: `wildcard`|`freehit`|`bboost`|`3xc` |
| `--no-transfers` | false | Skip transfer planning (lineup only) |
| `--no-lineup` | false | Skip lineup posting (transfers only) |
| `--max-hits` | 4 | Max points hits to accept from transfers |
| `--gw` | auto | Gameweek for display; apply backend targets its detected next gameweek |
| `--formula` | `1` | Scoring formula (passes through to scorer) |
| `--fresh` | false | Bypass FPL API cache |

### Examples

Dry-run (no posts, safe to run anytime):

```
fpl-picker apply
```

Save creds for future runs (passphrase prompted):

```
fpl-picker apply --save-cache
```

Apply for real (careful!):

```
fpl-picker apply --apply
```

Use a Wildcard chip:

```
fpl-picker apply --chip wildcard --apply
```

Limit points hits to 8 (i.e. max 2 transfers beyond free):

```
fpl-picker apply --max-hits 8 --apply
```

### Transfer validation and stale squad state

The apply flow enforces FPL's maximum of three players from one Premier League
team when team data is available locally. It still sends the transfer bundle to
FPL for authoritative validation.

FPL may briefly return a stale `/api/my-team/` response after transfers are
committed. When FPL reports `transfer_team_limit_reached`, the command refetches
the current squad, replans the transfers, and retries validation with bounded
exponential backoff. The same refresh-and-replan behavior protects the live
`--apply` path before committing transfers. Other validation errors are
reported immediately because retrying them will not change the underlying
request.

If you run another formula immediately after a successful apply and see a
transfer error referring to a player already picked or no longer in the team,
wait a few seconds and rerun the command. A dry-run without `--apply` is safe for
checking the refreshed transfer plan first.

### Security notes

- Credentials are NEVER logged or written to disk in plaintext.
- When `--save-cache` is set, the cache is encrypted with AES-256-GCM using
  a passphrase-derived key (PBKDF2-HMAC-SHA256, 100k iterations).
- The cache file is at `~/.config/fpl-picker/.cached` with `0600` perms.
- For maximum security, omit `--save-cache` and re-enter your password each run.
- A `User-Agent` matching the official FPL Android app is sent to avoid login
  403s — this is necessary because FPL blocks some non-browser UAs.
- Cookie storage is in-process only (Go's `net/http/cookiejar`). No cookies
  are persisted to disk by this tool.

### Cookie-paste auth

The default login flow POSTs to `https://users.premierleague.com/accounts/login/`,
which may be blocked by some networks (DNS unreachable, regional restrictions,
or FPL may migrate to OAuth-only auth). If `Login failed: ... no such host`
or similar errors appear, use `--cookies` instead:

1. Log into [fantasy.premierleague.com](https://fantasy.premierleague.com) in your
   browser (the modern login goes via `account.premierleague.com`).
2. Open DevTools (F12) → **Application** → **Cookies** → `https://fantasy.premierleague.com`
   (and `https://account.premierleague.com`).
3. Copy the cookie names + values. At minimum you need:
   - `pl_profile` — the gating auth cookie. Without this every private API
     call returns `403 "Authentication credentials were not provided."`
   - `csrftoken` — required for `POST /api/transfers/` and lineup updates.
   - `sessionid` — Django session cookie.
4. Pass them via flag or file:

```
fpl-picker apply --cookies "pl_profile=abc123; csrftoken=def456; sessionid=ghi789"
```

or save them to a file with the same format (one cookie per `name=value`,
separated by `;`) and use `--cookies-file ~/.fpl-cookies.txt`.

Cookies expire in ~30 days, just like a browser session. If you see
`Cookies invalid or expired`, re-export them.

**Treat the cookie file like a password** — it grants full access to your
FPL account. `chmod 600` it.

### Bearer token auth

The 2026-era FPL SPA uses OAuth through `account.premierleague.com` and stores
the resulting JWT `access_token` in `localStorage` (not as a cookie). The FPL
API accepts the token via the non-standard header **`x-api-authorization`** (not
the usual `Authorization`). If `IsLoggedIn` keeps returning `false` even after
loading cookies, use `--bearer` instead.

1. Log into [fantasy.premierleague.com](https://fantasy.premierleague.com) in your browser
2. DevTools → **Application** → **Local Storage** → `https://fantasy.premierleague.com`
3. Find the key named `oidc.user:https://account.premierleague.com/as:<client_id>`.
   Its value is a JSON object with an `access_token` field.
4. Pass either:
   - The **raw JWT string** (`eyJhbGc...`, copied from `access_token` field), OR
   - The **full oidc.user JSON** (the tool extracts `access_token` automatically)

```
fpl-picker apply --bearer "eyJhbGciOi..."
```

or save to a file (perm 600) — JSON form also works:

```bash
# Raw JWT
echo "eyJhbGciOi..." > ~/.fpl-bearer.txt
# OR full JSON from localStorage
cat > ~/.fpl-bearer.json <<'EOF'
{"access_token":"eyJ...","expires_at":1785794442,...}
EOF
chmod 600 ~/.fpl-bearer.json
fpl-picker apply --bearer-file ~/.fpl-bearer.json
```

The access token expires in ~11 hours (per the JWT `exp` claim). You'll need
to re-export from your browser when it does. The `refresh_token` field in the
same JSON object can be used to renew — this tool does not yet auto-refresh
(planned for a future iteration).

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

## Caching

Responses are cached locally in `.fpl-cache/`. The cache uses three layered optimizations:

- **gzip transport** — every request advertises `Accept-Encoding: gzip`. FPL serves ~1.5 MB of JSON as ~250 KB, so the first fetch is ~6× smaller on the wire.
- **ETag conditional requests** — the server's `ETag` header is persisted to `<endpoint>.etag`. On the next fetch we send `If-None-Match`; a `304 Not Modified` reply reuses the cached body, so only the ETag round-trip pays the network.
- **Split TTLs** — bootstrap-static contains both static data (teams, events, element_types) that changes once per season and dynamic data (elements: prices, ownership, status) that changes weekly. Per-field `fetched_at` timestamps are persisted in `<endpoint>.meta.json`:
  - `teams`, `events`, `element_types` → 24h TTL
  - `elements` → 1h TTL
  - `fixtures` → 1h TTL

  FPL returns the whole bootstrap-static response in one shot, so we can't selectively refetch — if ANY field is stale, we refetch the entire response and reset all timestamps. Pre-existing caches that lack a `.meta.json` fall back to the cache file's modification time, so old caches keep working without migration.

Use `-fresh` to bypass the cache entirely.

## Linting

The repo ships a minimal `.golangci.yml` enabling `gofumpt`, `govet`, `errcheck`, `ineffassign`, `staticcheck`, and `unused`.

Install:

```
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

Run:

```
golangci-lint run
```

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
