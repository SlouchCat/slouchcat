package com.slouchcat.app.mobile.downloads

import org.junit.Assert.assertEquals
import org.junit.Test
import java.util.Locale

/** Sizes as the web app writes them (sight check of the coordinator: «1,1 GB» in German). */
class SizesTest {
    private val nb = ' '

    @Test
    fun swissGermanWritesTheDecimalComma() {
        val de = Locale.forLanguageTag("de-CH")
        assertEquals("1,1${nb}GB", formatBytes(1_100_000_000, de))
        assertEquals("470${nb}MB", formatBytes(470_000_000, de))
        assertEquals("5,6${nb}GB", formatBytes(5_600_000_000, de))
        assertEquals("0${nb}B", formatBytes(0, de))
    }

    @Test
    fun englishWritesTheDecimalPoint() {
        val en = Locale.forLanguageTag("en")
        assertEquals("1.1${nb}GB", formatBytes(1_100_000_000, en))
        assertEquals("696${nb}MB", formatBytes(696_000_000, en))
        assertEquals("12${nb}kB", formatBytes(12_345, en))
    }
}
