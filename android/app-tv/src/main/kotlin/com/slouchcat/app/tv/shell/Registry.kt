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
    /** Whether [entry] is this entry's page (marks it); the default compares the route class. */
    val isCurrent: ((NavBackStackEntry) -> Boolean)? = null,
)

/** Makes media entries of the rail from the libraries the active profile sees (stage 2b, 4). */
fun interface RailMedia {
    fun entries(libraries: List<LibraryNav>): List<RailEntry>
}

/**
 * A row of the page «Einstellungen» (mockup `/settings`, `MD_SET`): the shell brings «Profil
 * wechseln», «Server und Konto», «Über» and «Erweitert»; later stages add theirs (Profile
 * verwalten 2b, Wiedergabe 3, Webradio 4, Benachrichtigungen 5). [order]: Profil wechseln 10,
 * Profile verwalten 20, Wiedergabe 30, Benachrichtigungen 40, Webradio 50, Server 60, Über 70,
 * Erweitert 80. [pane] draws the right half while the row has the focus.
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

/**
 * A part of the TV app that brings pages (one or more per stage). It names what it adds to the
 * rail and to the settings and registers its destinations in the one NavHost. A feature lives in a
 * file of its own; `Features.kt` lists it with one line (merge=union in .gitattributes).
 */
interface TvFeature {
    val rail: List<RailEntry> get() = emptyList()
    val media: List<RailMedia> get() = emptyList()
    val settings: List<SettingsEntry> get() = emptyList()

    /** Routes drawn without the rail and the clock (the player of stage 3). */
    val fullScreen: Set<KClass<*>> get() = emptySet()

    /** The page «Profile verwalten» (stage 2b): the profile choice offers it once it exists. */
    val manageProfiles: Any? get() = null

    fun NavGraphBuilder.register(app: TvScope)
}

/** What a feature's destinations get: the dependencies and the navigation of the app. */
class TvScope(val graph: TvGraph, val navigator: TvNavigator)

/** Everything the features add, collected once and checked. */
class TvRegistry(val features: List<TvFeature>) {
    val rail: List<RailEntry> = features.flatMap { it.rail }.sortedWith(compareBy({ it.section }, { it.order }))
    val media: List<RailMedia> = features.flatMap { it.media }
    val settings: List<SettingsEntry> = features.flatMap { it.settings }.sortedBy { it.order }
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
        requireUnique("settings row", settings.map { it.key })
        requireUnique("settings order", settings.map { it.order })
    }

    /** Where the app lands after the profile choice: Start once it exists, else the first entry. */
    val home: RailEntry get() = rail.firstOrNull { it.section == RailSection.Top } ?: rail.first()

    fun entry(key: String?): RailEntry? = rail.firstOrNull { it.key == key }

    fun mediaEntries(libraries: List<LibraryNav>): List<RailEntry> {
        val entries = media.flatMap { it.entries(libraries) }.sortedBy { it.order }
        requireUnique("media entry", entries.map { it.key })
        return entries
    }

    private fun requireUnique(what: String, values: List<Any>) {
        val twice = values.groupingBy { it }.eachCount().filterValues { it > 1 }.keys
        require(twice.isEmpty()) { "$what twice: $twice" }
    }
}
