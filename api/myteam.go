package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Pick mirrors a single element in a user's 15-man squad.
//
// Read-only fields (multiplier, selling_price, purchase_price, can_sub,
// has_played, is_sub, element_type) are populated by GET /api/my-team/ but
// stripped from the POST body — see LineupUpdate for the request shape.
type Pick struct {
	Element       int  `json:"element"`
	Position      int  `json:"position"`
	IsCaptain     bool `json:"is_captain"`
	IsViceCaptain bool `json:"is_vice_captain"`

	Multiplier    int  `json:"multiplier,omitempty"`
	SellingPrice  int  `json:"selling_price,omitempty"`
	PurchasePrice int  `json:"purchase_price,omitempty"`
	CanSub        bool `json:"can_sub,omitempty"`
	HasPlayed     bool `json:"has_played,omitempty"`
	IsSub         bool `json:"is_sub,omitempty"`
	ElementType   int  `json:"element_type,omitempty"`
}

// TransferStatus reports bank, squad value, free transfer budget, and how
// many transfers the user has already committed this gameweek.
type TransferStatus struct {
	Bank  int `json:"bank"`  // tenths of £m
	Value int `json:"value"` // squad value (without bank), tenths of £m
	Limit int `json:"limit"` // free transfers available this GW
	Made  int `json:"made"`  // transfers already used this GW
	Entry int `json:"entry"` // team_id (echoed by the server)
}

// ChipInfo describes one chip the user has left to play.
// The my-team endpoint returns the *remaining* chips (those not yet played).
type ChipInfo struct {
	ID         int    `json:"id"`
	Name       string `json:"name"` // "wildcard" | "freehit" | "bboost" | "3xc"
	Number     int    `json:"number"`
	StartEvent int    `json:"start_event"`
	StopEvent  int    `json:"stop_event"`
	ChipType   string `json:"chip_type"`
}

// MyTeam is the parsed GET /api/my-team/{tid}/ response.
type MyTeam struct {
	Picks     []Pick         `json:"picks"`
	Chips     []ChipInfo     `json:"chips"`
	Transfers TransferStatus `json:"transfers"`
}

// LineupUpdate is the POST body for /api/my-team/{tid}/. Only the writable
// fields (element, position, is_captain, is_vice_captain) appear in picks;
// the read-only fields are zero values and are stripped by omitempty.
//
// Chip is nil for no chip, or one of "wildcard", "freehit", "bboost", "3xc".
// Bench Boost and Triple Captain are activated ONLY through this endpoint;
// Wildcard and Free Hit may be set on either /api/my-team/ or /api/transfers/.
type LineupUpdate struct {
	Picks []Pick  `json:"picks"`
	Chip  *string `json:"chip"`
}

// GetMyTeam fetches the user's current squad for teamID.
func (c *AuthClient) GetMyTeam(teamID int) (*MyTeam, error) {
	path := "/api/my-team/" + strconv.Itoa(teamID) + "/"
	var team MyTeam
	if err := c.getJSON(path, &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// UpdateLineup posts a new lineup / chip activation. The server derives
// formation from positions 1..11 and expects exactly one captain and one
// vice-captain (different players).
//
// chip argument values:
//   - "" → no chip (LineupUpdate.Chip set to JSON null)
//   - "wildcard" / "freehit" / "bboost" / "3xc" → activate that chip
//
// Any other value is treated like "no chip" to keep the call safe from typos.
func (c *AuthClient) UpdateLineup(teamID int, update LineupUpdate) error {
	// Strip read-only fields from every pick — server derives them.
	for i := range update.Picks {
		update.Picks[i].Multiplier = 0
		update.Picks[i].SellingPrice = 0
		update.Picks[i].PurchasePrice = 0
		update.Picks[i].CanSub = false
		update.Picks[i].HasPlayed = false
		update.Picks[i].IsSub = false
		update.Picks[i].ElementType = 0
	}
	switch deref(update.Chip) {
	case "wildcard", "freehit", "bboost", "3xc":
		// keep
	default:
		update.Chip = nil
	}

	resp, err := c.doPOST("/api/my-team/"+strconv.Itoa(teamID)+"/", update)
	if err != nil {
		return err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%w: 403 on lineup update — session invalid", ErrAuthFailed)
	}
	return fmt.Errorf("lineup update failed: %d %s", resp.StatusCode, string(body))
}

// ChipPtr returns a pointer to its string arg. Tiny helper so callers can
// say Chip: api.ChipPtr("wildcard") when constructing a LineupUpdate.
func ChipPtr(s string) *string { return &s }

// deref returns the empty string when s is nil; otherwise the pointed-to value.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// LineupHasChanged returns true if the proposed LineupUpdate differs from
// the current team on any writable field (position, captaincy). Read-only
// fields are ignored so the server's transient state does not trigger a
// redundant POST.
func LineupHasChanged(current *MyTeam, proposed LineupUpdate) bool {
	if current == nil || len(current.Picks) != len(proposed.Picks) {
		return true
	}
	type key struct {
		element, position int
		cap, vice         bool
	}
	type val struct{}
	have := map[key]val{}
	for _, p := range current.Picks {
		have[key{p.Element, p.Position, p.IsCaptain, p.IsViceCaptain}] = val{}
	}
	for _, p := range proposed.Picks {
		k := key{p.Element, p.Position, p.IsCaptain, p.IsViceCaptain}
		if _, ok := have[k]; !ok {
			return true
		}
		delete(have, k)
	}
	return len(have) > 0
}

// EncodeMyTeam is a convenience wrapper for testing — returns the JSON
// encoding that the client would send on POST.
func EncodeMyTeam(update LineupUpdate) ([]byte, error) {
	return json.Marshal(update)
}
