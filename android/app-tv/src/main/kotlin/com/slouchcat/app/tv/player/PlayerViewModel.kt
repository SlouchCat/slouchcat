package com.slouchcat.app.tv.player

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.Quality
import com.slouchcat.core.api.generated.models.Track
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.media.data.ItemRepository
import com.slouchcat.core.media.playTarget
import com.slouchcat.core.player.AudioSetting
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DeviceSettings
import com.slouchcat.core.player.MeteredNetwork
import com.slouchcat.core.player.PlaybackController
import com.slouchcat.core.player.PlaybackServer
import com.slouchcat.core.player.PlayerEngine
import com.slouchcat.core.player.PlayerError
import com.slouchcat.core.player.PlayerState
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.FlowPreview
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.debounce
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import okhttp3.HttpUrl

/** What the page tells the player (the remote, the menus, the error pictures). */
interface PlayerEvents {
    fun onBack()
    fun onTogglePlay()
    fun onPlay()
    fun onPause()
    fun onSeekTo(positionMs: Long)
    fun onSpeed(speed: Float)
    fun onAudio(track: Track)
    fun onSubtitle(track: Track?)
    fun onQuality(quality: Quality)
    fun onSkipIntro()
    fun onSkipOutro()
    fun onPrevious()
    fun onNext()
    fun onEpisode(id: String)
    fun onCancelCountdown()
    fun onRetry()
    fun onEpisodesOpened()
    fun onSeason(seasonId: String)
    fun onSoundSettings()
}

/**
 * The seasons of the menu «Folgen» (mockup `brEpsMenu`): the chips of the series and the episodes
 * of the season shown; the episodes of the playing season come from the controller.
 */
data class SeasonsMenu(
    val seasons: List<ItemRef> = emptyList(),
    /** The season whose episodes the menu shows; `null`: the playing one. */
    val shown: String? = null,
    /** Episodes of other seasons by season id: `null` while loading. */
    val episodes: Map<String, List<ItemRef>?> = emptyMap(),
)

/**
 * What the sound does now: [output] is what the TV reported at the last start (formats passed
 * through, channels), [setting] the choice of the settings, [passthrough] what Media3 does with
 * the playing track (`true`: the encoded stream goes to the sound system), `null` before it said.
 */
data class SoundState(val output: SoundOutput? = null, val setting: AudioSetting = AudioSetting.Auto, val passthrough: Boolean? = null)

/**
 * One player page of the TV (concept 26 5.1): a [PlaybackController] of `:core:player` over
 * [engine] for the title of the route. A series or a season (the stage of Start opens it with
 * its own id) plays the episode `getItem(…).nextUp` names. The page reports the TV's
 * capabilities with the sound setting of the device; when an output comes or goes, it reads them
 * again and the server decides again (rule 12: no decision of its own). [end] stops the playback
 * from outside (a profile change, a sign-out); it ends with the page at the latest.
 */
@OptIn(FlowPreview::class)
class PlayerViewModel(
    private val itemId: String,
    private val startAtMs: Long?,
    private val fromStart: Boolean,
    val engine: PlayerEngine,
    private val server: PlaybackServer,
    private val items: ItemRepository,
    private val serverUrl: () -> HttpUrl?,
    /** The facts of the TV now (decoders, display, output); read before every start. */
    private val facts: suspend () -> DeviceFacts,
    private val settings: Flow<DeviceSettings>,
    outputChanges: Flow<Unit>,
    metered: MeteredNetwork,
    finalScope: CoroutineScope,
    clock: () -> Long,
    private val saved: SavedStateHandle? = null,
    /** Where the playback runs; `null`: the ViewModel's scope (tests give the background scope of the test). */
    runIn: CoroutineScope? = null,
) : ViewModel(), PlayerEvents {
    private val scope: CoroutineScope = runIn ?: viewModelScope

    private val reportedOutput = MutableStateFlow<SoundOutput?>(null)
    private val passthrough = MutableStateFlow<Boolean?>(null)

    private val controller = PlaybackController(
        scope = scope,
        finalScope = finalScope,
        server = server,
        engine = engine,
        capabilities = { narrowing ->
            val now = facts()
            reportedOutput.value = SoundOutput.of(now)
            passthrough.value = null
            CapabilityRules.build(now, settings.first().audio, narrowing)
        },
        settings = settings,
        metered = metered,
        clock = clock,
    )

    /** An error before the controller starts (the series has no episode, the server is gone). */
    private val resolveError = MutableStateFlow<PlayerError?>(null)

    val state: StateFlow<PlayerState> = combine(controller.state, resolveError) { state, error ->
        if (error != null) state.copy(error = error, loading = false) else state
    }.stateIn(scope, SharingStarted.Eagerly, PlayerState())

    private val _seasons = MutableStateFlow(SeasonsMenu())
    val seasons: StateFlow<SeasonsMenu> = _seasons.asStateFlow()

    val sound: StateFlow<SoundState> = combine(reportedOutput, settings, passthrough) { output, device, through ->
        SoundState(output, device.audio, through)
    }.stateIn(scope, SharingStarted.Eagerly, SoundState())

    /** Set by the page: Zurück and the error pictures leave through the navigation. */
    var leave: () -> Unit = {}

    /** Set by the page: «Ton-Einstellungen» of the menu «Ton». */
    var openSoundSettings: () -> Unit = {}

    private var ended = false

    init {
        begin()
        scope.launch {
            outputChanges.debounce(OUTPUT_SETTLE_MS).collect { outputChanged() }
        }
    }

    private fun begin() {
        resolveError.value = null
        // After the process died the route still names its first position (`Von vorn` = 0); the
        // server has the progress since then, so a restored page starts where the server resumes.
        val restored = saved?.get<Boolean>(STARTED) == true
        scope.launch {
            val target = resolve() ?: return@launch
            saved?.set(STARTED, true)
            if (restored) controller.start(target) else controller.start(target, startAtMs, fromStart)
        }
    }

    /** The title to play: the route's, or the episode `nextUp` names for a series or a season. */
    private suspend fun resolve(): String? {
        val url = serverUrl() ?: return itemId
        return when (val answer = items.item(url, itemId)) {
            is ApiResult.Ok -> {
                val detail = answer.value
                if (detail.kind != ItemKind.SERIES && detail.kind != ItemKind.SEASON) return itemId
                playTarget(detail)?.id ?: null.also { resolveError.value = PlayerError.NotFound }
            }
            ApiResult.NotFound -> null.also { resolveError.value = PlayerError.NotFound }
            ApiResult.Offline -> null.also { resolveError.value = PlayerError.Network(retrying = false) }
            ApiResult.SignedOut -> null.also { resolveError.value = PlayerError.SignedOut }
            // Anything else: the controller's start tells.
            else -> itemId
        }
    }

    /** An output came or went: when what the TV reports changed, a new start where it stands. */
    private suspend fun outputChanged() {
        val before = reportedOutput.value ?: return
        val now = SoundOutput.of(facts())
        val setting = settings.first().audio
        if (reported(setting, now) == reported(setting, before)) {
            reportedOutput.value = now
            return
        }
        Log.i(TAG, "audio output changed", "channels" to now.channels, "passthrough" to now.passthrough.joinToString(","))
        controller.capabilitiesChanged()
    }

    /** Media3 opened its sound output: [encoded] `true` when the stream goes out as it is (AC-3, E-AC-3, DTS, TrueHD). */
    fun onAudioOutput(encoded: Boolean) {
        passthrough.value = encoded
    }

    /** The app went to the background (ON_STOP: Home, screen saver, standby) and came back (ON_START). */
    fun onBackground() = controller.moveToBackground()

    fun onForeground() = controller.resumeFromBackground()

    /** Ends the playback now (a profile change, a sign-out): the last progress and the stop at the server. */
    fun end() {
        if (ended) return
        ended = true
        controller.close()
    }

    override fun onCleared() = end()

    override fun onBack() = leave()
    override fun onTogglePlay() = controller.togglePlay()
    override fun onPlay() = controller.play()
    override fun onPause() = controller.pause()
    override fun onSeekTo(positionMs: Long) = controller.seekTo(positionMs)
    override fun onSpeed(speed: Float) = controller.setSpeed(speed)
    override fun onAudio(track: Track) = controller.selectAudio(track)
    override fun onSubtitle(track: Track?) = controller.selectSubtitle(track)
    override fun onQuality(quality: Quality) = controller.selectQuality(quality)
    override fun onSkipIntro() = controller.skipIntro()
    override fun onSkipOutro() = controller.skipOutro()
    override fun onPrevious() = controller.playPrevious()
    override fun onNext() = controller.playNext()
    override fun onEpisode(id: String) {
        _seasons.update { it.copy(shown = null) }
        controller.playEpisode(id)
    }
    override fun onCancelCountdown() = controller.cancelCountdown()
    override fun onSoundSettings() = openSoundSettings()

    override fun onRetry() {
        if (resolveError.value != null) begin() else controller.retry()
    }

    override fun onEpisodesOpened() {
        controller.loadEpisodes()
        _seasons.update { it.copy(shown = null) }
        if (_seasons.value.seasons.isNotEmpty()) return
        val series = state.value.item?.seriesRef?.id ?: return
        val url = serverUrl() ?: return
        scope.launch {
            val detail = (items.item(url, series) as? ApiResult.Ok)?.value ?: return@launch
            _seasons.update { it.copy(seasons = detail.seasons.orEmpty()) }
        }
    }

    override fun onSeason(seasonId: String) {
        val playing = state.value.item?.parentId
        _seasons.update { it.copy(shown = seasonId.takeIf { id -> id != playing }) }
        if (seasonId == playing || _seasons.value.episodes[seasonId] != null) return
        _seasons.update { it.copy(episodes = it.episodes + (seasonId to null)) }
        scope.launch {
            val list = (server.episodes(seasonId) as? ApiResult.Ok)?.value.orEmpty()
            _seasons.update { it.copy(episodes = it.episodes + (seasonId to list)) }
        }
    }

    private companion object {
        const val TAG = "tv-player"
        const val STARTED = "started"

        /** An HDMI handshake sends several signals in a row: one new start after they settle. */
        const val OUTPUT_SETTLE_MS = 1_000L
    }
}
