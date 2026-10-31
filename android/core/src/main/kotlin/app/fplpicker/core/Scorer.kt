package app.fplpicker.core

import kotlin.math.max

private data class TeamStats(val attackPer90: Double, val defencePer90: Double)

private data class GameweekContext(
    val opponentId: Int,
    val isHome: Boolean,
    val opponentDefenceWeakness: Double,
    val opponentAttackWeakness: Double,
    val easyFdr: Double,
)

private data class RawMetrics(
    val ppg: Double,
    val expectedPoints: Double,
    val form: Double,
    val expectedGoalInvolvementPer90: Double,
    val ictPer90: Double,
    val opponentAttack: Double,
    val opponentDefence: Double,
    val totalPoints: Double,
    val easyFdr: Double,
)

class Scorer(
    teams: List<Team>,
    private val fixtures: List<Fixture>,
    events: List<Event>,
    allPlayers: List<Player>,
    formulaId: String = "1",
) {
    private val teamsById = teams.associateBy(Team::id)
    private val teamStats = calculateTeamStats(teams, allPlayers)
    val nextEventId: Int = events.firstOrNull(Event::isNext)?.id ?: 1
    private val formula = Formula.resolve(formulaId)
    private val gameweekContext = buildGameweekContext()
    private val doubleGameweekTeams = gameweekContext.filterValues { it.size > 1 }.keys

    fun fixturePairings(): Map<Int, List<FixturePairing>> = buildMap {
        fixtures.filter { it.event == nextEventId }.sortedBy(Fixture::id).forEach { fixture ->
            put(fixture.teamHome, getOrElse(fixture.teamHome, ::emptyList) + FixturePairing(fixture.teamAway, fixture.homeDifficulty))
            put(fixture.teamAway, getOrElse(fixture.teamAway, ::emptyList) + FixturePairing(fixture.teamHome, fixture.awayDifficulty))
        }
    }

    fun scoreAll(players: List<Player>): List<ScoredPlayer> {
        val eligible = players.filter(Player::isEligible)
        val raws = eligible.map(::rawMetrics)
        if (raws.isEmpty()) return emptyList()
        val normalizers = MetricNormalizers(raws)

        return eligible.mapIndexed { index, player ->
            val contexts = gameweekContext[player.team].orEmpty()
            val raw = raws[index]
            val base = if (contexts.isEmpty()) 0.0 else score(player, raw, normalizers)
            ScoredPlayer(
                player = player,
                score = base,
                teamName = teamsById[player.team]?.shortName.orEmpty(),
                expectedPoints = raw.expectedPoints,
                form = raw.form,
                pointsPerGame = raw.ppg,
                expectedGoalInvolvementPer90 = raw.expectedGoalInvolvementPer90,
                ictPer90 = raw.ictPer90,
                opponentScore = opponentScore(player.position, normalizers.normalize(raw)),
                opponentDescription = describeOpponents(contexts),
                hasFixture = contexts.isNotEmpty(),
                isHome = contexts.firstOrNull()?.isHome ?: false,
                isDoubleGameweek = player.team in doubleGameweekTeams,
                easyFixtureRating = raw.easyFdr,
                upcomingFixtures = describeUpcomingFixtures(player.team),
                valueRating = if (player.nowCost > 0) base / (player.nowCost / 10.0) else 0.0,
                ownershipPercent = player.selectedByPercent.numericValue(),
            )
        }
    }

    private fun rawMetrics(player: Player): RawMetrics {
        val games = max(1.0, player.minutes / 90.0)
        val contexts = gameweekContext[player.team].orEmpty()
        return RawMetrics(
            ppg = player.pointsPerGame.numericValue(),
            expectedPoints = player.epNext.numericValue(),
            form = player.form.numericValue(),
            expectedGoalInvolvementPer90 = player.expectedGoalInvolvements.numericValue() / games,
            ictPer90 = player.ictIndex.numericValue() / games,
            opponentAttack = contexts.map(GameweekContext::opponentAttackWeakness).averageOrZero(),
            opponentDefence = contexts.map(GameweekContext::opponentDefenceWeakness).averageOrZero(),
            totalPoints = player.totalPoints.toDouble(),
            easyFdr = contexts.map(GameweekContext::easyFdr).averageOrZero(),
        )
    }

    private fun score(player: Player, raw: RawMetrics, normalizers: MetricNormalizers): Double {
        val normalized = normalizers.normalize(raw)
        var score = formula.fdr * normalized.easyFdr + formula.points * normalized.totalPoints +
            formula.form * normalized.form + formula.expectedPoints * normalized.expectedPoints +
            formula.pointsPerGame * normalized.ppg + formula.expectedGoalInvolvement * normalized.expectedGoalInvolvementPer90 +
            formula.ict * normalized.ictPer90
        score += positionBonus(player, normalized)
        if (gameweekContext[player.team].orEmpty().firstOrNull()?.isHome == true) score += HOME_BONUS
        if (player.team in doubleGameweekTeams) score += DOUBLE_GAMEWEEK_BONUS
        return score
    }

    private fun positionBonus(player: Player, metrics: RawMetrics): Double {
        val games = max(1.0, player.minutes / 90.0)
        val opponent = opponentScore(player.position, metrics)
        return when (player.position) {
            Position.GK -> OPPONENT_WEIGHT * opponent + cleanSheetBonus(player, games)
            Position.DEF -> OPPONENT_WEIGHT * opponent + cleanSheetBonus(player, games) + 0.03 * player.expectedAssists.numericValue() / games
            Position.MID -> OPPONENT_WEIGHT * opponent + 0.04 * (player.expectedGoals.numericValue() + player.expectedAssists.numericValue()) / games
            Position.FWD -> OPPONENT_WEIGHT * opponent + 0.07 * player.expectedGoals.numericValue() / games
            Position.UNKNOWN -> 0.0
        }
    }

    private fun cleanSheetBonus(player: Player, games: Double): Double {
        val conceded = player.expectedGoalsConceded.numericValue() / games
        return if (conceded < 1.0) CLEAN_SHEET_WEIGHT * (1.0 - conceded) else 0.0
    }

    private fun buildGameweekContext(): Map<Int, List<GameweekContext>> {
        val output = mutableMapOf<Int, MutableList<GameweekContext>>()
        fixtures.filter { it.event == nextEventId }.sortedBy(Fixture::id).forEach { fixture ->
            val away = teamStats[fixture.teamAway] ?: TeamStats(0.0, 0.0)
            val home = teamStats[fixture.teamHome] ?: TeamStats(0.0, 0.0)
            output.getOrPut(fixture.teamHome, ::mutableListOf) += GameweekContext(
                fixture.teamAway, true, away.defencePer90, 1.0 / max(0.5, away.attackPer90), (6 - fixture.homeDifficulty).toDouble(),
            )
            output.getOrPut(fixture.teamAway, ::mutableListOf) += GameweekContext(
                fixture.teamHome, false, home.defencePer90, 1.0 / max(0.5, home.attackPer90), (6 - fixture.awayDifficulty).toDouble(),
            )
        }
        return output
    }

    private fun describeOpponents(contexts: List<GameweekContext>): String = if (contexts.isEmpty()) {
        "BLANK"
    } else {
        contexts.joinToString(" + ") { context ->
            val opponent = teamsById[context.opponentId]
            val stats = teamStats[context.opponentId] ?: TeamStats(0.0, 0.0)
            val attack = if (stats.attackPer90 > 1.8) "Strong Atk" else if (stats.attackPer90 > 1.3) "Avg Atk" else "Weak Atk"
            val defence = if (stats.defencePer90 > 1.5) "Leaky Def" else if (stats.defencePer90 > 1.0) "Avg Def" else "Solid Def"
            "${opponent?.shortName.orEmpty()}(${if (context.isHome) "H" else "A"}) [$attack, $defence]"
        }
    }

    private fun describeUpcomingFixtures(teamId: Int): String {
        val descriptions = fixtures.asSequence()
            .filter { it.event != null && !it.finished && it.event >= nextEventId && it.event < nextEventId + 3 }
            .filter { it.teamHome == teamId || it.teamAway == teamId }
            .sortedWith(compareBy({ it.event }, Fixture::id))
            .map { fixture ->
                if (fixture.teamHome == teamId) "${teamsById[fixture.teamAway]?.shortName.orEmpty()}(H)"
                else "${teamsById[fixture.teamHome]?.shortName.orEmpty()}(A)"
            }.toList()
        return descriptions.ifEmpty { listOf("BLANK") }.joinToString()
    }

    companion object {
        private const val OPPONENT_WEIGHT = 0.20
        private const val HOME_BONUS = 0.02
        private const val CLEAN_SHEET_WEIGHT = 0.04
        private const val DOUBLE_GAMEWEEK_BONUS = 0.20
    }
}

private fun calculateTeamStats(teams: List<Team>, players: List<Player>): Map<Int, TeamStats> {
    val goals = mutableMapOf<Int, Double>()
    val conceded = mutableMapOf<Int, Double>()
    val goalkeeperMinutes = mutableMapOf<Int, Int>()
    players.filter { it.minutes > 0 }.forEach { player ->
        goals[player.team] = goals.getOrDefault(player.team, 0.0) + player.expectedGoals.numericValue()
        if (player.position == Position.GK) {
            goalkeeperMinutes[player.team] = goalkeeperMinutes.getOrDefault(player.team, 0) + player.minutes
            if (player.minutes > 450) conceded[player.team] = conceded.getOrDefault(player.team, 0.0) + player.expectedGoalsConceded.numericValue()
        }
    }
    return teams.associate { team ->
        val games = max(90, goalkeeperMinutes.getOrDefault(team.id, 0)) / 90.0
        team.id to TeamStats(goals.getOrDefault(team.id, 0.0) / games, conceded.getOrDefault(team.id, 0.0) / games)
    }
}

private class MetricNormalizers(raws: List<RawMetrics>) {
    private val all = RawMetrics(
        ppg = 0.0, expectedPoints = 0.0, form = 0.0, expectedGoalInvolvementPer90 = 0.0, ictPer90 = 0.0,
        opponentAttack = 0.0, opponentDefence = 0.0, totalPoints = 0.0, easyFdr = 0.0,
    )
    private val normalizers = listOf<(RawMetrics) -> Double>(
        RawMetrics::ppg, RawMetrics::expectedPoints, RawMetrics::form, RawMetrics::expectedGoalInvolvementPer90,
        RawMetrics::ictPer90, RawMetrics::opponentAttack, RawMetrics::opponentDefence, RawMetrics::totalPoints, RawMetrics::easyFdr,
    ).map { selector -> Normalizer.from(raws.map(selector)) }

    fun normalize(raw: RawMetrics): RawMetrics = all.copy(
        ppg = normalizers[0].normalize(raw.ppg), expectedPoints = normalizers[1].normalize(raw.expectedPoints),
        form = normalizers[2].normalize(raw.form), expectedGoalInvolvementPer90 = normalizers[3].normalize(raw.expectedGoalInvolvementPer90),
        ictPer90 = normalizers[4].normalize(raw.ictPer90), opponentAttack = normalizers[5].normalize(raw.opponentAttack),
        opponentDefence = normalizers[6].normalize(raw.opponentDefence), totalPoints = normalizers[7].normalize(raw.totalPoints),
        easyFdr = normalizers[8].normalize(raw.easyFdr),
    )
}

private fun opponentScore(position: Position, metrics: RawMetrics): Double = when (position) {
    Position.GK -> metrics.opponentAttack
    Position.DEF -> 0.70 * metrics.opponentAttack + 0.30 * metrics.opponentDefence
    Position.MID -> 0.20 * metrics.opponentAttack + 0.80 * metrics.opponentDefence
    Position.FWD -> 0.10 * metrics.opponentAttack + 0.90 * metrics.opponentDefence
    Position.UNKNOWN -> 0.0
}

private fun List<Double>.averageOrZero(): Double = if (isEmpty()) 0.0 else average()
