package app.fplpicker.core

private data class BenchSearch(
    var bestScore: Double = Double.NEGATIVE_INFINITY,
    var bestCost: Int = Int.MAX_VALUE,
    var best: List<ScoredPlayer> = emptyList(),
)

internal fun findOptimalBench(
    candidates: List<ScoredPlayer>,
    starterIds: Set<Int>,
    starterTeamCounts: Map<Int, Int>,
    needs: Map<Position, Int>,
    budgetTenths: Int,
): List<ScoredPlayer> {
    val groups = Position.entries.filter { it != Position.UNKNOWN && needs.getOrDefault(it, 0) > 0 }.map { position ->
        val eligible = candidates.filter { it.player.position == position && it.player.id !in starterIds }
        val scoreLeaders = eligible.sortedWith(compareByDescending<ScoredPlayer> { it.score }.thenBy { it.player.id }).take(20)
        val bargains = eligible.sortedWith(compareBy<ScoredPlayer>({ it.player.nowCost }, { -it.score }, { it.player.id })).take(5)
        position to (scoreLeaders + bargains).distinctBy { it.player.id }
    }
    if (groups.any { (position, pool) -> pool.size < needs.getValue(position) }) return emptyList()

    val search = BenchSearch()
    chooseGroup(
        index = 0,
        groups = groups,
        needs = needs,
        budget = budgetTenths,
        teamCounts = starterTeamCounts.toMutableMap(),
        selected = mutableListOf(),
        currentCost = 0,
        currentScore = 0.0,
        result = search,
    )
    val selected = search.best
    val goalkeeper = selected.filter { it.player.position == Position.GK }
    val outfield = selected.filter { it.player.position != Position.GK }
        .sortedWith(compareByDescending<ScoredPlayer> { it.score }.thenBy { it.player.id })
    return goalkeeper + outfield
}

private fun chooseGroup(
    index: Int,
    groups: List<Pair<Position, List<ScoredPlayer>>>,
    needs: Map<Position, Int>,
    budget: Int,
    teamCounts: MutableMap<Int, Int>,
    selected: MutableList<ScoredPlayer>,
    currentCost: Int,
    currentScore: Double,
    result: BenchSearch,
) {
    if (index == groups.size) {
        if (currentScore > result.bestScore + EPSILON ||
            (kotlin.math.abs(currentScore - result.bestScore) <= EPSILON && currentCost < result.bestCost)
        ) {
            result.bestScore = currentScore
            result.bestCost = currentCost
            result.best = selected.toList()
        }
        return
    }

    val (position, pool) = groups[index]
    choosePlayers(
        pool = pool,
        required = needs.getValue(position),
        start = 0,
        chosen = mutableListOf(),
    ) { choice ->
        val addedCost = choice.sumOf { it.player.nowCost }
        if (currentCost + addedCost > budget) return@choosePlayers
        if (choice.groupingBy { it.player.team }.eachCount().any { (team, count) -> teamCounts.getOrDefault(team, 0) + count > 3 }) {
            return@choosePlayers
        }
        choice.forEach { player -> teamCounts[player.player.team] = teamCounts.getOrDefault(player.player.team, 0) + 1 }
        selected += choice
        chooseGroup(index + 1, groups, needs, budget, teamCounts, selected, currentCost + addedCost, currentScore + choice.sumOf(ScoredPlayer::score), result)
        repeat(choice.size) { selected.removeAt(selected.lastIndex) }
        choice.forEach { player -> teamCounts[player.player.team] = teamCounts.getValue(player.player.team) - 1 }
    }
}

private fun choosePlayers(
    pool: List<ScoredPlayer>,
    required: Int,
    start: Int,
    chosen: MutableList<ScoredPlayer>,
    consume: (List<ScoredPlayer>) -> Unit,
) {
    if (chosen.size == required) {
        consume(chosen.toList())
        return
    }
    for (index in start until pool.size) {
        chosen += pool[index]
        choosePlayers(pool, required, index + 1, chosen, consume)
        chosen.removeAt(chosen.lastIndex)
    }
}

private const val EPSILON = 1e-9
