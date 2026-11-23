package com.slouchcat.core.player

import androidx.media3.common.C
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/** Code review stage 7 L-2: a download keeps only the CPU awake, a stream also the Wi-Fi. */
class WakeModeTest {
    @Test
    fun aDownloadNeedsNoWifiLock() {
        assertEquals(C.WAKE_MODE_LOCAL, wakeModeFor(EngineKind.Speech, "file:///data/user/0/app/files/offline/1/audio.m4a"))
        assertEquals(C.WAKE_MODE_NETWORK, wakeModeFor(EngineKind.Speech, "https://media.example.test/api/v1/hls/1/main.m3u8"))
        assertEquals(C.WAKE_MODE_NETWORK, wakeModeFor(EngineKind.Music, "http://192.0.2.10:8096/api/v1/radio/live/l-1/index.m3u8"))
        assertNull(wakeModeFor(EngineKind.Video, "https://media.example.test/api/v1/hls/1/main.m3u8"))
    }
}
