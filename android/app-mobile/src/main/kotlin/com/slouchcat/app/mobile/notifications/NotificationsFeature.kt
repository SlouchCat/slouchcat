package com.slouchcat.app.mobile.notifications

import android.Manifest
import android.content.Context
import android.content.Intent
import android.os.Build
import android.provider.Settings
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.MORE_TARGET
import com.slouchcat.app.mobile.browse.NotificationsRoute
import com.slouchcat.app.mobile.media.ShowMessages
import com.slouchcat.app.mobile.push.PushNotifier
import com.slouchcat.app.mobile.push.push
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.MoreEntry
import com.slouchcat.app.mobile.shell.MoreSection
import com.slouchcat.app.mobile.wishes.WishesFeature
import com.slouchcat.app.mobile.wishes.WishesRoute
import com.slouchcat.core.api.generated.models.Channel
import com.slouchcat.core.api.generated.models.EventType
import com.slouchcat.core.api.generated.models.Notification
import com.slouchcat.core.api.generated.models.ProfileAnnouncement
import com.slouchcat.core.browse.notifications.ChannelMessage
import com.slouchcat.core.browse.notifications.ChannelSettingsViewModel
import com.slouchcat.core.browse.notifications.NotificationsMessage
import com.slouchcat.core.browse.notifications.NotificationsViewModel
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.media.DevicePush
import com.slouchcat.core.media.NotificationTarget
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.data.ChannelsRepository
import com.slouchcat.core.media.data.NotificationsRepository
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable

/** «Einstellungen › Benachrichtigungen» (mockup `/settings/notifications`). */
@Serializable
data object NotificationSettingsRoute

/**
 * Stage 8 «Meldungen» (concept 27, 2.5, 3.1): «Benachrichtigungen» behind the bell of the start
 * page (the inbox of the profile and the announcements), and «Einstellungen › Benachrichtigungen»
 * as a row of «Mehr» (the channels of the account and the push of this device). [notifications],
 * [channels], [devicePush]: tests bring fakes.
 */
class NotificationsFeature(
    private val notifications: (AppGraph) -> NotificationsRepository = { it.media.notifications },
    private val channels: (AppGraph) -> ChannelsRepository = { it.media.channels },
    private val devicePush: (AppGraph) -> DevicePush? = { it.push },
    /** Whether Android shows the app's notifications; tests decide it. */
    private val allowed: (Context) -> Boolean = PushNotifier::allowed,
) : Feature {
    override val more = listOf(
        MoreEntry("notifications", MoreSection.Settings, 30, R.string.nav_notifications, SlouchcatIcon.Bell, NotificationSettingsRoute, subtitle = { stringResource(R.string.app_notifications_settingsHint) }),
    )

    /** `notifications` and `settings/notifications` (web addresses). */
    override fun link(address: String): AppLink? = when (address.removePrefix("/").substringBefore('?').trimEnd('/')) {
        "notifications" -> AppLink("start", listOf(com.slouchcat.app.mobile.home.HomeRoute, NotificationsRoute))
        "settings/notifications" -> AppLink(MORE_TARGET, listOf(com.slouchcat.app.mobile.more.MoreRoute, NotificationSettingsRoute))
        else -> null
    }

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<NotificationsRoute> {
            val vm = viewModel { NotificationsViewModel(graph.session, notifications(graph), graph.media.unreadCount) }
            val state by vm.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            ShowMessages(vm.messages, snackbar) { message ->
                when (message) {
                    NotificationsMessage.Failed -> getString(R.string.app_state_failed)
                }
            }
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { vm.refresh() }
            val images = graph.session.server?.let { remember(it) { ServerImages(it) } }
            val events = remember(vm, navigator) {
                object : NotificationsEvents {
                    override fun onBack() = navigator.back()
                    override fun onRetry() = vm.retry()
                    override fun onMore() = vm.onMore()
                    override fun onOpen(entry: Notification) {
                        when (val target = vm.onOpen(entry)) {
                            is NotificationTarget.Item -> navigator.opener(ItemRoute(target.id))?.invoke()
                            NotificationTarget.Wishes -> if (navigator.canOpen(WishesRoute::class)) navigator.openTarget(WishesFeature.TARGET, WishesRoute(tab = WishesFeature.MINE))
                            NotificationTarget.None -> Unit
                        }
                    }
                    override fun onMarkAllRead() = vm.onMarkAllRead()
                    override fun onAcknowledge(announcement: ProfileAnnouncement) = vm.onAcknowledge(announcement)
                    override val onSettings: (() -> Unit) = { navigator.navigate(NotificationSettingsRoute) }
                }
            }
            NotificationsScreen(state, images, events, snackbar)
        }
        composable<NotificationSettingsRoute> {
            val context = LocalContext.current
            val push = devicePush(graph)
            val vm = viewModel { ChannelSettingsViewModel(graph.session, channels(graph), push) }
            val state by vm.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            var blocked by remember { mutableStateOf(!allowed(context)) }
            // Back from Android's settings: the permission may have changed.
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { blocked = !allowed(context) }
            val ask = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
                blocked = !allowed(context)
                // A refusal leaves push off; the page shows how to allow it later.
                if (granted) vm.onDevicePush(true)
            }
            ShowMessages(vm.messages, snackbar) { message ->
                when (message) {
                    ChannelMessage.Saved -> getString(R.string.settings_notifications_saved)
                    ChannelMessage.TestSent -> getString(R.string.settings_notifications_testSent)
                    ChannelMessage.Unlinked -> getString(R.string.settings_notifications_telegram_unlinked)
                    is ChannelMessage.Failed -> getString(errorText(message.code))
                }
            }
            val events = remember(vm, navigator) {
                object : ChannelSettingsEvents {
                    override fun onBack() = navigator.back()
                    override fun onRetry() = vm.retry()
                    override fun onOpen(channel: Channel) = vm.onOpen(channel)
                    override fun onClose() = vm.onClose()
                    override fun onDraftEnabled(on: Boolean) = vm.onDraftEnabled(on)
                    override fun onDraftEvent(event: EventType, on: Boolean) = vm.onDraftEvent(event, on)
                    override fun onSave() = vm.onSave()
                    override fun onTest(channel: Channel) = vm.onTest(channel)
                    override fun onLinkTelegram() = vm.onLinkTelegram()
                    override fun onUnlinkTelegram() = vm.onUnlinkTelegram()
                    override fun onDevicePush(on: Boolean) {
                        // The permission in the right moment: when the person switches push on (Android 13 and up).
                        if (on && needsPermission(context)) ask.launch(Manifest.permission.POST_NOTIFICATIONS) else vm.onDevicePush(on)
                    }
                    override fun onAllowNotifications() {
                        if (needsPermission(context)) ask.launch(Manifest.permission.POST_NOTIFICATIONS) else openSystemSettings(context)
                    }
                    override val onSwitchProfile: (() -> Unit) = navigator::toProfiles
                }
            }
            ChannelSettingsScreen(state, blocked, events, snackbar)
        }
    }
}

/** Android 13 and up asks for the permission (a refused one twice: the system's settings instead). */
private fun needsPermission(context: Context): Boolean =
    Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
        context.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != android.content.pm.PackageManager.PERMISSION_GRANTED

private fun openSystemSettings(context: Context) {
    val intent = Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
    runCatching { context.startActivity(intent) }
}

private fun errorText(code: String?): Int = when (code) {
    "primary_profile_required" -> R.string.errors_code_primary_profile_required
    "channel_unavailable" -> R.string.errors_code_channel_unavailable
    "channel_disabled" -> R.string.errors_code_channel_disabled
    "event_not_allowed" -> R.string.errors_code_event_not_allowed
    "push_unavailable" -> R.string.errors_code_push_unavailable
    "telegram_unreachable" -> R.string.errors_code_telegram_unreachable
    else -> R.string.app_state_failed
}
