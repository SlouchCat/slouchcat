package com.slouchcat.app.tv.shell

import androidx.annotation.StringRes
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavGraphBuilder
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.core.api.generated.models.LibraryNav
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

/** The groups of the rail, top to bottom (mockup `drawRail`). */
enum class RailSection {
    /** Start, Suche. */
    Top,

    /** «Medien»: every library and the radio as entries of their own. */
    Media,

    /** Wünsche, Einstellungen (below a gap). */
    Bottom,
}

/**
 * An entry of the left rail (concept 26, 3.4; mockup `NAV` and `drawRail` in core.js). [order]
 * sets the place within its section: Start 10, Suche 20 (Top); Wünsche 40, Einstellungen 50
 * (Bottom). [route] is the typed route of its page; every page navigated to from there counts to
 * this entry until another is chosen. The profile at the foot of the rail belongs to the shell.
 */
data class RailEntry(
    val key: String,
    val section: RailSection,
    val order: Int,
    val label: Label,
    val icon: ImageVector,
    val route: Any,
    /** The titles of a library, shown in the menu «Bibliotheken». */
    val count: Int? = null,
    /**
     * Whether [entry] is this entry's page or a page below it (marks it); the default compares the
     * route class. Media entries must bring it: all libraries share one route class.
     */
    val isCurrent: ((NavBackStackEntry) -> Boolean)? = null,
)

/**
 * Makes media entries of the rail from the libraries the active profile sees (stage 2b, 4): one
 * entry per library, section [RailSection.Media], each with its own `isCurrent` (for example
 * `{ it.isRoute(LibraryRoute::class) && it.toRoute<LibraryRoute>().id == library.id }` plus the pages
 * of that library), so a library opened from Start or Suche is marked as well.
 */
fun interface RailMedia {
    fun entries(libraries: List<LibraryNav>): List<RailEntry>
}

/**
 * A number on an entry of the rail (stage 5: the unread notifications on «Einstellungen», where
 * the mockup keeps «Benachrichtigungen»). [key] names the entry, which may belong to another
 * feature; [count] reads the number while the rail is shown (`null` or 0: no badge) and keeps it
 * current itself (it is drawn once per session epoch). [label] is what a screen reader adds to the
 * entry's name.
 */
class RailBadge(
    val key: String,
    val count: @Composable (TvGraph) -> Int?,
    val label: @Composable (Int) -> String,
)

/**
 * A row of the page «Einstellungen» (mockup `/settings`, `MD_SET`): the shell brings «Profil
 * wechseln», «Server und Konto», «Über» and «Erweitert»; later stages add theirs (Profile
 * verwalten 2b, Wiedergabe 3, Webradio 4, Benachrichtigungen 5). [order]: Profil wechseln 10,
 * Profile verwalten 20, Wiedergabe 30, Benachrichtigungen 40, Webradio 50, Server 60, Über 70,
 * Erweitert 80 ([SettingsOrder], [BUILT_IN_SETTINGS]). [pane] draws the right half while the row
 * has the focus.
 */
data class SettingsEntry(
    val key: String,
    val order: Int,
    @StringRes val title: Int,
    val icon: ImageVector,
    val route: Any,
    val subtitle: (@Composable (TvGraph) -> String?)? = null,
    val pane: (@Composable (TvGraph) -> Unit)? = null,
)

/** The places of the rows the shell brings itself on «Einstellungen». */
object SettingsOrder {
    const val SWITCH_PROFILE = 10
    const val SERVER = 60
    const val ABOUT = 70
    const val ADVANCED = 80
}

/** Keys and places of the shell's own settings rows: a feature takes neither (review M-2). */
val BUILT_IN_SETTINGS: Map<String, Int> = mapOf(
    "profile" to SettingsOrder.SWITCH_PROFILE,
    "server" to SettingsOrder.SERVER,
    "about" to SettingsOrder.ABOUT,
    "advanced" to SettingsOrder.ADVANCED,
)

/**
 * A part of the TV app that brings pages (one or more per stage). It names what it adds to the
 * rail and to the settings and registers its destinations in the one NavHost. A feature lives in a
 * file of its own; `Features.kt` lists it with one line (merge=union in .gitattributes).
 */
interface TvFeature {
    val rail: List<RailEntry> get() = emptyList()
    val media: List<RailMedia> get() = emptyList()
    val settings: List<SettingsEntry> get() = emptyList()

    /** Numbers on entries of the rail (stage 5). */
    val badges: List<RailBadge> get() = emptyList()

    /** Routes drawn without the rail and the clock (the player of stage 3). */
    val fullScreen: Set<KClass<*>> get() = emptySet()

    /**
     * «Meine Listen» of the menu «Bibliotheken» (Merkliste, Gesehen, Sammlungen): pages reached
     * from there and from the rows of Start, not from the rail (code review M-3, mockup).
     */
    val lists: List<RailEntry> get() = emptyList()

    /** The page «Profile verwalten» (stage 2b): the profile choice offers it once it exists. */
    val manageProfiles: Any? get() = null

    fun NavGraphBuilder.register(app: TvScope)
}

/** What a feature's destinations get: the dependencies and the navigation of the app. */
class TvScope(val graph: TvGraph, val navigator: TvNavigator)

/**
 * Everything the features add, collected once and checked: no key or place twice (also against
 * the shell's own settings rows). `RegistryTest` builds it from the real list `features`, so a
 * clash of two stages fails in the CI and not at the start of the app.
 */
class TvRegistry(val features: List<TvFeature>) {
    val rail: List<RailEntry> = features.flatMap { it.rail }.sortedWith(compareBy({ it.section }, { it.order }))
    val media: List<RailMedia> = features.flatMap { it.media }
    val lists: List<RailEntry> = features.flatMap { it.lists }.sortedBy { it.order }
    val settings: List<SettingsEntry> = features.flatMap { it.settings }.sortedBy { it.order }
    val badges: Map<String, RailBadge> = features.flatMap { it.badges }.associateBy { it.key }
    val fullScreen: Set<KClass<*>> = features.flatMapTo(mutableSetOf()) { it.fullScreen }
    val manageProfiles: Any? = features.mapNotNull { it.manageProfiles }.let {
        require(it.size <= 1) { "two features bring «Profile verwalten»" }
        it.firstOrNull()
    }

    init {
        require(rail.isNotEmpty()) { "the TV app needs at least one rail entry" }
        requireUnique("rail entry key", rail.map { it.key })
        requireUnique("rail entry order", rail.map { it.section to it.order })
        requireUnique("rail entry route", rail.map { it.route::class })
        requireUnique("list entry key", lists.map { it.key } + rail.map { it.key })
        requireUnique("settings row", settings.map { it.key } + BUILT_IN_SETTINGS.keys)
        requireUnique("settings order", settings.map { it.order } + BUILT_IN_SETTINGS.values)
        require(rail.none { it.section == RailSection.Media }) { "media entries come from RailMedia, not from rail" }
        requireUnique("rail badge", features.flatMap { it.badges }.map { it.key })
        // A badge on an entry nobody brings would never show (the entry of another stage missing).
        require(badges.keys.all { key -> rail.any { it.key == key } }) { "rail badge without its entry: ${badges.keys - rail.map { it.key }.toSet()}" }
    }

    /** Where the app lands after the profile choice: Start once it exists, else the first entry. */
    val home: RailEntry get() = rail.firstOrNull { it.section == RailSection.Top } ?: rail.first()

    fun entry(key: String?): RailEntry? = rail.firstOrNull { it.key == key }

    fun mediaEntries(libraries: List<LibraryNav>): List<RailEntry> {
        val entries = media.flatMap { it.entries(libraries) }.sortedBy { it.order }
        requireUnique("media entry", entries.map { it.key } + rail.map { it.key })
        require(entries.all { it.section == RailSection.Media && it.isCurrent != null }) {
            "a media entry needs section Media and its own isCurrent: ${entries.filter { it.isCurrent == null }.map { it.key }}"
        }
        return entries
    }

    private fun requireUnique(what: String, values: List<Any>) {
        val twice = values.groupingBy { it }.eachCount().filterValues { it > 1 }.keys
        require(twice.isEmpty()) { "$what twice: $twice" }
    }
}
