package com.slouchcat.app.tv.playback

import android.content.Context
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.tv.player.SoundOutput
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.PlaybackPrefs
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.api.map
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.toLoadable
import com.slouchcat.core.player.AudioSetting
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.DataStoreDeviceSettingsStore
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DeviceSettings
import com.slouchcat.core.player.DeviceSettingsStore
import com.slouchcat.core.player.PlaybackPrefsRepository
import com.slouchcat.core.player.QualityChoice
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

/**
 * The device settings of the TV: no cap on the quality before the user chose one (a TV plays 4K
 * files directly; the phone's defaults are 720p on mobile data, 1080p on Wi-Fi).
 */
fun tvDeviceSettings(context: Context): DeviceSettingsStore =
    DataStoreDeviceSettingsStore(context, DeviceSettings(qualityMobile = QualityChoice.Auto, qualityWifi = QualityChoice.Auto))

/** The choice menus of the page. */
enum class PlaybackMenu { Quality, AudioLanguage, SubtitleMode, SubtitleLanguage, Channels }

/**
 * The language preferences of the active profile in their order (the phone app's `LanguagePrefs`:
 * the contract types the lists as sets, and sets in another order are equal).
 */
data class LanguagePrefs(val audio: List<String>, val mode: SubtitleMode, val subtitles: List<String>) {
    fun toContract() = PlaybackPrefs(audio.toCollection(LinkedHashSet()), mode, subtitles.toCollection(LinkedHashSet()))

    companion object {
        fun of(prefs: PlaybackPrefs) = LanguagePrefs(prefs.audioLanguages.toList(), prefs.subtitleMode, prefs.subtitleLanguages.toList())
    }
}

data class PlaybackSettingsUiState(
    val device: DeviceSettings = DeviceSettings(),
    /** The language preferences of the active profile on the server. */
    val prefs: Loadable<LanguagePrefs> = Loadable.Loading,
    /** What the TV found at its output (Automatisch); `null` while it reads. */
    val detected: SoundOutput? = null,
    /** The formats the TV decodes itself apart from Dolby and DTS (they stay in «Von Hand» as they are). */
    val decoded: List<String> = emptyList(),
    /** What «Automatisch» reports (decoders and passthrough): «Von Hand» starts with it. */
    val found: List<String> = emptyList(),
    val menu: PlaybackMenu? = null,
)

sealed interface PlaybackMessage {
    data object Saved : PlaybackMessage
    data object Failed : PlaybackMessage
}

/**
 * «Einstellungen › Wiedergabe» of the TV (concept 26 3.3, 5.3; mockup `/settings/playback`,
 * photo `38-settings-playback`): the quality, the flow of series and the sound for this TV
 * (`DeviceSettingsStore`, as on the phone), the languages and the subtitle rule of the active
 * profile on the server (`getMyPlaybackPrefs`, `putMyPlaybackPrefs`). The sound: automatic (what
 * the output reports), always stereo, or formats and channels by hand (concept 26 5.3, ADR 0041).
 */
class PlaybackSettingsViewModel(
    private val store: DeviceSettingsStore,
    private val repository: PlaybackPrefsRepository,
    /** The facts of the TV (read off the main thread by the caller). */
    private val facts: suspend () -> DeviceFacts,
) : ViewModel() {
    private val _state = MutableStateFlow(PlaybackSettingsUiState())
    val state: StateFlow<PlaybackSettingsUiState> = _state.asStateFlow()

    private val _messages = Channel<PlaybackMessage>(Channel.BUFFERED)
    val messages: Flow<PlaybackMessage> = _messages.receiveAsFlow()

    /** The last preferences the server confirmed; a refusal goes back to them. */
    private var confirmed: LanguagePrefs? = null

    /** One save at a time, in the order of the choices (review m-7 of K27 stage 5). */
    private val saving = Mutex()

    init {
        viewModelScope.launch { store.settings.collect { device -> _state.update { it.copy(device = device) } } }
        viewModelScope.launch {
            val now = facts()
            _state.update {
                it.copy(
                    detected = SoundOutput.of(now),
                    decoded = CapabilityRules.decodedAudio(now).filter { c -> c !in CapabilityRules.PASSTHROUGH },
                    found = CapabilityRules.build(now, AudioSetting.Auto).audio.orEmpty(),
                )
            }
        }
        load()
    }

    fun retry() = load()

    private fun load() {
        _state.update { it.copy(prefs = Loadable.Loading) }
        viewModelScope.launch {
            val loaded = repository.prefs().map(LanguagePrefs::of).toLoadable()
            confirmed = (loaded as? Loadable.Ready)?.value
            _state.update { it.copy(prefs = loaded) }
        }
    }

    fun onMenu(menu: PlaybackMenu?) = _state.update { it.copy(menu = menu) }

    private fun device(change: (DeviceSettings) -> DeviceSettings) {
        _state.update { it.copy(menu = null) }
        viewModelScope.launch { store.update(change) }
    }

    /** A TV has one network: the choice goes for both. */
    fun onQuality(choice: QualityChoice) = device { it.copy(qualityMobile = choice, qualityWifi = choice) }
    fun onAutoplay(on: Boolean) = device { it.copy(autoplayNext = on) }
    fun onSkipIntro(on: Boolean) = device { it.copy(offerSkipIntro = on) }

    fun onSoundAuto() = device { it.copy(audio = AudioSetting.Auto) }
    fun onSoundStereo() = device { it.copy(audio = AudioSetting.Stereo) }

    /**
     * «Formate von Hand», filled with what «Automatisch» reports: every decoder (Dolby too, when the
     * TV decodes it) and the formats it passes through (K26 stage 0 point 2; review m-7).
     */
    fun onSoundManual() {
        val s = _state.value
        if (s.device.audio is AudioSetting.Manual) return
        val channels = s.detected?.channels ?: STEREO
        device { it.copy(audio = AudioSetting.Manual((s.decoded + s.found).toSet(), channels)) }
    }

    /** A format of «Von Hand» on or off; the formats the TV decodes stay. */
    fun onFormat(format: String, on: Boolean) = device { d ->
        val manual = d.audio as? AudioSetting.Manual ?: return@device d
        d.copy(audio = manual.copy(formats = (if (on) manual.formats + format else manual.formats - format) + _state.value.decoded))
    }

    fun onChannels(channels: Int) = device { d ->
        val manual = d.audio as? AudioSetting.Manual ?: return@device d
        d.copy(audio = manual.copy(channels = channels))
    }

    fun onAudioLanguage(code: String?) = prefs { p -> p.copy(audio = if (code == null) emptyList() else withFirst(p.audio, code)) }

    fun onSubtitleMode(mode: SubtitleMode) = prefs { it.copy(mode = mode) }

    fun onSubtitleLanguage(code: String?) = prefs { p -> p.copy(subtitles = if (code == null) emptyList() else withFirst(p.subtitles, code)) }

    /** Saves [change] of the profile's preferences: shown at once, back to the server's on a refusal. */
    private fun prefs(change: (LanguagePrefs) -> LanguagePrefs) {
        val before = (_state.value.prefs as? Loadable.Ready)?.value ?: return
        val after = change(before)
        if (confirmed == null) confirmed = before
        _state.update { it.copy(menu = null, prefs = Loadable.Ready(after)) }
        if (after == before) return
        viewModelScope.launch {
            saving.withLock {
                // A newer choice is shown by now: only the latest one goes to the server.
                val wanted = (_state.value.prefs as? Loadable.Ready)?.value ?: return@withLock
                if (wanted == confirmed) return@withLock
                when (val saved = repository.save(wanted.toContract())) {
                    is ApiResult.Ok -> {
                        val stored = LanguagePrefs.of(saved.value)
                        confirmed = stored
                        _state.update { s -> if ((s.prefs as? Loadable.Ready)?.value == wanted) s.copy(prefs = Loadable.Ready(stored)) else s }
                        _messages.send(PlaybackMessage.Saved)
                    }
                    else -> {
                        _state.update { s -> if ((s.prefs as? Loadable.Ready)?.value == wanted) s.copy(prefs = Loadable.Ready(confirmed ?: before)) else s }
                        _messages.send(PlaybackMessage.Failed)
                    }
                }
            }
        }
    }

    companion object {
        /** The length limit of a language list (contract `PlaybackLanguages`). */
        const val MAX_LANGUAGES = 10
        const val STEREO = 2

        /** The list with [code] in the first place (web `withFirst`). */
        fun withFirst(list: List<String>, code: String): List<String> = (listOf(code) + list.filter { it != code }).take(MAX_LANGUAGES)
    }
}
