package com.slouchcat.app.tv.audio

import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.core.player.AudioNow
import com.slouchcat.core.player.AudioPlayback
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.joinAll
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull

/** What plays, as the rail and the small indicator show it (mockup `mdSync`, `mdRailNow`). */
sealed interface Playing {
    /** An audiobook or an episode; [paused] by the user, a key or Home. */
    data class Title(val itemId: String, val title: String, val paused: Boolean) : Playing

    data class Radio(val stationId: Long, val name: String) : Playing
}

/**
 * The sound of the TV (concept 26 5.6): one [AudioPlayback] for the graph, as at the phone, but
 * without a service. It plays on across the pages while the app is in front and while the screen
 * saver of the TV runs; Home and standby end the session at the server ([background]: the title
 * comes back paused, a station ends), a video pauses the title and ends a station ([videoStarts],
 * mockup `hashchange`), a profile change and the sign-out end everything with the last progress
 * ([end]). [finals] holds the last progress and `endRadioListen` on their way.
 */
class TvAudio(val playback: AudioPlayback, private val finals: Job) {
    private val playing = mutableStateOf<Playing?>(null)

    /** What plays now, as Compose state: the rail entry «Läuft gerade» reads it while it is drawn. */
    val now: State<Playing?> get() = playing

    /** `true` after Home or standby until the app is in front again. */
    var away: Boolean = false
        private set

    /** Follows [AudioPlayback.now] into [now] (in [scope], the graph's). */
    fun follow(scope: CoroutineScope) {
        scope.launch {
            playback.now.collect { now ->
                playing.value = when (now) {
                    is AudioNow.Title -> now.state.item?.let { item -> Playing.Title(item.id, item.title, paused = !now.state.playing && !now.state.buffering && !now.state.loading) }
                    is AudioNow.Radio -> now.state.station?.let { station -> Playing.Radio(station.id, station.name) }
                    is AudioNow.Restored, null -> null
                }
            }
        }
    }

    /** Home or standby (TV-NP): the title ends its session and waits paused, a station ends. */
    fun background() {
        if (away) return
        away = true
        if (playback.radio != null) playback.stop() else playback.title?.moveToBackground()
    }

    /** The app is in front again: the title comes back paused where it stood. */
    fun foreground() {
        if (!away) return
        away = false
        playback.title?.resumeFromBackground()
    }

    /** A video starts (mockup: the audiobook pauses, the station ends). */
    fun videoStarts() {
        if (playback.radio != null) playback.stop() else playback.title?.pause()
    }

    /** A profile change or the sign-out: everything ends; waits (at most [STOP_WAIT_MS]) for the last reports. */
    suspend fun end() {
        playback.stop()
        withTimeoutOrNull(STOP_WAIT_MS) { finals.children.toList().joinAll() }
    }

    companion object {
        /** How long a profile change waits for the last progress or `endRadioListen`. */
        const val STOP_WAIT_MS = 3_000L

        /**
         * The audio of [graph], built by [make] once (the scope it gets keeps the last reports as
         * children of `finals`) and hooked to the profile change and the sign-out.
         */
        fun create(graph: TvGraph, make: (CoroutineScope) -> AudioPlayback): TvAudio {
            val finals = SupervisorJob(graph.scope.coroutineContext[Job])
            val audio = TvAudio(make(CoroutineScope(graph.scope.coroutineContext + finals)), finals)
            audio.follow(graph.scope)
            graph.hooks.onProfileOrSessionEnd { audio.end() }
            return audio
        }
    }
}

/**
 * Home, standby and the screen saver (concept 26 5.6, mockup: «Danach darf der Bildschirmschoner
 * des Fernsehers kommen»). The activity stops for all three. The screen saver tells itself with
 * `ACTION_DREAMING_STARTED`, standby with `ACTION_SCREEN_OFF`; both may come a moment after the
 * stop. So a stop waits [graceMs] before it sends the sound away, unless the screen saver came;
 * the screen going off sends it away at once.
 */
class AudioForeground(private val audio: TvAudio, private val scope: CoroutineScope, private val graceMs: Long = GRACE_MS) {
    private var dreaming = false
    private var pending: Job? = null

    fun onStop() {
        pending?.cancel()
        pending = scope.launch {
            delay(graceMs)
            if (!dreaming) audio.background()
        }
    }

    fun onStart() {
        pending?.cancel()
        dreaming = false
        audio.foreground()
    }

    fun onDreamingStarted() {
        dreaming = true
    }

    fun onDreamingStopped() {
        dreaming = false
    }

    fun onScreenOff() {
        pending?.cancel()
        audio.background()
    }

    companion object {
        const val GRACE_MS = 1_500L
    }
}
