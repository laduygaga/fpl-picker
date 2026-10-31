package app.fplpicker.core

internal fun player(
    id: Int,
    team: Int,
    position: Position,
    cost: Int = 50,
    score: Double = 0.5,
    minutes: Int = 900,
    status: String = "a",
    chance: Int? = null,
): Pair<Player, ScoredPlayer> {
    val player = Player(
        id = id,
        webName = "P$id",
        team = team,
        position = position,
        nowCost = cost,
        minutes = minutes,
        status = status,
        chanceOfPlayingNextRound = chance,
    )
    return player to ScoredPlayer(player = player, score = score, teamName = "T$team")
}

internal fun optimizerPool(): List<ScoredPlayer> = buildList {
    var id = 1
    repeat(4) { index -> add(player(id++, index + 1, Position.GK, 40, 0.31 + index * 0.01).second) }
    repeat(8) { index -> add(player(id++, index % 10 + 1, Position.DEF, 45, 0.51 + index * 0.01).second) }
    repeat(8) { index -> add(player(id++, index % 10 + 1, Position.MID, 50, 0.61 + index * 0.01).second) }
    repeat(5) { index -> add(player(id++, index % 10 + 1, Position.FWD, 70, 0.71 + index * 0.01).second) }
}
