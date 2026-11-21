package com.slouchcat.app.mobile.browse

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
        const val PICKS = "picks"
    }
}

/** One collection (mockup `/collection/:id`). */
@Serializable
data class CollectionRoute(val id: String)

/** The search (mockup `/search`); [genre] opens it filtered by a genre of the start page row «Genres». */
@Serializable
data class SearchRoute(val genre: String? = null)

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

/** The video player of stage 5 (mockup `/play/:i`); «Abspielen» shows only once it is registered. */
@Serializable
data class PlayerRoute(val itemId: String)

/** «Benachrichtigungen» of stage 8 (mockup `/notifications`); the bell of the start page shows only once it is registered. */
@Serializable
data object NotificationsRoute
