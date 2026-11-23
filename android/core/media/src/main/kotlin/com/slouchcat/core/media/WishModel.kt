package com.slouchcat.core.media

import androidx.compose.runtime.Immutable
import com.slouchcat.core.api.generated.models.DiscoverEntry
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.ExternalTitleRef
import com.slouchcat.core.api.generated.models.Notification
import com.slouchcat.core.api.generated.models.Wish
import com.slouchcat.core.api.generated.models.WishFailReason
import com.slouchcat.core.api.generated.models.WishProgress
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.api.generated.models.WishStateDetail
import com.slouchcat.core.media.data.OPEN_WISH_STATES
import java.time.OffsetDateTime

/*
 * The cards of the wishes and the inbox as both apps draw them (concept 27 stage 8; concept 26
 * stage 5): a title outside the library, a wish of «Meine Wünsche», an entry of the inbox. Only
 * what the server said, put into the app's shapes; no rule of its own (rule 12).
 */

/**
 * A title outside the library: a tile of «Entdecken», a hit of the wishes' search or of the
 * search's «Noch nicht in der Bibliothek». [itemId]: the library title the profile sees («In der
 * Bibliothek»); [wishState]: the profile's own open wish; [requested]: Seerr knows a request.
 */
@Immutable
data class ExternalCard(
    val media: ExternalMedia,
    val tmdbId: Long,
    val title: String,
    val year: Int?,
    val posters: ImageSet,
    val itemId: String? = null,
    val wishState: WishState? = null,
    val requested: Boolean = false,
    val canRequest: Boolean = false,
) {
    /** The key in a list: a film and a series may share a TMDB id. */
    val key: String get() = "${media.value}-$tmdbId"

    companion object {
        fun from(ref: ExternalTitleRef, images: ServerImages, itemId: String? = null, requested: Boolean = false) = ExternalCard(
            media = ref.media,
            tmdbId = ref.tmdbId,
            title = ref.title,
            year = ref.year,
            posters = images.externalPosters(ref.images),
            itemId = itemId,
            wishState = ref.wish?.state,
            requested = requested,
            canRequest = ref.canRequest,
        )

        fun from(entry: DiscoverEntry, images: ServerImages) = from(entry.title, images, entry.item?.id, entry.seerrRequested)
    }
}

/** One wish of «Meine Wünsche» (`listWishes`). */
@Immutable
data class WishCard(
    val id: Long,
    val media: ExternalMedia,
    val tmdbId: Long,
    val title: String,
    val year: Int?,
    val posters: ImageSet,
    val state: WishState,
    val stateDetail: WishStateDetail? = null,
    val failReason: WishFailReason? = null,
    /** The wished seasons of a series, ascending; empty for a film. */
    val seasons: List<Int> = emptyList(),
    /** The library title once the profile may see it: «Verfügbar» leads there. */
    val itemId: String? = null,
    val canWithdraw: Boolean = false,
    /** Another profile's wish this one joined («Mitgewünscht»): withdrawing only leaves it. */
    val joined: Boolean = false,
    val createdAt: OffsetDateTime,
    val progress: WishProgress? = null,
    /** Taken over from a request made directly in Seerr. */
    val fromSeerr: Boolean = false,
) {
    val open: Boolean get() = state in OPEN_WISH_STATES

    companion object {
        fun from(wish: Wish, images: ServerImages) = WishCard(
            id = wish.id,
            media = wish.media,
            tmdbId = wish.tmdbId,
            title = wish.title,
            year = wish.year,
            posters = images.externalPosters(wish.external?.images),
            state = wish.state,
            stateDetail = wish.stateDetail,
            failReason = wish.failReason,
            seasons = wish.seasons.map { it.number }.sorted(),
            itemId = wish.item?.id,
            canWithdraw = wish.canWithdraw,
            joined = wish.role == Wish.Role.JOINED,
            createdAt = wish.createdAt,
            progress = wish.progress,
            fromSeerr = wish.origin == Wish.Origin.SEERR,
        )
    }
}

/** The steps the wish page draws (mockup `/wish/:i`): Angefragt, Genehmigt, Wird geladen, Verfügbar. */
enum class WishStep { Requested, Approved, Loading, Available }

/**
 * How far a wish is, as the four steps of the wish page: waiting for the outbox or an approval is
 * the first step, approved and on its way the third (Seerr does not tell «approved» apart from
 * «searching»), a series with some seasons playable as well, playable the last. A closed wish that
 * did not arrive has no step (`null`): the page shows its state alone.
 */
fun wishStep(state: WishState): WishStep? = when (state) {
    WishState.QUEUED, WishState.PENDING -> WishStep.Requested
    WishState.PROCESSING, WishState.PARTIAL -> WishStep.Loading
    WishState.AVAILABLE -> WishStep.Available
    else -> null
}

/** Where an entry of the inbox leads (web `entryHref`): its title, else «Meine Wünsche» for a wish, else nowhere. */
sealed interface NotificationTarget {
    data class Item(val id: String) : NotificationTarget

    data object Wishes : NotificationTarget

    data object None : NotificationTarget
}

fun Notification.target(): NotificationTarget {
    val item = item
    return when {
        item != null -> NotificationTarget.Item(item.id)
        wish != null -> NotificationTarget.Wishes
        else -> NotificationTarget.None
    }
}

/** The kinds of an entry about a wish (concept 25, M-09). */
val Notification.aboutWish: Boolean
    get() = kind in setOf(
        Notification.Kind.WISH_AVAILABLE,
        Notification.Kind.WISH_SEASON_AVAILABLE,
        Notification.Kind.WISH_APPROVED,
        Notification.Kind.WISH_DECLINED,
        Notification.Kind.WISH_FAILED,
    )
