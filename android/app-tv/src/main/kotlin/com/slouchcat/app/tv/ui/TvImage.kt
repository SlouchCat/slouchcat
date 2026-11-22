package com.slouchcat.app.tv.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import coil3.ImageLoader
import coil3.SingletonImageLoader
import coil3.compose.AsyncImage
import coil3.compose.LocalPlatformContext

/**
 * The image loader of cards, backdrops and portraits: `null` takes the app's (`TvGraph.imageLoader`
 * over the one HTTP client, with the gate of the server's key). The showcase of androidTest puts
 * drawn pictures in place.
 */
val LocalTvImageLoader = staticCompositionLocalOf<ImageLoader?> { null }

/** An image of the server ([model]: an address or anything Coil loads), cropped to its box, read by nobody. */
@Composable
fun TvImage(model: Any?, modifier: Modifier = Modifier, contentScale: ContentScale = ContentScale.Crop, alignment: Alignment = Alignment.Center) {
    val loader = LocalTvImageLoader.current ?: SingletonImageLoader.get(LocalPlatformContext.current)
    AsyncImage(model = model, contentDescription = null, imageLoader = loader, alignment = alignment, contentScale = contentScale, modifier = modifier)
}
