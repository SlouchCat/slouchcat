package com.slouchcat.app.tv.player

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** Spulen with the remote (mockup `brPlSeek`): steps that grow in a streak and while a key is held. */
class ScrubTest {
    private val hour = 3_600_000L

    @Test
    fun pressesInQuickSuccessionGrowTheStep() {
        val scrub = Scrub()
        // 10, 10, 20, 30, 30, 60, 60, 120, 120 s, one press every 300 ms.
        repeat(9) { n -> assertTrue(scrub.step(1, n * 300L, positionMs = 60_000, durationMs = hour)) }
        assertEquals(60_000L + 460_000, scrub.targetMs)
        assertEquals(460_000L, scrub.deltaMs)
        assertEquals(520_000L, scrub.commit())
        assertNull(scrub.targetMs)
    }

    @Test
    fun aPauseOrTheOtherWayStartsAgainWithTenSeconds() {
        val scrub = Scrub()
        scrub.step(1, 0, 600_000, hour)
        scrub.step(1, 300, 600_000, hour)
        scrub.step(1, 600, 600_000, hour)
        assertEquals(640_000L, scrub.targetMs)
        // 800 ms later: no streak.
        scrub.step(1, 1_400, 600_000, hour)
        assertEquals(650_000L, scrub.targetMs)
        // The other way: 10 s back from the target, the way counts from where it began.
        scrub.step(-1, 1_500, 600_000, hour)
        assertEquals(640_000L, scrub.targetMs)
        assertEquals(40_000L, scrub.deltaMs)
    }

    @Test
    fun aHeldKeyStepsAtMostEveryHundredFiftyMilliseconds() {
        val scrub = Scrub()
        assertTrue(scrub.step(1, 0, 0, hour))
        // Repeats every 50 ms (Android's key repeat): two of three are dropped.
        assertFalse(scrub.step(1, 50, 0, hour, repeat = true))
        assertFalse(scrub.step(1, 100, 0, hour, repeat = true))
        assertTrue(scrub.step(1, 150, 0, hour, repeat = true))
        assertEquals(20_000L, scrub.targetMs)
        // Held until 1.5 s: a step at 300, 450 … 1500 ms, the last four of two minutes each.
        var t = 150L
        repeat(27) {
            t += 50
            scrub.step(1, t, 0, hour, repeat = true)
        }
        assertEquals((10L + 10 + 20 + 30 + 30 + 60 + 60 + 4 * 120) * 1000, scrub.targetMs)
    }

    @Test
    fun theTargetStaysWithinTheFile() {
        val scrub = Scrub()
        scrub.step(-1, 0, 5_000, hour)
        assertEquals(0L, scrub.targetMs)
        scrub.reset()
        scrub.step(1, 0, hour - 3_000, hour)
        assertEquals(hour - Scrub.END_GAP_MS, scrub.targetMs)
    }
}
