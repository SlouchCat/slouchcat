package com.slouchcat.app.mobile.shell

import androidx.compose.runtime.MutableState
import androidx.compose.runtime.mutableStateOf
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
    /** A page within the current target. */
    fun navigate(route: Any) = nav.navigate(route) { launchSingleTop = true }

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

    fun back() {
        nav.popBackStack()
    }

    /**
     * A target of the bar or the side column; each keeps its own pages (saved and restored). Tapping
     * the target shown: on its first page nothing, on a page below it back to its first page.
     */
    fun openTop(destination: TopDestination) {
        val here = registry.targetOf(nav.currentBackStackEntry)
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
        if (selected.value != target.key || registry.targetOf(nav.currentBackStackEntry) != target) {
            selected.value = target.key
            nav.navigate(target.route) {
                popUpTo(registry.home.route) { saveState = true }
                launchSingleTop = true
            }
        }
        navigate(entry.route)
    }

    /** An in-app jump: the target, then each route down from it, so Back walks them back. */
    fun open(link: AppLink) {
        val target = registry.destination(link.target) ?: registry.home
        selected.value = target.key
        nav.navigate(target.route) {
            popUpTo(registry.home.route)
            launchSingleTop = true
        }
        link.routes.forEach(::navigate)
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
