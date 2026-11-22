package com.slouchcat.app.tv.browse

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
