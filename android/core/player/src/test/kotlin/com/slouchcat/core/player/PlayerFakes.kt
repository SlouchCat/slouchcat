package com.slouchcat.core.player

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
import kotlinx.coroutines.flow.MutableSharedFlow
import java.math.BigDecimal
import java.time.OffsetDateTime

/** A player without Media3: the position runs with the test clock while it plays. */
class FakeEngine(private val clock: () -> Long) : PlayerEngine {
    val flow = MutableSharedFlow<EngineEvent>(extraBufferCapacity = 64)
    override val events = flow

    val loads = mutableListOf<Triple<EngineMedia, Long, Boolean>>()
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

    override fun load(media: EngineMedia, startMs: Long, play: Boolean) {
        loads += Triple(media, startMs, play)
        running = false
        wants = play
        base = startMs
    }

    /** Media3 has buffered: it plays if it was asked to, the first picture is there. */
    fun ready(audio: List<EngineTrack> = emptyList(), text: List<EngineTrack> = emptyList()) {
        flow.tryEmit(EngineEvent.Tracks(audio, text))
        flow.tryEmit(EngineEvent.WantsToPlay(wants))
        if (wants) start()
        flow.tryEmit(EngineEvent.FirstFrame)
    }

    private fun start() {
        base = positionMs
        since = clock()
        running = true
        flow.tryEmit(EngineEvent.Playing(true))
    }

    override fun play() {
        wants = true
        flow.tryEmit(EngineEvent.WantsToPlay(true))
        if (!running) start()
    }

    override fun pause() {
        base = positionMs
        running = false
        wants = false
        flow.tryEmit(EngineEvent.WantsToPlay(false))
        flow.tryEmit(EngineEvent.Playing(false))
    }

    override fun seekTo(positionMs: Long) {
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
        stops++
        base = positionMs
        running = false
    }

    override fun release() {
        released = true
    }

    fun end() {
        base = duration
        running = false
        flow.tryEmit(EngineEvent.Playing(false))
        flow.tryEmit(EngineEvent.Ended)
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

    override suspend fun start(itemId: String, request: PlaybackRequest): ApiResult<PlaybackInfo> {
        starts += itemId to request
        return answer(itemId, request)
    }

    override suspend fun progress(itemId: String, event: ProgressEvent): ApiResult<Progress> {
        progress += itemId to event
        return ApiResult.Ok(Progress(event.positionMs, event.durationMs, BigDecimal.ZERO, false, OffsetDateTime.now()))
    }

    override suspend fun state(sessionId: String): ApiResult<PlaybackSessionState> {
        stateCalls++
        return state
    }

    override suspend fun stop(sessionId: String): ApiResult<Unit> {
        stops += sessionId
        return ApiResult.Ok(Unit)
    }

    override suspend fun trickplay(itemId: String, mediaFileId: String): ApiResult<Trickplay> = ApiResult.NotFound

    override suspend fun episodes(seasonId: String): ApiResult<List<ItemRef>> = ApiResult.Ok(seasonEpisodes)

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
