package com.slouchcat.core.api

import com.slouchcat.core.api.generated.models.Progress
import com.slouchcat.core.api.generated.models.ProgressEvent
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.math.BigDecimal
import java.time.OffsetDateTime

/** Decimal numbers of the contract (`playbackRate`, `percent`) travel as JSON numbers, not strings (the server answered 400). */
class DecimalNumbersTest {
    @Test
    fun aSpeedGoesOutAsANumber() {
        val event = ProgressEvent(
            positionMs = 1, durationMs = 2, playedMs = 1, seq = 1, state = ProgressEvent.State.PAUSED,
            clientTime = OffsetDateTime.parse("2026-11-22T10:00:00Z"), playbackRate = BigDecimal("1.5"),
        )
        val json = SlouchcatApi.json.encodeToString(ProgressEvent.serializer(), event)
        assertTrue(json, json.contains("\"playbackRate\":1.5"))
    }

    @Test
    fun numbersAndStringsComeIn() {
        val number = """{"positionMs":1,"durationMs":2,"percent":38.5,"completed":false,"updatedAt":"2026-11-22T10:00:00Z","playbackRate":1.25}"""
        val progress = SlouchcatApi.json.decodeFromString(Progress.serializer(), number)
        assertEquals(BigDecimal("38.5"), progress.percent)
        assertEquals(BigDecimal("1.25"), progress.playbackRate)
        val string = number.replace("38.5", "\"38.5\"")
        assertEquals(BigDecimal("38.5"), SlouchcatApi.json.decodeFromString(Progress.serializer(), string).percent)
    }
}
