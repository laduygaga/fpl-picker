package app.fplpicker.core.model

import app.fplpicker.core.api.Player
import org.junit.Assert.*
import org.junit.Test

class ScorerTest {

    @Test
    fun testIsEligible() {
        val available = Player(id = 1, web_name = "A", team = 1, element_type = 1, now_cost = 50, minutes = 200, status = "a")
        assertTrue(isEligible(available))

        val injured = Player(id = 2, web_name = "B", team = 1, element_type = 1, now_cost = 50, minutes = 200, status = "i")
        assertFalse(isEligible(injured))

        val zeroMins = Player(id = 3, web_name = "C", team = 1, element_type = 1, now_cost = 50, minutes = 0, status = "a")
        assertFalse(isEligible(zeroMins))

        val doubtfulOk = Player(id = 4, web_name = "D", team = 1, element_type = 1, now_cost = 50, minutes = 200, status = "d", chance_of_playing_next_round = 75)
        assertTrue(isEligible(doubtfulOk))

        val doubtfulBad = Player(id = 5, web_name = "E", team = 1, element_type = 1, now_cost = 50, minutes = 200, status = "d", chance_of_playing_next_round = 25)
        assertFalse(isEligible(doubtfulBad))
    }

    @Test
    fun testNormalizerBounds() {
        val n = Normalizer(listOf(0.0, 5.0, 10.0))
        assertEquals(0.0, n.normalize(0.0), 1e-5)
        assertEquals(1.0, n.normalize(10.0), 1e-5)
        assertEquals(0.5, n.normalize(5.0), 1e-5)
        assertEquals(0.0, n.normalize(-5.0), 1e-5)
        assertEquals(1.0, n.normalize(15.0), 1e-5)
    }
}
