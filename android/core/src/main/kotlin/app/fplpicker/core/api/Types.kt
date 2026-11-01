package app.fplpicker.core.api

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement

@Serializable
data class BootstrapStaticResponse(
    val elements: List<Player>,
    val teams: List<Team>,
    val events: List<Event>,
    val element_types: List<ElementType> = emptyList()
)

@Serializable
data class Player(
    val id: Int,
    val first_name: String = "",
    val second_name: String = "",
    val web_name: String,
    val team: Int,
    val element_type: Int, // 1=GK, 2=DEF, 3=MID, 4=FWD
    val now_cost: Int, // tenths of £m
    val total_points: Int = 0,
    val minutes: Int = 0,
    val points_per_game: String = "0.0",
    val form: String = "0.0",
    val ep_next: String? = null,
    val selected_by_percent: String = "0.0",
    val status: String = "a",
    val chance_of_playing_next_round: Int? = null,
    val expected_goals: String = "0.0",
    val expected_assists: String = "0.0",
    val expected_goal_involvements: String = "0.0",
    val expected_goals_conceded: String = "0.0",
    val ict_index: String = "0.0"
)

@Serializable
data class Team(
    val id: Int,
    val name: String,
    val short_name: String,
    val strength: Int? = 3,
    val strength_overall_home: Int? = 3,
    val strength_overall_away: Int? = 3,
    val strength_attack_home: Int? = 3,
    val strength_attack_away: Int? = 3,
    val strength_defence_home: Int? = 3,
    val strength_defence_away: Int? = 3
)

@Serializable
data class Event(
    val id: Int,
    val name: String,
    val deadline_time: String = "",
    val is_current: Boolean = false,
    val is_next: Boolean = false,
    val finished: Boolean = false
)

@Serializable
data class ElementType(
    val id: Int,
    val singular_name: String,
    val plural_name: String,
    val squad_select: Int,
    val squad_min_play: Int,
    val squad_max_play: Int
)

@Serializable
data class Fixture(
    val id: Int,
    val event: Int? = null,
    val team_h: Int,
    val team_a: Int,
    val team_h_difficulty: Int = 3,
    val team_a_difficulty: Int = 3,
    val finished: Boolean = false,
    val started: Boolean = false
)
