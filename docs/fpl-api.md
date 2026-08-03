# Fantasy Premier League HTTP API — Authenticated Reference

This document is a technical reference for the unofficial Fantasy Premier League
REST API at `https://fantasy.premierleague.com/api/`, scoped to the
capabilities required by the `fpl-picker` `apply` subcommand:

1. Authenticate a user
2. Read the current squad (picks, chips, bank)
3. Submit transfers
4. Update lineup / captain / formation
5. Activate chips (Wildcard / Free Hit / Bench Boost / Triple Captain)
6. Detect the next gameweek and its deadline

Cross-referenced against several production FPL libraries:

- **`amosbastian/fpl`** (Python) — login flow, `transfers/` payload, `my-team/` POST for lineup, captain/substitute logic, two-step `confirmed: false` then `true` dance
- **`ConorAspell` gist** (Python) — working end-to-end bot; headers, payload order, no-CSRF
- **`sgloutnikov/fpl-data-scrape`** (Python) — CSRF cookie retrieval from initial GET to `fantasy.premierleague.com`
- **`chrisbrownlie/fantasy`** (R) — shape of `/api/my-team/` (picks / chips / transfers blocks)
- **`mcclowes/fpl-oas`** (OpenAPI), **`nguyenanhducs/fpl-mcp-server` docs** — endpoint inventory cross-check
- **Bram Vanherle, 2019** (`medium.com`) — login form fields + cookie table
- **SO #38819570** — real-world login failure modes (form encoding, headers)
- **`amosbastian/fpl` issue #102** — `/api/transfers/` returns **no `Content-Type`** on success

Live curl checks from this environment confirmed:

- Public endpoints → HTTP 200 JSON
- Private endpoints unauthenticated → HTTP 403 `{"detail":"Authentication credentials were not provided."}`
- Missing trailing slash → 301 redirect (sometimes dropping the auth cookie)

> **Confidence** is noted per section in §11. *verified* = seen in production
> code or via curl here. *inferred* = pieced from multiple sources.

---

## 1. Base URLs

| Service                     | Host                                  |
| --------------------------- | ------------------------------------- |
| Game data + private API     | `https://fantasy.premierleague.com`   |
| Login (forms auth)          | `https://users.premierleague.com`     |

All API paths are under `https://fantasy.premierleague.com/api/`. Every path
**must end in a trailing slash** — without it, Fastly returns `301` and then
the redirected request is treated as a different endpoint. *(verified by curl:
`/api/me` → 301, `/api/me/` → 200.)*

---

## 2. Endpoint inventory

### 2.1 Public (no auth required)

| Method | Path                                              | Purpose                                                         |
| ------ | ------------------------------------------------- | --------------------------------------------------------------- |
| GET    | `/api/bootstrap-static/`                          | Players, teams, events/chips, game settings, totals             |
| GET    | `/api/bootstrap-dynamic/`                         | **Deprecated** — returns HTML 404 in current season             |
| GET    | `/api/fixtures/`                                  | All season fixtures                                              |
| GET    | `/api/fixtures/?event={gw}`                       | Fixtures for a given gameweek                                    |
| GET    | `/api/event/{gw}/live/`                           | Live scoring for a gameweek (`{elements: [...]}`)                |
| GET    | `/api/element-summary/{element_id}/`              | Per-player season + history summary                              |
| GET    | `/api/entry/{team_id}/`                           | Manager profile (read-only public view)                          |
| GET    | `/api/entry/{team_id}/history/`                   | Past + current season history incl. `chips` array                |
| GET    | `/api/entry/{team_id}/transfers/`                 | All transfers this season                                        |
| GET    | `/api/entry/{team_id}/event/{gw}/picks/`          | A past gameweek's picks (public-readable)                        |
| GET    | `/api/game-settings/`                             | League / squad / scoring / transfer rules                        |
| GET    | `/api/leagues-classic/{league_id}/standings/`     | Classic league standings                                         |
| GET    | `/api/leagues-h2h/{league_id}/standings/`         | H2H league standings                                             |
| GET    | `/api/teams/`                                     | All Premier League clubs                                         |
| GET    | `/api/regions/`                                   | UI region list                                                   |
| GET    | `/api/element-types/`                             | Position metadata (implicit in `bootstrap-static`)              |

### 2.2 Private (require session cookie from login)

| Method | Path                                          | Purpose                                              |
| ------ | --------------------------------------------- | ---------------------------------------------------- |
| GET    | `/api/me/`                                    | Logged-in user profile (entry id, watched list)      |
| GET    | `/api/my-team/{team_id}/`                     | Current squad, chips, transfer status                |
| POST   | `/api/my-team/{team_id}/`                     | **Update lineup / activate chip**                    |
| GET    | `/api/entry/{team_id}/transfers-latest/`      | Transfers made in current period (pending + done)    |
| POST   | `/api/transfers/`                             | **Submit / validate transfers + chip activation**    |

> The `/api/entry/{tid}/event/{gw}/picks/` endpoint is technically readable
> without auth (we tested entry 1 and got `{"detail":"..."}` 404 — not 403 — for
> a non-existent team), but it returns the *picks at a past gameweek*. For the
> **current** gameweek's picks you must use `/api/my-team/{tid}/`.

---

## 3. Authentication

### 3.1 The login endpoint

```
POST https://users.premierleague.com/accounts/login/
Content-Type: application/x-www-form-urlencoded
Referer:     https://fantasy.premierleague.com/
Origin:      https://fantasy.premierleague.com
```

Form body (verified in `amosbastian/fpl` + multiple SO answers):

```
login=you@example.com
password=your-password
app=plfpl-web
redirect_uri=https://fantasy.premierleague.com/a/login
```

Optional: `csrfmiddlewaretoken` — taken from a `csrftoken` cookie issued by an
initial `GET https://fantasy.premierleague.com/`. Some libraries omit it; FPL
will still accept the request most of the time but in heavy traffic it helps
avoid 403s. The `csrftoken` cookie is set with `Domain=.premierleague.com` so
the login endpoint can read it.

### 3.2 Cookies set on success

| Cookie        | Domain                  | Role                                            |
| ------------- | ----------------------- | ----------------------------------------------- |
| `csrftoken`   | `.premierleague.com`    | Anti-CSRF token for subsequent POSTs            |
| `sessionid`   | `users.premierleague.com` | Identity for `users.premierleague.com` (login) |
| `sessionid`   | `fantasy.premierleague.com` | Identity for `fantasy.premierleague.com` (API)|
| `pl_profile`  | `.premierleague.com`    | Tells the API you're "logged in" — **essential**|

> Multiple sources (Bram Vanherle's Medium write-up, the SO answers,
> `amosbastian/fpl`) all converge on `pl_profile` being the cookie that
> distinguishes an authenticated request from a logged-out one. Without it,
> every private endpoint returns `403 Authentication credentials were not
> provided.`

### 3.3 Success vs. failure response

The login endpoint is a Django auth view: on success it 302-redirects to
`https://fantasy.premierleague.com/a/login?state=success`; on failure it
redirects to the same URL with `?state=fail&reason=<reason>`. The redirect
target is what you inspect (`response.url.query["state"]` in `amosbastian/fpl`),
not the response body — there isn't a useful JSON body.

Failure reasons seen in the wild:

- `state=fail&reason=InvalidLogin`
- `state=fail&reason=LockedAccount` (after too many failed attempts)
- Plain **403** if you send a non-browser `User-Agent` and FPL's edge has
  flagged the IP — the `amosbastian/fpl` source code handles this with a
  hint: *"consider setting `FPL_COOKIE` environment variable to the cookie
  in your browser when logged into the fpl website."*

### 3.4 Recommended User-Agent

`amosbastian/fpl` defaults to the FPL Android app's UA which avoids the 403:

```
Dalvik/2.1.0 (Linux; U; Android 5.1; PRO 5 Build/LMY47D)
```

A normal browser UA also works (`Mozilla/5.0 …`). We confirmed by curl that
**the public endpoints do NOT block empty or default Go `User-Agent` strings**
(200 in all cases), but login may. Use the Android UA to be safe.

### 3.5 Go reference snippet — login

```go
// POST https://users.premierleague.com/accounts/login/
// (note: uses jar across both domains; no body is JSON — form-encoded)
type LoginRequest struct {
    Login       string // email
    Password    string
    App         string // "plfpl-web"
    RedirectURI string // "https://fantasy.premierleague.com/a/login"
    CSRFToken   string // optional, from csrftoken cookie
}

// After login, do a cheap authenticated probe:
func (c *Client) IsLoggedIn() bool {
    resp, _ := c.httpClient.Get("https://fantasy.premierleague.com/api/me/")
    var body struct{ Player json.RawMessage }
    _ = json.NewDecoder(resp.Body).Decode(&body)
    return body.Player != nil // "player": null when anonymous
}
```

---

## 4. Current squad (`GET /api/my-team/{team_id}/`)

Returns the user's *current, in-flight* squad. This is the authoritative read
for the lineup you'll modify. Same response shape is implied by both
`amosbastian/fpl` (Python) and the `chrisbrownlie/fantasy` (R) wrapper.

### 4.1 Response shape (verified)

```json
{
  "picks": [
    {
      "element": 318,           // player id (== element_id in bootstrap-static)
      "position": 1,            // 1..15 — slot in the *ordered* squad;
                                //   FPL re-sorts so formation reads naturally
                                //   (GKP -> DEF -> MID -> FWD -> bench)
      "multiplier": 2,          // 0 = bench, 1 = starter, 2 = captain,
                                //   3 = triple-captain. Read-only — server
                                //   derives from is_captain + active_chip.
      "is_captain": true,
      "is_vice_captain": false,
      "selling_price": 71,
      "purchase_price": 70,
      "can_sub": true,          // false if already auto-subbed in
      "has_played": false,
      "is_sub": false,
      "element_type": 4
    }
    // ... 14 more, positions 2..15
  ],
  "chips": [                    // chips AVAILABLE to play this GW
    {
      "id": 1,
      "name": "wildcard",
      "number": 1,              // 1 or 2
      "start_event": 2,
      "stop_event": 19,
      "chip_type": "transfer",
      "overrides": { "rules": {}, "scoring": {}, "element_types": [], "pick_multiplier": null }
    }
    // ... freehit, bboost, 3xc
  ],
  "transfers": {
    "bank": 26,                 // tenths of £m → £2.6m in the bank
    "value": 1011,              // squad value (without bank)
    "limit": 1,                 // free transfers for this GW
    "made": 0,                  // transfers already made this GW
    "entry": 3808385
  }
}
```

> The same `chips` array is also present in `bootstrap-static` (top-level
> `chips` key) — that one describes the *chip catalog for the season*; the
> one in `/api/my-team/` describes chips the user has **left** (already-played
> chips are missing from the array).

### 4.2 Go reference snippet — read squad

```go
type Pick struct {
    Element        int  `json:"element"`
    Position       int  `json:"position"`
    Multiplier     int  `json:"multiplier"`
    IsCaptain      bool `json:"is_captain"`
    IsViceCaptain  bool `json:"is_vice_captain"`
    SellingPrice   int  `json:"selling_price"`
    PurchasePrice  int  `json:"purchase_price"`
    CanSub         bool `json:"can_sub"`
    HasPlayed      bool `json:"has_played"`
    IsSub          bool `json:"is_sub"`
    ElementType    int  `json:"element_type"`
}

type TransferStatus struct {
    Bank  int `json:"bank"`
    Value int `json:"value"`
    Limit int `json:"limit"`
    Made  int `json:"made"`
    Entry int `json:"entry"`
}

type MyTeam struct {
    Picks     []Pick         `json:"picks"`
    Chips     []any          `json:"chips"`     // narrow to your chip type if needed
    Transfers TransferStatus `json:"transfers"`
}
```

---

## 5. Lineup / chip activation (`POST /api/my-team/{team_id}/`)

Same path as the read; **POST** overwrites the squad's `position`, captaincy
and (optionally) activates a chip. Verified in both `amosbastian/fpl` (Python)
and the `ConorAspell` gist.

### 5.1 Request body (verified)

```json
{
  "picks": [
    { "element": 113, "position":  1, "is_captain": false, "is_vice_captain": false },  // GKP
    { "element":  10, "position":  2, "is_captain": false, "is_vice_captain": false },  // DEF
    { "element": 285, "position":  3, "is_captain": false, "is_vice_captain": true  },  // DEF (vice)
    { "element": 448, "position":  4, "is_captain": false, "is_vice_captain": false },
    { "element": 146, "position":  5, "is_captain": false, "is_vice_captain": false },  // MID
    { "element": 283, "position":  6, "is_captain": false, "is_vice_captain": false },
    { "element":  19, "position":  7, "is_captain": false, "is_vice_captain": false },
    { "element": 246, "position":  8, "is_captain": false, "is_vice_captain": false },
    { "element": 314, "position":  9, "is_captain": false, "is_vice_captain": false },  // FWD
    { "element": 318, "position": 10, "is_captain": true,  "is_vice_captain": false },  // FWD (c)
    { "element":  28, "position": 11, "is_captain": false, "is_vice_captain": false },
    { "element": 254, "position": 12, "is_captain": false, "is_vice_captain": false },  // bench
    { "element": 295, "position": 13, "is_captain": false, "is_vice_captain": false },
    { "element": 346, "position": 14, "is_captain": false, "is_vice_captain": false },
    { "element":  54, "position": 15, "is_captain": false, "is_vice_captain": false }
  ],
  "chip": null
}
```

Required headers (verified):

```
Content-Type: application/json
Origin:      https://fantasy.premierleague.com
Referer:     https://fantasy.premierleague.com/my-team   (or /a/team/my)
X-Requested-With: XMLHttpRequest
```

> The trailing `/` on the URL is **required** (we got 301 → 403 on bare
> paths). The `Referer` must point at a real FPL page; the SPA routes that
> trigger this POST are `/a/team/my` (lineup) and `/a/squad/transfers`
> (transfers).

### 5.2 Field semantics

- `picks[i].position` must be **1..15, all distinct, contiguous**. Positions
  1..11 are starters, 12..15 are bench. Bench order is bench-order 1..4
  (1 = first sub).
- `is_captain` may be `true` on **exactly one** pick; `is_vice_captain` on
  **exactly one** (different) pick.
- `multiplier` is *not* sent in the request — the server derives it from
  `is_captain` and the active chip (so `3xc` ⇒ multiplier 3 for captain).
- `selling_price`, `purchase_price`, `can_sub`, `has_played`, `is_sub` are
  *not* in the request body. Strip them before posting.
- `chip`: one of `"wildcard"`, `"freehit"`, `"bboost"`, `"3xc"`, or `null`.
  This is also how you activate Bench Boost (`bboost`) or Triple Captain
  (`3xc`) — those chips don't affect the `transfers/` endpoint at all.
- The server **derives formation** from positions 1..11; you don't send a
  formation field. You must still respect the formation validity rules (1
  GKP, 3–5 DEF, 2–5 MID, 1–3 FWD — see `bootstrap-static.game_settings`).

### 5.3 Response

On success the server returns the new `/api/my-team/` payload (same shape as
the GET). On failure, it returns HTTP 4xx with `{"detail":"…"}` or a
DRF-style validation dict (see §9).

---

## 6. Transfers (`POST /api/transfers/`)

The single endpoint for submitting player swaps. Method GET is **forbidden**
(verified: `allow: POST, OPTIONS`).

### 6.1 Two-step dance (verified in `amosbastian/fpl` + gist)

FPL's web UI does this:

1. POST with `"confirmed": false` to *validate* (free of side effects).
2. If validation passes (no error in response), POST again with
   `"confirmed": true` to *commit*.

`amosbastian/fpl` is explicit about this in `transfer()`:
> *"Send POST requests with `confirmed` set to False; this basically checks
> if there are any errors from FPL's side for this transfer… Everything is
> okay, so push the transfer through! `payload["confirmed"] = True`."*

You can usually get away with a single POST with `confirmed: true` if the
payload is well-formed, but the safe pattern is the two-step one — it gives
you the points-hit and validation errors before they hit your live team.

### 6.2 Request body (verified)

```json
{
  "confirmed": false,
  "entry": 3808385,
  "event": 1,                          // gameweek id
  "transfers": [
    {
      "element_in":      514,
      "element_out":     437,
      "purchase_price":  75,            // price of element_in at submit time
      "selling_price":   68             // current selling price of element_out
                                        // (from my-team.picks[].selling_price)
    }
  ],
  "chip": null,                         // OR "wildcard" / "freehit"
  "wildcard": false,
  "freehit": false
}
```

Notes:

- `event` is the **target** gameweek (i.e. the next GW, not the current one).
  `amosbastian/fpl` literally does `event = self.current_event + 1`.
- `purchase_price` is in 0.1m units (matches `bootstrap-static.elements[i].now_cost`).
- `selling_price` must match what `/api/my-team/` returns; the server
  cross-checks this and rejects if it's stale.
- `chip`, `wildcard`, `freehit`: all three must agree. Setting `chip: "wildcard"`
  and `wildcard: true` is what both `amosbastian/fpl` and `ConorAspell` do.
- Points-hits are computed **server-side**: you don't pre-compute them; the
  server returns `spent_points` in the response. With `chip: "wildcard"` or
  `"freehit"` no points are spent regardless of how many transfers you bundle.

### 6.3 Response shapes (verified in code, *inferred* in shape)

**Validation success, `confirmed: false`** (you'll see this just before the
real commit):

```
HTTP/1.1 200 OK
(no Content-Type header — must not call resp.json() blindly; see §9)
<empty body>
```

**Validation failure** (insufficient funds, same team as another, deadline
passed, illegal substitution, etc.):

```json
{
  "non_form_errors": [
    "You don't have enough funds in your team to make this transfer."
  ],
  "spent_points": 4,
  "entry": 3808385
}
```

or, for chip/role issues:

```json
{
  "non_form_errors": ["You can't play your wildcard and free hit in the same gameweek."]
}
```

The `spent_points` field is what `amosbastian/fpl` uses to enforce a max-hit
guard before re-posting with `confirmed: true`.

**Commit success** (`confirmed: true` accepted): HTTP 200 with no body, OR
the new `/api/my-team/` snapshot — implementations disagree. Treat the
absence of `non_form_errors` in the body as success.

### 6.4 Go reference snippet — transfer

```go
type Transfer struct {
    ElementIn     int `json:"element_in"`
    ElementOut    int `json:"element_out"`
    PurchasePrice int `json:"purchase_price"`
    SellingPrice  int `json:"selling_price"`
}

type TransferRequest struct {
    Confirmed bool       `json:"confirmed"`
    Entry     int        `json:"entry"`     // team_id
    Event     int        `json:"event"`     // gameweek id (next, not current)
    Transfers []Transfer `json:"transfers"`
    Chip      *string    `json:"chip"`      // "wildcard","freehit","bboost","3xc",null
    Wildcard  bool       `json:"wildcard"`
    Freehit   bool       `json:"freehit"`
}

type TransferError struct {
    NonFormErrors []string `json:"non_form_errors"`
    SpentPoints   int      `json:"spent_points"`
}
```

---

## 7. Deadlines & gameweek detection

All derived from `bootstrap-static.events[]`. No extra endpoint is needed.

### 7.1 `events[]` shape (verified live)

```json
{
  "id": 1,
  "name": "Gameweek 1",
  "deadline_time":          "2026-08-21T17:30:00Z",   // ISO-8601 UTC
  "deadline_time_epoch":    1787333400,                // Unix seconds
  "deadline_time_game_offset": 0,
  "average_entry_score":    0,
  "finished":               false,
  "data_checked":           false,
  "is_previous":            false,
  "is_current":             false,
  "is_next":                true,
  "chip_plays": [                                    // populated after GW finishes
    { "chip_name": "wildcard", "num_played": 1370000 }
  ],
  "highest_score":          null,
  "top_element":            null,
  "transfers_made":         0,
  "most_selected":          null,
  "most_captained":         null,
  "can_enter":              true,
  "can_manage":             true,
  "released":               true,
  "ranked_count":           0
  // + a handful of *_info / most_* nulls before this
}
```

Across 38 events you will see exactly:

- 0–1 with `is_previous: true` (the last completed GW)
- 0–1 with `is_current: true` (a GW that has started but not finished)
- 0–1 with `is_next: true` (the next GW to manage)

Use `is_next` to find the GW you should be editing. Treat
`event.deadline_time_epoch` as the lock time — once `now >= deadline_time_epoch`
the server rejects writes to `/api/transfers/` and `/api/my-team/` for that
GW.

### 7.2 Server-side lock enforcement (inferred)

There is **no explicit `locked` or `confirmed` flag** in the events payload.
The server enforces the deadline implicitly:

- POSTs to `/api/transfers/` and `/api/my-team/` after `deadline_time_epoch`
  return `non_form_errors` like *"The gameweek is no longer open for
  transfers."* (observed in multiple sources; exact wording varies).
- `event.deadline_time_epoch` shifts if matches are postponed; refetch
  `bootstrap-static/` before each write.

### 7.3 `deadline_time_game_offset`

`deadline_time_game_offset: 0` means the deadline lines up with the first
kickoff of that GW. Positive values mean the deadline is *later* than first
kickoff (rare; only used in dead-rubber or rearranged fixtures).

---

## 8. Chips — full picture

Chips appear in three places, with subtly different meanings:

| Source                  | Field                  | Meaning                                                                 |
| ----------------------- | ---------------------- | ----------------------------------------------------------------------- |
| `bootstrap-static.chips`| top-level `chips[]`    | Season catalog: every chip available to anyone (8 entries: 2× each of WC, FH, BB, TC; first-half + second-half variants). |
| `/api/my-team/`         | `chips[]`              | Chips the **user** still has available to play. Already-played chips are absent. |
| `/api/my-team/`         | `active_chip` (none in picks here — see below) | (server-internal) |

Chip *names* are short strings, all lowercase except `3xc`:

```
"wildcard"  "freehit"  "bboost"  "3xc"   nil
```

Activation rules:

- **Wildcard** (`wildcard`) — set `chip: "wildcard"` on either `/api/transfers/`
  (when you're also making transfers) **or** `/api/my-team/` (lineup-only).
- **Free Hit** (`freehit`) — same as Wildcard but team reverts after the GW.
  Same two endpoints.
- **Bench Boost** (`bboost`) — set `chip: "bboost"` on `/api/my-team/` only.
  It doesn't change transfers; it just multiplies bench points.
- **Triple Captain** (`3xc`) — set `chip: "3xc"` on `/api/my-team/` only.

`bootstrap-static.chips` shows you the *window* each chip is valid in
(`start_event` / `stop_event`). For 2026/27 these are 2–19 (first half) and
20–38 (second half). Trying to activate a chip outside its window returns
a `non_form_errors` validation failure.

---

## 9. Error responses

| Status | Where                                | Body shape                                                                       | Cause                                                          |
| ------ | ------------------------------------ | -------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| 403    | any private endpoint                 | `{"detail":"Authentication credentials were not provided."}`                     | No session / wrong cookies / not your own team_id              |
| 403    | `/api/transfers/` GET                | same                                                                             | Endpoint is POST-only                                          |
| 403    | `users.premierleague.com/.../login/` | 403 (no JSON)                                                                    | Bot-like User-Agent + IP reputation (retry with browser UA)    |
| 404    | `/api/entry/{tid}/event/{gw}/picks/` | `{"detail":"Not found."}`                                                        | Unknown team_id or GW                                          |
| 400    | `/api/my-team/` POST                 | DRF field errors `{"element":[...], "non_field_errors":[...]}`                   | Malformed picks (e.g. duplicates, missing captain, bad pos)     |
| 400    | `/api/transfers/` POST               | `{"non_form_errors":["..."], "spent_points":N}`                                  | Insufficient funds, deadline passed, 3+ players from one club  |
| 200    | `/api/transfers/` POST               | empty body (no `Content-Type`)                                                   | Success — **don't try to JSON-decode this**, see below         |

### 9.1 The `transfers/` 200-with-no-content-type trap

`amosbastian/fpl` issue #102 documents that `/api/transfers/` POST success
returns **no `Content-Type` header**. Both `amosbastian/fpl` and several
custom implementations work around this by reading the body as raw text and
treating an empty body as success. In Go:

```go
resp, err := http.DefaultClient.Do(req)
defer resp.Body.Close()
body, _ := io.ReadAll(resp.Body)
if resp.StatusCode/100 != 2 {
    return fmt.Errorf("fpl transfers: %s", body)
}
// success — body is typically empty
```

### 9.2 Rate limiting / bot detection (inferred)

We did not hit any rate limit during testing. What we *did* observe:

- Public endpoints accept any UA (incl. default Go) — **no CAPTCHA**.
- Login is the only place where we've seen soft blocking (403 with
  suspicious UA from bad IPs).
- The FPL API goes down briefly around gameweek deadlines (community
  knowledge, confirmed in the `fpl-intelligence` package's troubleshooting
  docs).

There is **no documented API quota**. Be polite — one transfer POST every few
seconds is more than fine; spamming will eventually be caught by the Fastly
edge.

---

## 10. End-to-end flow for `fpl-picker apply`

1. **Login** once (or reuse a cookie jar). Persist the jar to disk so the user
   doesn't re-auth every run.
2. **Fetch `bootstrap-static/`** (already done). Find `events[].is_next` ⇒
   target `event.id` and `event.deadline_time_epoch`.
3. **Reject** if `now >= deadline_time_epoch`.
4. **Fetch `/api/my-team/{tid}/`** → read current squad + `transfers.bank`.
5. **Plan** lineup (positions 1..11 starters, 12..15 bench, one captain, one
   vice-captain). Optionally pick a chip.
6. **POST `/api/my-team/{tid}/`** with the new `picks` and `chip`. Use
   `Referer: https://fantasy.premierleague.com/a/team/my`.
7. **POST `/api/transfers/`** with `confirmed: false`, validate, then
   `confirmed: true`. Use `Referer: https://fantasy.premierleague.com/a/squad/transfers`.
8. **Re-fetch `/api/my-team/{tid}/`** to confirm server state matches the plan.

---

## 11. Source-attached confidence table

| Section                                    | Confidence   | Evidence                                                                            |
| ------------------------------------------ | ------------ | ----------------------------------------------------------------------------------- |
| §3 Login flow + cookies                    | **High**     | Verified in 4 independent production libraries + SO answers                         |
| §4 my-team GET shape                       | **High**     | Verified in `amosbastian/fpl` `get_team()` parsing code + R wrapper                  |
| §5 my-team POST (lineup + chip)            | **High**     | Verified in `amosbastian/fpl` `_post_substitutions()` + ConorAspell gist            |
| §6 transfers POST shape (incl. 2-step)     | **High**     | Verified in `amosbastian/fpl` `_get_transfer_payload()` + `transfer()` + gist       |
| §6 `chip`/`wildcard`/`freehit` redundancy  | **Medium**   | Verified in source; exact server-side interpretation not observed                   |
| §6 `spent_points`, `non_form_errors`       | **High**     | Verified — `amosbastian/fpl` parses these for max-hit and exception raising         |
| §6.3 Empty body on commit success          | **Medium**   | Verified (amosbastian issue #102) but exact behaviour undocumented                  |
| §7 Deadline / `deadline_time_epoch`        | **High**     | Live curl of `bootstrap-static/`                                                    |
| §7 Server-side lock wording                | **Low–Med**  | Inferred from community docs; exact message text not observed personally            |
| §8 Chip catalog / activation rules         | **High**     | Verified live `bootstrap-static.chips[]` + confirmed in `amosbastian/fpl`           |
| §9.1 `transfers/` empty 200                | **High**     | Confirmed by amosbastian/fpl issue #102 + gist success path                          |
| §9 Rate limit / bot detection              | **Medium**   | No rate-limit hit during testing; details inferred from FPL edge behaviour          |
| §10 End-to-end apply flow                  | **Medium**   | Composite inference; not validated against live write                               |

---

## 12. Surprises / non-obvious things

1. **Two different hosts** for auth (`users.premierleague.com`) and the API
   (`fantasy.premierleague.com`). The cookies are scoped per-domain.
2. **Trailing slashes** are non-optional; Fastly 301s you otherwise, and
   sometimes the redirect drops the auth cookie.
3. **`pl_profile` is the magic cookie.** Drop it and you get 403 on
   everything, even with `sessionid` set.
4. **`/api/transfers/` success returns no `Content-Type`.** Don't `json.Decode`
   it; treat empty body as success.
5. **Two-step transfers** (`confirmed: false` then `confirmed: true`) is the
   standard pattern. The first call gives you `spent_points` so you can warn
   the user about points hits before committing.
6. **`event` in the transfer payload is `current_event + 1`**, not the
   current event. If you're managing GW 5, you POST for GW 5.
7. **Activation of Bench Boost and Triple Captain** does not go through
   `/api/transfers/` at all — they're set via `chip` on `/api/my-team/`.
8. **Formation is implicit**, derived from positions 1..11. You don't send a
   formation field; the server reads your element types and validates against
   `game_settings.squad_min_play` / `squad_max_play`.
9. **The chip catalog in `bootstrap-static.chips[]` has 8 entries** (2 × WC,
   2 × FH, 2 × BB, 2 × TC) — one per half-season window. The user's *remaining*
   chips in `/api/my-team/.chips[]` are the subset that haven't been played.
10. **No CSRF token is required** on the transfer / lineup POSTs. The
    `csrftoken` cookie is set during login but the actual API endpoints don't
    echo it back in a header (verified across all sources we read). The
    `Referer` header is the closest thing to a CSRF guard on writes.