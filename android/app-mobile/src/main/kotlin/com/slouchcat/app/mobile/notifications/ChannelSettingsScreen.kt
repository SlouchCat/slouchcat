package com.slouchcat.app.mobile.notifications

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.core.api.generated.models.Channel
import com.slouchcat.core.api.generated.models.ChannelSetting
import com.slouchcat.core.api.generated.models.EventType
import com.slouchcat.core.browse.notifications.ChannelSettingsUiState
import com.slouchcat.core.browse.notifications.VIEWER_EVENTS
import com.slouchcat.core.design.Badge
import com.slouchcat.core.design.BadgeTone
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.ListRow
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.SwitchRow
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.DevicePushState
import com.slouchcat.core.media.PushFailure
import com.slouchcat.core.strings.R

/** What «Einstellungen › Benachrichtigungen» does on a tap. */
interface ChannelSettingsEvents {
    fun onBack()
    fun onRetry()
    fun onOpen(channel: Channel)
    fun onClose()
    fun onDraftEnabled(on: Boolean)
    fun onDraftEvent(event: EventType, on: Boolean)
    fun onSave()
    fun onTest(channel: Channel)
    fun onLinkTelegram()
    fun onUnlinkTelegram()

    /** «Push-Meldungen» of this device; the feature asks Android for the permission first. */
    fun onDevicePush(on: Boolean)

    /** Asks Android for the permission to show notifications (or opens its settings after a refusal). */
    fun onAllowNotifications()

    /** «Profil wechseln» for a profile that is not the primary one. */
    val onSwitchProfile: (() -> Unit)?
}

/**
 * «Einstellungen › Benachrichtigungen» (mockup `/settings/notifications`, reference photo handy
 * 10): «Auf diesem Gerät» with «Push-Meldungen» and what stands in its way (this build has no
 * push, the server has none, Android does not show the app's notifications, the account's push
 * channel is off); «Kanäle des Kontos» with e-mail, push and Telegram and their state; a channel
 * opens its sheet (on or off, events, test message, Telegram link). Changing a channel needs the
 * primary profile (the page says so); the device's own subscription works with every profile.
 * [blocked]: Android does not show the app's notifications now.
 */
@Composable
fun ChannelSettingsScreen(state: ChannelSettingsUiState, blocked: Boolean, events: ChannelSettingsEvents, snackbar: SnackbarHostState, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Box(modifier.fillMaxSize().background(c.bg)) {
        Column(Modifier.fillMaxSize()) {
            TopBar(stringResource(R.string.nav_notifications), navigationLabel = stringResource(R.string.common_back), onNavigation = events::onBack)
            Column(
                Modifier
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = gutter())
                    .then(if (isWideLayout()) Modifier.widthIn(max = 760.dp) else Modifier),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                Section(stringResource(R.string.app_notifications_device))
                Group { DeviceRow(state, blocked, events) }
                DeviceNote(state, blocked, events)
                Section(stringResource(R.string.app_notifications_accountChannels))
                LoadableContent(state.channels, onRetry = events::onRetry) { channels ->
                    Group {
                        channels.forEachIndexed { index, setting ->
                            if (index > 0) HorizontalDivider(color = c.line)
                            ChannelRow(setting, state.canChange) { events.onOpen(setting.channel) }
                        }
                    }
                }
                if (!state.canChange) {
                    Note(stringResource(R.string.settings_notifications_onlyPrimary_body), title = stringResource(R.string.settings_notifications_onlyPrimary_title))
                    events.onSwitchProfile?.let { SlouchcatButton(stringResource(R.string.settings_notifications_onlyPrimary_action), onClick = it, kind = ButtonKind.Secondary) }
                }
                Spacer(Modifier.height(24.dp))
            }
        }
        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter))
    }
    val draft = state.draft
    val setting = draft?.let { state.channel(it.channel) }
    if (draft != null && setting != null) ChannelSheet(state, setting, events)
}

@Composable
private fun Section(text: String) {
    Text(text.uppercase(), style = SlouchcatType.section, color = SlouchcatTheme.colors.tx2, modifier = Modifier.padding(top = 14.dp).semantics { heading() })
}

@Composable
private fun Group(content: @Composable () -> Unit) {
    val c = SlouchcatTheme.colors
    Column(Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(c.s1).border(1.dp, c.line, RoundedCornerShape(14.dp))) { content() }
}

@Composable
private fun DeviceRow(state: ChannelSettingsUiState, blocked: Boolean, events: ChannelSettingsEvents) {
    val device = state.device
    val possible = device != DevicePushState.NotInBuild && device != DevicePushState.ServerOff
    val on = (device == DevicePushState.On || device == DevicePushState.Busy || device is DevicePushState.Failed) && !blocked
    SwitchRow(
        stringResource(R.string.app_notifications_push),
        checked = on,
        onCheckedChange = events::onDevicePush,
        subtitle = if (device == DevicePushState.Busy) stringResource(R.string.app_notifications_pushBusy) else stringResource(R.string.app_notifications_pushHint),
        enabled = possible && device != DevicePushState.Busy,
    )
}

/** What stands in the way of push on this device, with the one thing the person can do about it. */
@Composable
private fun DeviceNote(state: ChannelSettingsUiState, blocked: Boolean, events: ChannelSettingsEvents) {
    when (val device = state.device) {
        DevicePushState.NotInBuild -> Note(stringResource(R.string.app_notifications_pushNotInBuild))
        DevicePushState.ServerOff -> Note(stringResource(R.string.app_notifications_pushServerOff))
        is DevicePushState.Failed -> Note(
            stringResource(
                when (device.failure) {
                    PushFailure.Offline -> R.string.app_notifications_pushFailed_offline
                    PushFailure.Relay -> R.string.app_notifications_pushFailed_relay
                    PushFailure.Google -> R.string.app_notifications_pushFailed_google
                    PushFailure.Server -> R.string.app_notifications_pushFailed_server
                },
            ),
            warning = true,
        )
        else -> Unit
    }
    if (blocked && state.device != DevicePushState.NotInBuild && state.device != DevicePushState.ServerOff) {
        Note(stringResource(R.string.app_notifications_pushBlocked), warning = true)
        SlouchcatButton(stringResource(R.string.app_notifications_pushAllow), onClick = events::onAllowNotifications, kind = ButtonKind.Secondary, leadingIcon = SlouchcatIcon.Bell)
    }
    if (state.device == DevicePushState.On && state.pushChannelOff) Note(stringResource(R.string.app_notifications_pushChannelOff), warning = true)
}

@Composable
private fun channelName(channel: Channel): String = stringResource(
    when (channel) {
        Channel.EMAIL -> R.string.settings_notifications_channel_email
        Channel.TELEGRAM -> R.string.settings_notifications_channel_telegram
        else -> R.string.app_notifications_pushChannel
    },
)

@Composable
private fun ChannelRow(setting: ChannelSetting, canChange: Boolean, onOpen: () -> Unit) {
    val name = channelName(setting.channel)
    val subtitle = when (setting.channel) {
        Channel.EMAIL -> setting.email?.address
        Channel.TELEGRAM -> stringResource(if (setting.telegram?.linked == true) R.string.settings_notifications_telegram_linked else R.string.settings_notifications_telegram_notLinked)
        else -> setting.push?.subscriptions?.size?.let { com.slouchcat.app.mobile.media.pluralCount(R.plurals.settings_notifications_push_count, it) }
    }
    val (state, tone) = when {
        !setting.available -> stringResource(R.string.settings_notifications_state_unavailable) to BadgeTone.Neutral
        setting.enabled -> stringResource(R.string.settings_notifications_state_on) to BadgeTone.Ok
        else -> stringResource(R.string.settings_notifications_state_off) to BadgeTone.Neutral
    }
    val icon = when (setting.channel) {
        Channel.EMAIL -> SlouchcatIcon.Globe
        Channel.TELEGRAM -> SlouchcatIcon.Share
        else -> SlouchcatIcon.Bell
    }
    ListRow(
        name,
        icon = icon,
        subtitle = subtitle,
        onClick = if (canChange && setting.available) onOpen else null,
        end = { Badge(state, tone = tone) },
    )
}

@Composable
private fun ChannelSheet(state: ChannelSettingsUiState, setting: ChannelSetting, events: ChannelSettingsEvents) {
    val draft = state.draft ?: return
    val name = channelName(setting.channel)
    val title = stringResource(R.string.app_notifications_channelSheet, name)
    SlouchcatSheet(title, onDismiss = events::onClose) {
        SheetTitle(title)
        SwitchRow(stringResource(R.string.settings_notifications_enabled), checked = draft.enabled, onCheckedChange = events::onDraftEnabled, subtitle = stringResource(R.string.settings_notifications_enabledHelp))
        SheetText(stringResource(R.string.settings_notifications_events_help))
        VIEWER_EVENTS.forEach { event ->
            SwitchRow(eventName(event), checked = event in draft.events, onCheckedChange = { events.onDraftEvent(event, it) })
        }
        if (setting.channel == Channel.TELEGRAM) Telegram(setting, state.linking, events)
        SheetActions {
            SlouchcatButton(stringResource(R.string.settings_notifications_test), onClick = { events.onTest(setting.channel) }, kind = ButtonKind.Secondary, busy = state.testing == setting.channel, enabled = setting.enabled, modifier = Modifier.weight(1f))
            SlouchcatButton(stringResource(R.string.common_save), onClick = events::onSave, kind = ButtonKind.Accent, busy = state.saving, modifier = Modifier.weight(1f))
        }
    }
}

@Composable
private fun Telegram(setting: ChannelSetting, linking: Boolean, events: ChannelSettingsEvents) {
    val telegram = setting.telegram ?: return
    val code = telegram.linkCode
    when {
        telegram.linked -> {
            SheetText(telegram.chatId?.let { stringResource(R.string.settings_notifications_telegram_linkedChat, it) } ?: stringResource(R.string.settings_notifications_telegram_linked))
            Box(Modifier.padding(horizontal = gutter())) { SlouchcatButton(stringResource(R.string.settings_notifications_telegram_unlink), onClick = events::onUnlinkTelegram, kind = ButtonKind.Danger, busy = linking) }
        }
        code != null -> {
            val bot = telegram.botUsername
            SheetText(if (bot != null) stringResource(R.string.settings_notifications_telegram_instruction, bot, code) else stringResource(R.string.settings_notifications_telegram_instructionNoBot, code))
            Box(Modifier.padding(horizontal = gutter())) { SlouchcatButton(stringResource(R.string.settings_notifications_telegram_renew), onClick = events::onLinkTelegram, kind = ButtonKind.Secondary, busy = linking) }
        }
        else -> Box(Modifier.padding(horizontal = gutter())) {
            SlouchcatButton(stringResource(R.string.settings_notifications_telegram_link), onClick = events::onLinkTelegram, kind = ButtonKind.Secondary, busy = linking)
        }
    }
}

@Composable
private fun eventName(event: EventType): String = stringResource(
    when (event) {
        EventType.NEW_CONTENT -> R.string.settings_notifications_events_new_content
        EventType.WISH_AVAILABLE -> R.string.settings_notifications_events_wish_available
        EventType.WISH_STATUS -> R.string.settings_notifications_events_wish_status
        EventType.ANNOUNCEMENTS -> R.string.settings_notifications_events_announcements
        EventType.INVITATIONS -> R.string.settings_notifications_events_invitations
        EventType.ERRORS -> R.string.settings_notifications_events_errors
        else -> R.string.settings_notifications_events_wish_review
    },
)
