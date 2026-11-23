package com.slouchcat.app.mobile.browse

import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.media.isAudio
import kotlinx.serialization.Serializable

/*
 * The typed routes of stage 4 «Stöbern» (concept 27, 3.1) and the two later pages the start page
 * points to, in one place for every feature. A route is only an address: the page behind it is
 * registered by the feature that builds it (`composable<ItemRoute> { … }` in its `register`).
 *
 * Link to a route only through `Navigator.opener(route)`: it gives `null` while no feature
 * registered the route, so a card, a row title or a button is not a control until its page exists
 * and pages of later features hook in by themselves. Never `navigate(…)` to a route of this file
 * without that check.
 */

/** A film, series, season, episode, audiobook or podcast (`getItem`, mockup `/item/:i`). */
@Serializable
data class ItemRoute(val id: String)

/** A person with the titles the profile sees (`getPerson`, mockup `/person/:id`). */
@Serializable
data class PersonRoute(val id: Long)

/**
 * A library (`listLibraryItems`, mockup `/library/:key`). [sort]: the order it opens with, a value
 * of `LibrarySort` (`added` from the start page row «Neu in …»); `null` = the page's default.
 */
@Serializable
data class LibraryRoute(val id: Long, val sort: String? = null)

/**
 * The whole list behind a row of the start page (mockup `/list/:key`): a `HomeListKey`
 * (`continue`, `recent`, `released`, `spotlight`, `listHomeRow`) or [PICKS] (`listPicks`).
 */
@Serializable
data class ListRoute(val key: String) {
    companion object {
        /** «User-Empfehlungen» (`listPicks`). */
        const val PICKS = com.slouchcat.core.browse.lists.ListKind.PICKS
    }
}

/** One collection (mockup `/collection/:id`). */
@Serializable
data class CollectionRoute(val id: String)

/** The search (mockup `/search`); [genre] opens it filtered by a genre of the start page row «Genres». */
@Serializable
data class SearchRoute(val genre: String? = null)

/** The key of the target «Suche» of the bar: the search of a title or a library opens the target, not a page of it. */
const val SEARCH_TARGET = "search"

/** The key of the target «Mehr» of the bar: its pages (settings) open under it. */
const val MORE_TARGET = "more"

/** «Merkliste» (mockup `/watchlist`). */
@Serializable
data object WatchlistRoute

/** «Gesehen» (mockup `/watched`). */
@Serializable
data object WatchedRoute

/** «Sammlungen» (mockup `/collections`). */
@Serializable
data object CollectionsRoute

/** «Profile verwalten» (mockup `/settings/profiles`). */
@Serializable
data object ProfilesSettingsRoute

/**
 * The video player of stage 5 (mockup `/play/:i`); «Abspielen» shows only once it is registered.
 * [startAtMs]: a position to start at ([NO_POSITION]: where the server resumes); [fromStart]: «Von
 * vorn» (mockup `?from=0`). Both have defaults, so `PlayerRoute(id)` stays the plain «Abspielen».
 */
@Serializable
data class PlayerRoute(val itemId: String, val startAtMs: Long = NO_POSITION, val fromStart: Boolean = false) {
    companion object {
        /** No position of its own: the server's resume position. */
        const val NO_POSITION = -1L
    }
}

/** «Benachrichtigungen» of stage 8 (mockup `/notifications`); the bell of the start page shows only once it is registered. */
@Serializable
data object NotificationsRoute

/**
 * The audio player of stage 7 (mockup `/listen/:i`): an audiobook or a podcast episode, played by
 * the app's `AudioPlayback` (it plays on when the page goes). [startAtMs]: a chapter's start
 * ([PlayerRoute.NO_POSITION]: where the server resumes); [fromStart]: «Von vorn»; [offline]: from a
 * download of stage 6.
 */
@Serializable
data class ListenRoute(
    val itemId: String,
    val startAtMs: Long = PlayerRoute.NO_POSITION,
    val fromStart: Boolean = false,
    val offline: Boolean = false,
    /** `false`: only open the player (the mini player, «Zuletzt gehört»); nothing starts or resumes (code review M-1). */
    val play: Boolean = true,
)

/** The series of an audiobook (web `/libraries/$libraryId/series/$seriesId`). */
@Serializable
data class AudioSeriesRoute(val libraryId: Long, val seriesId: String)

/** «Hörbücher» of «Mehr»: the first audiobook library the profile sees. */
@Serializable
data object AudiobooksRoute

/** «Podcasts» of «Mehr»: the first podcast library the profile sees. */
@Serializable
data object PodcastsRoute

/** «Webradio» (mockup `/radio`). */
@Serializable
data object RadioRoute

/** The route that plays [itemId]: the audio player for audiobooks and podcast episodes, the video player else. */
fun playRoute(itemId: String, kind: ItemKind?, startAtMs: Long = PlayerRoute.NO_POSITION, fromStart: Boolean = false): Any =
    if (kind != null && isAudio(kind)) ListenRoute(itemId, startAtMs, fromStart) else PlayerRoute(itemId, startAtMs, fromStart)
