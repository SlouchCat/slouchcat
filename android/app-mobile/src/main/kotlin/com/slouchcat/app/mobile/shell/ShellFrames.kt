package com.slouchcat.app.mobile.shell

import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavDestination.Companion.hasRoute

/**
 * The target whose first page [entry] is, `null` for any other page. A target's route pushed as a
 * page of another target (the search from a title, code review M-2) is no first page: only the
 * bottom page ([ShellRegistry.home]) or the page right above it ([below]) counts.
 */
fun ShellRegistry.targetOf(entry: NavBackStackEntry?, below: NavBackStackEntry? = null): TopDestination? =
    entry?.let { page -> destinations.firstOrNull { page.destination.hasRoute(it.route::class) } }
        ?.takeIf { it == home || below == null || below.destination.hasRoute(home.route::class) }

/**
 * The frame for the current page [current]: no frame for the entry pages and the features'
 * full-screen routes. The marked target is the one of the current page when it is a target's
 * first page, otherwise [lastTarget] (a page counts to the target it was opened from); the marked
 * media entry is the current page's.
 */
fun ShellRegistry.frame(current: NavBackStackEntry?, lastTarget: String?, media: List<MediaEntry>, below: NavBackStackEntry? = null): ShellFrame {
    val fullScreen = current == null || (ENTRY_ROUTES + fullScreen).any { current.destination.hasRoute(it) }
    val selectedMedia = current?.let { page ->
        media.firstOrNull { entry -> entry.isCurrent?.invoke(page) ?: page.destination.hasRoute(entry.route::class) }
    }
    return ShellFrame(
        fullScreen = fullScreen,
        targets = if (showsTargets) destinations else emptyList(),
        selectedTarget = targetOf(current, below)?.key ?: lastTarget,
        media = media,
        selectedMedia = selectedMedia?.key,
    )
}
