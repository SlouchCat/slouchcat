package com.slouchcat.app.mobile.playback

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.PlaybackPrefs
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.api.map
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.toLoadable
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

/** The choice sheets of the page (mockup `suChoice`) and the question before forgetting the series. */
enum class PlaybackSheet { QualityMobile, QualityWifi, AudioLanguage, SubtitleMode, SubtitleLanguage, ResetSeries }

/**
 * The language preferences of the active profile in their order. The contract types the lists as
 * sets, and two sets with the same languages in another order are equal: a state of sets would
 * swallow a new first language.
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
    val sheet: PlaybackSheet? = null,
    val resetting: Boolean = false,
)

sealed interface PlaybackMessage {
    data object Saved : PlaybackMessage
    data object Failed : PlaybackMessage
    data object SeriesReset : PlaybackMessage
}

interface PlaybackSettingsEvents {
    fun onBack()
    fun onRetry()
    fun onSheet(sheet: PlaybackSheet?)
    fun onQualityMobile(choice: QualityChoice)
    fun onQualityWifi(choice: QualityChoice)
    fun onAutoplay(on: Boolean)
    fun onSkipIntro(on: Boolean)
    fun onWifiOnly(on: Boolean)

    /** `null`: the original language (no audio language). */
    fun onAudioLanguage(code: String?)
    fun onSubtitleMode(mode: SubtitleMode)

    /** `null`: no subtitle language. */
    fun onSubtitleLanguage(code: String?)
    fun onResetSeries()
}

/**
 * «Einstellungen › Wiedergabe» (mockup `/settings/playback`; web «Einstellungen»): the quality per
 * network, the flow and the downloads for this device, the languages and the subtitle rule of the
 * active profile on the server (`getMyPlaybackPrefs`, `putMyPlaybackPrefs`, `resetMySeriesTracks`).
 * A choice saves at once; a refusal puts the old value back.
 */
class PlaybackSettingsViewModel(
    private val store: DeviceSettingsStore,
    private val repository: PlaybackPrefsRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(PlaybackSettingsUiState())
    val state: StateFlow<PlaybackSettingsUiState> = _state.asStateFlow()

    private val _messages = Channel<PlaybackMessage>(Channel.BUFFERED)
    val messages: Flow<PlaybackMessage> = _messages.receiveAsFlow()

    init {
        viewModelScope.launch { store.settings.collect { device -> _state.update { it.copy(device = device) } } }
        load()
    }

    fun retry() = load()

    private fun load() {
        _state.update { it.copy(prefs = Loadable.Loading) }
        viewModelScope.launch {
            val loaded = repository.prefs().map(LanguagePrefs::of).toLoadable()
            _state.update { it.copy(prefs = loaded) }
        }
    }

    fun onSheet(sheet: PlaybackSheet?) = _state.update { it.copy(sheet = sheet) }

    private fun device(change: (DeviceSettings) -> DeviceSettings) {
        _state.update { it.copy(sheet = null) }
        viewModelScope.launch { store.update(change) }
    }

    fun onQualityMobile(choice: QualityChoice) = device { it.copy(qualityMobile = choice) }
    fun onQualityWifi(choice: QualityChoice) = device { it.copy(qualityWifi = choice) }
    fun onAutoplay(on: Boolean) = device { it.copy(autoplayNext = on) }
    fun onSkipIntro(on: Boolean) = device { it.copy(offerSkipIntro = on) }
    fun onWifiOnly(on: Boolean) = device { it.copy(downloadsWifiOnly = on) }

    fun onAudioLanguage(code: String?) = prefs { p -> p.copy(audio = if (code == null) emptyList() else withFirst(p.audio, code)) }

    fun onSubtitleMode(mode: SubtitleMode) = prefs { it.copy(mode = mode) }

    fun onSubtitleLanguage(code: String?) = prefs { p -> p.copy(subtitles = if (code == null) emptyList() else withFirst(p.subtitles, code)) }

    /** Saves [change] of the profile's preferences; the page shows it at once and goes back on a refusal. */
    private fun prefs(change: (LanguagePrefs) -> LanguagePrefs) {
        val before = (_state.value.prefs as? Loadable.Ready)?.value ?: return
        val after = change(before)
        _state.update { it.copy(sheet = null, prefs = Loadable.Ready(after)) }
        if (after == before) return
        viewModelScope.launch {
            when (val saved = repository.save(after.toContract())) {
                is ApiResult.Ok -> {
                    _state.update { it.copy(prefs = Loadable.Ready(LanguagePrefs.of(saved.value))) }
                    _messages.send(PlaybackMessage.Saved)
                }
                else -> {
                    _state.update { it.copy(prefs = Loadable.Ready(before)) }
                    _messages.send(PlaybackMessage.Failed)
                }
            }
        }
    }

    fun onResetSeries() {
        _state.update { it.copy(sheet = null, resetting = true) }
        viewModelScope.launch {
            val done = repository.resetSeries() is ApiResult.Ok
            _state.update { it.copy(resetting = false) }
            _messages.send(if (done) PlaybackMessage.SeriesReset else PlaybackMessage.Failed)
        }
    }

    companion object {
        /** The length limit of a language list (contract `PlaybackLanguages`). */
        const val MAX_LANGUAGES = 10

        /** The list with [code] in the first place (web `withFirst`). */
        fun withFirst(list: List<String>, code: String): List<String> = (listOf(code) + list.filter { it != code }).take(MAX_LANGUAGES)
    }
}
