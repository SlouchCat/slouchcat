package com.slouchcat.core.design

import androidx.compose.foundation.Canvas
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.CompositingStrategy
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.unit.dp

/**
 * The quiet backdrop of the entry pages (`.su-art` in the mockup's setup.css): three tilted rows of
 * cover-shaped tiles that fade out towards the form, below on a phone ([fadeToEnd] false) and to
 * the right on a tablet. Before the app knows a server it has no covers to show, so the tiles are
 * the accent colours of the tokens over the surface, like the placeholder of a missing cover.
 * Decorative: no semantics.
 */
@Composable
fun EntryBackdrop(modifier: Modifier = Modifier, fadeToEnd: Boolean = false) {
    val tileWidth = if (fadeToEnd) 132.dp else 92.dp
    val gap = if (fadeToEnd) 14.dp else 10.dp
    val base = SlouchcatTheme.colors.s1
    Canvas(modifier.graphicsLayer(compositingStrategy = CompositingStrategy.Offscreen, alpha = 0.5f)) {
        val w = tileWidth.toPx()
        val h = w * 1.5f
        val g = gap.toPx()
        val radius = CornerRadius(if (fadeToEnd) 12.dp.toPx() else 10.dp.toPx())
        val rows = if (fadeToEnd) ((size.height + 2 * h) / (h + g)).toInt() + 1 else 3
        rotate(degrees = -9f) {
            val top = if (fadeToEnd) -h else -30.dp.toPx()
            for (row in 0 until rows) {
                val shift = row * 60.dp.toPx() + w
                val y = top + row * (h + g)
                var col = 0
                var x = -shift
                while (x < size.width + w) {
                    val tint = ACCENTS[(row * 3 + col) % ACCENTS.size]
                    drawRoundRect(
                        brush = Brush.linearGradient(
                            listOf(tint.copy(alpha = 0.55f), base),
                            start = Offset(x, y),
                            end = Offset(x + w, y + h),
                        ),
                        topLeft = Offset(x, y),
                        size = Size(w, h),
                        cornerRadius = radius,
                    )
                    x += w + g
                    col++
                }
            }
        }
        // The fade (`mask-image` in the mockup): only the alpha counts, opaque at the start.
        val fade = if (fadeToEnd) {
            Brush.horizontalGradient(0.45f to base, 1f to Color.Transparent)
        } else {
            Brush.verticalGradient(0.3f to base, 1f to Color.Transparent)
        }
        drawRect(fade, blendMode = BlendMode.DstIn)
    }
}

private val ACCENTS = listOf(
    SlouchcatTokens.Color.Accent.Ember.Base,
    SlouchcatTokens.Color.Accent.Sky.Base,
    SlouchcatTokens.Color.Accent.Violet.Base,
    SlouchcatTokens.Color.Accent.Teal.Base,
    SlouchcatTokens.Color.Accent.Rose.Base,
    SlouchcatTokens.Color.Accent.Amber.Base,
    SlouchcatTokens.Color.Accent.Lime.Base,
    SlouchcatTokens.Color.Accent.Coral.Base,
)
