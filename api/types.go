package api

// BootstrapResponse is the top-level response from /api/bootstrap-static/
type BootstrapResponse struct {
	Elements     []Player      `json:"elements"`
	Teams        []Team        `json:"teams"`
	Events       []Event       `json:"events"`
	ElementTypes []ElementType `json:"element_types"`
}

// Player represents a single FPL player from the elements array.
// String-typed numeric fields match the FPL API's JSON format.
type Player struct {
	ID                       int    `json:"id"`
	FirstName                string `json:"first_name"`
	SecondName               string `json:"second_name"`
	WebName                  string `json:"web_name"`
	Team                     int    `json:"team"`
	ElementType              int    `json:"element_type"` // 1=GK, 2=DEF, 3=MID, 4=FWD
	NowCost                  int    `json:"now_cost"`     // tenths of £M (e.g. 100 = £10.0M)
	Form                     string `json:"form"`
	PointsPerGame            string `json:"points_per_game"`
	TotalPoints              int    `json:"total_points"`
	Minutes                  int    `json:"minutes"`
	GoalsScored              int    `json:"goals_scored"`
	Assists                  int    `json:"assists"`
	CleanSheets              int    `json:"clean_sheets"`
	ExpectedGoals            string `json:"expected_goals"`
	ExpectedAssists          string `json:"expected_assists"`
	ExpectedGoalInvolvements string `json:"expected_goal_involvements"`
	ExpectedGoalsConceded    string `json:"expected_goals_conceded"`
	ICTIndex                 string `json:"ict_index"`
	Influence                string `json:"influence"`
	Creativity               string `json:"creativity"`
	Threat                   string `json:"threat"`
	EPNext                   string `json:"ep_next"`
	EPThis                   string `json:"ep_this"`
	SelectedByPercent        string `json:"selected_by_percent"`
	Status                   string `json:"status"` // a=available, d=doubtful, i=injured, s=suspended, u=unavailable
	ChanceOfPlayingNextRound *int   `json:"chance_of_playing_next_round"`
	News                     string `json:"news"`
}

// Team represents a Premier League team with strength ratings.
type Team struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	ShortName           string `json:"short_name"`
	Strength            int    `json:"strength"`
	StrengthOverallHome int    `json:"strength_overall_home"`
	StrengthOverallAway int    `json:"strength_overall_away"`
	StrengthAttackHome  int    `json:"strength_attack_home"`
	StrengthAttackAway  int    `json:"strength_attack_away"`
	StrengthDefenceHome int    `json:"strength_defence_home"`
	StrengthDefenceAway int    `json:"strength_defence_away"`
}

// Event represents a single gameweek.
type Event struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	DeadlineTime string `json:"deadline_time"`
	IsCurrent    bool   `json:"is_current"`
	IsNext       bool   `json:"is_next"`
	Finished     bool   `json:"finished"`
}

// ElementType describes a player position category.
type ElementType struct {
	ID           int    `json:"id"`
	SingularName string `json:"singular_name"`
	PluralName   string `json:"plural_name"`
	SquadSelect  int    `json:"squad_select"`
	SquadMinPlay int    `json:"squad_min_play"`
	SquadMaxPlay int    `json:"squad_max_play"`
}

// Fixture represents a single match.
type Fixture struct {
	ID              int  `json:"id"`
	Event           *int `json:"event"` // nullable for unscheduled fixtures
	TeamH           int  `json:"team_h"`
	TeamA           int  `json:"team_a"`
	TeamHDifficulty int  `json:"team_h_difficulty"`
	TeamADifficulty int  `json:"team_a_difficulty"`
	Finished        bool `json:"finished"`
	Started         bool `json:"started"`
}
