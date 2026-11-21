package com.slouchcat.app.mobile.player

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.generated.models.PlaybackCapabilities
import com.slouchcat.core.api.generated.models.Quality
import com.slouchcat.core.api.generated.models.Track
import com.slouchcat.core.player.DeviceSettings
import com.slouchcat.core.player.MeteredNetwork
import com.slouchcat.core.player.Narrowing
import com.slouchcat.core.player.PlaybackController
import com.slouchcat.core.player.PlaybackServer
import com.slouchcat.core.player.PlayerEngine
import com.slouchcat.core.player.PlayerState
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.StateFlow

/**
 * One player page: a [PlaybackController] over [engine] for the title of the route. It starts at
 * once and ends the playback when the page is gone ([onCleared]): the last progress and the stop
 * run in [finalScope], which outlives the page.
 */
class PlayerViewModel(
    itemId: String,
    startAtMs: Long?,
    fromStart: Boolean,
    val engine: PlayerEngine,
    server: PlaybackServer,
    capabilities: suspend (Narrowing) -> PlaybackCapabilities,
    settings: Flow<DeviceSettings>,
    metered: MeteredNetwork,
    finalScope: CoroutineScope,
    clock: () -> Long,
) : ViewModel(), PlayerEvents {
    private val controller = PlaybackController(viewModelScope, finalScope, server, engine, capabilities, settings, metered, clock)
    val state: StateFlow<PlayerState> = controller.state

    /** Set by the page: Zurück and the error pictures leave through the navigation. */
    var leave: () -> Unit = {}

    init {
        controller.start(itemId, startAtMs, fromStart)
    }

    /** The app went to the background (ON_STOP) and came back (ON_START). */
    fun onBackground() = controller.moveToBackground()

    fun onForeground() = controller.resumeFromBackground()

    override fun onCleared() = controller.close()

    override fun onBack() = leave()
    override fun onTogglePlay() = controller.togglePlay()
    override fun onSeekTo(positionMs: Long) = controller.seekTo(positionMs)
    override fun onSeekBy(deltaMs: Long) = controller.seekBy(deltaMs)
    override fun onSpeed(speed: Float) = controller.setSpeed(speed)
    override fun onAudio(track: Track) = controller.selectAudio(track)
    override fun onSubtitle(track: Track?) = controller.selectSubtitle(track)
    override fun onQuality(quality: Quality) = controller.selectQuality(quality)
    override fun onSkipIntro() = controller.skipIntro()
    override fun onSkipOutro() = controller.skipOutro()
    override fun onPrevious() = controller.playPrevious()
    override fun onNext() = controller.playNext()
    override fun onEpisode(id: String) = controller.playEpisode(id)
    override fun onCancelCountdown() = controller.cancelCountdown()
    override fun onRetry() = controller.retry()
    override fun onEpisodesOpened() = controller.loadEpisodes()
}
