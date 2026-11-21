package com.slouchcat.app.tv.browse

import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.unit.Dp
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.media.MediaData

/**
 * The repositories and counts of this graph (`:core:media`): the server's unless a test or the
 * showcase put others in place (`MediaData.install(graph, …)`). The counts forget themselves with
 * the profile; the images of a profile go with it as well (`TvGraph` clears Coil's caches).
 */
val TvGraph.media: MediaData
    get() = MediaData.of(this) {
        MediaData.api(api, http).also { data -> hooks.onProfileOrSessionEnd { data.clearCounts() } }
    }

/**
 * The size of [this] for a view [width] wide on this screen: a TV of 1080p draws 960 dp at xhdpi,
 * so a poster card of 120 dp takes w400 and the backdrop w1920, never the 4K one (`ImageSet.pick`).
 */
@Composable
fun ImageSet.forWidth(width: Dp): String? {
    val px = with(LocalDensity.current) { width.roundToPx() }
    return pick(px)
}

/** The width of the screen: the backdrop of a page spans it. */
@Composable
fun screenWidth(): Dp = with(LocalDensity.current) { LocalWindowInfo.current.containerSize.width.toDp() }
