package com.slouchcat.app.mobile.shell

import androidx.annotation.StringRes
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavGraphBuilder
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.design.SlouchcatIcon
import kotlin.reflect.KClass

/** A text that is either a string resource or a value from the server (a library's name). */
sealed interface Label {
    data class Res(@StringRes val id: Int) : Label

    data class Plain(val text: String) : Label

    @Composable
    fun text(): String = when (this) {
        is Res -> stringResource(id)
        is Plain -> text
    }
}

/**
 * A target of the bottom bar (phone) and of the side column (from 700 dp): Start, Suche,
 * Downloads, Wünsche, Mehr (concept 27, 3.2; mockup `NAV` in core.js). [order] sets the place:
 * Start 10, Suche 20, Downloads 30, Wünsche 40, Mehr 50. [route] is the typed route of its first
 * page; every page navigated to from there counts to this target until another one is chosen.
 */
data class TopDestination(
    val key: String,
    val order: Int,
    val route: Any,
    @StringRes val label: Int,
    val icon: SlouchcatIcon,
)

/** The two groups of the media choice (BAUKASTEN «Medienauswahl links»). */
enum class MediaSection {
    /** «Medien»: the libraries of the profile, Webradio. */
    Media,

    /** «Meine Medien»: Merkliste, Gesehen, Sammlungen. */
    MyMedia,
}

/** One entry of the media choice: drawer on the phone, fixed in the side column from 700 dp. */
data class MediaEntry(
    val key: String,
    val section: MediaSection,
    val order: Int,
    val label: Label,
    val icon: SlouchcatIcon,
    val route: Any,
    /** The number at the end of the row (titles, stations); `null` shows none. */
    val count: Int? = null,
    /**
     * Whether [entry] is the page of this entry (marks it). The default compares the route class;
     * a route with arguments (a library id) checks them (`entry.toRoute<LibraryRoute>().id == id`).
     */
    val isCurrent: ((NavBackStackEntry) -> Boolean)? = null,
)

/**
 * Makes media entries from the libraries the active profile sees (`listVisibleLibraries`). A
 * provider that needs no libraries ignores them (Webradio, Merkliste).
 */
fun interface MediaEntries {
    fun entries(libraries: List<LibraryNav>): List<MediaEntry>
}

/** The groups of the page «Mehr», in the order of the mockup. */
enum class MoreSection { Library, Devices, Settings }

/** One row of «Mehr» (mockup `/more`). */
data class MoreEntry(
    val key: String,
    val section: MoreSection,
    val order: Int,
    @StringRes val title: Int,
    val icon: SlouchcatIcon,
    val route: Any,
    /** The second line; it may load what it shows (`produceState` over the graph's repositories). */
    val subtitle: (@Composable (AppGraph) -> String?)? = null,
)

/**
 * A part of the app that brings pages (concept 27, 3.1: one per stage or page group). It names
 * what it adds to the shell and registers its destinations in the one NavHost. Add a feature by
 * adding it to [features] in `Features.kt`; the shell never changes for it.
 *
 * ```
 * object SearchFeature : Feature {
 *     override val destinations = listOf(TopDestination("search", 20, SearchRoute, R.string.nav_search, SlouchcatIcon.Search))
 *     override fun NavGraphBuilder.register(app: AppScope) {
 *         composable<SearchRoute> { … SearchScreen(…) }
 *     }
 * }
 * ```
 */
interface Feature {
    val destinations: List<TopDestination> get() = emptyList()
    val media: List<MediaEntries> get() = emptyList()
    val more: List<MoreEntry> get() = emptyList()

    /** Routes drawn without bars, side column and media choice (the player of stage 5). */
    val fullScreen: Set<KClass<*>> get() = emptySet()

    fun NavGraphBuilder.register(app: AppScope)
}

/** What a feature's destinations get: the dependencies and the navigation of the app. */
class AppScope(val graph: AppGraph, val navigator: Navigator)

/** Everything the features add, collected once. */
class ShellRegistry(val features: List<Feature>) {
    val destinations: List<TopDestination> = features.flatMap { it.destinations }.sortedBy { it.order }
    val media: List<MediaEntries> = features.flatMap { it.media }
    val more: List<MoreEntry> = features.flatMap { it.more }.sortedWith(compareBy({ it.section }, { it.order }))
    val fullScreen: Set<KClass<*>> = features.flatMapTo(mutableSetOf()) { it.fullScreen }

    init {
        require(destinations.map { it.key }.toSet().size == destinations.size) { "two top destinations with one key" }
        require(destinations.isNotEmpty()) { "the app needs at least one top destination" }
    }

    /** The first target (Start once it exists): where the app lands after the profile choice. */
    val home: TopDestination get() = destinations.first()

    /** The bottom bar and the side column show their targets only when there is a choice (no bar with one target). */
    val showsTargets: Boolean get() = destinations.size > 1

    fun mediaEntries(libraries: List<LibraryNav>): List<MediaEntry> =
        media.flatMap { it.entries(libraries) }.sortedWith(compareBy({ it.section }, { it.order }))
}
