package app.fplpicker.core.model

import app.fplpicker.core.api.LineupPick
import app.fplpicker.core.api.LineupUpdate
import app.fplpicker.core.api.MyTeam
import app.fplpicker.core.api.Pick

data class TransferSuggestion(
    val out: ScoredPlayer,
    val inPlayer: ScoredPlayer,
    val sellingPrice: Int,
    val purchasePrice: Int,
    val scoreUplift: Double
)

fun planTransfers(
    current: MyTeam?,
    optimal: SquadResult,
    maxHits: Int = 4,
    idToTeam: Map<Int, Int>,
    scoredById: Map<Int, ScoredPlayer>? = null,
    ignoreMaxHits: Boolean = true
): List<TransferSuggestion> {
    if (current == null || current.picks.isEmpty()) return emptyList()

    val currentById = current.picks.associateBy { it.element }
    val optimalById = mutableMapOf<Int, ScoredPlayer>()
    optimal.starters.forEach { optimalById[it.player.id] = it }
    optimal.bench.forEach { optimalById[it.player.id] = it }

    val incoming = mutableListOf<ScoredPlayer>()
    optimalById.forEach { (id, sp) ->
        if (!currentById.containsKey(id)) {
            incoming.add(sp)
        }
    }
    incoming.sortByDescending { it.score }

    class OutCand(val pick: Pick, val elementType: Int)
    val outgoing = mutableListOf<OutCand>()
    currentById.forEach { (id, pick) ->
        if (!optimalById.containsKey(id)) {
            outgoing.add(OutCand(pick, pick.element_type ?: 1))
        }
    }
    outgoing.sortBy { it.pick.selling_price ?: 0 }

    var bank = current.transfers.bank
    val usedFree = current.transfers.made
    val teamCounts = getTeamCounts(current.picks, idToTeam)

    val limit = current.transfers.limit
    val freeLimit = limit ?: 0

    val suggestions = mutableListOf<TransferSuggestion>()
    val used = BooleanArray(outgoing.size)

    val freeRemaining = maxOf(0, freeLimit - usedFree)
    val maxByHits = if (ignoreMaxHits || freeRemaining == 0) 15 else maxOf(0, freeRemaining + (maxHits / 4))

    for (inPlayer in incoming) {
        if (suggestions.size >= maxByHits) break

        val inTeam = idToTeam[inPlayer.player.id]
        val inTeamKnown = inTeam != null

        var bestIdx = -1
        var bestUplift = -1e18

        for (i in outgoing.indices) {
            if (used[i]) continue
            val outCand = outgoing[i]
            if (outCand.elementType != inPlayer.player.element_type) continue

            val sellingPrice = outCand.pick.selling_price ?: outCand.pick.purchase_price ?: inPlayer.player.now_cost
            val netCost = inPlayer.player.now_cost - sellingPrice
            if (netCost > bank) continue

            val outSP = scoredById?.get(outCand.pick.element)
            val outScore = outSP?.score ?: 0.0
            val uplift = inPlayer.score - outScore
            if (uplift > bestUplift) {
                bestUplift = uplift
                bestIdx = i
            }
        }

        if (bestIdx < 0) continue

        val outPick = outgoing[bestIdx].pick
        val sellingPrice = outPick.selling_price ?: outPick.purchase_price ?: inPlayer.player.now_cost

        if (inTeamKnown && inTeam != null) {
            var postInCount = teamCounts[inTeam] ?: 0
            val outTeam = idToTeam[outPick.element]
            if (outTeam == inTeam) {
                postInCount--
            }
            if (postInCount >= 3) continue
        }

        var outSP = scoredById?.get(outPick.element)
        if (outSP == null) {
            outSP = ScoredPlayer(
                player = app.fplpicker.core.api.Player(
                    id = outPick.element,
                    web_name = "Player #${outPick.element}",
                    team = idToTeam[outPick.element] ?: 0,
                    element_type = outPick.element_type ?: 1,
                    now_cost = sellingPrice
                ),
                score = 0.0,
                teamName = "???",
                positionName = "",
                oppDesc = "",
                hasFixture = true,
                isHome = false,
                isDgw = false
            )
        }

        suggestions.add(
            TransferSuggestion(
                out = outSP,
                inPlayer = inPlayer,
                sellingPrice = sellingPrice,
                purchasePrice = inPlayer.player.now_cost,
                scoreUplift = bestUplift
            )
        )

        bank -= (inPlayer.player.now_cost - sellingPrice)
        used[bestIdx] = true

        val outTeam = idToTeam[outPick.element]
        if (outTeam != null) {
            teamCounts[outTeam] = maxOf(0, (teamCounts[outTeam] ?: 0) - 1)
        }
        if (inTeam != null) {
            teamCounts[inTeam] = (teamCounts[inTeam] ?: 0) + 1
        }
    }

    return suggestions
}

fun planLineup(optimal: SquadResult): LineupUpdate {
    val picks = mutableListOf<LineupPick>()
    var posCounter = 1

    val allSquad = optimal.starters + optimal.bench
    val starterIds = optimal.starters.map { it.player.id }.toSet()

    // Sort starters by position then score
    val sortedStarters = optimal.starters.sortedWith(compareBy({ it.player.element_type }, { -it.score }))
    sortedStarters.forEach { sp ->
        val isCap = optimal.captain?.player?.id == sp.player.id
        val isVc = optimal.viceCaptain?.player?.id == sp.player.id
        picks.add(
            LineupPick(
                element = sp.player.id,
                position = posCounter++,
                is_captain = isCap,
                is_vice_captain = isVc
            )
        )
    }

    // Sort bench: GK first, then outfield by score
    val benchGk = optimal.bench.filter { it.player.element_type == POS_GK }
    val benchOutfield = optimal.bench.filter { it.player.element_type != POS_GK }.sortedByDescending { it.score }
    val sortedBench = benchGk + benchOutfield

    sortedBench.forEach { sp ->
        picks.add(
            LineupPick(
                element = sp.player.id,
                position = posCounter++,
                is_captain = false,
                is_vice_captain = false
            )
        )
    }

    return LineupUpdate(picks = picks)
}

private fun getTeamCounts(picks: List<Pick>, idToTeam: Map<Int, Int>): MutableMap<Int, Int> {
    val counts = mutableMapOf<Int, Int>()
    picks.forEach { p ->
        val t = idToTeam[p.element]
        if (t != null) {
            counts[t] = (counts[t] ?: 0) + 1
        }
    }
    return counts
}
