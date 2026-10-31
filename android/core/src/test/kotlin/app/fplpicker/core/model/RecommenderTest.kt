package app.fplpicker.core.model

import app.fplpicker.core.api.Player
import org.junit.Assert.*
import org.junit.Test

class RecommenderTest {

    @Test
    fun testPickCaptains() {
        val p1 = ScoredPlayer(Player(id = 1, web_name = "P1", team = 1, element_type = 3, now_cost = 80), 0.5, "ARS", "MID", "OPP", true, true, false)
        val p2 = ScoredPlayer(Player(id = 2, web_name = "P2", team = 2, element_type = 4, now_cost = 100), 0.9, "CHE", "FWD", "OPP", true, true, false)
        val p3 = ScoredPlayer(Player(id = 3, web_name = "P3", team = 3, element_type = 3, now_cost = 70), 0.7, "LIV", "MID", "OPP", true, true, false)

        val (cap, vc) = pickCaptains(listOf(p1, p2, p3))
        assertEquals(2, cap?.player?.id)
        assertEquals(3, vc?.player?.id)
    }

    @Test
    fun testFindBestSquadBasic() {
        val players = mutableListOf<ScoredPlayer>()
        for (i in 1..4) {
            players.add(ScoredPlayer(Player(id = i, web_name = "GK$i", team = i, element_type = POS_GK, now_cost = 40), 0.3 + i * 0.01, "T$i", "GK", "OPP", true, true, false))
        }
        for (i in 5..12) {
            players.add(ScoredPlayer(Player(id = i, web_name = "DEF$i", team = (i % 10) + 1, element_type = POS_DEF, now_cost = 45), 0.5 + i * 0.01, "T$i", "DEF", "OPP", true, true, false))
        }
        for (i in 13..20) {
            players.add(ScoredPlayer(Player(id = i, web_name = "MID$i", team = (i % 10) + 1, element_type = POS_MID, now_cost = 50), 0.6 + i * 0.01, "T$i", "MID", "OPP", true, true, false))
        }
        for (i in 21..25) {
            players.add(ScoredPlayer(Player(id = i, web_name = "FWD$i", team = (i % 10) + 1, element_type = POS_FWD, now_cost = 70), 0.7 + i * 0.01, "T$i", "FWD", "OPP", true, true, false))
        }

        val res = findBestSquad(players, 1000)
        assertEquals(11, res.starters.size)
        assertEquals(4, res.bench.size)
        assertNotNull(res.formation)
        assertTrue(res.totalScore > 0.0)
        assertTrue(res.totalCost <= res.budget)
    }
}
