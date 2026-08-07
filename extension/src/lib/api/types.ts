/**
 * TypeScript types for the FPL public API, ported from the Go source
 * at api/types.go, api/myteam.go, api/transfers.go.
 *
 * Field names match the Go source 1:1, so the underlying JSON keys
 * (snake_case from the FPL API) work directly via these interfaces.
 */

// ---------------------------------------------------------------------------
// Position constants (mirrors api/types.go)
// ---------------------------------------------------------------------------

export const PosGK = 1;
export const PosDEF = 2;
export const PosMID = 3;
export const PosFWD = 4;

export const POSITION_NAMES: Record<number, string> = {
  [PosGK]: 'GK',
  [PosDEF]: 'DEF',
  [PosMID]: 'MID',
  [PosFWD]: 'FWD',
};

// ---------------------------------------------------------------------------
// Bootstrap-static response (api/types.go BootstrapResponse)
// ---------------------------------------------------------------------------

export interface BootstrapStaticResponse {
  elements: Player[];
  teams: Team[];
  events: Event[];
  element_types: ElementType[];
  game_settings: GameSettings;
  // FPL returns additional fields; allow them via index signature.
  [key: string]: unknown;
}

// ---------------------------------------------------------------------------
// Player (api/types.go Player struct, json tags are the field names)
// ---------------------------------------------------------------------------

export interface Player {
  id: number;
  first_name: string;
  second_name: string;
  web_name: string;
  team: number;
  team_code: number;
  element_type: number; // 1=GK, 2=DEF, 3=MID, 4=FWD
  now_cost: number; // tenths of £m (e.g. 45 = £4.5M)
  cost_change_start: number;
  cost_change_event: number;
  total_points: number;
  event_points: number;
  points_per_game: string; // FPL returns as string
  form: string; // FPL returns as string
  ep_next: string | null;
  ep_this: string | null;
  selected_by_percent: string;
  status: 'a' | 'd' | 'i' | 's' | 'u' | 'n'; // available, doubtful, injured, suspended, unavailable, not-in-squad
  news: string;
  news_added: string | null;
  minutes: number;
  goals_scored: number;
  assists: number;
  clean_sheets: number;
  goals_conceded: number;
  own_goals: number;
  penalties_saved: number;
  penalties_missed: number;
  yellow_cards: number;
  red_cards: number;
  saves: number;
  bonus: number;
  bps: number;
  influence: string;
  creativity: string;
  threat: string;
  ict_index: string;
  starts: number;
  expected_goals: string;
  expected_assists: string;
  expected_goal_involvements: string;
  expected_goals_conceded: string;
  chance_of_playing_next_round: number | null;
  // allow extra fields
  [key: string]: unknown;
}

// ---------------------------------------------------------------------------
// Team (api/types.go Team struct)
// ---------------------------------------------------------------------------

export interface Team {
  id: number;
  name: string;
  short_name: string;
  code: number;
  strength: number;
  strength_overall_home: number;
  strength_overall_away: number;
  strength_attack_home: number;
  strength_attack_away: number;
  strength_defence_home: number;
  strength_defence_away: number;
  [key: string]: unknown;
}

// ---------------------------------------------------------------------------
// Fixture (api/types.go Fixture struct)
// ---------------------------------------------------------------------------

export interface Fixture {
  id: number;
  code: number;
  event: number | null; // gameweek
  team_h: number;
  team_a: number;
  team_h_difficulty: number; // 1-5 FDR
  team_a_difficulty: number;
  kickoff_time: string;
  finished: boolean;
  started: boolean;
  team_h_score: number | null;
  team_a_score: number | null;
  [key: string]: unknown;
}

// ---------------------------------------------------------------------------
// Event (api/types.go Event struct)
// ---------------------------------------------------------------------------

export interface Event {
  id: number;
  name: string;
  deadline_time: string;
  average_entry_score: number;
  finished: boolean;
  is_current: boolean;
  is_next: boolean;
  is_previous: boolean;
  deadline_time_epoch: number;
  [key: string]: unknown;
}

// ---------------------------------------------------------------------------
// ElementType (api/types.go ElementType struct)
// ---------------------------------------------------------------------------

export interface ElementType {
  id: number;
  singular_name: string;
  singular_name_short: string;
  plural_name: string;
  plural_name_short: string;
  squad_select: number;
  squad_min_play: number;
  squad_max_play: number;
}

// ---------------------------------------------------------------------------
// GameSettings (lightweight subset — FPL returns a large object)
// ---------------------------------------------------------------------------

export interface GameSettings {
  squad_squadplay: number; // 15
  squad_squad_size: number;
  squad_team_limit: number; // 3
  [key: string]: unknown;
}

// ---------------------------------------------------------------------------
// MyTeam + Pick (api/myteam.go)
// ---------------------------------------------------------------------------

/**
 * Pick mirrors a single element in a user's 15-man squad (api/myteam.go Pick).
 *
 * Read-only fields (selling_price, purchase_price, can_sub, has_played, is_sub,
 * element_type) are populated by GET /api/my-team/ but stripped from the POST
 * body — see LineupUpdate for the request shape.
 */
export interface Pick {
  element: number; // player id
  position: number; // 1-15 (squad slot)
  multiplier: number; // 0=bench, 1=starter, 2=captain, 3=VC
  is_captain: boolean;
  is_vice_captain: boolean;
  // read-only fields returned by GET /api/my-team/
  selling_price?: number;
  purchase_price?: number;
  can_sub?: boolean;
  has_played?: boolean;
  is_sub?: boolean;
  element_type?: number;
  [key: string]: unknown;
}

/** TransferStatus (api/myteam.go TransferStatus struct). */
export interface MyTeamTransfers {
  bank: number; // tenths of £m
  value: number;
  limit: number | null; // free transfer limit; null means unlimited (Wildcard/Free Hit)
  made: number;
  entry?: number;
  status?: string;
  [key: string]: unknown;
}

/** ChipInfo describes one chip the user has left to play (api/myteam.go ChipInfo). */
export interface ChipInfo {
  id: number;
  name: string; // 'wildcard' | 'freehit' | 'bboost' | '3xc'
  number: number;
  start_event: number;
  stop_event: number;
  chip_type: string;
}

/** MyTeam is the parsed GET /api/my-team/{tid}/ response (api/myteam.go MyTeam). */
export interface MyTeam {
  entry_id?: number; // not always returned; we set from auth
  picks: Pick[];
  chips?: ChipInfo[];
  transfers: MyTeamTransfers;
  [key: string]: unknown;
}

// ---------------------------------------------------------------------------
// Apply-related types (api/transfers.go)
// ---------------------------------------------------------------------------

export type Chip = 'wildcard' | 'freehit' | 'bboost' | '3xc';

/**
 * Transfer describes a single player swap (api/transfers.go Transfer struct).
 * Named TransferSuggestion in the plan to avoid collision with the DOM Transfer type.
 */
export interface TransferSuggestion {
  element_in: number; // player id
  element_out: number;
  purchase_price: number;
  selling_price: number;
}

/**
 * TransferRequest is the body for /api/transfers/ (api/transfers.go TransferRequest).
 */
export interface TransferRequest {
  confirmed?: boolean; // false=validate, true=commit
  entry: number;
  event: number;
  transfers: TransferSuggestion[];
  chip?: Chip | null;
  wildcard?: boolean;
  freehit?: boolean;
  [key: string]: unknown;
}

/**
 * LineupUpdate is the POST body for /api/my-team/{tid}/ (api/myteam.go LineupUpdate).
 */
export interface LineupUpdate {
  picks: Array<{
    element: number;
    position: number;
    is_captain: boolean;
    is_vice_captain: boolean;
  }>;
  chip?: Chip | null;
  [key: string]: unknown;
}

/**
 * TransferErrorResponse is the failure body from /api/transfers/
 * (api/transfers.go TransferError).
 */
export interface TransferErrorResponse {
  non_form_errors?: string[];
  non_field_errors?: unknown;
  spent_points?: number;
  entry?: number;
  code?: string;
  message?: string;
  [key: string]: unknown;
}
