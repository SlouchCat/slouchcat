package com.slouchcat.app.mobile.testing

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.PlaybackCapabilities
import com.slouchcat.core.api.generated.models.PlaybackInfo
import com.slouchcat.core.api.generated.models.PlaybackPrefs
import com.slouchcat.core.api.generated.models.PlaybackRequest
import com.slouchcat.core.api.generated.models.PlaybackSessionState
import com.slouchcat.core.api.generated.models.Progress
import com.slouchcat.core.api.generated.models.ProgressEvent
import com.slouchcat.core.api.generated.models.Quality
import com.slouchcat.core.api.generated.models.QualityOption
import com.slouchcat.core.api.generated.models.Segment
import com.slouchcat.core.api.generated.models.Segments
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.api.generated.models.Track
import com.slouchcat.core.api.generated.models.TrackList
import com.slouchcat.core.api.generated.models.TrackSelection
import com.slouchcat.core.api.generated.models.Trickplay
import com.slouchcat.core.player.EngineEvent
import com.slouchcat.core.player.EngineMedia
import com.slouchcat.core.player.PlaybackPrefsRepository
import com.slouchcat.core.player.PlaybackServer
import com.slouchcat.core.player.PlayerEngine
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.player.TrackChoice
import kotlinx.coroutines.flow.MutableSharedFlow
import java.math.BigDecimal
import java.time.OffsetDateTime

/** A player that only records: no picture, no time. */
class FakePlayerEngine : PlayerEngine {
    val flow = MutableSharedFlow<EngineEvent>(extraBufferCapacity = 16)
    override val events = flow
    val loads = mutableListOf<EngineMedia>()
    var released = false
    override val positionMs: Long = 0
    override val durationMs: Long = 0
    override val bufferedMs: Long = 0
    override fun load(media: EngineMedia, startMs: Long, play: Boolean) {
        loads += media
    }
    override fun play() = Unit
    override fun pause() = Unit
    override fun seekTo(positionMs: Long) = Unit
    override fun setSpeed(speed: Float) = Unit
    override fun selectAudio(index: Int) = Unit
    override fun selectText(index: Int?) = Unit
    override fun stop() = Unit
    override fun release() {
        released = true
    }
}

/** A server that answers every start with [playbackInfo] for the title and records the stops. */
class FakePlaybackServer : PlaybackServer {
    val starts = mutableListOf<String>()
    val requests = mutableListOf<PlaybackRequest>()
    val stops = mutableListOf<String>()
    override suspend fun start(itemId: String, request: PlaybackRequest): ApiResult<PlaybackInfo> {
        starts += itemId
        requests += request
        return ApiResult.Ok(playbackInfo(itemId))
    }
    override suspend fun progress(itemId: String, event: ProgressEvent): ApiResult<Progress> =
        ApiResult.Ok(Progress(event.positionMs, event.durationMs, BigDecimal.ZERO, false, OffsetDateTime.now()))
    override suspend fun state(sessionId: String): ApiResult<PlaybackSessionState> =
        ApiResult.Ok(PlaybackSessionState(PlaybackSessionState.State.PlaybackStateActive))
    override suspend fun stop(sessionId: String): ApiResult<Unit> {
        stops += sessionId
        return ApiResult.Ok(Unit)
    }
    override suspend fun trickplay(itemId: String, mediaFileId: String): ApiResult<Trickplay> = ApiResult.NotFound
    override suspend fun episodes(seasonId: String): ApiResult<List<ItemRef>> = ApiResult.Ok(emptyList())
    override fun absolute(path: String): String = "https://media.example.test$path"
}

class FakePrefs(var stored: PlaybackPrefs = PlaybackPrefs(setOf("de", "en"), SubtitleMode.FORCED, setOf("de"))) : PlaybackPrefsRepository {
    var refuse = false
    val saved = mutableListOf<PlaybackPrefs>()
    var resets = 0
    override suspend fun prefs(): ApiResult<PlaybackPrefs> = ApiResult.Ok(stored)
    override suspend fun save(prefs: PlaybackPrefs): ApiResult<PlaybackPrefs> {
        saved += prefs
        if (refuse) return ApiResult.Failed(422, "validation_failed")
        stored = prefs
        return ApiResult.Ok(prefs)
    }
    override suspend fun resetSeries(): ApiResult<Unit> {
        resets++
        return ApiResult.Ok(Unit)
    }
}

val AUDIO_DE = Track(id = 10, index = 1, kind = Track.Kind.AUDIO, codec = "eac3", default = true, forced = false, source = Track.Source.EMBEDDED, text = false, language = "ger", channels = 6)
val AUDIO_EN = Track(id = 11, index = 2, kind = Track.Kind.AUDIO, codec = "dts", default = false, forced = false, source = Track.Source.EMBEDDED, text = false, language = "eng", channels = 6)
val SUB_DE = Track(id = 20, index = 3, kind = Track.Kind.SUBTITLE, codec = "subrip", default = false, forced = false, source = Track.Source.EMBEDDED, text = true, language = "ger")
val SUB_EN = Track(id = 21, index = -1, kind = Track.Kind.SUBTITLE, codec = "subrip", default = false, forced = true, source = Track.Source.SIDECAR, text = true, language = "eng")

fun playbackInfo(id: String, item: ItemRef = itemRef(1, title = "Night Train", runtimeMinutes = 145).copy(id = id)) = PlaybackInfo(
    mode = PlaybackInfo.Mode.DIRECT,
    mediaKind = PlaybackInfo.MediaKind.VIDEO,
    item = item,
    mediaFile = mediaFile("f1").copy(durationMs = 145 * 60_000L, bitrate = 8_300_000, width = 1920, height = 802),
    streamUrl = "/api/v1/items/$id/stream?st=t",
    playbackSessionId = "session-$id",
    resumePositionMs = 0,
    tracks = TrackList("f1", listOf(AUDIO_DE, AUDIO_EN), listOf(SUB_DE, SUB_EN)),
    selection = TrackSelection(audioTrack = 1),
    qualities = listOf(
        QualityOption(Quality.AUTO, available = true, current = true, direct = true, bitrate = 8_300_000),
        QualityOption(Quality.ORIGINAL, available = true, current = false, bitrate = 8_300_000),
        QualityOption(Quality._1080P, available = true, current = false, bitrate = 8_000_000),
        QualityOption(Quality._720P, available = true, current = false, bitrate = 4_000_000),
        QualityOption(Quality._480P, available = false, current = false),
    ),
    chapters = listOf(Chapter(0, 600_000, "Anfang"), Chapter(600_000, 4_000_000, "Mitte"), Chapter(4_000_000, 8_700_000, "Ende")),
    segments = Segments(),
)

/** The state of a film that stands at 1 s (controls stay, as when paused). */
fun filmState(info: PlaybackInfo = playbackInfo(ulid(1))) = PlayerState(
    item = info.item,
    info = info,
    capabilities = PlaybackCapabilities(video = listOf("h264"), audio = listOf("aac"), hls = PlaybackCapabilities.Hls.MSE, name = "Pixel 9 · Android 16"),
    loading = false,
    playing = false,
    positionMs = 1_000,
    durationMs = info.mediaFile.durationMs!!,
    audio = info.tracks.audio.map { TrackChoice(it, it.index == 1) },
    subtitles = info.tracks.subtitles.map { TrackChoice(it, false) },
    qualities = info.qualities,
    chapters = info.chapters,
)

/** An episode in its intro with a previous and a next episode. */
fun episodeState(): PlayerState {
    val ep = ItemRef(
        id = ulid(301), kind = ItemKind.EPISODE, title = "Shadows of the Past", inWatchlist = false, images = posters(ulid(301)),
        seasonNumber = 1, episodeNumber = 2, parentId = ulid(300), seriesTitle = "Quiet Ring", runtimeMinutes = 66,
    )
    val info = playbackInfo(ep.id, ep).copy(
        segments = Segments(intro = Segment(0, 90_000, Segment.Source.CHAPTER)),
        previousEpisode = ep.copy(id = ulid(302), episodeNumber = 1, title = "Arrival"),
        nextEpisode = ep.copy(id = ulid(303), episodeNumber = 3, title = "Cowardice"),
    )
    return filmState(info).copy(showSkipIntro = true, previous = info.previousEpisode, next = info.nextEpisode)
}
