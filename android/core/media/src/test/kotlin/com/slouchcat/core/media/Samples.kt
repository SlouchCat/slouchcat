package com.slouchcat.core.media

import com.slouchcat.core.api.generated.models.ImageRef
import com.slouchcat.core.api.generated.models.ItemImages
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.Progress
import java.math.BigDecimal
import java.time.OffsetDateTime

/* Sample data of the tests of this module (the apps keep their own fakes next to their pages). */

/** A ULID-shaped id from a short number (the server's ids are ULIDs). */
fun ulid(n: Int): String = "01J0000000000000000000" + n.toString().padStart(4, '0')

/** An image of the server at [path] (below the API) in [width]. */
fun image(path: String, width: Int, profile: Long = 1): ImageRef =
    ImageRef("/api/v1/$path?v=hash&p=$profile", width, width * 3 / 2)

/** The three poster sizes and a backdrop of [id] as the server lists them. */
fun posters(id: String, profile: Long = 1) = ItemImages(
    posterSmall = image("items/$id/images/poster/w200", 200, profile),
    poster = image("items/$id/images/poster/w400", 400, profile),
    posterLarge = image("items/$id/images/poster/w800", 800, profile),
    backdrop = image("items/$id/images/backdrop/w1280", 1280, profile),
)

fun itemRef(
    n: Int,
    title: String = "Title $n",
    kind: ItemKind = ItemKind.MOVIE,
    images: ItemImages = ItemImages(),
    progress: Progress? = null,
    watched: Boolean? = null,
    watchedCount: Int? = null,
    childCount: Int? = null,
) = ItemRef(
    id = ulid(n),
    kind = kind,
    title = title,
    inWatchlist = false,
    images = images,
    year = 2000 + n % 25,
    progress = progress,
    watched = watched,
    watchedCount = watchedCount,
    childCount = childCount,
)

/** A progress at [percent] of a title of [minutes]. */
fun progress(percent: Int, minutes: Int = 100, watched: Boolean = false) = Progress(
    positionMs = minutes * 60_000L * percent / 100,
    durationMs = minutes * 60_000L,
    percent = BigDecimal(percent),
    completed = percent >= 90,
    updatedAt = OffsetDateTime.parse("2026-11-20T20:00:00Z"),
    watchedAt = if (watched) OffsetDateTime.parse("2026-11-20T21:00:00Z") else null,
)
