package com.slouchcat.app.mobile.media

import android.graphics.Bitmap
import android.graphics.Color
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.io.File

/**
 * The blurred backdrop of the detail pages without `RenderEffect` (stage 7, item 4): drawn on the
 * CPU from a small copy, so it shows also on a GPU without effects (the emulator of the showcase).
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class CpuBlurTest {
    @Test
    fun theBlurIsSmallAndSoftensEdges() {
        val source = Bitmap.createBitmap(400, 600, Bitmap.Config.ARGB_8888)
        for (y in 0 until 600) for (x in 0 until 400) source.setPixel(x, y, if (x < 200) Color.BLACK else Color.WHITE)
        val blurred = CpuBlur.blur(source)
        assertTrue(blurred.width <= 64)
        // At the former edge the colours mix: neither black nor white.
        val edge = blurred.getPixel(blurred.width / 2, blurred.height / 2)
        assertNotEquals(Color.BLACK, edge)
        assertNotEquals(Color.WHITE, edge)
    }

    @Test
    fun noPageUsesTheRenderEffectBlur() {
        val roots = listOf(File("src/main/kotlin"), File("../app-tv/src/main/kotlin"))
        // The modifier `blur(30.dp)` (its import, or a call with a size); `CpuBlur.blur(bitmap)` is fine.
        val modifier = Regex("""import androidx\.compose\.ui\.draw\.blur|\.blur\(\s*[0-9]|import android\.graphics\.RenderEffect|RenderEffect\.create""")
        val uses = roots.filter { it.exists() }.flatMap { root -> root.walk().filter { it.extension == "kt" && modifier.containsMatchIn(it.readText()) }.toList() }
        assertEquals(emptyList<File>(), uses)
    }
}
