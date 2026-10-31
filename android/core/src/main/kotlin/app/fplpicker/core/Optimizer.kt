package app.fplpicker.core

object Optimizer {
    fun findBestSquad(
        players: List<ScoredPlayer>,
        budgetTenths: Int,
        fixturePairings: Map<Int, List<FixturePairing>> = emptyMap(),
    ): SquadResult {
        val sorted = players.sortedWith(compareBy<ScoredPlayer>({ it.player.team }, { it.player.position.apiId }, { -it.score }, { it.player.id }))
        var bestObjective = Double.NEGATIVE_INFINITY
        var best = SquadResult(budgetTenths = budgetTenths)

        validFormations.forEach { formation ->
            val reserve = estimateBenchReserve(sorted, formation)
            val solution = solveStarters(sorted, formation, budgetTenths - reserve, fixturePairings) ?: return@forEach
            val starters = solution.players.sortedWith(playerOrdering)
            val starterIds = starters.mapTo(mutableSetOf()) { it.player.id }
            val teamCounts = starters.groupingBy { it.player.team }.eachCount()
            val positionCounts = starters.groupingBy { it.player.position }.eachCount()
            val needs = squadSlots.mapValues { (position, total) -> total - positionCounts.getOrDefault(position, 0) }
            val bench = optimizeBench(sorted, starterIds, teamCounts, needs, budgetTenths - solution.cost)
            if (bench.size != 4) return@forEach
            val totalCost = solution.cost + bench.sumOf { it.player.nowCost }
            if (totalCost > budgetTenths) return@forEach

            if (solution.objective > bestObjective) {
                val (captain, viceCaptain) = pickCaptains(starters)
                bestObjective = solution.objective
                best = SquadResult(
                    formation = formation.name,
                    starters = starters,
                    bench = bench,
                    captain = captain,
                    viceCaptain = viceCaptain,
                    totalScore = starters.sumOf(ScoredPlayer::score),
                    xiCostTenths = solution.cost,
                    totalCostTenths = totalCost,
                    budgetTenths = budgetTenths,
                )
            }
        }
        return best
    }

    fun pickCaptains(starters: List<ScoredPlayer>): Pair<ScoredPlayer?, ScoredPlayer?> {
        val sorted = starters.sortedWith(compareByDescending<ScoredPlayer> { it.score }.thenBy { it.player.id })
        val eligible = sorted.filter { it.player.chanceOfPlayingNextRound != 0 }
        val pool = eligible.ifEmpty { sorted }
        return pool.getOrNull(0) to pool.getOrNull(1)
    }

    fun optimizeBench(
        candidates: List<ScoredPlayer>,
        starterIds: Set<Int>,
        starterTeamCounts: Map<Int, Int>,
        needs: Map<Position, Int>,
        budgetTenths: Int,
    ): List<ScoredPlayer> = findOptimalBench(candidates, starterIds, starterTeamCounts, needs, budgetTenths)

    private fun estimateBenchReserve(players: List<ScoredPlayer>, formation: Formation): Int {
        val needs = squadSlots.mapValues { (position, total) -> total - formation.starterNeeds().getValue(position) }
        return needs.entries.sumOf { (position, count) ->
            players.filter { it.player.position == position }.sortedBy { it.player.nowCost }.take(count).sumOf { it.player.nowCost }
        }
    }
}
