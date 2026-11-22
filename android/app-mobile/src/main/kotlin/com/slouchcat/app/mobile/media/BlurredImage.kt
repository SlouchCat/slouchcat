package com.slouchcat.app.mobile.media

import android.graphics.Bitmap
import android.util.LruCache
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import coil3.SingletonImageLoader
import coil3.request.ImageRequest
import coil3.request.SuccessResult
import coil3.request.allowHardware
import coil3.toBitmap
import com.slouchcat.core.media.ImageSet
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * The blurred cover behind a head (detail page, collection): the smallest size of [images], shrunk,
 * blurred on the CPU once per address and kept, drawn scaled up. Not a `RenderEffect`: the software
 * GPU of the emulators (local and on the CI runners) crashed on the blur over the whole width.
 */
@Composable
fun BlurredImage(images: ImageSet, modifier: Modifier = Modifier) {
    val url = images.pick(1) ?: return
    val context = LocalContext.current
    val loader = LocalMediaImageLoader.current ?: SingletonImageLoader.get(context)
    val picture by produceState<ImageBitmap?>(BlurCache.get(url), url) {
        if (value != null) return@produceState
        val request = ImageRequest.Builder(context).data(url).allowHardware(false).build()
        val source = (loader.execute(request) as? SuccessResult)?.image?.toBitmap() ?: return@produceState
        value = withContext(Dispatchers.Default) { CpuBlur.blur(source).asImageBitmap() }.also { BlurCache.put(url, it) }
    }
    Box(modifier) {
        picture?.let { Image(it, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.matchParentSize()) }
    }
}

/** The last blurred covers (small: 48 px wide each). */
private object BlurCache {
    private val cache = LruCache<String, ImageBitmap>(32)

    fun get(url: String): ImageBitmap? = cache.get(url)

    fun put(url: String, image: ImageBitmap) {
        cache.put(url, image)
    }
}

/** A box blur on a small copy of the picture: three passes look like a Gaussian blur. */
object CpuBlur {
    /** The width the picture is shrunk to before the blur. */
    const val WIDTH = 48
    private const val RADIUS = 3
    private const val PASSES = 3

    fun blur(source: Bitmap): Bitmap {
        val height = maxOf(1, source.height * WIDTH / maxOf(1, source.width))
        val small = Bitmap.createScaledBitmap(source, WIDTH, height, true)
        val pixels = IntArray(WIDTH * height)
        small.getPixels(pixels, 0, WIDTH, 0, 0, WIDTH, height)
        repeat(PASSES) {
            boxPass(pixels, WIDTH, height, horizontal = true)
            boxPass(pixels, WIDTH, height, horizontal = false)
        }
        return Bitmap.createBitmap(pixels, WIDTH, height, Bitmap.Config.ARGB_8888)
    }

    /** One pass of a box of [RADIUS] along rows or columns (clamped at the edges). */
    fun boxPass(pixels: IntArray, width: Int, height: Int, horizontal: Boolean) {
        val lines = if (horizontal) height else width
        val length = if (horizontal) width else height
        val line = IntArray(length)
        for (l in 0 until lines) {
            for (i in 0 until length) line[i] = pixels[if (horizontal) l * width + i else i * width + l]
            for (i in 0 until length) {
                var a = 0
                var r = 0
                var g = 0
                var b = 0
                for (k in -RADIUS..RADIUS) {
                    val p = line[(i + k).coerceIn(0, length - 1)]
                    a += p ushr 24
                    r += (p shr 16) and 0xFF
                    g += (p shr 8) and 0xFF
                    b += p and 0xFF
                }
                val n = 2 * RADIUS + 1
                pixels[if (horizontal) l * width + i else i * width + l] = ((a / n) shl 24) or ((r / n) shl 16) or ((g / n) shl 8) or (b / n)
            }
        }
    }
}
