package com.slouchcat.app.tv.audio

import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.core.player.AudioNow
import com.slouchcat.core.player.AudioPlayback
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

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
 * comes back paused, a station ends), closing the app ends everything ([leave]), a video ends the
 * title's session and a station ([videoStarts]; review M-2, as the phone after K27 stage 7 M-3), a
 * profile change and the sign-out end everything with the last reports ([end]).
 */
class TvAudio(val playback: AudioPlayback) {
    private val playing = mutableStateOf<Playing?>(null)

    /** What plays now, as Compose state: the rail entry «Läuft gerade» reads it while it is drawn. */
    val now: State<Playing?> get() = playing

    /** `true` after Home or standby until the app is in front again. */
    var away: Boolean = false
        private set

    /** `true` while a video runs over a title that waits for it. */
    private var underVideo = false

    /** Home, standby, the screen saver, HDMI: decided in the graph's scope, not in a page's (review B-1). */
    lateinit var lifecycle: AudioForeground
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
        if (playback.radio != null) playback.stop(forget = false) else playback.title?.moveToBackground()
    }

    /** The app is in front again: the title comes back paused where it stood (not under a video). */
    fun foreground() {
        if (!away) return
        away = false
        if (!underVideo) playback.title?.resumeFromBackground()
    }

    /** The app closes (the second Back of the start page, `finish()`): everything ends at once (review B-1). */
    fun leave() {
        away = false
        underVideo = false
        playback.stop(forget = false)
    }

    /**
     * A video opens (mockup: the audiobook waits paused, the station ends): the title's session ends
     * before the video's starts (review M-2); «Läuft gerade» keeps it, [videoEnds] brings it back paused.
     */
    fun videoStarts() {
        if (playback.radio != null) {
            playback.stop(forget = false)
        } else if (playback.title != null && !underVideo) {
            underVideo = true
            playback.title?.moveToBackground()
        }
    }

    /** The video is closed: the title is there again, paused where it stood. */
    fun videoEnds() {
        if (!underVideo) return
        underVideo = false
        if (!away) playback.title?.resumeFromBackground()
    }

    /** A profile change or the sign-out: everything ends; waits (at most [STOP_WAIT_MS]) for the last reports. */
    suspend fun end() {
        underVideo = false
        away = false
        playback.stopAndFlush(STOP_WAIT_MS)
    }

    companion object {
        /** How long a profile change waits for the last progress or `endRadioListen`. */
        const val STOP_WAIT_MS = 3_000L

        /** The audio of [graph], built by [make] in the graph's scope, hooked to the profile change and the sign-out. */
        fun create(graph: TvGraph, make: (CoroutineScope) -> AudioPlayback): TvAudio {
            val audio = TvAudio(make(graph.scope))
            audio.lifecycle = AudioForeground(audio, graph.scope)
            audio.follow(graph.scope)
            graph.hooks.onProfileOrSessionEnd { audio.end() }
            return audio
        }
    }
}

/**
 * Home, standby, the screen saver and the HDMI output (concept 26 5.6, mockup: «Danach darf der
 * Bildschirmschoner des Fernsehers kommen»). The activity stops for Home, standby and the screen
 * saver. The screen saver tells itself with `ACTION_DREAMING_STARTED`, standby with
 * `ACTION_SCREEN_OFF`; both may come a moment after the stop. So a stop waits [graceMs] before it
 * sends the sound away, unless the screen saver came; the screen going off sends it away at once.
 * When the screen saver ends without the app in front (Home, another app; review M-1) the same wait
 * starts again. An HDMI output that goes away (another input of the TV; review m-7) sends the sound
 * away after [graceMs], unless it comes back meanwhile (a handshake sends several signals).
 * [scope] is the graph's: an activity or a page that ends takes no decision with it (review B-1).
 */
class AudioForeground(private val audio: TvAudio, private val scope: CoroutineScope, private val graceMs: Long = GRACE_MS) {
    private var dreaming = false
    private var started = true
    private var pending: Job? = null
    private var hdmi: Job? = null

    private fun later() {
        pending?.cancel()
        pending = scope.launch {
            delay(graceMs)
            if (!dreaming && !started) audio.background()
        }
    }

    fun onStop() {
        started = false
        later()
    }

    fun onStart() {
        started = true
        pending?.cancel()
        dreaming = false
        audio.foreground()
    }

    /** The activity goes for good (`finish()`): the sound ends at once, without waiting. */
    fun onFinish() {
        pending?.cancel()
        hdmi?.cancel()
        audio.leave()
    }

    fun onDreamingStarted() {
        dreaming = true
    }

    fun onDreamingStopped() {
        dreaming = false
        if (!started) later()
    }

    fun onScreenOff() {
        pending?.cancel()
        audio.background()
    }

    /** [plugged]: the HDMI output (`ACTION_HDMI_AUDIO_PLUG`) came or went. */
    fun onHdmi(plugged: Boolean) {
        hdmi?.cancel()
        if (plugged) {
            // Back on this input with the app in front: the title waits paused (as after Home).
            if (started) audio.foreground()
            return
        }
        hdmi = scope.launch {
            delay(graceMs)
            audio.background()
        }
    }

    companion object {
        const val GRACE_MS = 1_500L
    }
}
