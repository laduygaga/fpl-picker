package app.fplpicker.app.viewmodel

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import app.fplpicker.app.data.FplRepository
import app.fplpicker.core.api.MyTeam
import app.fplpicker.core.api.TransferRequest
import app.fplpicker.core.api.TransferSuggestionDto
import app.fplpicker.core.model.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

sealed interface UiState {
    data object Initial : UiState
    data object Loading : UiState
    data class Success(
        val result: SquadResult,
        val myTeam: MyTeam?,
        val transfers: List<TransferSuggestion>,
        val nextGw: Int,
        val scoredCount: Int,
        val message: String? = null,
        val players: List<app.fplpicker.core.api.Player> = emptyList(),
        val teams: List<app.fplpicker.core.api.Team> = emptyList()
    ) : UiState
    data class Error(val message: String) : UiState
}

class MainViewModel(application: Application) : AndroidViewModel(application) {
    private val repository = FplRepository(application)

    private val _uiState = MutableStateFlow<UiState>(UiState.Initial)
    val uiState: StateFlow<UiState> = _uiState.asStateFlow()

    private val _budget = MutableStateFlow("100.0")
    val budget: StateFlow<String> = _budget.asStateFlow()

    private val _formula = MutableStateFlow("1")
    val formula: StateFlow<String> = _formula.asStateFlow()

    private val _teamId = MutableStateFlow("")
    val teamId: StateFlow<String> = _teamId.asStateFlow()

    private val _cookies = MutableStateFlow("")
    val cookies: StateFlow<String> = _cookies.asStateFlow()

    fun updateBudget(newBudget: String) { _budget.value = newBudget }
    fun updateFormula(newFormula: String) { _formula.value = newFormula }
    fun updateTeamId(newTeamId: String) { _teamId.value = newTeamId }
    fun updateCookies(newCookies: String) {
        _cookies.value = newCookies
        if (newCookies.isNotBlank()) {
            autoDetectTeamId { success ->
                if (success) {
                    val tid = _teamId.value.toIntOrNull()
                    if (tid != null) {
                        viewModelScope.launch(Dispatchers.IO) {
                            try {
                                val team = repository.getMyTeam(tid, newCookies)
                                val bootstrap = repository.fetchBootstrap(false)
                                val fixtures = repository.fetchFixtures(false)
                                val scorer = Scorer(bootstrap.teams, fixtures, bootstrap.events, bootstrap.elements, _formula.value)
                                val scoredPlayers = scorer.scoreAll(bootstrap.elements)
                                val optimal = findBestSquad(scoredPlayers, (_budget.value.toDoubleOrNull() ?: 100.0).times(10).toInt(), scorer.getFixturePairings())
                                val transfers = planTransfers(team, optimal, 4, bootstrap.elements.associate { it.id to it.team }, scoredPlayers.associateBy { it.player.id })
                                cachedOptimal = optimal
                                cachedTransfers = transfers
                                cachedNextGw = scorer.getNextEventId()
                                withContext(Dispatchers.Main) {
                                    _uiState.value = UiState.Success(
                                        result = optimal,
                                        myTeam = team,
                                        transfers = transfers,
                                        nextGw = cachedNextGw,
                                        scoredCount = scoredPlayers.size,
                                        players = bootstrap.elements,
                                        teams = bootstrap.teams,
                                        message = "🎉 Successfully connected and loaded live team!"
                                    )
                                }
                            } catch (e: Exception) {
                                android.util.Log.e("FplPicker", "Preload live team failed: ${e.message}", e)
                            }
                        }
                    }
                }
            }
        }
    }

    fun autoDetectTeamId(onResult: ((Boolean) -> Unit)? = null) {
        viewModelScope.launch(Dispatchers.IO) {
            try {
                val tid = repository.getMyEntryId(_cookies.value)
                if (tid != null) {
                    withContext(Dispatchers.Main) {
                        _teamId.value = tid.toString()
                        onResult?.invoke(true)
                    }
                } else {
                    withContext(Dispatchers.Main) {
                        onResult?.invoke(false)
                    }
                }
            } catch (e: Exception) {
                android.util.Log.e("FplPicker", "AutoDetectTeamId failed: ${e.message}", e)
                withContext(Dispatchers.Main) {
                    onResult?.invoke(false)
                }
            }
        }
    }

    private var cachedOptimal: SquadResult? = null
    private var cachedTransfers: List<TransferSuggestion> = emptyList()
    private var cachedNextGw: Int = 1

    private var cachedMyTeam: MyTeam? = null

    fun optimizeSquad(fresh: Boolean = false) {
        viewModelScope.launch(Dispatchers.Default) {
            _uiState.value = UiState.Loading
            try {
                val budgetVal = _budget.value.toDoubleOrNull() ?: 100.0
                val budgetTenths = (budgetVal * 10).toInt()
                val tid = _teamId.value.toIntOrNull()

                val bootstrap = repository.fetchBootstrap(fresh)
                val fixtures = repository.fetchFixtures(fresh)

                val myTeam = cachedMyTeam ?: if (tid != null) {
                    try { 
                        repository.getMyTeam(tid, _cookies.value).also { cachedMyTeam = it }
                    } catch (e: Exception) { 
                        android.util.Log.e("FplPicker", "Failed to fetch myTeam: ${e.message}", e)
                        null 
                    }
                } else null

                val scorer = Scorer(
                    teams = bootstrap.teams,
                    fixtures = fixtures,
                    events = bootstrap.events,
                    players = bootstrap.elements,
                    formulaId = _formula.value
                )

                val scoredPlayers = scorer.scoreAll(bootstrap.elements)
                val pairings = scorer.getFixturePairings()

                val optimal = findBestSquad(scoredPlayers, budgetTenths, pairings)
                cachedOptimal = optimal
                cachedNextGw = scorer.getNextEventId()

                val idToTeam = bootstrap.elements.associate { it.id to it.team }
                val scoredById = scoredPlayers.associateBy { it.player.id }

                val transfers = if (myTeam != null) {
                    planTransfers(myTeam, optimal, 4, idToTeam, scoredById, true)
                } else emptyList()
                cachedTransfers = transfers

                if (optimal.starters.isEmpty()) {
                    _uiState.value = UiState.Error("No valid squad found. Check budget.")
                } else {
                    _uiState.value = UiState.Success(
                        result = optimal,
                        myTeam = myTeam,
                        transfers = transfers,
                        nextGw = cachedNextGw,
                        scoredCount = scoredPlayers.size,
                        players = bootstrap.elements,
                        teams = bootstrap.teams
                    )
                }
            } catch (e: Exception) {
                _uiState.value = UiState.Error(e.message ?: "Unknown error occurred.")
            }
        }
    }

    fun fetchMyTeamOnly() {
        viewModelScope.launch(Dispatchers.Default) {
            _uiState.value = UiState.Loading
            try {
                val tid = _teamId.value.toIntOrNull()
                if (tid == null || tid <= 0) {
                    _uiState.value = UiState.Error("Please enter or auto-detect a valid Team ID first.")
                    return@launch
                }
                val bootstrap = repository.fetchBootstrap(false)
                val myTeam = repository.getMyTeam(tid, _cookies.value)
                cachedMyTeam = myTeam

                val currentState = _uiState.value
                if (currentState is UiState.Success) {
                    _uiState.value = currentState.copy(
                        myTeam = myTeam,
                        message = "✅ Successfully fetched live team!"
                    )
                } else {
                    val scorer = Scorer(bootstrap.teams, repository.fetchFixtures(false), bootstrap.events, bootstrap.elements, _formula.value)
                    val scoredPlayers = scorer.scoreAll(bootstrap.elements)
                    val optimal = cachedOptimal ?: findBestSquad(scoredPlayers, (_budget.value.toDoubleOrNull() ?: 100.0).times(10).toInt(), scorer.getFixturePairings())
                    cachedOptimal = optimal
                    _uiState.value = UiState.Success(
                        result = optimal,
                        myTeam = myTeam,
                        transfers = cachedTransfers,
                        nextGw = scorer.getNextEventId(),
                        scoredCount = scoredPlayers.size,
                        players = bootstrap.elements,
                        teams = bootstrap.teams,
                        message = "✅ Successfully fetched live team!"
                    )
                }
            } catch (e: Exception) {
                _uiState.value = UiState.Error("Failed to fetch live team: ${e.message}")
            }
        }
    }

    fun applyChanges() {
        val tid = _teamId.value.toIntOrNull()
        val cookieStr = _cookies.value
        val opt = cachedOptimal

        if (tid == null || tid <= 0) {
            _uiState.value = UiState.Error("Please log in or enter a valid FPL Team ID.")
            return
        }
        if (cookieStr.isBlank()) {
            _uiState.value = UiState.Error("Please log into your FPL account first.")
            return
        }
        if (opt == null) {
            _uiState.value = UiState.Error("Please run 'Optimize Squad' first.")
            return
        }

        viewModelScope.launch(Dispatchers.Default) {
            _uiState.value = UiState.Loading
            try {
                if (cachedTransfers.isNotEmpty()) {
                    val transferReq = TransferRequest(
                        confirmed = true,
                        entry = tid,
                        event = cachedNextGw,
                        transfers = cachedTransfers.map { t ->
                            TransferSuggestionDto(
                                element_in = t.inPlayer.player.id,
                                element_out = t.out.player.id,
                                purchase_price = t.purchasePrice,
                                selling_price = t.sellingPrice
                            )
                        }
                    )
                    repository.postTransfers(transferReq, cookieStr)
                }

                val refreshedTeam = repository.getMyTeam(tid, cookieStr)
                cachedMyTeam = refreshedTeam
                val bootstrap = repository.fetchBootstrap(true)
                val scorer = Scorer(bootstrap.teams, repository.fetchFixtures(true), bootstrap.events, bootstrap.elements, _formula.value)
                val allScored = scorer.scoreAll(bootstrap.elements)
                val scoredById = allScored.associateBy { it.player.id }

                val current15 = refreshedTeam.picks.mapNotNull { scoredById[it.element] }
                val postResult = bestXIFromSquad(current15)
                val lineupUpdate = planLineup(postResult)

                repository.postLineup(tid, lineupUpdate, cookieStr)

                _uiState.value = UiState.Success(
                    result = opt,
                    myTeam = refreshedTeam,
                    transfers = emptyList(),
                    nextGw = cachedNextGw,
                    scoredCount = allScored.size,
                    players = bootstrap.elements,
                    teams = bootstrap.teams,
                    message = "✅ Applied successfully to FPL team and lineup!"
                )
            } catch (e: Exception) {
                _uiState.value = UiState.Error("Failed to apply: ${e.message}")
            }
        }
    }
}

fun bestXIFromSquad(squad: List<ScoredPlayer>): SquadResult {
    val byPos = mutableMapOf<Int, MutableList<ScoredPlayer>>()
    listOf(POS_GK, POS_DEF, POS_MID, POS_FWD).forEach { byPos[it] = mutableListOf() }
    squad.forEach { byPos[it.player.element_type]?.add(it) }
    byPos.values.forEach { it.sortByDescending { sp -> sp.score } }

    var bestStarters = listOf<ScoredPlayer>()
    var bestFormation = ""
    var maxScore = -1.0

    for (fm in VALID_FORMATIONS) {
        val needs = mapOf(POS_GK to fm.gk, POS_DEF to fm.def, POS_MID to fm.mid, POS_FWD to fm.fwd)
        var valid = true
        for (pos in listOf(POS_GK, POS_DEF, POS_MID, POS_FWD)) {
            if ((byPos[pos]?.size ?: 0) < needs[pos]!!) valid = false
        }
        if (!valid) continue

        val trial = mutableListOf<ScoredPlayer>()
        var score = 0.0
        listOf(POS_GK, POS_DEF, POS_MID, POS_FWD).forEach { pos ->
            val n = needs[pos]!!
            val pool = byPos[pos]!!
            for (j in 0 until n) {
                trial.add(pool[j])
                score += pool[j].score
            }
        }
        if (score > maxScore) {
            maxScore = score
            bestStarters = trial
            bestFormation = fm.name
        }
    }

    val starterIds = bestStarters.map { it.player.id }.toSet()
    val bench = squad.filter { !starterIds.contains(it.player.id) }.sortedByDescending { it.score }
    val (cap, vc) = pickCaptains(bestStarters)
    val xiCost = bestStarters.sumOf { it.player.now_cost }
    val totalCost = xiCost + bench.sumOf { it.player.now_cost }

    return SquadResult(
        formation = bestFormation,
        starters = bestStarters,
        bench = bench,
        captain = cap,
        viceCaptain = vc,
        totalScore = maxScore,
        xiCost = xiCost / 10.0,
        totalCost = totalCost / 10.0,
        budget = totalCost / 10.0
    )
}
