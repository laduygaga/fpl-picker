package app.fplpicker.app.viewmodel

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import app.fplpicker.app.data.FplRepository
import app.fplpicker.core.model.ScoredPlayer
import app.fplpicker.core.model.Scorer
import app.fplpicker.core.model.SquadResult
import app.fplpicker.core.model.findBestSquad
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

sealed interface UiState {
    data object Initial : UiState
    data object Loading : UiState
    data class Success(
        val result: SquadResult,
        val nextGw: Int,
        val scoredCount: Int,
        val isStale: Boolean = false
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

    fun updateBudget(newBudget: String) {
        _budget.value = newBudget
    }

    fun updateFormula(newFormula: String) {
        _formula.value = newFormula
    }

    fun optimizeSquad(fresh: Boolean = false) {
        viewModelScope.launch(Dispatchers.Default) {
            _uiState.value = UiState.Loading
            try {
                val budgetVal = _budget.value.toDoubleOrNull() ?: 100.0
                val budgetTenths = (budgetVal * 10).toInt()

                val bootstrap = repository.fetchBootstrap(fresh)
                val fixtures = repository.fetchFixtures(fresh)

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

                if (optimal.starters.isEmpty()) {
                    _uiState.value = UiState.Error("No valid squad found. Check budget or fixture data.")
                } else {
                    _uiState.value = UiState.Success(
                        result = optimal,
                        nextGw = scorer.getNextEventId(),
                        scoredCount = scoredPlayers.size
                    )
                }
            } catch (e: Exception) {
                _uiState.value = UiState.Error(e.message ?: "Unknown error occurred during optimization.")
            }
        }
    }
}
