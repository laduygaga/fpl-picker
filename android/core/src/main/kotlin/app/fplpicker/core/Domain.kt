package app.fplpicker.core

enum class Position(val apiId: Int, val label: String) {
    GK(1, "GK"),
    DEF(2, "DEF"),
    MID(3, "MID"),
    FWD(4, "FWD"),
    UNKNOWN(0, "???");

    companion object {
        fun fromApiId(id: Int): Position = entries.firstOrNull { it.apiId == id } ?: UNKNOWN
    }
}

data class Player(
    val id: Int,
    val firstName: String = "",
    val secondName: String = "",
    val webName: String,
    val team: Int,
    val position: Position,
    val nowCost: Int,
    val form: String = "",
    val pointsPerGame: String = "",
    val totalPoints: Int = 0,
    val minutes: Int = 0,
    val expectedGoals: String = "",
    val expectedAssists: String = "",
    val expectedGoalInvolvements: String = "",
    val expectedGoalsConceded: String = "",
    val ictIndex: String = "",
    val epNext: String? = null,
    val selectedByPercent: String = "",
    val status: String = "a",
    val chanceOfPlayingNextRound: Int? = null,
)

data class Team(val id: Int, val name: String, val shortName: String)

data class Event(val id: Int, val isNext: Boolean = false)

data class Fixture(
    val id: Int,
    val event: Int?,
    val teamHome: Int,
    val teamAway: Int,
    val homeDifficulty: Int,
    val awayDifficulty: Int,
    val finished: Boolean = false,
)

enum class Formula(
    val displayName: String,
    val fdr: Double,
    val points: Double,
    val form: Double,
    val expectedPoints: Double,
    val pointsPerGame: Double,
    val expectedGoalInvolvement: Double,
    val ict: Double,
) {
    BALANCED("Balanced", 0.30, 0.20, 0.15, 0.05, 0.05, 0.03, 0.02),
    ATTACKER("Attacker", 0.15, 0.10, 0.25, 0.10, 0.10, 0.15, 0.15),
    DEFENDER("Defender", 0.35, 0.25, 0.10, 0.05, 0.10, 0.05, 0.10);

    companion object {
        fun resolve(value: String): Formula = when (value.trim().lowercase()) {
            "1", "balanced" -> BALANCED
            "2", "attacker" -> ATTACKER
            "3", "defender" -> DEFENDER
            else -> BALANCED
        }
    }
}

fun Player.isEligible(): Boolean =
    status !in setOf("i", "s", "u") &&
        minutes >= 90 &&
        !(status == "d" && chanceOfPlayingNextRound != null && chanceOfPlayingNextRound < 50)

internal fun String?.numericValue(): Double = this?.toDoubleOrNull() ?: 0.0

internal class Normalizer private constructor(private val minimum: Double, private val maximum: Double) {
    fun normalize(value: Double): Double {
        val range = maximum - minimum
        return if (range == 0.0) 0.0 else ((value - minimum) / range).coerceIn(0.0, 1.0)
    }

    companion object {
        fun from(values: List<Double>): Normalizer = if (values.isEmpty()) {
            Normalizer(0.0, 1.0)
        } else {
            Normalizer(values.min(), values.max())
        }
    }
}
