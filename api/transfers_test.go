package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type transferTestServer struct {
	calls      int
	lastBody   []byte
	lastMethod string
	lastPath   string

	// Per-call response configuration. ValidateCalls records confirm-false;
	// CommitCalls records confirm-true; if both are non-zero we can assert
	// the dance happened.
	validateCalls int
	commitCalls   int

	// Response config
	statusCode int
	body       string
	omitType   bool // emulate the no-Content-Type success quirk
}

func newTransferServer(t *testing.T, ts *transferTestServer) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/transfers/", func(w http.ResponseWriter, r *http.Request) {
		ts.calls++
		ts.lastMethod = r.Method
		ts.lastPath = r.URL.Path
		body, _ := readAll(r.Body)
		ts.lastBody = body

		var req TransferRequest
		_ = json.Unmarshal(body, &req)
		if req.Confirmed {
			ts.commitCalls++
		} else {
			ts.validateCalls++
		}

		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST, OPTIONS")
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}

		if ts.omitType {
			w.Header().Del("Content-Type")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(ts.statusCode)
		_, _ = w.Write([]byte(ts.body))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// readAll is a tiny wrapper so we don't import io everywhere.
func readAll(r interface{ Read(p []byte) (int, error) }) ([]byte, error) {
	var out []byte
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return out, nil
			}
			return out, err
		}
	}
}

func sampleTransferRequest() TransferRequest {
	return TransferRequest{
		Confirmed: false,
		Entry:     3808385,
		Event:     1,
		Transfers: []Transfer{
			{ElementIn: 514, ElementOut: 437, PurchasePrice: 75, SellingPrice: 68},
		},
	}
}

func TestValidateTransfersSuccess(t *testing.T) {
	ts := &transferTestServer{statusCode: http.StatusOK, body: "", omitType: true}
	srv := newTransferServer(t, ts)
	client := newMyTeamClient(srv.URL)

	spent, err := client.ValidateTransfers(sampleTransferRequest())
	if err != nil {
		t.Fatalf("ValidateTransfers: %v", err)
	}
	if spent != 0 {
		t.Errorf("spent = %d, want 0 on empty success body", spent)
	}
	if ts.validateCalls != 1 {
		t.Errorf("validate calls = %d, want 1", ts.validateCalls)
	}
	if ts.commitCalls != 0 {
		t.Errorf("commit calls = %d, want 0", ts.commitCalls)
	}
	var got TransferRequest
	if err := json.Unmarshal(ts.lastBody, &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Confirmed {
		t.Error("validated body has confirmed=true")
	}
	if len(got.Transfers) != 1 || got.Transfers[0].ElementIn != 514 {
		t.Errorf("transfer element_in = %d, want 514", got.Transfers[0].ElementIn)
	}
}

func TestValidateTransfersErrorWithSpentPoints(t *testing.T) {
	ts := &transferTestServer{
		statusCode: http.StatusOK,
		body:       `{"non_form_errors":["You don't have enough funds in your team to make this transfer."],"spent_points":4,"entry":3808385}`,
	}
	srv := newTransferServer(t, ts)
	client := newMyTeamClient(srv.URL)

	spent, err := client.ValidateTransfers(sampleTransferRequest())
	if err == nil {
		t.Fatal("expected validation error")
	}
	if spent != 4 {
		t.Errorf("spent = %d, want 4", spent)
	}
	te := AsTransferError(err)
	if te == nil {
		t.Fatalf("err is not *TransferError: %v", err)
	}
	if len(te.NonFormErrors) != 1 || !contains(te.NonFormErrors[0], "enough funds") {
		t.Errorf("NonFormErrors = %v", te.NonFormErrors)
	}
}

func TestValidateTransfersStructuredNonFieldError(t *testing.T) {
	ts := &transferTestServer{
		statusCode: http.StatusBadRequest,
		body:       `{"non_field_errors":[{"code":"transfer_team_limit_reached","message":"You cannot have more than 3 players from the same team."}],"spent_points":4}`,
	}
	srv := newTransferServer(t, ts)
	client := newMyTeamClient(srv.URL)

	spent, err := client.ValidateTransfers(sampleTransferRequest())
	if err == nil {
		t.Fatal("expected validation error")
	}
	if spent != 4 {
		t.Errorf("spent = %d, want 4", spent)
	}
	te := AsTransferError(err)
	if te == nil {
		t.Fatalf("err is not *TransferError: %v", err)
	}
	if !te.HasCode("transfer_team_limit_reached") {
		t.Fatal("error code transfer_team_limit_reached was not decoded")
	}
	if !strings.Contains(err.Error(), "status 400") || !strings.Contains(err.Error(), "more than 3 players") {
		t.Errorf("error = %q, want existing status text and decoded message", err)
	}
}

func TestCommitTransfersSuccess(t *testing.T) {
	ts := &transferTestServer{statusCode: http.StatusOK, body: "", omitType: true}
	srv := newTransferServer(t, ts)
	client := newMyTeamClient(srv.URL)

	if err := client.CommitTransfers(sampleTransferRequest()); err != nil {
		t.Fatalf("CommitTransfers: %v", err)
	}
	if ts.commitCalls != 1 {
		t.Errorf("commit calls = %d, want 1", ts.commitCalls)
	}
	var got TransferRequest
	if err := json.Unmarshal(ts.lastBody, &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !got.Confirmed {
		t.Error("commit body has confirmed=false")
	}
}

func TestCommitTransfersRejectsErrorBody(t *testing.T) {
	ts := &transferTestServer{
		statusCode: http.StatusOK,
		body:       `{"non_form_errors":["The gameweek is no longer open for transfers."],"spent_points":4}`,
	}
	srv := newTransferServer(t, ts)
	client := newMyTeamClient(srv.URL)

	err := client.CommitTransfers(sampleTransferRequest())
	if err == nil {
		t.Fatal("expected commit error")
	}
	te := AsTransferError(err)
	if te == nil {
		t.Fatalf("err is not *TransferError: %v", err)
	}
	if !contains(te.NonFormErrors[0], "no longer open") {
		t.Errorf("NonFormErrors = %v", te.NonFormErrors)
	}
}

func TestTransferRequestChipAlignment(t *testing.T) {
	ts := &transferTestServer{statusCode: http.StatusOK, body: "", omitType: true}
	srv := newTransferServer(t, ts)
	client := newMyTeamClient(srv.URL)

	req := sampleTransferRequest()
	req.Chip = ChipPtr("wildcard")
	if _, err := client.ValidateTransfers(req); err != nil {
		t.Fatalf("ValidateTransfers: %v", err)
	}
	var got TransferRequest
	if err := json.Unmarshal(ts.lastBody, &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Chip == nil || *got.Chip != "wildcard" {
		t.Errorf("chip = %v, want wildcard", got.Chip)
	}
	if !got.Wildcard {
		t.Error("Wildcard flag should be true")
	}
	if got.Freehit {
		t.Error("Freehit flag should be false")
	}
}

func TestTransferRequestChipStrippedWhenUnknown(t *testing.T) {
	ts := &transferTestServer{statusCode: http.StatusOK, body: "", omitType: true}
	srv := newTransferServer(t, ts)
	client := newMyTeamClient(srv.URL)

	req := sampleTransferRequest()
	req.Chip = ChipPtr("bboost") // not valid for /api/transfers/
	if _, err := client.ValidateTransfers(req); err != nil {
		t.Fatalf("ValidateTransfers: %v", err)
	}
	var got TransferRequest
	if err := json.Unmarshal(ts.lastBody, &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Chip != nil {
		t.Errorf("chip = %v, want nil", got.Chip)
	}
}

func TestTransferForbidden(t *testing.T) {
	ts := &transferTestServer{statusCode: http.StatusForbidden, body: `{"detail":"Authentication credentials were not provided."}`}
	srv := newTransferServer(t, ts)
	client := newMyTeamClient(srv.URL)

	_, err := client.ValidateTransfers(sampleTransferRequest())
	if err == nil {
		t.Fatal("expected auth error")
	}
}

func TestTransferRequestMethodPOST(t *testing.T) {
	ts := &transferTestServer{statusCode: http.StatusOK, body: "", omitType: true}
	srv := newTransferServer(t, ts)
	client := newMyTeamClient(srv.URL)

	if _, err := client.ValidateTransfers(sampleTransferRequest()); err != nil {
		t.Fatalf("ValidateTransfers: %v", err)
	}
	if ts.lastMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", ts.lastMethod)
	}
}

func TestEncodeTransferRequest(t *testing.T) {
	req := sampleTransferRequest()
	b, err := EncodeTransferRequest(req)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	var roundTrip TransferRequest
	if err := json.Unmarshal(b, &roundTrip); err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	if roundTrip.Entry != req.Entry {
		t.Errorf("Entry round-trip = %d, want %d", roundTrip.Entry, req.Entry)
	}
	if len(roundTrip.Transfers) != 1 || roundTrip.Transfers[0].ElementIn != 514 {
		t.Errorf("Transfers round-trip = %+v", roundTrip.Transfers)
	}
}

func TestTwoStepDance(t *testing.T) {
	ts := &transferTestServer{statusCode: http.StatusOK, body: "", omitType: true}
	srv := newTransferServer(t, ts)
	client := newMyTeamClient(srv.URL)

	if _, err := client.ValidateTransfers(sampleTransferRequest()); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := client.CommitTransfers(sampleTransferRequest()); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if ts.validateCalls != 1 || ts.commitCalls != 1 {
		t.Errorf("validate=%d commit=%d, want 1 each", ts.validateCalls, ts.commitCalls)
	}
}

func TestBytesTrimSpace(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"\t\n", ""},
		{"a", "a"},
		{"  a  ", "a"},
		{"\n{\"x\":1}\t", `{"x":1}`},
	}
	for _, c := range cases {
		got := string(bytesTrimSpace([]byte(c.in)))
		if got != c.want {
			t.Errorf("bytesTrimSpace(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
