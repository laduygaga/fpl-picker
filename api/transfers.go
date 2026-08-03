package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Transfer describes a single player swap in a TransferRequest.
type Transfer struct {
	ElementIn     int `json:"element_in"`
	ElementOut    int `json:"element_out"`
	PurchasePrice int `json:"purchase_price"`
	SellingPrice  int `json:"selling_price"`
}

// TransferRequest is the body for /api/transfers/.
//
// Confirmed controls the two-step dance: false to validate, true to commit.
// Event is the *target* gameweek (current+1 per amosbastian/fpl), NOT the
// currently-playing GW. Chip is "wildcard" or "freehit" (or nil);
// Wildcard + Freehit bools must agree with Chip.
type TransferRequest struct {
	Confirmed bool       `json:"confirmed"`
	Entry     int        `json:"entry"`
	Event     int        `json:"event"`
	Transfers []Transfer `json:"transfers"`
	Chip      *string    `json:"chip"`
	Wildcard  bool       `json:"wildcard"`
	Freehit   bool       `json:"freehit"`
}

// TransferError is the failure body returned by /api/transfers/.
// SpentPoints is the points hit the server WOULD charge even on failure —
// useful for surfacing a preview before re-issuing confirmed=true.
type TransferError struct {
	NonFormErrors []string `json:"non_form_errors"`
	SpentPoints   int      `json:"spent_points"`
	Entry         int      `json:"entry,omitempty"`
}

// Error implements the error interface for TransferError.
func (t *TransferError) Error() string {
	if len(t.NonFormErrors) == 0 {
		return fmt.Sprintf("/api/transfers/: spent %d points", t.SpentPoints)
	}
	return fmt.Sprintf("/api/transfers/: %s (would spend %d points)",
		t.NonFormErrors[0], t.SpentPoints)
}

// transferPath is the unique endpoint for both validation and commit.
const transferPath = "/api/transfers/"

// ValidateTransfers POSTs with confirmed=false. On success returns the
// spent_points preview the server would charge. On failure returns a
// *TransferError (or wrapped error) so the caller can surface why the
// transfer bundle was rejected.
func (c *AuthClient) ValidateTransfers(req TransferRequest) (int, error) {
	req.Confirmed = false
	req.Chip, req.Wildcard, req.Freehit = alignChip(req.Chip, req.Wildcard, req.Freehit)
	return c.postTransfer(req, true /* wantJSON */)
}

// CommitTransfers POSTs with confirmed=true. Returns nil on HTTP 200,
// *TransferError on validation failure, or another wrapped error otherwise.
func (c *AuthClient) CommitTransfers(req TransferRequest) error {
	req.Confirmed = true
	req.Chip, req.Wildcard, req.Freehit = alignChip(req.Chip, req.Wildcard, req.Freehit)
	_, err := c.postTransfer(req, false /* wantJSON */)
	return err
}

// alignChip keeps the three chip fields consistent — Chip is the canonical
// name, Wildcard/Freehit are boolean flags that mirror it for the server.
// Anything outside the allowed set normalises to no-chip.
func alignChip(chip *string, wild, free bool) (*string, bool, bool) {
	name := deref(chip)
	switch name {
	case "wildcard":
		return chip, true, false
	case "freehit":
		return chip, false, true
	default:
		return nil, false, false
	}
}

// postTransfer issues the POST and parses the response according to wantJSON.
//   - wantJSON=true (validation): expect either an empty 200 (no Content-Type
//     quirk on /api/transfers/, see fpl-api.md §9.1) or a JSON body with
//     non_form_errors / spent_points.
//   - wantJSON=false (commit): only HTTP 200 with empty body counts as
//     success; any JSON body is treated as a TransferError.
//
// In both cases, an HTTP 200 with no body and no Content-Type header is a
// successful response.
func (c *AuthClient) postTransfer(req TransferRequest, wantJSON bool) (int, error) {
	resp, err := c.doPOST(transferPath, req)
	if err != nil {
		return 0, err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusForbidden {
		return 0, fmt.Errorf("%w: 403 from /api/transfers/ — session invalid", ErrAuthFailed)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("/api/transfers/ status %d: %s", resp.StatusCode, string(body))
	}

	// Empty body — could be a successful validation or commit; the server
	// frequently omits Content-Type on this path, so we can't probe for it.
	if len(bytesTrimSpace(body)) == 0 {
		if wantJSON {
			return 0, nil
		}
		return 0, nil
	}

	// Non-empty body — try to parse as TransferError.
	var terr TransferError
	if err := json.Unmarshal(body, &terr); err != nil {
		return 0, fmt.Errorf("/api/transfers/ returned non-JSON body: %q", string(body))
	}
	if len(terr.NonFormErrors) > 0 {
		return terr.SpentPoints, &terr
	}
	if wantJSON {
		return terr.SpentPoints, nil
	}
	return 0, nil
}

// bytesTrimSpace returns s with leading and trailing whitespace removed
// without allocating. We hand-roll it to avoid importing bytes for one call.
func bytesTrimSpace(s []byte) []byte {
	start, end := 0, len(s)
	for start < end {
		c := s[start]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		start++
	}
	for end > start {
		c := s[end-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		end--
	}
	return s[start:end]
}

// AsTransferError returns the *TransferError wrapped inside err, if any.
// Returns nil when err is not a *TransferError.
func AsTransferError(err error) *TransferError {
	var te *TransferError
	if errors.As(err, &te) {
		return te
	}
	return nil
}

// EncodeTransferRequest returns the JSON encoding that would be sent on POST.
func EncodeTransferRequest(req TransferRequest) ([]byte, error) {
	return json.Marshal(req)
}
