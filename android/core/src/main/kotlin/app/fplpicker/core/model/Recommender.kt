package app.fplpicker.core.model

data class Formation(
    val name: String,
    val gk: Int,
    val def: Int,
    val mid: Int,
    val fwd: Int
)

val VALID_FORMATIONS = listOf(
    Formation("3-4-3", 1, 3, 4, 3),
    Formation("3-5-2", 1, 3, 5, 2),
    Formation("4-3-3", 1, 4, 3, 3),
    Formation("4-4-2", 1, 4, 4, 2),
    Formation("4-5-1", 1, 4, 5, 1),
    Formation("5-3-2", 1, 5, 3, 2),
    Formation("5-4-1", 1, 5, 4, 1)
)

val SQUAD_SLOTS = mapOf(
    POS_GK to 2,
    POS_DEF to 5,
    POS_MID to 5,
    POS_FWD to 3
)

data class SquadResult(
    val formation: String,
    val starters: List<ScoredPlayer>,
    val bench: List<ScoredPlayer>,
    val captain: ScoredPlayer?,
    val viceCaptain: ScoredPlayer?,
    val totalScore: Double,
    val xiCost: Double,
    val totalCost: Double,
    val budget: Double
)

data class PosCounts(
    val sGK: Int,
    val sDEF: Int,
    val sMID: Int,
    val sFWD: Int
) {
    val key: String get() = "$sGK,$sDEF,$sMID,$sFWD"

    fun add(other: PosCounts) = PosCounts(
        sGK + other.sGK,
        sDEF + other.sDEF,
        sMID + other.sMID,
        sFWD + other.sFWD
    )

    fun fitsWithin(target: PosCounts): Boolean =
        sGK <= target.sGK && sDEF <= target.sDEF && sMID <= target.sMID && sFWD <= target.sFWD
}

data class TeamOption(
    val counts: PosCounts,
    val cost: Int,
    val score: Double,
    val players: List<ScoredPlayer>
)

data class DPNode(
    val cost: Int,
    val score: Double
)

fun findBestSquad(
    players: List<ScoredPlayer>,
    budgetTenths: Int,
    fixturePairings: Map<Int, List<FixturePairing>>? = null
): SquadResult {
    val byTeam = players.groupBy { it.player.team }
    val teamIds = byTeam.keys.sorted()

    val byPosCostAsc = mutableMapOf<Int, List<ScoredPlayer>>()
    val byPosScoreDesc = mutableMapOf<Int, List<ScoredPlayer>>()

    listOf(POS_GK, POS_DEF, POS_MID, POS_FWD).forEach { pos ->
        val posPlayers = players.filter { it.player.element_type == pos }
        byPosCostAsc[pos] = posPlayers.sortedBy { it.player.now_cost }
        byPosScoreDesc[pos] = posPlayers.sortedByDescending { it.score }
    }

    var bestResult = SquadResult(
        formation = "",
        starters = emptyList(),
        bench = emptyList(),
        captain = null,
        viceCaptain = null,
        totalScore = 0.0,
        xiCost = 0.0,
        totalCost = 0.0,
        budget = budgetTenths / 10.0
    )
    var bestObj = -Double.MAX_VALUE

    for (fm in VALID_FORMATIONS) {
        val targetStarters = PosCounts(fm.gk, fm.def, fm.mid, fm.fwd)
        val benchReserve = estimateBenchCost(byPosCostAsc, fm)
        val dpBudget = budgetTenths - benchReserve
        if (dpBudget <= 0) continue

        val res = solveDP(teamIds, byTeam, targetStarters, dpBudget)
        if (res == null) continue

        val (starters, xiScore, xiCost) = res
        if (starters.size != 11) continue

        sortByPosAndScore(starters)
        val (captain, viceCaptain) = pickCaptains(starters)

        val xiIds = starters.map { it.player.id }.toSet()
        val teamCount = mutableMapOf<Int, Int>()
        val xiPosCounts = mutableMapOf(POS_GK to 0, POS_DEF to 0, POS_MID to 0, POS_FWD to 0)

        starters.forEach { p ->
            teamCount[p.player.team] = (teamCount[p.player.team] ?: 0) + 1
            xiPosCounts[p.player.element_type] = (xiPosCounts[p.player.element_type] ?: 0) + 1
        }

        val bench = fillBench(
            byPosCostAsc,
            byPosScoreDesc,
            xiIds,
            teamCount,
            xiPosCounts,
            budgetTenths - xiCost
        )

        var totalCost = xiCost
        bench.forEach { totalCost += it.player.now_cost }

        var rawScore = 0.0
        starters.forEach { rawScore += it.score }

        if (xiScore > bestObj) {
            bestObj = xiScore
            bestResult = SquadResult(
                formation = fm.name,
                starters = starters,
                bench = bench,
                captain = captain,
                viceCaptain = viceCaptain,
                totalScore = rawScore,
                xiCost = xiCost / 10.0,
                totalCost = totalCost / 10.0,
                budget = budgetTenths / 10.0
            )
        }
    }

    return bestResult
}

private data class DPResult(val starters: List<ScoredPlayer>, val score: Double, val cost: Int)

private fun solveDP(
    teamIds: List<Int>,
    byTeam: Map<Int, List<ScoredPlayer>>,
    target: PosCounts,
    budget: Int
): DPResult? {
    val allOpts: List<List<TeamOption>> = teamIds.map { tid ->
        generateTeamOptions(byTeam[tid] ?: emptyList(), target)
    }

    val stages = Array(teamIds.size + 1) { mutableMapOf<String, MutableList<DPNode>>() }
    val initCounts = PosCounts(0, 0, 0, 0)
    stages[0][initCounts.key] = mutableListOf(DPNode(0, 0.0))

    for (ti in teamIds.indices) {
        val opts = allOpts[ti]
        val prev = stages[ti]
        val next = stages[ti + 1]

        prev.forEach { (stateKey, nodes) ->
            val parts = stateKey.split(",").map { it.toInt() }
            val state = PosCounts(parts[0], parts[1], parts[2], parts[3])

            nodes.forEach { n ->
                addToFrontier(next, state.key, DPNode(n.cost, n.score))
            }

            opts.forEach { opt ->
                val ns = state.add(opt.counts)
                if (!ns.fitsWithin(target)) return@forEach

                nodes.forEach { n ->
                    val nc = n.cost + opt.cost
                    if (nc <= budget) {
                        addToFrontier(next, ns.key, DPNode(nc, n.score + opt.score))
                    }
                }
            }
        }
    }

    val finalFrontier = stages[teamIds.size][target.key]
    if (finalFrontier.isNullOrEmpty()) return null

    var bestScore = -1.0
    var bestCost = 0
    finalFrontier.forEach { n ->
        if (n.score > bestScore) {
            bestScore = n.score
            bestCost = n.cost
        }
    }

    val starters = reconstructStarters(teamIds, allOpts, stages, target, bestScore, bestCost)
    return DPResult(starters, bestScore, bestCost)
}

private fun reconstructStarters(
    teamIds: List<Int>,
    allOpts: List<List<TeamOption>>,
    stages: Array<MutableMap<String, MutableList<DPNode>>>,
    target: PosCounts,
    bestScore: Double,
    bestCost: Int
): List<ScoredPlayer> {
    var starters = listOf<ScoredPlayer>()
    var remainScore = bestScore
    var remainCost = bestCost
    var remainState = target

    for (ti in teamIds.indices.reversed()) {
        val prevStage = stages[ti]
        var found = false

        for (opt in allOpts[ti]) {
            val prevState = PosCounts(
                remainState.sGK - opt.counts.sGK,
                remainState.sDEF - opt.counts.sDEF,
                remainState.sMID - opt.counts.sMID,
                remainState.sFWD - opt.counts.sFWD
            )
            if (prevState.sGK < 0 || prevState.sDEF < 0 || prevState.sMID < 0 || prevState.sFWD < 0) continue

            val wantCost = remainCost - opt.cost
            val wantScore = remainScore - opt.score

            val nodes = prevStage[prevState.key]
            if (nodes != null && matchNode(nodes, wantCost, wantScore)) {
                starters = opt.players + starters
                remainScore = wantScore
                remainCost = wantCost
                remainState = prevState
                found = true
                break
            }
        }

        if (!found) {
            val nodes = prevStage[remainState.key]
            if (nodes != null) {
                matchNode(nodes, remainCost, remainScore)
            }
        }
    }

    return starters
}

private fun matchNode(nodes: List<DPNode>, wantCost: Int, wantScore: Double): Boolean {
    for (n in nodes) {
        if (n.cost == wantCost && kotlin.math.abs(n.score - wantScore) < 1e-6) {
            return true
        }
    }
    return false
}

private fun addToFrontier(dp: MutableMap<String, MutableList<DPNode>>, stateKey: String, node: DPNode) {
    val existing = dp.getOrPut(stateKey) { mutableListOf() }
    for (e in existing) {
        if ((e.cost < node.cost && e.score >= node.score) || (e.cost <= node.cost && e.score > node.score)) {
            return
        }
    }
    val kept = existing.filter { e ->
        !((node.cost < e.cost && node.score >= e.score) || (node.cost <= e.cost && node.score > e.score))
    }.toMutableList()
    kept.add(node)
    dp[stateKey] = kept
}

private fun generateTeamOptions(pool: List<ScoredPlayer>, target: PosCounts): List<TeamOption> {
    if (pool.isEmpty()) return emptyList()

    val posPool = pool.groupBy { it.player.element_type }
    val limited = mutableListOf<ScoredPlayer>()
    listOf(POS_GK, POS_DEF, POS_MID, POS_FWD).forEach { pos ->
        val pp = (posPool[pos] ?: emptyList()).sortedByDescending { it.score }
        limited.addAll(pp.take(3))
    }

    val opts = mutableListOf<TeamOption>()
    val n = limited.size
    val idx = IntArray(3)

    for (size in 1..3) {
        if (size <= n) {
            enumerateStarterSubsets(limited, size, 0, idx, 0, target, opts)
        }
    }

    return pruneTeamOptions(opts)
}

private fun enumerateStarterSubsets(
    pool: List<ScoredPlayer>,
    size: Int,
    start: Int,
    idx: IntArray,
    depth: Int,
    target: PosCounts,
    opts: MutableList<TeamOption>
) {
    if (depth == size) {
        val players = mutableListOf<ScoredPlayer>()
        var sGK = 0
        var sDEF = 0
        var sMID = 0
        var sFWD = 0
        var totalCost = 0
        var totalScore = 0.0

        for (d in 0 until depth) {
            val p = pool[idx[d]]
            players.add(p)
            totalCost += p.player.now_cost
            totalScore += p.score

            when (p.player.element_type) {
                POS_GK -> sGK++
                POS_DEF -> sDEF++
                POS_MID -> sMID++
                POS_FWD -> sFWD++
            }
        }

        val counts = PosCounts(sGK, sDEF, sMID, sFWD)
        if (!counts.fitsWithin(target)) return

        opts.add(TeamOption(counts, totalCost, totalScore, players))
        return
    }

    for (i in start until pool.size) {
        idx[depth] = i
        enumerateStarterSubsets(pool, size, i + 1, idx, depth + 1, target, opts)
    }
}

private fun pruneTeamOptions(opts: List<TeamOption>): List<TeamOption> {
    val groups = opts.groupBy { it.counts.key }
    val pruned = mutableListOf<TeamOption>()

    groups.values.forEach { group ->
        val sorted = group.sortedWith(compareBy({ it.cost }, { -it.score }))
        var bestScore = -1.0
        for (o in sorted) {
            if (o.score > bestScore) {
                pruned.add(o)
                bestScore = o.score
            }
        }
    }

    return pruned
}

private fun estimateBenchCost(byPosCostAsc: Map<Int, List<ScoredPlayer>>, fm: Formation): Int {
    val benchNeeds = mapOf(
        POS_GK to (SQUAD_SLOTS[POS_GK]!! - fm.gk),
        POS_DEF to (SQUAD_SLOTS[POS_DEF]!! - fm.def),
        POS_MID to (SQUAD_SLOTS[POS_MID]!! - fm.mid),
        POS_FWD to (SQUAD_SLOTS[POS_FWD]!! - fm.fwd)
    )

    var total = 0
    listOf(POS_GK, POS_DEF, POS_MID, POS_FWD).forEach { pos ->
        val need = benchNeeds[pos] ?: 0
        val sorted = byPosCostAsc[pos] ?: emptyList()
        for (i in 0 until need) {
            if (i < sorted.size) total += sorted[i].player.now_cost
        }
    }
    return total
}

private fun fillBench(
    byPosCostAsc: Map<Int, List<ScoredPlayer>>,
    byPosScoreDesc: Map<Int, List<ScoredPlayer>>,
    xiIds: Set<Int>,
    teamCount: MutableMap<Int, Int>,
    xiPosCounts: Map<Int, Int>,
    remainingBudgetTenths: Int
): List<ScoredPlayer> {
    val remBudget = remainingBudgetTenths

    val benchNeeds = mapOf(
        POS_GK to (SQUAD_SLOTS[POS_GK]!! - (xiPosCounts[POS_GK] ?: 0)),
        POS_DEF to (SQUAD_SLOTS[POS_DEF]!! - (xiPosCounts[POS_DEF] ?: 0)),
        POS_MID to (SQUAD_SLOTS[POS_MID]!! - (xiPosCounts[POS_MID] ?: 0)),
        POS_FWD to (SQUAD_SLOTS[POS_FWD]!! - (xiPosCounts[POS_FWD] ?: 0))
    )

    class PosGroup(val pos: Int, val need: Int, val candidates: List<ScoredPlayer>)
    val groups = mutableListOf<PosGroup>()

    listOf(POS_GK, POS_DEF, POS_MID, POS_FWD).forEach { pos ->
        val need = benchNeeds[pos] ?: 0
        if (need <= 0) return@forEach

        val filtered = mutableListOf<ScoredPlayer>()
        val seen = mutableSetOf<Int>()

        val scoreDesc = byPosScoreDesc[pos] ?: emptyList()
        for (p in scoreDesc) {
            if (xiIds.contains(p.player.id) || (teamCount[p.player.team] ?: 0) >= 3) continue
            if (p.player.now_cost > remBudget) continue
            filtered.add(p)
            seen.add(p.player.id)
        }

        val costAsc = byPosCostAsc[pos] ?: emptyList()
        for (p in costAsc) {
            if (seen.contains(p.player.id) || xiIds.contains(p.player.id) || (teamCount[p.player.team] ?: 0) >= 3) continue
            if (p.player.now_cost > remBudget) continue
            filtered.add(p)
            seen.add(p.player.id)
        }

        groups.add(PosGroup(pos, need, filtered))
    }

    if (groups.isEmpty()) return emptyList()

    val bestBench = mutableListOf<ScoredPlayer>()
    val tc = teamCount.toMutableMap()

    // Simple greedy/fallback allocator for stability
    listOf(POS_GK, POS_DEF, POS_MID, POS_FWD).forEach { pos ->
        val need = benchNeeds[pos] ?: 0
        val pool = byPosCostAsc[pos] ?: emptyList()
        var added = 0
        for (p in pool) {
            if (added >= need) break
            if (xiIds.contains(p.player.id) || (tc[p.player.team] ?: 0) >= 3) continue
            bestBench.add(p)
            tc[p.player.team] = (tc[p.player.team] ?: 0) + 1
            added++
        }
    }

    val gkBench = bestBench.filter { it.player.element_type == POS_GK }
    val outfieldBench = bestBench.filter { it.player.element_type != POS_GK }.sortedByDescending { it.score }
    return gkBench + outfieldBench
}

fun pickCaptains(starters: List<ScoredPlayer>): Pair<ScoredPlayer?, ScoredPlayer?> {
    if (starters.isEmpty()) return Pair(null, null)
    val sorted = starters.sortedByDescending { it.score }
    val eligible = sorted.filter { it.player.chance_of_playing_next_round != 0 }
    if (eligible.isEmpty()) {
        return Pair(sorted.getOrNull(0), sorted.getOrNull(1))
    }
    return Pair(eligible.getOrNull(0), eligible.getOrNull(1))
}

fun sortByPosAndScore(players: List<ScoredPlayer>) {
    players.sortedWith(compareBy({ it.player.element_type }, { -it.score }))
}
