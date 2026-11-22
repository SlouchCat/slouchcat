package com.slouchcat.app.mobile.shell

import androidx.compose.runtime.MutableState
import androidx.compose.runtime.mutableStateOf
import androidx.lifecycle.Lifecycle
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavHostController
import androidx.navigation.toRoute
import kotlinx.serialization.Serializable
import kotlin.reflect.KClass

/** The pages of the entry, drawn without the shell. */
@Serializable
data object SetupRoute

/** The sign-in; [confirmKey]: the user chose to confirm a new key of the server (section 6). */
@Serializable
data class SignInRoute(val confirmKey: Boolean = false)

@Serializable
data object ProfilesRoute

/** The routes the shell never frames: setup, sign-in, profile choice. */
val ENTRY_ROUTES = setOf(SetupRoute::class, SignInRoute::class, ProfilesRoute::class)

/**
 * The navigation of the app for pages and features: forward, back, a target of the bar, a media
 * entry, an in-app jump, and the entry pages. One instance per NavHost; [selected] is the target
 * the current page counts to (kept by the NavHost across recreation).
 */
class Navigator(
    private val nav: NavHostController,
    private val registry: ShellRegistry,
    val selected: MutableState<String?> = mutableStateOf(null),
) {
    /**
     * A page within the current target, opened by a tap (only for taps: a call from the program while
     * a page comes in is dropped too, code review L-5 of part a). Not again when it is the page shown (the
     * same route with the same arguments: a double tap, the same media entry, a repeated jump), and
     * not while the shown page is still coming in (a tap during the transition, code review M-1).
     * Another title from a title is a new page: `ItemRoute(b)` above `ItemRoute(a)`, with its own
     * ViewModel, so Back returns to the first one.
     */
    fun navigate(route: Any) {
        val top = nav.currentBackStackEntry
        if (top != null && !top.lifecycle.currentState.isAtLeast(Lifecycle.State.RESUMED)) return
        push(route)
    }

    /** [route] above the current page unless it is the page shown; for the steps of a jump, no lifecycle check. */
    private fun push(route: Any) {
        if (shows(nav.currentBackStackEntry, route)) return
        nav.navigate(route)
    }

    /** Whether [entry] is the page of [route]: same destination, equal arguments. */
    private fun shows(entry: NavBackStackEntry?, route: Any): Boolean {
        if (entry == null || !entry.destination.hasRoute(route::class)) return false
        return runCatching { entry.toRoute<Any>(route::class) == route }.getOrDefault(false)
    }

    /**
     * Whether a feature registered a page for routes of [route]'s class. Pages of a later feature
     * (the player, the detail page) appear by themselves once their feature is in `Features.kt`.
     */
    fun canOpen(route: KClass<*>): Boolean {
        @Suppress("UNCHECKED_CAST")
        val type = route as KClass<Any>
        return nav.graph.any { it.hasRoute(type) }
    }

    /**
     * The click of a link to [route]: `null` while no feature registered its page, so the card or
     * button that would open it is no control (no button that leads nowhere).
     */
    fun opener(route: Any): (() -> Unit)? = if (canOpen(route::class)) ({ navigate(route) }) else null

    /**
     * [route] as the first page of the target [key] (the search of a genre from the start page,
     * code review M-2 of part a): the target is marked, the current target keeps its pages, and the
     * target's last pages make way for the new question.
     */
    fun openTarget(key: String, route: Any) {
        val top = nav.currentBackStackEntry
        if (top != null && !top.lifecycle.currentState.isAtLeast(Lifecycle.State.RESUMED)) return
        if (registry.destination(key) == null) return
        selected.value = key
        nav.navigate(route) {
            popUpTo(registry.home.route) { saveState = true }
        }
    }

    /**
     * The click of a target of the bar from a page (the search of a title or a library): the target
     * itself, not a page of the current one (code review M-2); `null` while no feature has it.
     */
    fun openerOf(key: String): (() -> Unit)? = registry.destination(key)?.let { destination -> { openTop(destination) } }

    fun back() {
        nav.popBackStack()
    }

    /**
     * A target of the bar or the side column; each keeps its own pages (saved and restored). Tapping
     * the target shown: on its first page nothing, on a page below it back to its first page.
     */
    fun openTop(destination: TopDestination) {
        val here = registry.targetOf(nav.currentBackStackEntry, nav.previousBackStackEntry)
        if (here == destination) return
        if (selected.value == destination.key && nav.popBackStack(destination.route, inclusive = false)) return
        selected.value = destination.key
        nav.navigate(destination.route) {
            popUpTo(registry.home.route) { saveState = true }
            launchSingleTop = true
            restoreState = true
        }
    }

    /** A media entry: its page opens under its target, «Start» unless it names another (code review M-7). */
    fun openMedia(entry: MediaEntry) {
        val target = registry.destination(entry.target) ?: registry.home
        val top = nav.currentBackStackEntry
        // A tap while a page comes in, or on the entry shown: nothing (code review M-1).
        if (top != null && !top.lifecycle.currentState.isAtLeast(Lifecycle.State.RESUMED)) return
        // The entry's own test first: «Neu in Filme › Alle» is the library with its order (code review m-7 of part a).
        val current = top != null && (entry.isCurrent?.invoke(top) ?: shows(top, entry.route))
        if (selected.value == target.key && current) return
        if (selected.value != target.key || registry.targetOf(top, nav.previousBackStackEntry) != target) {
            selected.value = target.key
            nav.navigate(target.route) {
                popUpTo(registry.home.route) { saveState = true }
                launchSingleTop = true
            }
        }
        push(entry.route)
    }

    /** An in-app jump: the target, then each route down from it, so Back walks them back. */
    fun open(link: AppLink) {
        val target = registry.destination(link.target) ?: registry.home
        selected.value = target.key
        nav.navigate(target.route) {
            popUpTo(registry.home.route)
            launchSingleTop = true
        }
        link.routes.forEach(::push)
    }

    /** «Profil wechseln»: the choice above the current page, back returns. */
    fun toProfiles() = nav.navigate(ProfilesRoute) { launchSingleTop = true }

    /** The sign-in without anything behind it; [confirmKey] after the user chose to confirm a new key. */
    fun toSignIn(confirmKey: Boolean = false) = nav.navigate(SignInRoute(confirmKey)) { popUpTo(0) { inclusive = true } }

    /** «Ändern» on the sign-in: the address, back returns to the sign-in. */
    fun toSetup() = nav.navigate(SetupRoute) { launchSingleTop = true }

    companion object {
        /** The [SignInRoute] of [entry]. */
        fun signIn(entry: NavBackStackEntry): SignInRoute = entry.toRoute()
    }
}
