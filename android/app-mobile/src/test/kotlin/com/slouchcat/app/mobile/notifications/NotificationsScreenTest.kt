package com.slouchcat.app.mobile.notifications

import android.content.Context
import androidx.compose.material3.SnackbarHostState
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsOff
import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isToggleable
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.core.api.generated.models.Channel
import com.slouchcat.core.api.generated.models.EventType
import com.slouchcat.core.api.generated.models.Notification
import com.slouchcat.core.api.generated.models.ProfileAnnouncement
import com.slouchcat.core.browse.notifications.ChannelDraft
import com.slouchcat.core.browse.notifications.ChannelSettingsUiState
import com.slouchcat.core.browse.notifications.NotificationsUiState
import com.slouchcat.core.browse.testing.SERVER_IMAGES
import com.slouchcat.core.browse.testing.announcement
import com.slouchcat.core.browse.testing.channelSettings
import com.slouchcat.core.browse.testing.notification
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.DevicePushState
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.Paged
import com.slouchcat.core.media.PushFailure
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private const val PHONE = "w412dp-h915dp"
private const val TABLET = "w1280dp-h800dp-land"

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

internal fun inboxState() = NotificationsUiState(
    announcements = listOf(announcement(9, "Maintenance on Sunday")),
    entries = Paged(
        Loadable.Ready(
            listOf(
                notification(1),
                notification(2, Notification.Kind.WISH_AVAILABLE, item = null, wishTitle = "Resident"),
                notification(3, read = true),
            ),
        ),
        total = 3,
    ),
    unread = 2,
)

/** «Benachrichtigungen» (rule 8): two widths, 200 % font, every control labelled. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class NotificationsScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val recorder = object : NotificationsEvents {
        override fun onBack() { events += "back" }
        override fun onRetry() { events += "retry" }
        override fun onMore() { events += "more" }
        override fun onOpen(entry: Notification) { events += "open:${entry.id}" }
        override fun onMarkAllRead() { events += "readAll" }
        override fun onAcknowledge(announcement: ProfileAnnouncement) { events += "ack:${announcement.id}" }
        override val onSettings: () -> Unit = { events += "settings" }
    }

    private fun show(state: NotificationsUiState) = compose.setContent { SlouchcatTheme { NotificationsScreen(state, SERVER_IMAGES, recorder, SnackbarHostState()) } }

    private fun page() {
        show(inboxState())
        compose.onNodeWithText("Maintenance on Sunday").assertIsDisplayed()
        // An entry is one control: the screen reader hears what happened, the title and when.
        compose.onNode(hasContentDescription(s(R.string.notifications_wish_available), substring = true)).assertExists()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.announcements_acknowledge)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.notifications_markAllRead)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasContentDescription(s(R.string.notifications_entryLabelUnread, "Resident", s(R.string.notifications_wish_available), ""), substring = true) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.app_notifications_settings)) and hasClickAction())).performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("ack:9", "readAll", "open:2", "settings", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun pagePhone() = page()

    @Test @Config(qualifiers = TABLET)
    fun pageTablet() = page()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun pagePhoneLargeFont() = page()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun pageTabletLargeFont() = page()

    @Test @Config(qualifiers = PHONE)
    fun anEmptyInbox() {
        show(NotificationsUiState(entries = Paged(Loadable.Empty)))
        compose.onNodeWithText(s(R.string.notifications_empty_title)).assertIsDisplayed()
    }
}

/** «Einstellungen › Benachrichtigungen» (rule 8): the device's push, the channels, the sheet. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class ChannelSettingsScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val recorder = object : ChannelSettingsEvents {
        override fun onBack() { events += "back" }
        override fun onRetry() { events += "retry" }
        override fun onOpen(channel: Channel) { events += "open:${channel.value}" }
        override fun onClose() { events += "close" }
        override fun onDraftEnabled(on: Boolean) { events += "enabled:$on" }
        override fun onDraftEvent(event: EventType, on: Boolean) { events += "event:${event.value}:$on" }
        override fun onSave() { events += "save" }
        override fun onTest(channel: Channel) { events += "test:${channel.value}" }
        override fun onLinkTelegram() { events += "link" }
        override fun onUnlinkTelegram() { events += "unlink" }
        override fun onDevicePush(on: Boolean) { events += "push:$on" }
        override fun onAllowNotifications() { events += "allow" }
        override val onSwitchProfile: () -> Unit = { events += "profiles" }
    }

    private fun show(state: ChannelSettingsUiState, blocked: Boolean = false) =
        compose.setContent { SlouchcatTheme { ChannelSettingsScreen(state, blocked, recorder, SnackbarHostState()) } }

    private fun page() {
        show(ChannelSettingsUiState(Loadable.Ready(channelSettings()), canChange = true, device = DevicePushState.On))
        compose.onNode(hasText(s(R.string.app_notifications_push)) and isToggleable()).assertIsOn()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNode(hasText(s(R.string.app_notifications_push)) and isToggleable()).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.settings_notifications_channel_email)) and hasClickAction())).performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("push:false", "open:email", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun pagePhone() = page()

    @Test @Config(qualifiers = TABLET)
    fun pageTablet() = page()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun pagePhoneLargeFont() = page()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun pageTabletLargeFont() = page()

    @Test @Config(qualifiers = PHONE)
    fun anotherProfileOnlyReadsAndThePermissionIsMissing() {
        show(ChannelSettingsUiState(Loadable.Ready(channelSettings(pushEnabled = false)), canChange = false, device = DevicePushState.Off), blocked = true)
        compose.onNode(hasText(s(R.string.app_notifications_push)) and isToggleable()).assertIsOff()
        compose.onNodeWithText(s(R.string.settings_notifications_onlyPrimary_title)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.app_notifications_pushAllow)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.settings_notifications_onlyPrimary_action)) and hasClickAction())).performClick()
        assertEquals(listOf("allow", "profiles"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun aBuildWithoutPushSaysSo() {
        show(ChannelSettingsUiState(Loadable.Ready(channelSettings()), canChange = true, device = DevicePushState.NotInBuild))
        compose.onNodeWithText(s(R.string.app_notifications_pushNotInBuild)).assertIsDisplayed()
    }

    @Test @Config(qualifiers = PHONE)
    fun aFailedSubscriptionNamesTheRelay() {
        show(ChannelSettingsUiState(Loadable.Ready(channelSettings()), canChange = true, device = DevicePushState.Failed(PushFailure.Relay)))
        compose.reveal(compose.onNodeWithText(s(R.string.app_notifications_pushFailed_relay))).assertIsDisplayed()
    }

    private fun sheet() {
        val state = ChannelSettingsUiState(Loadable.Ready(channelSettings()), canChange = true, device = DevicePushState.On)
        show(state.copy(draft = ChannelDraft(Channel.TELEGRAM, enabled = false, events = emptySet())))
        compose.onNodeWithText(s(R.string.app_notifications_channelSheet, s(R.string.settings_notifications_channel_telegram))).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.settings_notifications_events_new_content)) and isToggleable())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.settings_notifications_telegram_link)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.common_save)) and hasClickAction())).performClick()
        assertEquals(listOf("event:new_content:true", "link", "save"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun sheetPhone() = sheet()

    @Test @Config(qualifiers = TABLET)
    fun sheetTablet() = sheet()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun sheetPhoneLargeFont() = sheet()
}
