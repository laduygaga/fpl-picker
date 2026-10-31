package app.fplpicker.core.model

import app.fplpicker.core.api.Event
import app.fplpicker.core.api.Fixture
import app.fplpicker.core.api.Player
import app.fplpicker.core.api.Team
import kotlin.math.max
import kotlin.math.min

data class Formula(
    val name: String,
    val fdr: Double,
    val pts: Double,
    val form: Double,
    val ep: Double,
    val ppg: Double,
    val xgi: Double,
    val ict: Double
)

val FORMULAS = mapOf(
    "1" to Formula("Balanced", 0.30, 0.20, 0.15, 0.05, 0.05, 0.03, 0.02),
    "2" to Formula("Attacker", 0.15, 0.10, 0.25, 0.10, 0.10, 0.15, 0.15),
    "3" to Formula("Defender", 0.35, 0.25, 0.10, 0.05, 0.10, 0.05, 0.10)
)

val FORMULA_ALIASES = mapOf(
    "balanced" to "1",
    "attacker" to "2",
    "defender" to "3"
)

fun getFormula(idOrName: String): Formula {
    val key = idOrName.trim().lowercase()
    val mapped = FORMULA_ALIASES[key] ?: key
    return FORMULAS[mapped] ?: FORMULAS["1"]!!
}

val POS_GK = 1
val POS_DEF = 2
val POS_MID = 3
val POS_FWD = 4

val POSITION_NAMES = mapOf(
    POS_GK to "GK",
    POS_DEF to "DEF",
    POS_MID to "MID",
    POS_FWD to "FWD"
)

data class ScoredPlayer(
    val player: Player,
    var score: Double,
    val teamName: String,
    val positionName: String,
    val oppDesc: String,
    val hasFixture: Boolean,
    val isHome: Boolean,
    val isDgw: Boolean
)

data class FixturePairing(
    val opponentId: Int,
    val difficulty: Int
)

data class GWContext(
    val oppId: Int,
    val isHome: Boolean,
    val oppDefWeakness: Double,
    val oppAttWeakness: Double,
    val easyFDR: Double
)

data class RawMetrics(
    val ppg: Double,
    val ep: Double,
    val form: Double,
    val xgi90: Double,
    val ict90: Double,
    val oppAtk: Double,
    val oppDef: Double,
    val totalPts: Double,
    val easyFDR: Double
)

class Normalizer(vals: List<Double>) {
    private val minVal: Double
    private val maxVal: Double

    init {
        if (vals.isEmpty()) {
            minVal = 0.0
            maxVal = 1.0
        } else {
            var lo = vals[0]
            var hi = vals[0]
            for (i in 1 until vals.size) {
                if (vals[i] < lo) lo = vals[i]
                if (vals[i] > hi) hi = vals[i]
            }
            minVal = lo
            maxVal = hi
        }
    }

    fun normalize(v: Double): Double {
        val range = maxVal - minVal
        if (range == 0.0) return 0.0
        return max(0.0, min(1.0, (v - minVal) / range))
    }
}

class Scorer(
    teams: List<Team>,
    private val fixtures: List<Fixture>,
    events: List<Event>,
    players: List<Player>,
    formulaId: String
) {
    private val teamMap = teams.associateBy { it.id }
    private val nextEventId: Int
    private val formula = getFormula(formulaId)
    private val teamAttackP90 = mutableMapOf<Int, Double>()
    private val teamDefenceP90 = mutableMapOf<Int, Double>()
    private val gwCtx = mutableMapOf<Int, MutableList<GWContext>>()
    private val dgwTeams = mutableSetOf<Int>()

    init {
        var nextGW = 1
        for (e in events) {
            if (e.is_next) {
                nextGW = e.id
                break
            }
        }
        nextEventId = nextGW
        computeTeamStats(players)
        buildGWContext()
    }

    fun getNextEventId(): Int = nextEventId

    fun getFixturePairings(): Map<Int, List<FixturePairing>> {
        val pairings = mutableMapOf<Int, MutableList<FixturePairing>>()
        for (f in fixtures) {
            if (f.event != nextEventId) continue
            pairings.getOrPut(f.team_h) { mutableListOf() }.add(FixturePairing(f.team_a, f.team_h_difficulty))
            pairings.getOrPut(f.team_a) { mutableListOf() }.add(FixturePairing(f.team_h, f.team_a_difficulty))
        }
        return pairings
    }

    private fun computeTeamStats(players: List<Player>) {
        val teamXG = mutableMapOf<Int, Double>()
        val teamXGA = mutableMapOf<Int, Double>()
        val teamGKMinutes = mutableMapOf<Int, Double>()

        for (p in players) {
            if (p.minutes == 0) continue
            val xG = parseFloatNum(p.expected_goals)
            teamXG[p.team] = (teamXG[p.team] ?: 0.0) + xG

            if (p.element_type == POS_GK && p.minutes > 450) {
                val xGC = parseFloatNum(p.expected_goals_conceded)
                teamXGA[p.team] = (teamXGA[p.team] ?: 0.0) + xGC
            }
            if (p.element_type == POS_GK) {
                teamGKMinutes[p.team] = (teamGKMinutes[p.team] ?: 0.0) + p.minutes.toDouble()
            }
        }

        teamMap.keys.forEach { id ->
            var mins = teamGKMinutes[id] ?: 0.0
            if (mins < 90.0) mins = 90.0
            val gp = mins / 90.0
            teamAttackP90[id] = (teamXG[id] ?: 0.0) / gp
            teamDefenceP90[id] = (teamXGA[id] ?: 0.0) / gp
        }
    }

    private fun buildGWContext() {
        for (f in fixtures) {
            if (f.event != nextEventId) continue

            val oppAttH = 1.0 / max(0.5, teamAttackP90[f.team_a] ?: 1.0)
            val oppDefH = teamDefenceP90[f.team_a] ?: 1.0

            val oppAttA = 1.0 / max(0.5, teamAttackP90[f.team_h] ?: 1.0)
            val oppDefA = teamDefenceP90[f.team_h] ?: 1.0

            gwCtx.getOrPut(f.team_h) { mutableListOf() }.add(
                GWContext(f.team_a, true, oppDefH, oppAttH, (6 - f.team_h_difficulty).toDouble())
            )
            gwCtx.getOrPut(f.team_a) { mutableListOf() }.add(
                GWContext(f.team_h, false, oppDefA, oppAttA, (6 - f.team_a_difficulty).toDouble())
            )
        }

        gwCtx.forEach { (teamId, ctxs) ->
            if (ctxs.size > 1) {
                dgwTeams.add(teamId)
            }
        }
    }

    fun scoreAll(players: List<Player>): List<ScoredPlayer> {
        val scored = mutableListOf<ScoredPlayer>()
        val raws = mutableListOf<RawMetrics>()

        for (p in players) {
            if (!isEligible(p)) continue

            val gp = max(1.0, p.minutes / 90.0)
            val ctxList = gwCtx[p.team] ?: emptyList()
            val hasFix = ctxList.isNotEmpty()

            var oppAtk = 0.0
            var oppDef = 0.0
            var easyFDR = 0.0
            var oppDesc = "BLANK"
            var isHome = false

            if (hasFix) {
                for (ctx in ctxList) {
                    oppAtk += ctx.oppAttWeakness
                    oppDef += ctx.oppDefWeakness
                    easyFDR += ctx.easyFDR
                }
                val n = ctxList.size.toDouble()
                oppAtk /= n
                oppDef /= n
                easyFDR /= n
                isHome = ctxList[0].isHome
                oppDesc = describeOpponents(ctxList)
            }

            val raw = RawMetrics(
                ppg = parseFloatNum(p.points_per_game),
                ep = parseFloatNum(p.ep_next),
                form = parseFloatNum(p.form),
                xgi90 = parseFloatNum(p.expected_goal_involvements) / gp,
                ict90 = parseFloatNum(p.ict_index) / gp,
                oppAtk = oppAtk,
                oppDef = oppDef,
                totalPts = p.total_points.toDouble(),
                easyFDR = easyFDR
            )
            raws.add(raw)

            val teamName = teamMap[p.team]?.short_name ?: "???"
            val positionName = POSITION_NAMES[p.element_type] ?: "???"

            scored.add(
                ScoredPlayer(
                    player = p,
                    score = 0.0,
                    teamName = teamName,
                    positionName = positionName,
                    oppDesc = oppDesc,
                    hasFixture = hasFix,
                    isHome = isHome,
                    isDgw = dgwTeams.contains(p.team)
                )
            )
        }

        if (raws.isEmpty()) return scored

        val nPPG = Normalizer(raws.map { it.ppg })
        val nEP = Normalizer(raws.map { it.ep })
        val nForm = Normalizer(raws.map { it.form })
        val nXGI = Normalizer(raws.map { it.xgi90 })
        val nICT = Normalizer(raws.map { it.ict90 })
        val nOA = Normalizer(raws.map { it.oppAtk })
        val nOD = Normalizer(raws.map { it.oppDef })
        val nTP = Normalizer(raws.map { it.totalPts })
        val nFDR = Normalizer(raws.map { it.easyFDR })

        for (i in scored.indices) {
            if (!scored[i].hasFixture) {
                scored[i] = scored[i].copy(score = 0.0)
                continue
            }

            val r = raws[i]
            val norm = mapOf(
                "ppg" to nPPG.normalize(r.ppg),
                "ep" to nEP.normalize(r.ep),
                "form" to nForm.normalize(r.form),
                "xgi90" to nXGI.normalize(r.xgi90),
                "ict90" to nICT.normalize(r.ict90),
                "oppAtk" to nOA.normalize(r.oppAtk),
                "oppDef" to nOD.normalize(r.oppDef),
                "totalPts" to nTP.normalize(r.totalPts),
                "easyFDR" to nFDR.normalize(r.easyFDR)
            )

            var baseScore = formula.fdr * norm["easyFDR"]!! +
                    formula.pts * norm["totalPts"]!! +
                    formula.form * norm["form"]!! +
                    formula.ep * norm["ep"]!! +
                    formula.ppg * norm["ppg"]!! +
                    formula.xgi * norm["xgi90"]!! +
                    formula.ict * norm["ict90"]!!

            val gp = max(1.0, scored[i].player.minutes / 90.0)
            val bonus = scorePositionBonus(scored[i].player, norm["oppAtk"]!!, norm["oppDef"]!!, gp)
            baseScore += bonus

            if (scored[i].isHome) baseScore += 0.02
            if (scored[i].isDgw) baseScore += 0.20

            scored[i] = scored[i].copy(score = baseScore)
        }

        return scored
    }

    private fun describeOpponents(ctxList: List<GWContext>): String {
        return ctxList.joinToString(" + ") { describeOpponent(it) }
    }

    private fun describeOpponent(ctx: GWContext): String {
        val oppTeam = teamMap[ctx.oppId]
        val shortName = oppTeam?.short_name ?: "???"
        val atkRating = teamAttackP90[ctx.oppId] ?: 0.0
        val defRating = teamDefenceP90[ctx.oppId] ?: 0.0

        val atkLabel = when {
            atkRating > 1.8 -> "Strong Atk"
            atkRating > 1.3 -> "Avg Atk"
            else -> "Weak Atk"
        }
        val defLabel = when {
            defRating > 1.5 -> "Leaky Def"
            defRating > 1.0 -> "Avg Def"
            else -> "Solid Def"
        }
        val ha = if (ctx.isHome) "H" else "A"
        return "$shortName($ha) [$atkLabel, $defLabel]"
    }
}

private fun scorePositionBonus(p: Player, oppAtk: Double, oppDef: Double, gp: Double): Double {
    val wOpponentQuality = 0.20
    val wCleanSheetBonus = 0.04

    return when (p.element_type) {
        POS_GK -> {
            val opp = oppAtk
            var bonus = wOpponentQuality * opp
            val xgc = parseFloatNum(p.expected_goals_conceded) / gp
            if (xgc < 1.0) bonus += wCleanSheetBonus * (1 - xgc)
            bonus
        }
        POS_DEF -> {
            val opp = 0.70 * oppAtk + 0.30 * oppDef
            var bonus = wOpponentQuality * opp
            val xgc = parseFloatNum(p.expected_goals_conceded) / gp
            if (xgc < 1.0) bonus += wCleanSheetBonus * (1 - xgc)
            bonus += 0.03 * (parseFloatNum(p.expected_assists) / gp)
            bonus
        }
        POS_MID -> {
            val opp = 0.20 * oppAtk + 0.80 * oppDef
            var bonus = wOpponentQuality * opp
            bonus += 0.04 * (parseFloatNum(p.expected_goals) / gp + parseFloatNum(p.expected_assists) / gp)
            bonus
        }
        POS_FWD -> {
            val opp = 0.10 * oppAtk + 0.90 * oppDef
            var bonus = wOpponentQuality * opp
            bonus += 0.07 * (parseFloatNum(p.expected_goals) / gp)
            bonus
        }
        else -> 0.0
    }
}

fun isEligible(p: Player): Boolean {
    if (p.status == "i" || p.status == "s" || p.status == "u") return false
    if (p.minutes < 90) return false
    if (p.status == "d" && p.chance_of_playing_next_round != null && p.chance_of_playing_next_round < 50) {
        return false
    }
    return true
}

fun parseFloatNum(s: String?): Double {
    if (s.isNullOrBlank()) return 0.0
    return s.toDoubleOrNull() ?: 0.0
}
