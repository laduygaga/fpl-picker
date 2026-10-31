package app.fplpicker.core

private data class PositionCounts(val gk: Int = 0, val def: Int = 0, val mid: Int = 0, val fwd: Int = 0) {
    operator fun plus(other: PositionCounts) = PositionCounts(gk + other.gk, def + other.def, mid + other.mid, fwd + other.fwd)
    fun fits(target: PositionCounts) = gk <= target.gk && def <= target.def && mid <= target.mid && fwd <= target.fwd
}

private data class TeamOption(
    val counts: PositionCounts,
    val cost: Int,
    val objective: Double,
    val players: List<ScoredPlayer>,
)

private data class DpNode(val cost: Int, val objective: Double, val players: List<ScoredPlayer>)

internal data class StarterSolution(val players: List<ScoredPlayer>, val objective: Double, val cost: Int)

internal fun solveStarters(
    players: List<ScoredPlayer>,
    formation: Formation,
    budgetTenths: Int,
    fixturePairings: Map<Int, List<FixturePairing>>,
): StarterSolution? {
    val target = formation.starterNeeds().toCounts()
    val byTeam = players.groupBy { it.player.team }
    val opponentAttackers = byTeam.mapValues { (_, pool) -> pool.any { it.player.position in setOf(Position.MID, Position.FWD) } }
    var states = mapOf(PositionCounts() to listOf(DpNode(0, 0.0, emptyList())))

    byTeam.keys.sorted().forEach { teamId ->
        val options = generateOptions(byTeam.getValue(teamId), target, clashScale(teamId, opponentAttackers, fixturePairings))
        val next = mutableMapOf<PositionCounts, MutableList<DpNode>>()
        states.toSortedMap(compareBy<PositionCounts>({ it.gk }, { it.def }, { it.mid }, { it.fwd })).forEach { (counts, nodes) ->
            nodes.forEach { node -> addToFrontier(next, counts, node) }
            options.forEach { option ->
                val newCounts = counts + option.counts
                if (newCounts.fits(target)) {
                    nodes.forEach { node ->
                        val cost = node.cost + option.cost
                        if (cost <= budgetTenths) {
                            addToFrontier(next, newCounts, DpNode(cost, node.objective + option.objective, node.players + option.players))
                        }
                    }
                }
            }
        }
        states = next
    }

    val best = states[target].orEmpty().maxWithOrNull(compareBy<DpNode>({ it.objective }, { -it.cost }, { it.players.idKey() }))
    return best?.takeIf { it.players.size == 11 }?.let { StarterSolution(it.players, it.objective, it.cost) }
}

private fun generateOptions(pool: List<ScoredPlayer>, target: PositionCounts, clashScale: Double): List<TeamOption> {
    val limited = Position.entries.filter { it != Position.UNKNOWN }.flatMap { position ->
        pool.filter { it.player.position == position }.sortedWith(compareByDescending<ScoredPlayer> { it.score }.thenBy { it.player.id }).take(3)
    }
    val options = mutableListOf<TeamOption>()
    for (first in limited.indices) {
        addOption(listOf(limited[first]), target, clashScale, options)
        for (second in first + 1 until limited.size) {
            addOption(listOf(limited[first], limited[second]), target, clashScale, options)
            for (third in second + 1 until limited.size) {
                addOption(listOf(limited[first], limited[second], limited[third]), target, clashScale, options)
            }
        }
    }
    return options.groupBy(TeamOption::counts).values.flatMap(::paretoOptions)
}

private fun addOption(players: List<ScoredPlayer>, target: PositionCounts, clashScale: Double, output: MutableList<TeamOption>) {
    val counts = players.groupingBy { it.player.position }.eachCount().toCounts()
    if (!counts.fits(target)) return
    val rawScore = players.sumOf(ScoredPlayer::score)
    val defensiveScore = players.filter { it.player.position in setOf(Position.GK, Position.DEF) }.sumOf(ScoredPlayer::score)
    output += TeamOption(counts, players.sumOf { it.player.nowCost }, rawScore - defensiveScore * clashScale, players)
}

private fun paretoOptions(group: List<TeamOption>): List<TeamOption> {
    var bestScore = Double.NEGATIVE_INFINITY
    return group.sortedWith(compareBy<TeamOption>({ it.cost }, { -it.objective }, { it.players.idKey() })).filter { option ->
        val keep = option.objective > bestScore
        if (keep) bestScore = option.objective
        keep
    }
}

private fun addToFrontier(states: MutableMap<PositionCounts, MutableList<DpNode>>, counts: PositionCounts, candidate: DpNode) {
    val frontier = states.getOrPut(counts, ::mutableListOf)
    if (frontier.any { it.cost <= candidate.cost && it.objective >= candidate.objective }) return
    frontier.removeAll { candidate.cost <= it.cost && candidate.objective >= it.objective }
    frontier += candidate
}

private fun clashScale(
    teamId: Int,
    opponentAttackers: Map<Int, Boolean>,
    fixturePairings: Map<Int, List<FixturePairing>>,
): Double {
    val hardest = fixturePairings[teamId].orEmpty().filter { opponentAttackers[it.opponentId] == true }.maxOfOrNull(FixturePairing::difficulty)
    return if (hardest == null) 0.0 else 0.50 * hardest / 5.0
}

private fun Map<Position, Int>.toCounts() = PositionCounts(
    gk = getOrDefault(Position.GK, 0),
    def = getOrDefault(Position.DEF, 0),
    mid = getOrDefault(Position.MID, 0),
    fwd = getOrDefault(Position.FWD, 0),
)

private fun List<ScoredPlayer>.idKey(): String = sortedBy { it.player.id }.joinToString(",") { it.player.id.toString().padStart(4, '0') }
