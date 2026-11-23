package com.slouchcat.app.tv.shell

import androidx.compose.runtime.MutableState
import androidx.compose.runtime.mutableStateOf
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavHostController
import androidx.navigation.toRoute
import kotlinx.serialization.Serializable
import kotlin.reflect.KClass

/** The setup: choice between the phone and by hand (mockup `/setup`). */
@Serializable
data object SetupRoute

/** Setup with the phone: the QR code (mockup `/setup/phone`). */
@Serializable
data object PairRoute

/** Setup by hand: address, e-mail, password (mockup `/setup/hand`). */
@Serializable
data object HandSetupRoute

/** «Wer schaut?» (mockup `/profiles`). */
@Serializable
data object ProfilesRoute

/** The pages the shell never frames: setup and profile choice (BAUKASTEN: `full`). */
val ENTRY_ROUTES: Set<KClass<*>> = setOf(SetupRoute::class, PairRoute::class, HandSetupRoute::class, ProfilesRoute::class)

/** Whether this page is the destination of [route]. */
fun NavBackStackEntry.isRoute(route: KClass<*>): Boolean = destination.hasRoute(route)

/** The entry of the rail [entry] counts to, or `null`; [media] are the library entries shown now. */
fun TvRegistry.entryOf(entry: NavBackStackEntry?, media: List<RailEntry> = emptyList()): RailEntry? {
    entry ?: return null
    return (rail + media).firstOrNull { it.isCurrent?.invoke(entry) ?: entry.isRoute(it.route::class) }
}

/**
 * The navigation of the TV app for pages and features: forward, back, an entry of the rail, and
 * the entry pages. One instance per NavHost; [selected] is the rail entry the current page counts
 * to (kept by the NavHost across recreation); [media] gives the library entries of the rail now.
 */
class TvNavigator(
    private val nav: NavHostController,
    private val registry: TvRegistry,
    val selected: MutableState<String?> = mutableStateOf(null),
    private val media: () -> List<RailEntry> = { emptyList() },
    /** Opens the menu «Bibliotheken» of the shell (all libraries, «Meine Listen»). */
    val openLibraries: () -> Unit = {},
) {
    /** The entry of the rail the current page counts to (also a library). */
    fun current(): RailEntry? = registry.entryOf(nav.currentBackStackEntry, media())

    /** A page below the current one; Back returns, and the page before keeps focus and scroll. */
    fun navigate(route: Any) {
        // Not again when it is the page shown (a double OK); another title from a title is a new
        // page above it (`ItemRoute(b)` over `ItemRoute(a)`), so Back returns to the first one.
        val top = nav.currentBackStackEntry
        if (top != null && shows(top, route)) return
        // Title → person → the same title (code review m-8): back to the page it already has
        // instead of a stack without end.
        if (has(route) && nav.popBackStack(route, inclusive = false)) return
        nav.navigate(route)
    }

    /**
     * [route] instead of the page shown (a title outside the library that is in the library
     * meanwhile): Back then skips the replaced page.
     */
    fun replace(route: Any) {
        val top = nav.currentBackStackEntry ?: return navigate(route)
        nav.navigate(route) { popUpTo(top.destination.id) { inclusive = true } }
    }

    /** Whether a page of [route] (equal arguments) is in the stack. */
    private fun has(route: Any): Boolean = runCatching { nav.getBackStackEntry(route) }.isSuccess

    /** The rail entry whose first page is not its own route (a genre search), until the entry opens plain. */
    private var foreign: String? = null

    /** Whether [entry] is the page of [route]: same destination, equal arguments. */
    private fun shows(entry: NavBackStackEntry, route: Any): Boolean =
        entry.isRoute(route::class) && runCatching { entry.toRoute<Any>(route::class) == route }.getOrDefault(false)

    /**
     * [route] as the first page of the rail entry [key] (the search of a genre from the start
     * page, code review M-2, as the phone app's `openTarget`): the entry is marked, the current
     * entry keeps its pages, and the entry's earlier pages make way for the new question.
     */
    fun openTarget(key: String, route: Any) {
        if (registry.entry(key) == null) return
        selected.value = key
        foreign = key
        nav.navigate(route) {
            popUpTo(registry.home.route) { saveState = true }
        }
    }

    /**
     * Whether a feature registered a page for routes of [route]'s class. Pages of a later stage
     * (the player of stage 3) appear by themselves once their feature is in `Features.kt`.
     */
    fun canOpen(route: KClass<*>): Boolean {
        @Suppress("UNCHECKED_CAST")
        val type = route as KClass<Any>
        return nav.graph.any { it.hasRoute(type) }
    }

    /**
     * The click of a link to [route]: `null` while no feature registered its page, so the button
     * that would open it is not drawn (no button that leads nowhere, no stand-in player).
     */
    fun opener(route: Any): (() -> Unit)? = if (canOpen(route::class)) ({ navigate(route) }) else null

    /**
     * An entry of the rail by its key from a page (the search of a title, a library of the start
     * page); `null` while no feature brings it.
     */
    fun openerOf(key: String): (() -> Unit)? = (registry.entry(key) ?: media().firstOrNull { it.key == key })?.let { entry -> { openRail(entry) } }

    /** Back one page; `false` on the first page (the shell then counts towards leaving the app). */
    fun back(): Boolean = nav.popBackStack()

    /**
     * An entry of the rail; each keeps its own pages. On its own first page nothing happens, below
     * it back to its first page.
     */
    fun openRail(entry: RailEntry) {
        val here = current()
        val top = nav.currentBackStackEntry
        // On its own first page nothing happens; on a first page with other arguments (the search
        // of a genre) the entry opens plain.
        if (here?.key == entry.key && top != null && shows(top, entry.route)) return
        if (selected.value == entry.key && has(entry.route) && nav.popBackStack(entry.route, inclusive = false)) return
        selected.value = entry.key
        // All libraries share one route class: their pages are neither kept nor restored by class
        // (library 8 must not get the pages of library 7), and the same class is no reason to stay.
        val ownClass = entry.section != RailSection.Media
        // The pages of an entry whose first page is not its own (a genre search) are not kept: the
        // entry would come back with them (code review M-2).
        val keep = here != null && here.section != RailSection.Media && here.key != foreign
        if (here?.key == foreign || entry.key == foreign) foreign = null
        nav.navigate(entry.route) {
            popUpTo(registry.home.route) { saveState = keep }
            launchSingleTop = ownClass && keep
            restoreState = ownClass
        }
    }

    /**
     * «Profil wechseln»: the choice above the current page; Back returns. Lies the choice just
     * below (opened «Profile verwalten» from there), back to it instead of a second one (m-2).
     */
    fun toProfiles() {
        if (nav.previousBackStackEntry?.isRoute(ProfilesRoute::class) == true) {
            nav.popBackStack()
            return
        }
        nav.navigate(ProfilesRoute) { launchSingleTop = true }
    }

    /** A setup page from the choice. */
    fun toSetup(route: Any) = nav.navigate(route) { launchSingleTop = true }
}
