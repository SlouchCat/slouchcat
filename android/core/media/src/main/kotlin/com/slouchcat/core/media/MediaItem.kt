package com.slouchcat.core.media

import androidx.compose.runtime.Immutable
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.Progress

/** The form of a card (mockup `card(i, {wide, sq})`). */
enum class CardShape {
    /** 2:3, films and series. */
    Poster,

    /** 1:1, audiobooks and podcasts. */
    Square,

    /** 16:9 with the backdrop, the play mark and the progress bar («Weiterschauen»). */
    Wide,
}

/**
 * A title as every card, row and grid of the app shows it: the shared model of the media building
 * blocks, made from an `ItemRef` of any list or from an `ItemDetail` ([from]). The server decided
 * what is visible (rules 2, 12); this model only carries what it sent.
 */
@Immutable
data class MediaItem(
    val id: String,
    val kind: ItemKind,
    val title: String,
    val year: Int? = null,
    /** Author of an audiobook or podcast: their caption instead of the year. */
    val author: String? = null,
    /** Effective minimum age; `null` when unknown. */
    val rating: Int? = null,
    val runtimeMinutes: Int? = null,
    val poster: ImageSet = ImageSet.NONE,
    val backdrop: ImageSet = ImageSet.NONE,
    val logo: ImageSet = ImageSet.NONE,
    /** Played share 0..1 of a started title (a position above 0), else `null`. */
    val progress: Float? = null,
    /** Minutes left of a started title, at least 1. */
    val remainingMinutes: Int? = null,
    /** Finished: a series or podcast with every episode watched, any other title with `watchedAt`. */
    val watched: Boolean = false,
    val inWatchlist: Boolean = false,
    /** The series of an episode: its card shows the series' title and poster («Weiterschauen»). */
    val seriesTitle: String? = null,
    val seasonNumber: Int? = null,
    val episodeNumber: Int? = null,
    val episodeEnd: Int? = null,
    /** The main user's note on a curated spotlight title. */
    val note: String? = null,
) {
    /** Audiobooks, podcasts and their episodes: square covers, the author as caption. */
    val isAudio: Boolean get() = kind == ItemKind.AUDIOBOOK || kind == ItemKind.PODCAST || kind == ItemKind.PODCAST_EPISODE

    /** The shape a card of this title takes when the page does not choose. */
    val defaultShape: CardShape get() = if (isAudio) CardShape.Square else CardShape.Poster

    companion object {
        /** A card from any list of the server. */
        fun from(item: ItemRef, images: ServerImages): MediaItem {
            val series = item.seriesRef
            val started = item.progress?.takeIf { it.positionMs > 0 }
            return MediaItem(
                id = item.id,
                kind = item.kind,
                title = item.title,
                year = item.year,
                author = item.author,
                rating = item.rating,
                runtimeMinutes = item.runtimeMinutes,
                // An episode has no poster of its own: its card shows the series' (contract 1.14.0).
                poster = images.posters(item.images).or(images.posters(series?.images)),
                backdrop = images.backdrops(item.images).or(images.backdrops(series?.images)),
                logo = images.logo(item.images),
                progress = started?.let(::share),
                remainingMinutes = started?.let(::minutesLeft),
                watched = finished(item.kind, item.watched, item.progress),
                inWatchlist = item.inWatchlist,
                seriesTitle = series?.title ?: item.seriesTitle,
                seasonNumber = item.seasonNumber,
                episodeNumber = item.episodeNumber,
                episodeEnd = item.episodeEnd,
                note = item.note,
            )
        }

        /** A card of the title of a detail page (`getItem`). */
        fun from(item: ItemDetail, images: ServerImages): MediaItem {
            val started = item.progress?.takeIf { it.positionMs > 0 }
            return MediaItem(
                id = item.id,
                kind = item.kind,
                title = item.title,
                year = item.year,
                author = item.author,
                rating = item.rating,
                runtimeMinutes = item.runtimeMinutes,
                poster = images.posters(item.images).or(images.posters(item.seriesRef?.images)),
                backdrop = images.backdrops(item.images).or(images.backdrops(item.seriesRef?.images)),
                logo = images.logo(item.images),
                progress = started?.let(::share),
                remainingMinutes = started?.let(::minutesLeft),
                watched = finished(item.kind, item.watched, item.progress),
                inWatchlist = item.inWatchlist,
                seriesTitle = item.seriesRef?.title ?: item.seriesTitle,
                seasonNumber = item.seasonNumber,
                episodeNumber = item.episodeNumber,
                episodeEnd = item.episodeEnd,
                note = item.note,
            )
        }

        /** The check mark of the web app (`isFinished`): series and podcasts by `watched`, others by `watchedAt`. */
        private fun finished(kind: ItemKind, watched: Boolean?, progress: Progress?): Boolean =
            if (kind == ItemKind.SERIES || kind == ItemKind.PODCAST) watched == true else progress?.watchedAt != null
    }
}
