package com.slouchcat.app.tv.notifications

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.tv.material3.Text
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.browse.ItemRoute
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.shell.RailBadge
import com.slouchcat.app.tv.shell.SettingsEntry
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.wishes.WishesFeature
import com.slouchcat.app.tv.wishes.WishesRoute
import com.slouchcat.core.browse.notifications.NotificationsMessage
import com.slouchcat.core.browse.notifications.NotificationsViewModel
import com.slouchcat.core.browse.notifications.badge
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.NotificationTarget
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.data.NotificationsRepository
import com.slouchcat.core.strings.R
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable

/** «Benachrichtigungen» (mockup `/notifications`). */
@Serializable
data object NotificationsRoute

/**
 * Stage 5 «Benachrichtigungen» of the TV app (concept 26, 3.3: only the list; the TV has no push and
 * no channels, 4.5 and F-18): the row of «Einstellungen» (order 40, bell, «2 ungelesen»; mockup
 * `MD_SET`), the page with the open announcements («Verstanden») and the inbox of the profile, and
 * the number of unread entries and open announcements on «Einstellungen» in the rail (where the
 * mockup keeps the page). An entry leads to its title or to «Meine Wünsche». [repository]: tests
 * bring a fake.
 */
class NotificationsFeature(
    private val repository: (TvGraph) -> NotificationsRepository = { it.media.notifications },
) : TvFeature {
    override val settings = listOf(
        SettingsEntry(
            key = "notifications",
            order = ORDER,
            title = R.string.notifications_title,
            icon = SlouchcatIcon.Bell.vector,
            route = NotificationsRoute,
            subtitle = { graph -> summary(graph.media.unreadCount.count.collectAsStateWithLifecycle().value) },
            pane = { graph -> Pane(graph) },
        ),
    )

    override val badges = listOf(
        RailBadge(
            key = SETTINGS,
            count = { graph -> badgeCount(graph) },
            label = { count -> pluralCount(R.plurals.notifications_unreadLabel, count) },
        ),
    )

    override fun NavGraphBuilder.register(app: TvScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<NotificationsRoute> {
            val vm = viewModel { NotificationsViewModel(graph.session, repository(graph), graph.media.unreadCount) }
            val state by vm.state.collectAsStateWithLifecycle()
            // Back from a title: entries may have come or been read elsewhere. Not on the first
            // showing, which the model's own start reads already (code review L-7).
            var shown by rememberSaveable { mutableStateOf(false) }
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { if (shown) vm.refresh() else shown = true }
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    when (message) {
                        NotificationsMessage.Failed -> graph.toasts.show(R.string.app_state_failed)
                    }
                }
            }
            val links = remember(navigator) {
                NotificationsLinks(
                    open = { target ->
                        when (target) {
                            is NotificationTarget.Item -> if (navigator.canOpen(ItemRoute::class)) navigator.navigate(ItemRoute(target.id))
                            // Above this page (code review m-7): Back returns to the entry, as from a title.
                            NotificationTarget.Wishes -> navigator.navigate(WishesRoute(tab = WishesFeature.MINE))
                            NotificationTarget.None -> Unit
                        }
                    },
                    home = navigator.openerOf(graph.registry.home.key),
                )
            }
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            NotificationsScreen(state, images, links, vm)
        }
    }

    companion object {
        /** After «Wiedergabe» (30), before «Webradio» (50): the place `SettingsOrder` keeps for stage 5. */
        const val ORDER = 40

        /** The entry of the rail that leads to the page (mockup: «Benachrichtigungen» lives in «Einstellungen»). */
        const val SETTINGS = "settings"
    }
}

/** «2 ungelesen» or «Alles gelesen»; nothing while the number is not known. */
@Composable
private fun summary(unread: Int?): String? = when {
    unread == null -> null
    unread > 0 -> pluralCount(R.plurals.notifications_summary, unread)
    else -> stringResource(R.string.tv_notifications_allRead)
}

/**
 * The number on «Einstellungen»: read when the rail is drawn for a profile (also after the player,
 * which draws without the rail: two requests a film) and when the app comes
 * back to the front (the TV has no push), kept current by the page while it is open.
 */
@Composable
private fun badgeCount(graph: TvGraph): Int? {
    val count by graph.media.unreadCount.count.collectAsStateWithLifecycle()
    val scope = rememberCoroutineScope()
    // Also on the first drawing: a new observer gets ON_RESUME while the app is in front.
    LifecycleEventEffect(Lifecycle.Event.ON_RESUME) {
        val server = graph.session.server ?: return@LifecycleEventEffect
        scope.launch { graph.media.unreadCount.refresh { graph.media.notifications.badge(server) } }
    }
    return count
}

/** The right half of «Einstellungen» while the row has the focus (mockup `mdSetPane('notes')`). */
@Composable
private fun Pane(graph: TvGraph) {
    val unread by graph.media.unreadCount.count.collectAsStateWithLifecycle()
    Column(verticalArrangement = Arrangement.spacedBy(TvDimens.l)) {
        TvIcon(SlouchcatIcon.Bell, TvDimens.Browse.HeadIcon * 2, tint = SlouchcatTheme.colors.tx2)
        Text(summary(unread) ?: stringResource(R.string.notifications_title), style = TvType.h2)
    }
}
