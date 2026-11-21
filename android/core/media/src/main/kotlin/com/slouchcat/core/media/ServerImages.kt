package com.slouchcat.core.media

import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.models.ImageRef
import com.slouchcat.core.api.generated.models.ItemImages
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/** One size of an image: its address on the server and its width in pixels. */
data class ImageSource(val url: String, val width: Int)

/**
 * The sizes of one image the server listed (`ItemImages`: poster in w200, w400, w800; backdrop in
 * w780, w1280, w1920, w3840), smallest first. [pick] chooses the size for the pixels a view draws.
 */
data class ImageSet(val sources: List<ImageSource> = emptyList()) {
    val isEmpty: Boolean get() = sources.isEmpty()

    /** The smallest size at least [widthPx] wide, else the largest; `null` without an image. */
    fun pick(widthPx: Int): String? =
        (sources.firstOrNull { it.width >= widthPx } ?: sources.lastOrNull())?.url

    /** [this], or [other] when this one has no image (an episode without a still shows its series). */
    fun or(other: ImageSet): ImageSet = if (isEmpty) other else this

    companion object {
        val NONE = ImageSet()
    }
}

/**
 * The addresses of the images of one server (`getItemImage`, `getPersonImage`,
 * `getCollectionImage`). The server lists every image as an `ImageRef` whose `url` is a path below
 * the API (`/api/v1/items/…/images/poster/w400?v=…&p=…`); [url] puts it under the server address,
 * a sub path included. The address carries the profile (`p`), so the image cache of the app keeps
 * the images of each profile apart, as the browser does (ADR 0005); a profile change also empties
 * the cache (concept 27, 8.4, `AppGraph`). Images load through `graph.imageLoader` over the one
 * HTTP client, with its headers and the gate of the server's key.
 */
class ServerImages(private val server: HttpUrl) {
    /** The absolute address of [ref]; `null` without one or when it names another host. */
    fun url(ref: ImageRef?): String? {
        val path = ref?.url?.takeIf { it.isNotBlank() } ?: return null
        val absolute = when {
            path.contains("://") -> path.toHttpUrlOrNull()?.takeIf { it.host == server.host && it.port == server.port }
            path.startsWith(API_PREFIX) -> (base + path.removePrefix(API_PREFIX.trimEnd('/'))).toHttpUrlOrNull()
            else -> (base + "/" + path.trimStart('/')).toHttpUrlOrNull()
        }
        return absolute?.toString()
    }

    /** A single image ([ref]) as a set of one size: a person's portrait, a collection's poster. */
    fun single(ref: ImageRef?): ImageSet = ImageSet(listOfNotNull(source(ref)))

    /** The poster sizes of a title. */
    fun posters(images: ItemImages?): ImageSet =
        set(images?.posterSmall, images?.poster, images?.posterLarge)

    /** The backdrop sizes of a title. */
    fun backdrops(images: ItemImages?): ImageSet =
        set(images?.backdropSmall, images?.backdrop, images?.backdropLarge, images?.backdropUhd)

    /** The title logo (only on the detail and the spotlight). */
    fun logo(images: ItemImages?): ImageSet = single(images?.logo)

    private val base: String = SlouchcatApi.basePath(server)

    private fun source(ref: ImageRef?): ImageSource? = ref?.let { r -> url(r)?.let { ImageSource(it, r.width) } }

    private fun set(vararg refs: ImageRef?): ImageSet =
        ImageSet(refs.mapNotNull(::source).distinctBy { it.url }.sortedBy { it.width })

    private companion object {
        const val API_PREFIX = "/${SlouchcatApi.BASE_PATH}/"
    }
}
