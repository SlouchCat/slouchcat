package com.slouchcat.core.browse.notifications

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Channel
import com.slouchcat.core.api.generated.models.ChannelSetting
import com.slouchcat.core.api.generated.models.ChannelSettingUpdate
import com.slouchcat.core.api.generated.models.EventType
import com.slouchcat.core.api.map
import com.slouchcat.core.media.DevicePush
import com.slouchcat.core.media.DevicePushState
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.data.ChannelsRepository
import com.slouchcat.core.media.toLoadable
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.channels.Channel as MessageChannel

/** The events a viewer chooses per channel (`errors` and `wish_review` are the administrators'; they stay as they are). */
val VIEWER_EVENTS: List<EventType> = listOf(
    EventType.NEW_CONTENT,
    EventType.WISH_AVAILABLE,
    EventType.WISH_STATUS,
    EventType.ANNOUNCEMENTS,
    EventType.INVITATIONS,
)

/** The channel being changed in its sheet: on or off, and its events. */
data class ChannelDraft(val channel: Channel, val enabled: Boolean, val events: Set<EventType>)

/**
 * «Einstellungen › Benachrichtigungen». [canChange]: the session has no profile or the primary one
 * (else the page explains `primary_profile_required` and changes nothing); [device]: this device's
 * push subscription.
 */
data class ChannelSettingsUiState(
    val channels: Loadable<List<ChannelSetting>> = Loadable.Loading,
    val canChange: Boolean = false,
    val device: DevicePushState = DevicePushState.NotInBuild,
    val draft: ChannelDraft? = null,
    val saving: Boolean = false,
    /** The channel whose test message is on its way. */
    val testing: Channel? = null,
    val linking: Boolean = false,
) {
    fun channel(channel: Channel): ChannelSetting? = (channels as? Loadable.Ready)?.value?.firstOrNull { it.channel == channel }

    /** The account's push channel is off: subscriptions get nothing until the primary profile turns it on. */
    val pushChannelOff: Boolean get() = channel(Channel.PUSH)?.let { it.available && !it.enabled } == true
}

sealed interface ChannelMessage {
    data object Saved : ChannelMessage

    data object TestSent : ChannelMessage

    data object Unlinked : ChannelMessage

    /** [code]: the problem's code (`primary_profile_required`, `channel_disabled` …). */
    data class Failed(val code: String?) : ChannelMessage
}

/**
 * The channels of the account (mockup `/settings/notifications`, web `ChannelSettingsPage`):
 * e-mail, push and Telegram with «Aktiv», their events, a test message and the Telegram link;
 * changing them needs the primary profile (concept 27, 2.5). Next to them «Push-Meldungen» of this
 * device ([devicePush]; `null` in a build without push). The server decides who gets which message
 * (rule 12); the page only shows and changes the switches.
 */
class ChannelSettingsViewModel(
    private val session: SessionManager,
    private val repository: ChannelsRepository,
    private val devicePush: DevicePush?,
) : ViewModel() {
    private val local = MutableStateFlow(ChannelSettingsUiState(canChange = canChange()))

    val state: StateFlow<ChannelSettingsUiState> =
        combine(local, devicePush?.state ?: flowOf(DevicePushState.NotInBuild)) { own, device -> own.copy(device = device) }
            .stateIn(viewModelScope, SharingStarted.Eagerly, local.value)

    private val _messages = MessageChannel<ChannelMessage>(MessageChannel.BUFFERED)
    val messages: Flow<ChannelMessage> = _messages.receiveAsFlow()

    init {
        load()
        devicePush?.let { push -> viewModelScope.launch { push.refresh() } }
    }

    private fun canChange(): Boolean = when (val s = session.state.value) {
        is SessionState.Ready -> s.profile.isPrimary
        is SessionState.ChooseProfile -> true
        else -> false
    }

    private fun load() {
        val server = session.server ?: run {
            local.update { it.copy(channels = Loadable.Failed(LoadError.Offline)) }
            return
        }
        local.update { it.copy(channels = Loadable.Loading, canChange = canChange()) }
        viewModelScope.launch {
            val result = repository.channels(server)
            local.update { it.copy(channels = result.map { s -> s.channels }.toLoadable { l -> l.isEmpty() }) }
        }
    }

    fun retry() = load()

    /** Opens the sheet of [channel] (only with the primary profile; else the page shows why not). */
    fun onOpen(channel: Channel) {
        val setting = local.value.channel(channel) ?: return
        if (!local.value.canChange || !setting.available) return
        local.update { it.copy(draft = ChannelDraft(channel, setting.enabled, setting.events)) }
    }

    fun onClose() = local.update { if (it.saving) it else it.copy(draft = null) }

    fun onDraftEnabled(on: Boolean) = local.update { it.copy(draft = it.draft?.copy(enabled = on)) }

    fun onDraftEvent(event: EventType, on: Boolean) = local.update { now ->
        now.copy(draft = now.draft?.let { d -> d.copy(events = if (on) d.events + event else d.events - event) })
    }

    fun onSave() {
        val draft = local.value.draft ?: return
        val server = session.server ?: return
        if (local.value.saving) return
        local.update { it.copy(saving = true) }
        viewModelScope.launch {
            when (val result = repository.update(server, draft.channel, ChannelSettingUpdate(draft.enabled, draft.events))) {
                is ApiResult.Ok -> {
                    replace(result.value)
                    local.update { it.copy(saving = false, draft = null) }
                    _messages.send(ChannelMessage.Saved)
                    if (draft.channel == Channel.PUSH) devicePush?.refresh()
                }
                else -> {
                    local.update { it.copy(saving = false) }
                    _messages.send(ChannelMessage.Failed(codeOf(result)))
                }
            }
        }
    }

    fun onTest(channel: Channel) {
        val server = session.server ?: return
        if (local.value.testing != null) return
        local.update { it.copy(testing = channel) }
        viewModelScope.launch {
            val result = repository.test(server, channel)
            local.update { it.copy(testing = null) }
            _messages.send(if (result is ApiResult.Ok) ChannelMessage.TestSent else ChannelMessage.Failed(codeOf(result)))
        }
    }

    /** A new Telegram link code (an older one stops working). */
    fun onLinkTelegram() = telegram { server ->
        when (val result = repository.linkTelegram(server)) {
            is ApiResult.Ok -> replace(result.value)
            else -> _messages.send(ChannelMessage.Failed(codeOf(result)))
        }
    }

    fun onUnlinkTelegram() = telegram { server ->
        when (val result = repository.unlinkTelegram(server)) {
            is ApiResult.Ok -> {
                _messages.send(ChannelMessage.Unlinked)
                (repository.channels(server) as? ApiResult.Ok)?.let { all -> local.update { it.copy(channels = Loadable.Ready(all.value.channels)) } }
            }
            else -> _messages.send(ChannelMessage.Failed(codeOf(result)))
        }
    }

    private fun telegram(block: suspend (okhttp3.HttpUrl) -> Unit) {
        val server = session.server ?: return
        if (local.value.linking || !local.value.canChange) return
        local.update { it.copy(linking = true) }
        viewModelScope.launch {
            try {
                block(server)
            } finally {
                local.update { it.copy(linking = false) }
            }
        }
    }

    /** «Push-Meldungen» of this device. The app asks for the system's permission before [on] is `true`. */
    fun onDevicePush(on: Boolean) {
        val push = devicePush ?: return
        viewModelScope.launch { if (on) push.enable() else push.disable() }
    }

    private fun replace(setting: ChannelSetting) = local.update { now ->
        val list = (now.channels as? Loadable.Ready)?.value ?: return@update now
        now.copy(channels = Loadable.Ready(list.map { if (it.channel == setting.channel) setting else it }))
    }

    private fun codeOf(result: ApiResult<*>): String? = when (result) {
        is ApiResult.Failed -> result.code
        is ApiResult.NotAllowed -> result.code
        else -> null
    }
}
