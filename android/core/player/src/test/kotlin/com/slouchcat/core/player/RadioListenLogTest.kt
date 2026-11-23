package com.slouchcat.core.player

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.RadioListen
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.api.log.LogSink
import com.slouchcat.core.player.testing.station
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Rule 10 (code review stage 7 L-5): the answer of `startRadioListen` carries the token `rt` in its
 * playlist address; the log never shows it, also when the whole answer goes in by its `toString`.
 */
class RadioListenLogTest {
    @Test
    fun theListenTokenNeverReachesTheLog() {
        val listen = RadioListen(
            listenId = "l-1",
            playlistUrl = "/api/v1/radio/live/l-1/index.m3u8?rt=RADIOTOKEN123",
            heartbeatSeconds = 30,
            startOffsetSeconds = 6,
            timeShiftSeconds = 0,
            station = station(7),
        )
        val lines = mutableListOf<String>()
        val before = Log.sink
        Log.sink = object : LogSink {
            override fun write(priority: Int, tag: String, message: String) {
                lines += message
            }
        }
        try {
            Log.i("radio", "listen $listen", "answer" to ApiResult.Ok(listen), "playlistUrl" to listen.playlistUrl)
        } finally {
            Log.sink = before
        }
        val out = lines.single()
        assertFalse(out, "RADIOTOKEN123" in out)
        assertTrue(out, "Testradio 7" in out)
    }
}
