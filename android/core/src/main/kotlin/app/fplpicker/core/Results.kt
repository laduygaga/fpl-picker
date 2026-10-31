package app.fplpicker.core

data class ScoredPlayer(
    val player: Player,
    val score: Double = 0.0,
    val teamName: String = "",
    val positionName: String = player.position.label,
    val expectedPoints: Double = 0.0,
    val form: Double = 0.0,
    val pointsPerGame: Double = 0.0,
    val expectedGoalInvolvementPer90: Double = 0.0,
    val ictPer90: Double = 0.0,
    val opponentScore: Double = 0.0,
    val opponentDescription: String = "BLANK",
    val hasFixture: Boolean = false,
    val isHome: Boolean = false,
    val isDoubleGameweek: Boolean = false,
    val easyFixtureRating: Double = 0.0,
    val upcomingFixtures: String = "BLANK",
    val valueRating: Double = 0.0,
    val ownershipPercent: Double = 0.0,
)

data class FixturePairing(val opponentId: Int, val difficulty: Int)

data class Formation(
    val name: String,
    val goalkeepers: Int,
    val defenders: Int,
    val midfielders: Int,
    val forwards: Int,
) {
    fun starterNeeds(): Map<Position, Int> = linkedMapOf(
        Position.GK to goalkeepers,
        Position.DEF to defenders,
        Position.MID to midfielders,
        Position.FWD to forwards,
    )
}

val validFormations = listOf(
    Formation("3-4-3", 1, 3, 4, 3),
    Formation("3-5-2", 1, 3, 5, 2),
    Formation("4-3-3", 1, 4, 3, 3),
    Formation("4-4-2", 1, 4, 4, 2),
    Formation("4-5-1", 1, 4, 5, 1),
    Formation("5-3-2", 1, 5, 3, 2),
    Formation("5-4-1", 1, 5, 4, 1),
)

data class SquadResult(
    val formation: String = "",
    val starters: List<ScoredPlayer> = emptyList(),
    val bench: List<ScoredPlayer> = emptyList(),
    val captain: ScoredPlayer? = null,
    val viceCaptain: ScoredPlayer? = null,
    val totalScore: Double = 0.0,
    val xiCostTenths: Int = 0,
    val totalCostTenths: Int = 0,
    val budgetTenths: Int = 0,
) {
    val xiCost: Double get() = xiCostTenths / 10.0
    val totalCost: Double get() = totalCostTenths / 10.0
    val budget: Double get() = budgetTenths / 10.0
}

internal val squadSlots = mapOf(
    Position.GK to 2,
    Position.DEF to 5,
    Position.MID to 5,
    Position.FWD to 3,
)

internal val playerOrdering = compareBy<ScoredPlayer>({ it.player.position.apiId }, { -it.score }, { it.player.id })
