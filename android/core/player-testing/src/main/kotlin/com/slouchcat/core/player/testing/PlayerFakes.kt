package com.slouchcat.core.player.testing

import com.slouchcat.core.player.EngineEvent
import com.slouchcat.core.player.EngineMedia
import com.slouchcat.core.player.EngineSignal
import com.slouchcat.core.player.EngineTrack
import com.slouchcat.core.player.PlaybackServer
import com.slouchcat.core.player.PlayerEngine

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemImages
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.MediaFileInfo
import com.slouchcat.core.api.generated.models.PlaybackInfo
import com.slouchcat.core.api.generated.models.PlaybackRequest
import com.slouchcat.core.api.generated.models.PlaybackSessionState
import com.slouchcat.core.api.generated.models.Progress
import com.slouchcat.core.api.generated.models.ProgressEvent
import com.slouchcat.core.api.generated.models.Segment
import com.slouchcat.core.api.generated.models.Segments
import com.slouchcat.core.api.generated.models.Track
import com.slouchcat.core.api.generated.models.TrackList
import com.slouchcat.core.api.generated.models.TrackSelection
import com.slouchcat.core.api.generated.models.Trickplay
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.flow.MutableSharedFlow
import java.math.BigDecimal
import java.time.OffsetDateTime

/*
 * The one test double of the player for :core:player, the phone app and the TV app (only as
 * testImplementation): a player that behaves like Media3 and a server that answers what a test sets.
 */

/**
 * A player without Media3: the position runs with the test clock while it plays. Like Media3 it
 * numbers its loads and answers [stop] with empty tracks and the end of playing, under a number no
 * load has; after [stop] (Media3: `stop()` and `clearMediaItems()`) the position is 0 and the wish
 * to play stays (`playWhenReady`). [emit] sends an event of the current load (as Media3 does for
 * the player's own events).
 */
class FakeEngine(private val clock: () -> Long) : PlayerEngine {
    val signals = MutableSharedFlow<EngineSignal>(extraBufferCapacity = 64)
    override val events = signals
    private var current = PlayerEngine.NO_LOAD

    /** Whether the player wants to play (play/pause of the page). */
    val wantsPlay: Boolean get() = wants

    /** The number of the current load (to send a late event of an old one in a test). */
    val load: Long get() = current

    fun emit(event: EngineEvent, load: Long = current) {
        signals.tryEmit(EngineSignal(load, event))
    }

    val loads = mutableListOf<Triple<EngineMedia, Long, Boolean>>()
    val seeks = mutableListOf<Long>()
    val audioSelections = mutableListOf<Int>()
    val textSelections = mutableListOf<Int?>()
    var stops = 0
    var released = false
    var duration = 60_000L
    private var running = false
    private var wants = false
    private var base = 0L
    private var since = 0L

    override val positionMs: Long get() = minOf(duration, if (running) base + (clock() - since) else base)
    override val durationMs: Long get() = duration
    override val bufferedMs: Long get() = positionMs

    override fun load(media: EngineMedia, startMs: Long, play: Boolean, load: Long) {
        current = load
        loads += Triple(media, startMs, play)
        running = false
        wants = play
        base = startMs
        // Media3 says what it wants while load() still runs.
        emit(EngineEvent.WantsToPlay(play))
    }

    /** Media3 has buffered: it plays if it was asked to, the first picture is there. */
    fun ready(audio: List<EngineTrack> = emptyList(), text: List<EngineTrack> = emptyList()) {
        emit(EngineEvent.Tracks(audio, text))
        emit(EngineEvent.WantsToPlay(wants))
        if (wants) start()
        emit(EngineEvent.FirstFrame)
    }

    private fun start() {
        base = positionMs
        since = clock()
        running = true
        emit(EngineEvent.Playing(true))
    }

    override fun play() {
        wants = true
        emit(EngineEvent.WantsToPlay(true))
        if (!running) start()
    }

    override fun pause() {
        base = positionMs
        running = false
        wants = false
        emit(EngineEvent.WantsToPlay(false))
        emit(EngineEvent.Playing(false))
    }

    override fun seekTo(positionMs: Long) {
        seeks += positionMs
        base = positionMs
        since = clock()
    }

    override fun setSpeed(speed: Float) = Unit

    override fun selectAudio(index: Int) {
        audioSelections += index
    }

    override fun selectText(index: Int?) {
        textSelections += index
    }

    override fun stop() {
        current = PlayerEngine.NO_LOAD
        stops++
        // Media3 empties the player: no media, position 0; playWhenReady stays as it was.
        base = 0
        val was = running
        running = false
        // What Media3 says after stop() and clearMediaItems().
        emit(EngineEvent.Tracks(emptyList(), emptyList()))
        if (was) emit(EngineEvent.Playing(false))
    }

    override fun release() {
        released = true
    }

    fun end() {
        base = duration
        running = false
        emit(EngineEvent.Playing(false))
        emit(EngineEvent.Ended)
    }
}

/** A server that answers what the test sets and records what the controller sends. */
class FakeServer : PlaybackServer {
    val starts = mutableListOf<Pair<String, PlaybackRequest>>()
    val progress = mutableListOf<Pair<String, ProgressEvent>>()
    val stops = mutableListOf<String>()
    var stateCalls = 0
    var state: ApiResult<PlaybackSessionState> = ApiResult.Ok(PlaybackSessionState(PlaybackSessionState.State.PlaybackStateActive))
    var answer: (String, PlaybackRequest) -> ApiResult<PlaybackInfo> = { id, request -> ApiResult.Ok(info(id, resumeMs = request.positionMs ?: 0)) }
    var seasonEpisodes: List<ItemRef> = emptyList()

    /** Episodes of one season by its id (the menu «Folgen» with seasons); [seasonEpisodes] else. */
    var episodesBySeason: Map<String, List<ItemRef>> = emptyMap()

    /** When set, a start waits for it (the server still deciding). */
    var startGate: CompletableDeferred<Unit>? = null

    /** When set, a state request waits for it (an answer under way). */
    var stateGate: CompletableDeferred<Unit>? = null

    /** Progress and stops in the order they reached the server. */
    val calls = mutableListOf<String>()

    override suspend fun start(itemId: String, request: PlaybackRequest): ApiResult<PlaybackInfo> {
        starts += itemId to request
        startGate?.await()
        return answer(itemId, request)
    }

    override suspend fun progress(itemId: String, event: ProgressEvent): ApiResult<Progress> {
        progress += itemId to event
        calls += "progress:${event.playbackSessionId}"
        return ApiResult.Ok(Progress(event.positionMs, event.durationMs, BigDecimal.ZERO, false, OffsetDateTime.now()))
    }

    override suspend fun state(sessionId: String): ApiResult<PlaybackSessionState> {
        stateCalls++
        stateGate?.await()
        return state
    }

    override suspend fun stop(sessionId: String): ApiResult<Unit> {
        stops += sessionId
        calls += "stop:$sessionId"
        return ApiResult.Ok(Unit)
    }

    override suspend fun trickplay(itemId: String, mediaFileId: String): ApiResult<Trickplay> = ApiResult.NotFound

    override suspend fun episodes(seasonId: String): ApiResult<List<ItemRef>> = ApiResult.Ok(episodesBySeason[seasonId] ?: seasonEpisodes)

    override fun absolute(path: String): String = "http://server.test$path"
}

fun ref(id: String, title: String = "Title $id", kind: ItemKind = ItemKind.MOVIE, season: Int? = null, episode: Int? = null, parent: String? = null) =
    ItemRef(id = id, kind = kind, title = title, inWatchlist = false, images = ItemImages(), seasonNumber = season, episodeNumber = episode, parentId = parent, seriesTitle = if (episode != null) "Series" else null)

fun audio(id: Long, index: Int, language: String, codec: String = "aac", title: String? = null) =
    Track(id = id, index = index, kind = Track.Kind.AUDIO, codec = codec, default = index == 1, forced = false, source = Track.Source.EMBEDDED, text = false, language = language, title = title, channels = 2)

fun subtitle(id: Long, index: Int, language: String, codec: String = "subrip", source: Track.Source = Track.Source.EMBEDDED) =
    Track(id = id, index = index, kind = Track.Kind.SUBTITLE, codec = codec, default = false, forced = false, source = source, text = true, language = language)

/** A playback description as the server answers it; [direct] or HLS. */
fun info(
    id: String,
    direct: Boolean = true,
    audio: List<Track> = listOf(audio(10, 1, "deu")),
    subtitles: List<Track> = emptyList(),
    selection: TrackSelection = TrackSelection(audioTrack = audio.firstOrNull()?.index),
    item: ItemRef = ref(id),
    resumeMs: Long = 0,
    next: ItemRef? = null,
    intro: Segment? = null,
    outro: Segment? = null,
    subtitleUrl: String? = null,
    session: String = "session-$id",
    chapters: List<Chapter> = emptyList(),
) = PlaybackInfo(
    mode = if (direct) PlaybackInfo.Mode.DIRECT else PlaybackInfo.Mode.HLS,
    mediaKind = PlaybackInfo.MediaKind.VIDEO,
    item = item,
    mediaFile = MediaFileInfo(id = "file-$id", container = "mkv", sizeBytes = 1000, available = true, videoCodec = "hevc", audioCodec = "aac", durationMs = 60_000),
    streamUrl = if (direct) "/api/v1/items/$id/stream?st=token" else "/api/v1/playback/$session/master.m3u8?st=token",
    playbackSessionId = session,
    resumePositionMs = resumeMs,
    tracks = TrackList("file-$id", audio, subtitles),
    selection = selection,
    qualities = emptyList(),
    chapters = chapters,
    segments = Segments(intro, outro),
    subtitleUrl = subtitleUrl,
    nextEpisode = next,
)

fun group(language: String, label: String? = null, mime: String = "audio/mp4a-latm", supported: Boolean = true, selected: Boolean = false, sideloaded: Boolean = false) =
    EngineTrack(language = language, label = label, mime = mime, supported = supported, selected = selected, sideloaded = sideloaded)
