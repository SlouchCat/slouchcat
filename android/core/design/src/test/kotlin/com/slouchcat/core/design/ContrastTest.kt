package com.slouchcat.core.design

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.compositeOver
import androidx.compose.ui.graphics.luminance
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Concept 27, 3.2 (Marco's rules of 16 November 2026): frames at least 3:1, muted text at least
 * 8:1. Checked on the colours of the theme as the app paints them (translucent frames laid over
 * the surface). Muted text never sits on s3, the surface of inputs and pressed buttons.
 */
class ContrastTest {
    private val c = SlouchcatColors()

    private fun ratio(fg: Color, bg: Color): Double {
        val a = fg.compositeOver(bg).luminance() + 0.05
        val b = bg.luminance() + 0.05
        return (maxOf(a, b) / minOf(a, b)).toDouble()
    }

    private fun atLeast(min: Double, fg: Color, bg: Color, what: String) {
        val r = ratio(fg, bg)
        assertTrue("$what: ${"%.2f".format(r)}:1 < $min:1", r >= min)
    }

    @Test
    fun textOnEverySurface() {
        for ((name, bg) in listOf("bg" to c.bg, "elev" to c.elev, "s1" to c.s1, "s2" to c.s2, "s3" to c.s3)) {
            atLeast(7.0, c.tx, bg, "tx on $name")
            atLeast(7.0, c.tx2, bg, "tx2 on $name")
        }
    }

    @Test
    fun mutedTextAtLeastEightToOne() {
        for ((name, bg) in listOf("bg" to c.bg, "elev" to c.elev, "s1" to c.s1, "s2" to c.s2)) {
            atLeast(8.0, c.tx3, bg, "tx3 on $name")
        }
    }

    @Test
    fun framesAndControlsAtLeastThreeToOne() {
        for ((name, bg) in listOf("bg" to c.bg, "elev" to c.elev, "s1" to c.s1)) {
            atLeast(3.0, c.frame, bg, "frame on $name")
            atLeast(3.0, c.ctl, bg, "ctl on $name")
        }
        atLeast(3.0, c.ctl, c.s3, "ctl on s3 (secondary button)")
    }

    @Test
    fun stateColoursAndButtons() {
        for ((name, bg) in listOf("bg" to c.bg, "elev" to c.elev, "s1" to c.s1)) {
            atLeast(4.5, c.danger, bg, "danger on $name")
            atLeast(4.5, c.warn, bg, "warn on $name")
            atLeast(4.5, c.ok, bg, "ok on $name")
        }
        atLeast(7.0, c.bg, c.tx, "primary button")
        atLeast(4.5, c.onAccent, c.accent, "accent button")
    }
}
