package com.slouchcat.app.mobile.shell

import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavHostController
import androidx.navigation.toRoute
import kotlinx.serialization.Serializable

/** The pages of the entry, drawn without the shell. */
@Serializable
data object SetupRoute

/** The sign-in; [confirmKey]: the user signs in again to confirm a new key of the server (section 6). */
@Serializable
data class SignInRoute(val confirmKey: Boolean = false)

@Serializable
data object ProfilesRoute

/** The routes the shell never frames: setup, sign-in, profile choice. */
val ENTRY_ROUTES = setOf(SetupRoute::class, SignInRoute::class, ProfilesRoute::class)

/**
 * The navigation of the app for pages and features: forward, back, a target of the bar, and the
 * entry pages. One instance per NavHost.
 */
class Navigator(private val nav: NavHostController, private val registry: ShellRegistry) {
    /** A page within the current target. */
    fun navigate(route: Any) = nav.navigate(route) { launchSingleTop = true }

    fun back() {
        nav.popBackStack()
    }

    /** A target of the bar or the side column; each keeps its own pages (saved and restored). */
    fun openTop(destination: TopDestination) {
        nav.navigate(destination.route) {
            popUpTo(registry.home.route) { saveState = true }
            launchSingleTop = true
            restoreState = true
        }
    }

    /** After the profile choice: the first target, nothing behind it. */
    fun toHome() = nav.navigate(registry.home.route) { popUpTo(0) { inclusive = true } }

    /** «Profil wechseln»: the choice above the current page, back returns. */
    fun toProfiles() = nav.navigate(ProfilesRoute) { launchSingleTop = true }

    /** The profile choice as the first page (after the sign-in). */
    fun toProfilesFirst() = nav.navigate(ProfilesRoute) { popUpTo(0) { inclusive = true } }

    fun toSignIn(confirmKey: Boolean = false) = nav.navigate(SignInRoute(confirmKey)) { popUpTo(0) { inclusive = true } }

    /** «Ändern» on the sign-in: the address, back returns to the sign-in. */
    fun toSetup() = nav.navigate(SetupRoute) { launchSingleTop = true }

    companion object {
        /** The [SignInRoute] of [entry]. */
        fun signIn(entry: NavBackStackEntry): SignInRoute = entry.toRoute()
    }
}
