package com.slouchcat.app.mobile.shell

import androidx.annotation.StringRes
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavGraphBuilder
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.design.SlouchcatIcon
import kotlinx.coroutines.flow.Flow
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

/** What a target of the bar shows at its symbol (mockup: the dot at «Wünsche», the number of new notes). */
sealed interface ShellBadge {
    /** A number, «99+» above 99. */
    data class Count(val count: Int) : ShellBadge

    /** Something new, without a number. */
    data object Dot : ShellBadge
}

/**
 * A target of the bottom bar (phone) and of the side column (from 700 dp): Start, Suche,
 * Downloads, Wünsche, Mehr (concept 27, 3.2; mockup `NAV` in core.js). [order] sets the place:
 * Start 10, Suche 20, Downloads 30, Wünsche 40, Mehr 50; two targets with one order are an error.
 * [route] is the typed route of its first page; every page navigated to from there counts to this
 * target until another one is chosen. [badge]: a flow over the graph's repositories, `null` for none.
 */
data class TopDestination(
    val key: String,
    val order: Int,
    val route: Any,
    @StringRes val label: Int,
    val icon: SlouchcatIcon,
    val badge: ((AppGraph) -> Flow<ShellBadge?>)? = null,
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
    /** The number at the end of the row when it is known with the entry (titles of a library). */
    val count: Int? = null,
    /** A number that needs its own request (stations of the radio); wins over [count]. */
    val liveCount: ((AppGraph) -> Flow<Int?>)? = null,
    /**
     * The target the page opens under; `null`: the first target (Start). The mockup counts media
     * pages to «Start» (code review M-7).
     */
    val target: String? = null,
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
    /**
     * The second line. It may load what it shows; load it from a repository that keeps the value
     * (one request per visit of «Mehr» and row is the most).
     */
    val subtitle: (@Composable (AppGraph) -> String?)? = null,
)

/**
 * An in-app jump to a page (code review M-4): the target it belongs to and the routes from the
 * target's first page down, so Back returns through them. Only inside the app (a notification, a
 * page): the manifest exports no deep link, a pairing code can never arrive as a link.
 */
data class AppLink(val target: String, val routes: List<Any>)

/**
 * A part of the app that brings pages (concept 27, 3.1: one per stage or page group). It names
 * what it adds to the shell and registers its destinations in the one NavHost. A feature lives in
 * a file of its own; `Features.kt` lists it with one line (merge=union in .gitattributes).
 */
interface Feature {
    val destinations: List<TopDestination> get() = emptyList()
    val media: List<MediaEntries> get() = emptyList()
    val more: List<MoreEntry> get() = emptyList()

    /** Routes drawn without bars, side column and media choice (the player of stage 5). */
    val fullScreen: Set<KClass<*>> get() = emptySet()

    /**
     * What the shell draws above the bottom bar (phone) or at the foot of the side column (from
     * 700 dp): the small player of stage 7 (mockup `drawMini`). It draws nothing when it has nothing
     * to show. At most one feature brings one.
     */
    val accessory: (@Composable (AppScope) -> Unit)? get() = null

    /** Turns an in-app address (`item/42` of a notification) into a jump; `null` when it is not this feature's. */
    fun link(address: String): AppLink? = null

    fun NavGraphBuilder.register(app: AppScope)
}

/** What a feature's destinations get: the dependencies and the navigation of the app. */
class AppScope(val graph: AppGraph, val navigator: Navigator)

/** Everything the features add, collected once and checked. */
class ShellRegistry(val features: List<Feature>) {
    val destinations: List<TopDestination> = features.flatMap { it.destinations }.sortedBy { it.order }
    val media: List<MediaEntries> = features.flatMap { it.media }
    val more: List<MoreEntry> = features.flatMap { it.more }.sortedWith(compareBy({ it.section }, { it.order }))
    val fullScreen: Set<KClass<*>> = features.flatMapTo(mutableSetOf()) { it.fullScreen }
    val accessory: (@Composable (AppScope) -> Unit)? = features.mapNotNull { it.accessory }.let {
        require(it.size <= 1) { "two features bring an accessory" }
        it.firstOrNull()
    }

    init {
        require(destinations.isNotEmpty()) { "the app needs at least one top destination" }
        requireUnique("top destination key", destinations.map { it.key })
        requireUnique("top destination order", destinations.map { it.order })
        requireUnique("top destination route", destinations.map { it.route::class })
        requireUnique("row of «Mehr»", more.map { it.key })
        requireUnique("order of a row of «Mehr»", more.map { it.section to it.order })
    }

    /** The first target (Start once it exists): where the app lands after the profile choice. */
    val home: TopDestination get() = destinations.first()

    /** The bottom bar and the side column show their targets only when there is a choice (no bar with one target). */
    val showsTargets: Boolean get() = destinations.size > 1

    fun destination(key: String?): TopDestination? = destinations.firstOrNull { it.key == key }

    fun mediaEntries(libraries: List<LibraryNav>): List<MediaEntry> {
        val entries = media.flatMap { it.entries(libraries) }.sortedWith(compareBy({ it.section }, { it.order }))
        requireUnique("media entry", entries.map { it.key })
        return entries
    }

    /** The jump to [address], from the first feature that knows it. */
    fun link(address: String): AppLink? = features.firstNotNullOfOrNull { it.link(address) }

    private fun requireUnique(what: String, values: List<Any>) {
        val twice = values.groupingBy { it }.eachCount().filterValues { it > 1 }.keys
        require(twice.isEmpty()) { "$what twice: $twice" }
    }
}
