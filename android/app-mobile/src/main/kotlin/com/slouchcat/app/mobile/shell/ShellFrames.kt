package com.slouchcat.app.mobile.shell

import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavDestination.Companion.hasRoute

/** The target whose first page [entry] is, `null` for any other page. */
fun ShellRegistry.targetOf(entry: NavBackStackEntry?): TopDestination? =
    entry?.let { page -> destinations.firstOrNull { page.destination.hasRoute(it.route::class) } }

/**
 * The frame for the current page [current]: no frame for the entry pages and the features'
 * full-screen routes. The marked target is the one of the current page when it is a target's
 * first page, otherwise [lastTarget] (a page counts to the target it was opened from); the marked
 * media entry is the current page's.
 */
fun ShellRegistry.frame(current: NavBackStackEntry?, lastTarget: String?, media: List<MediaEntry>): ShellFrame {
    val fullScreen = current == null || (ENTRY_ROUTES + fullScreen).any { current.destination.hasRoute(it) }
    val selectedMedia = current?.let { page ->
        media.firstOrNull { entry -> entry.isCurrent?.invoke(page) ?: page.destination.hasRoute(entry.route::class) }
    }
    return ShellFrame(
        fullScreen = fullScreen,
        targets = if (showsTargets) destinations else emptyList(),
        selectedTarget = targetOf(current)?.key ?: lastTarget,
        media = media,
        selectedMedia = selectedMedia?.key,
    )
}
