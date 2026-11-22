package com.slouchcat.app.tv.testing

import com.slouchcat.app.tv.player.PlayerEvents
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemImages
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.MediaFileInfo
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
import com.slouchcat.core.api.generated.models.SeriesRef
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.api.generated.models.Track
import com.slouchcat.core.api.generated.models.TrackList
import com.slouchcat.core.api.generated.models.TrackSelection
import com.slouchcat.core.api.generated.models.Trickplay
import com.slouchcat.core.player.Countdown
import com.slouchcat.core.player.DecoderInfo
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DeviceType
import com.slouchcat.core.player.DisplayInfo
import com.slouchcat.core.player.EngineEvent
import com.slouchcat.core.player.EngineMedia
import com.slouchcat.core.player.EngineSignal
import com.slouchcat.core.player.EngineTrack
import com.slouchcat.core.player.PlaybackPrefsRepository
import com.slouchcat.core.player.testing.FakeServer
import com.slouchcat.core.player.PlayerEngine
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.player.TrackChoice
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.flow.MutableSharedFlow
import java.math.BigDecimal
import java.time.OffsetDateTime

/**
 * The server of the TV tests: the shared [FakeServer] of `:core:player-testing` answering with the
 * playback description of the TV tests ([playbackInfo]).
 */
fun tvServer(): FakeServer = FakeServer().apply {
    answer = { id, request -> ApiResult.Ok(playbackInfo(id, resumeMs = request.positionMs ?: 0)) }
}

class FakePrefs(var stored: PlaybackPrefs = PlaybackPrefs(setOf("de", "en"), SubtitleMode.FORCED, setOf("de"))) : PlaybackPrefsRepository {
    var refuse = false
    val saved = mutableListOf<PlaybackPrefs>()
    override suspend fun prefs(): ApiResult<PlaybackPrefs> = ApiResult.Ok(stored)
    override suspend fun save(prefs: PlaybackPrefs): ApiResult<PlaybackPrefs> {
        saved += prefs
        if (refuse) return ApiResult.Failed(422, "validation_failed")
        stored = prefs
        return ApiResult.Ok(prefs)
    }
    override suspend fun resetSeries(): ApiResult<Unit> = ApiResult.Ok(Unit)
}

/** The page's events as a list of what happened. */
class RecordedEvents : PlayerEvents {
    val calls = mutableListOf<String>()
    override fun onBack() { calls += "back" }
    override fun onTogglePlay() { calls += "toggle" }
    override fun onPlay() { calls += "play" }
    override fun onPause() { calls += "pause" }
    override fun onSeekTo(positionMs: Long) { calls += "seek:$positionMs" }
    override fun onAudio(track: Track) { calls += "audio:${track.id}" }
    override fun onSubtitle(track: Track?) { calls += "subtitle:${track?.id}" }
    override fun onQuality(quality: Quality) { calls += "quality:${quality.value}" }
    override fun onSkipIntro() { calls += "skipIntro" }
    override fun onSkipOutro() { calls += "skipOutro" }
    override fun onPrevious() { calls += "previous" }
    override fun onNext() { calls += "next" }
    override fun onEpisode(id: String) { calls += "episode:$id" }
    override fun onCancelCountdown() { calls += "cancelCountdown" }
    override fun onRetry() { calls += "retry" }
    override fun onEpisodesOpened() { calls += "episodes" }
    override fun onSeason(seasonId: String) { calls += "season:$seasonId" }
    override fun onSoundSettings() { calls += "soundSettings" }
}

/** The emulator of K26 stage 0: H.264, HEVC Main, AAC/FLAC/Opus, no passthrough, 1080p without HDR. */
val EMULATOR_TV = DeviceFacts(
    decoders = listOf(DecoderInfo("video/avc"), DecoderInfo("audio/mp4a-latm"), DecoderInfo("audio/flac"), DecoderInfo("audio/opus")),
    passthrough = emptySet(),
    outputChannels = 2,
    display = DisplayInfo(1920, 1080),
    type = DeviceType.Tv,
    name = "Shield · Android TV 11",
)

val AUDIO_DE = Track(id = 10, index = 1, kind = Track.Kind.AUDIO, codec = "eac3", default = true, forced = false, source = Track.Source.EMBEDDED, text = false, language = "ger", channels = 6)
val AUDIO_EN = Track(id = 11, index = 2, kind = Track.Kind.AUDIO, codec = "aac", default = false, forced = false, source = Track.Source.EMBEDDED, text = false, language = "eng", channels = 2)
val SUB_DE = Track(id = 20, index = 3, kind = Track.Kind.SUBTITLE, codec = "subrip", default = false, forced = false, source = Track.Source.EMBEDDED, text = true, language = "ger")
val SUB_PGS = Track(id = 21, index = 4, kind = Track.Kind.SUBTITLE, codec = "hdmv_pgs_subtitle", default = false, forced = false, source = Track.Source.EMBEDDED, text = false, language = "eng")

fun playbackInfo(
    id: String,
    item: ItemRef = itemRef(1, title = "Night Train", runtimeMinutes = 145).copy(id = id),
    resumeMs: Long = 0,
    next: ItemRef? = null,
    previous: ItemRef? = null,
    outro: Segment? = null,
    intro: Segment? = null,
) = PlaybackInfo(
    mode = PlaybackInfo.Mode.DIRECT,
    mediaKind = PlaybackInfo.MediaKind.VIDEO,
    item = item,
    mediaFile = MediaFileInfo(id = "f-$id", container = "mkv", sizeBytes = 1000, available = true, videoCodec = "hevc", audioCodec = "eac3", durationMs = 145 * 60_000L, bitrate = 8_300_000, width = 1920, height = 802),
    streamUrl = "/api/v1/items/$id/stream?st=t",
    playbackSessionId = "session-$id",
    resumePositionMs = resumeMs,
    tracks = TrackList("f-$id", listOf(AUDIO_DE, AUDIO_EN), listOf(SUB_DE, SUB_PGS)),
    selection = TrackSelection(audioTrack = 1),
    qualities = listOf(
        QualityOption(Quality.AUTO, available = true, current = true, direct = true, bitrate = 8_300_000),
        QualityOption(Quality.ORIGINAL, available = true, current = false, bitrate = 8_300_000),
        QualityOption(Quality._1080P, available = true, current = false, bitrate = 8_000_000),
        QualityOption(Quality._720P, available = true, current = false, bitrate = 4_000_000),
    ),
    chapters = listOf(Chapter(0, 600_000, "Anfang"), Chapter(600_000, 4_000_000, "Mitte")),
    segments = Segments(intro, outro),
    nextEpisode = next,
    previousEpisode = previous,
)

/** An episode of «Quiet Ring» (season 1). */
fun playerEpisode(n: Int, title: String = "Episode $n") = ItemRef(
    id = ulid(300 + n), kind = ItemKind.EPISODE, title = title, inWatchlist = false, images = ItemImages(),
    seasonNumber = 1, episodeNumber = n, parentId = SEASON_1, seriesTitle = "Quiet Ring", runtimeMinutes = 66,
    seriesRef = SeriesRef(SERIES_ID, "Quiet Ring", posters(SERIES_ID)),
)

/** A film that stands at 1 s (as when paused). */
fun filmState(info: PlaybackInfo = playbackInfo(ulid(1))) = PlayerState(
    item = info.item,
    info = info,
    capabilities = PlaybackCapabilities(video = listOf("h264"), audio = listOf("aac"), hls = PlaybackCapabilities.Hls.MSE, name = "Shield · Android TV 11", selectsTracks = true),
    loading = false,
    playing = false,
    positionMs = 1_000,
    durationMs = info.mediaFile.durationMs!!,
    audio = info.tracks.audio.map { TrackChoice(it, it.index == 1) },
    subtitles = info.tracks.subtitles.map { TrackChoice(it, false) },
    qualities = info.qualities,
    chapters = info.chapters,
)

/** Episode 2 in its intro, with a previous and a next episode. */
fun episodeState(): PlayerState {
    val ep = playerEpisode(2, "Shadows of the Past")
    val info = playbackInfo(ep.id, ep, previous = playerEpisode(1, "Arrival"), next = playerEpisode(3, "Cowardice"), intro = Segment(0, 90_000, Segment.Source.CHAPTER))
    return filmState(info).copy(showSkipIntro = true, previous = info.previousEpisode, next = info.nextEpisode, intro = info.segments.intro)
}

/** Episode 2 at its end credits with the card of episode 3. */
fun countdownState(): PlayerState {
    val s = episodeState()
    return s.copy(showSkipIntro = false, positionMs = s.durationMs - 20_000, countdown = Countdown(s.next!!, 7))
}
