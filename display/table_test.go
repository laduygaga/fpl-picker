package display

import (
	"io"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"fpl-picker/api"
	"fpl-picker/model"
)

func TestHeaderLineContainsTitle(t *testing.T) {
	tests := []struct {
		title string
	}{
		{"TOP PICKS BY POSITION"},
		{"DIFFERENTIALS (Low Ownership, High Score)"},
		{"A"},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			line := headerLine(tt.title)
			if !strings.Contains(line, tt.title) {
				t.Errorf("headerLine(%q) = %q, does not contain title", tt.title, line)
			}
		})
	}
}

func TestHeaderLineWidth(t *testing.T) {
	line := headerLine("TEST")
	runeCount := utf8.RuneCountInString(line)
	if runeCount != 120 {
		t.Errorf("headerLine rune count = %d, want 120", runeCount)
	}
}

func TestHeaderLineUsesBoxChars(t *testing.T) {
	line := headerLine("HELLO")
	if !strings.Contains(line, "═") {
		t.Error("headerLine should use ═ characters for padding")
	}
}

func TestPosFullName(t *testing.T) {
	tests := []struct {
		pos  int
		want string
	}{
		{model.PosGK, "GOALKEEPERS"},
		{model.PosDEF, "DEFENDERS"},
		{model.PosMID, "MIDFIELDERS"},
		{model.PosFWD, "FORWARDS"},
		{99, "UNKNOWN"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := posFullName(tt.pos)
			if got != tt.want {
				t.Errorf("posFullName(%d) = %q, want %q", tt.pos, got, tt.want)
			}
		})
	}
}

func TestSortByPosAndScoreSortsByPositionFirst(t *testing.T) {
	players := []model.ScoredPlayer{
		{Player: api.Player{ElementType: model.PosFWD}, Score: 0.9},
		{Player: api.Player{ElementType: model.PosGK}, Score: 0.5},
		{Player: api.Player{ElementType: model.PosDEF}, Score: 0.8},
		{Player: api.Player{ElementType: model.PosMID}, Score: 0.7},
	}

	model.SortByPosAndScore(players)

	for i := 1; i < len(players); i++ {
		if players[i].Player.ElementType < players[i-1].Player.ElementType {
			t.Errorf("position out of order at index %d: %d < %d",
				i, players[i].Player.ElementType, players[i-1].Player.ElementType)
		}
	}
}

func TestSortByPosAndScoreSortsByScoreWithinPosition(t *testing.T) {
	players := []model.ScoredPlayer{
		{Player: api.Player{ElementType: model.PosDEF, ID: 1}, Score: 0.3},
		{Player: api.Player{ElementType: model.PosDEF, ID: 2}, Score: 0.9},
		{Player: api.Player{ElementType: model.PosDEF, ID: 3}, Score: 0.6},
	}

	model.SortByPosAndScore(players)

	if players[0].Player.ID != 2 {
		t.Errorf("first DEF should be highest score (ID=2), got ID=%d", players[0].Player.ID)
	}
	if players[1].Player.ID != 3 {
		t.Errorf("second DEF should be ID=3, got ID=%d", players[1].Player.ID)
	}
	if players[2].Player.ID != 1 {
		t.Errorf("third DEF should be lowest score (ID=1), got ID=%d", players[2].Player.ID)
	}
}

func TestSortByPosAndScoreEmpty(t *testing.T) {
	var players []model.ScoredPlayer
	model.SortByPosAndScore(players)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = origStdout
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading pipe: %v", err)
	}
	return string(out)
}

func TestPrintDifferentialsFiltersAboveThreshold(t *testing.T) {
	players := []model.ScoredPlayer{
		{Player: api.Player{WebName: "HighOwn", ElementType: model.PosMID, NowCost: 80}, Score: 0.9, OwnershipPct: 25.0, PositionName: "MID", TeamName: "ARS", OppDesc: "CHE(H)"},
		{Player: api.Player{WebName: "LowOwn", ElementType: model.PosMID, NowCost: 60}, Score: 0.7, OwnershipPct: 5.0, PositionName: "MID", TeamName: "LIV", OppDesc: "TOT(A)"},
	}

	output := captureStdout(t, func() {
		PrintDifferentials(players, 10.0, 5)
	})

	if strings.Contains(output, "HighOwn") {
		t.Error("player with 25% ownership should be excluded when maxOwnership=10")
	}
	if !strings.Contains(output, "LowOwn") {
		t.Error("player with 5% ownership should be included when maxOwnership=10")
	}
}

func TestPrintDifferentialsExcludesZeroOwnership(t *testing.T) {
	players := []model.ScoredPlayer{
		{Player: api.Player{WebName: "ZeroOwn", ElementType: model.PosFWD, NowCost: 50}, Score: 0.8, OwnershipPct: 0.0, PositionName: "FWD", TeamName: "MCI", OppDesc: "BUR(H)"},
	}

	output := captureStdout(t, func() {
		PrintDifferentials(players, 10.0, 5)
	})

	if strings.Contains(output, "ZeroOwn") {
		t.Error("player with 0% ownership should be excluded")
	}
	if !strings.Contains(output, "No differentials found") {
		t.Error("should show 'No differentials found' message")
	}
}

func TestPrintDifferentialsLimitsToN(t *testing.T) {
	players := []model.ScoredPlayer{
		{Player: api.Player{WebName: "P1", ElementType: model.PosMID, NowCost: 50}, Score: 0.9, OwnershipPct: 3.0, PositionName: "MID", TeamName: "ARS", OppDesc: "CHE(H)"},
		{Player: api.Player{WebName: "P2", ElementType: model.PosMID, NowCost: 50}, Score: 0.8, OwnershipPct: 4.0, PositionName: "MID", TeamName: "LIV", OppDesc: "TOT(A)"},
		{Player: api.Player{WebName: "P3", ElementType: model.PosMID, NowCost: 50}, Score: 0.7, OwnershipPct: 5.0, PositionName: "MID", TeamName: "MCI", OppDesc: "BUR(H)"},
	}

	output := captureStdout(t, func() {
		PrintDifferentials(players, 10.0, 2)
	})

	if !strings.Contains(output, "P1") {
		t.Error("P1 (highest score) should be included in top 2")
	}
	if !strings.Contains(output, "P2") {
		t.Error("P2 (second highest) should be included in top 2")
	}
	if strings.Contains(output, "P3") {
		t.Error("P3 should be excluded when n=2")
	}
}

func TestPrintDifferentialsSortsByScore(t *testing.T) {
	players := []model.ScoredPlayer{
		{Player: api.Player{WebName: "Slowpoke", ElementType: model.PosFWD, NowCost: 50}, Score: 0.3, OwnershipPct: 2.0, PositionName: "FWD", TeamName: "ARS", OppDesc: "CHE(H)"},
		{Player: api.Player{WebName: "Speedy", ElementType: model.PosFWD, NowCost: 50}, Score: 0.9, OwnershipPct: 2.0, PositionName: "FWD", TeamName: "LIV", OppDesc: "TOT(A)"},
	}

	output := captureStdout(t, func() {
		PrintDifferentials(players, 10.0, 10)
	})

	speedyIdx := strings.Index(output, "Speedy")
	slowIdx := strings.Index(output, "Slowpoke")
	if speedyIdx < 0 || slowIdx < 0 {
		t.Fatal("both players should appear in output")
	}
	if speedyIdx > slowIdx {
		t.Error("higher scoring player should appear before lower scoring player")
	}
}
