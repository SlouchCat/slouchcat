package com.slouchcat.core.player

import kotlinx.coroutines.flow.Flow

/**
 * The player under [PlaybackController] (the seam of the tests: a fake engine). [Media3Engine]
 * plays with ExoPlayer; stage 6 (offline) gives it a local address, stage 7 lifts its Media3
 * player into a `MediaSessionService`. Every call comes from the main thread.
 */
interface PlayerEngine {
    /** What happens in the player; a hot flow, collected by the controller. */
    val events: Flow<EngineEvent>

    val positionMs: Long
    val durationMs: Long
    val bufferedMs: Long

    /** Loads [media] at [startMs]; [play]: start at once. */
    fun load(media: EngineMedia, startMs: Long, play: Boolean)

    fun play()

    fun pause()

    fun seekTo(positionMs: Long)

    fun setSpeed(speed: Float)

    /** Plays the [index]-th audio group of the file (the order of [EngineEvent.Tracks.audio]). */
    fun selectAudio(index: Int)

    /** Shows the [index]-th text group (the order of [EngineEvent.Tracks.text]); `null` shows none. */
    fun selectText(index: Int?)

    /** Stops loading and drops the media; the engine stays usable for the next [load]. */
    fun stop()

    fun release()
}

/** What [PlayerEngine.load] plays. [subtitleUrl]: a WebVTT file of the server beside the stream. */
data class EngineMedia(
    val url: String,
    val hls: Boolean,
    val subtitleUrl: String? = null,
    val subtitleLanguage: String? = null,
    /** The title for the media session (headphone controls, lock screen in stage 7). */
    val title: String? = null,
    val subtitle: String? = null,
)

/** One audio or text group of the player, in the order of the file. */
data class EngineTrack(
    /** The language Media3 read (ISO 639-1 where it can, `deu` becomes `de`). */
    val language: String?,
    /** The title of the track in the file. */
    val label: String?,
    val mime: String?,
    val channels: Int = 0,
    /** A decoder or the output can play it. */
    val supported: Boolean = true,
    val selected: Boolean = false,
    /** A text track the app added from the server (`subtitleUrl`), not one of the file. */
    val sideloaded: Boolean = false,
)

/** Why the player stopped with an error. */
enum class EngineFailure {
    /** No connection, a timeout: worth waiting for the network. */
    Network,

    /** The server refused the stream (401: the stream token is gone; 404). */
    Refused,

    /** The video decoder failed or the format exceeds the device. */
    VideoDecoder,

    /** The audio decoder or output failed. */
    AudioDecoder,

    /** Anything else (an unreadable file). */
    Other,
}

sealed interface EngineEvent {
    /** The groups of the file after loading or after a change of selection. */
    data class Tracks(val audio: List<EngineTrack>, val text: List<EngineTrack>) : EngineEvent

    /** The first picture is on the surface (the measure of «Tippen bis erstes Bild»). */
    data object FirstFrame : EngineEvent

    /** The player runs or stands (pause, buffering, end). */
    data class Playing(val playing: Boolean) : EngineEvent

    /**
     * The player wants to play or was paused: by the page, a headphone button, the media session
     * or an unplugged headphone. Buffering is no pause.
     */
    data class WantsToPlay(val play: Boolean) : EngineEvent

    data class Buffering(val buffering: Boolean) : EngineEvent

    data object Ended : EngineEvent

    data class Failed(val failure: EngineFailure, val httpStatus: Int? = null) : EngineEvent

    /** The size of the picture (the info sheet). */
    data class VideoSize(val width: Int, val height: Int) : EngineEvent
}
