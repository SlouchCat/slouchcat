package com.slouchcat.app.tv.browse

import com.slouchcat.core.api.generated.models.ItemKind
import kotlinx.serialization.Serializable

/*
 * The typed routes of stage 2b «Stöbern» (concept 26, 3.3) and the player of stage 3, in one place
 * for every feature. A route is only an address: the page behind it is registered by the feature
 * that builds it (`composable<ItemRoute> { … }` in its `register`).
 *
 * Link to a route only through `TvNavigator.opener(route)`: it gives `null` while no feature
 * registered the route, so a card or a button is no control until its page exists, and pages of
 * later stages (the player of stage 3) hook in by themselves.
 */

/** A film, series, season or episode (`getItem`, mockup `/item/:i`). */
@Serializable
data class ItemRoute(val id: String)

/** A person with the titles the profile sees (`getPerson`, mockup `/person/:id`). */
@Serializable
data class PersonRoute(val id: Long)

/** A library (`listLibraryItems`, mockup `/library/:key`); [sort]: a `LibrarySort` value to open with. */
@Serializable
data class LibraryRoute(val id: Long, val sort: String? = null)

/**
 * The whole list behind a row of the start page (mockup `/list/:key`): a `HomeListKey`
 * (`continue`, `recent`, `released`, `spotlight`) or [PICKS] (`listPicks`).
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

/** «Sammlungen» (mockup `/collections`). */
@Serializable
data object CollectionsRoute

/** The search (mockup `/search`); [genre] opens it filtered by a genre of the start page. */
@Serializable
data class SearchRoute(val genre: String? = null)

/** «Merkliste» (mockup `/watchlist`). */
@Serializable
data object WatchlistRoute

/** «Gesehen» (mockup `/watched`). */
@Serializable
data object WatchedRoute

/** «Profile verwalten» (mockup `/settings/profiles`). */
@Serializable
data object ProfilesManageRoute

/**
 * The video player of stage 3 (mockup `/play/:i`): «Abspielen» shows only once a feature registers
 * it. [itemId]: the film or the episode; [startAtMs]: a position to start at ([NO_POSITION]: where
 * the server resumes); [fromStart]: «Von vorn». Same shape as the phone app's `PlayerRoute`.
 */
@Serializable
data class PlayerRoute(val itemId: String, val startAtMs: Long = NO_POSITION, val fromStart: Boolean = false) {
    companion object {
        /** No position of its own: the server's resume position. */
        const val NO_POSITION = -1L
    }
}

/**
 * The audio player of stage 4 (concept 26 5.6; mockup `/listen/:i`): an audiobook or a podcast
 * episode; [startAtMs] a chapter's start ([PlayerRoute.NO_POSITION]: where the server resumes),
 * [fromStart] «Von vorn». The title that plays already plays on when the page opens again; [show]
 * only shows what plays, paused or not («Läuft gerade» of the rail).
 */
@Serializable
data class ListenRoute(val itemId: String, val startAtMs: Long = PlayerRoute.NO_POSITION, val fromStart: Boolean = false, val show: Boolean = false)

/** The series of an audiobook (web `/libraries/$libraryId/series/$seriesId`). */
@Serializable
data class AudioSeriesRoute(val libraryId: Long, val seriesId: String)

/** «Webradio» (mockup `/radio`): countries, stations, «Meine Sender», the search. */
@Serializable
data object RadioRoute

/** The station that plays, as a page of its own without the rail (the audio player of a station). */
@Serializable
data object RadioListenRoute

/**
 * Where «Abspielen» of a title leads (stage 4): an audiobook or an episode to the audio player, a
 * podcast to its page (the episodes; its big button plays the newest), every other title to the
 * video player. Open it through `TvNavigator.opener`, which gives `null` while its page is missing.
 */
fun playRoute(itemId: String, kind: ItemKind?, startAtMs: Long = PlayerRoute.NO_POSITION, fromStart: Boolean = false): Any = when (kind) {
    ItemKind.AUDIOBOOK, ItemKind.PODCAST_EPISODE -> ListenRoute(itemId, startAtMs, fromStart)
    ItemKind.PODCAST -> ItemRoute(itemId)
    else -> PlayerRoute(itemId, startAtMs, fromStart)
}

/**
 * «Abspielen» of a title on a card or a stage ([playRoute]): the video player, the audio player or
 * the podcast's page, else the title's page when the audio player is not registered; `null` while
 * the video player is not registered (no button, as before stage 4).
 */
fun com.slouchcat.app.tv.shell.TvNavigator.player(): ((itemId: String, kind: ItemKind) -> Unit)? =
    if (canOpen(PlayerRoute::class)) ({ id, kind -> (opener(playRoute(id, kind)) ?: opener(ItemRoute(id)))?.invoke() }) else null
