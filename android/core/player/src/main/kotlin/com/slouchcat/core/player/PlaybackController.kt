package com.slouchcat.core.player

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.PlaybackCapabilities
import com.slouchcat.core.api.generated.models.PlaybackInfo
import com.slouchcat.core.api.generated.models.PlaybackRequest
import com.slouchcat.core.api.generated.models.PlaybackSessionState
import com.slouchcat.core.api.generated.models.ProgressEvent
import com.slouchcat.core.api.generated.models.Quality
import com.slouchcat.core.api.generated.models.Track
import com.slouchcat.core.api.generated.models.TrackSelection
import com.slouchcat.core.api.generated.models.Trickplay
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.math.BigDecimal
import java.time.OffsetDateTime
import kotlin.math.ceil

/**
 * One playback from the start to the end, for any player page (concept 27 8.1; the web player's
 * `PlayerPage`, `useProgressReporter`, `usePlaybackStopWatch` as the model of behaviour):
 *
 * - the start with the device's capabilities; the server decides (rule 12);
 * - progress every 10 s while playing, at a pause, at the end and when the page is left;
 * - the state of the session every 4 s while playing: a stop by the server ends the playback;
 * - a new start where the server must change something (another track it has to remux, a quality,
 *   a burned subtitle), a switch inside the player where it can (direct play with `selectsTracks`);
 * - the fallbacks: a track Media3 plays silently starts again without its codec, a failing video
 *   decoder once with narrower capabilities, then the error;
 * - marks (intro, end credits), the countdown to the next episode, trickplay, the episode list.
 *
 * [scope] lives as long as the page (the ViewModel); [finalScope] outlives it for the last progress
 * and the stop. [clock] is monotonic milliseconds; tests give the virtual time of the test scheduler.
 */
class PlaybackController(
    private val scope: CoroutineScope,
    private val finalScope: CoroutineScope,
    private val server: PlaybackServer,
    private val engine: PlayerEngine,
    private val capabilities: suspend (Narrowing) -> PlaybackCapabilities,
    private val settings: Flow<DeviceSettings>,
    private val metered: MeteredNetwork = MeteredNetwork { false },
    private val clock: () -> Long,
    private val now: () -> OffsetDateTime = OffsetDateTime::now,
) {
    private val mutable = MutableStateFlow(PlayerState())
    val state: StateFlow<PlayerState> = mutable.asStateFlow()

    /** The subtitle wish of the next start: the server's rule, none, or a track. */
    private sealed interface SubtitleWish {
        data object Server : SubtitleWish
        data object Off : SubtitleWish
        data class Track(val id: Long) : SubtitleWish
    }

    private var device = DeviceSettings()
    private var itemId: String? = null
    private var info: PlaybackInfo? = null
    private var sessionId: String? = null
    private var media: EngineMedia? = null
    private var narrowing = Narrowing()
    private var videoRetried = false
    private var audioDecoderRetried = false
    private var containerRetried = false

    // What the user chose; a start after a change sends both explicitly (contract 1.11.0).
    private var audioWish: Int? = null
    private var subtitleWish: SubtitleWish = SubtitleWish.Server
    private var qualityWish: Quality? = null
    private var manualChoice = false

    private var engineAudio: List<EngineTrack> = emptyList()
    private var engineText: List<EngineTrack> = emptyList()
    private var tracksApplied = false
    private var wantsToPlay = false

    /**
     * Whether the user wants it to play: set by a start, Play/Pause and a pause of the player itself
     * (a headphone, the media session). A new start of the same title keeps it (a paused title stays
     * paused however often the output changes).
     */
    private var userPlay = true
    private var started = false
    private var startedAt = 0L
    private var seq = 1L
    private var playedMs = 0L
    private var playingSince: Long? = null
    private var countdownEnd: Long? = null

    /** The rest of the countdown while the player is paused (it stands with the picture). */
    private var countdownLeftMs: Long? = null
    private var countdownCancelled = false

    /** Where the next start after the background begins (`null`: where the server resumes). */
    private class Resume(val positionMs: Long?)

    private var resume: Resume? = null
    private var inBackground = false

    /** The number of the engine's current load; events of other loads are dropped. */
    private var loadId = NO_LOAD
    private var loads = 0L

    /** The position the running start asked for (the place to come back to from the background). */
    private var pendingPositionMs: Long? = null

    private var startJob: Job? = null
    private var sessionJobs: List<Job> = emptyList()
    private var reconnectJob: Job? = null

    init {
        scope.launch {
            engine.events.collect { signal -> if (signal.load == loadId) onEvent(signal.event) }
        }
        scope.launch { settings.collect { device = it } }
        scope.launch {
            while (isActive) {
                delay(TICK_MS)
                tick()
            }
        }
    }

    // ---- commands ---------------------------------------------------------------------------

    /**
     * Starts [id]: at [startAtMs], from the beginning with [fromStart], else where the server
     * resumes. A second call for another title ends the playing one first.
     */
    fun start(id: String, startAtMs: Long? = null, fromStart: Boolean = false) {
        if (itemId != null && itemId != id) leaveCurrent()
        itemId = id
        resetChoices()
        val at = if (fromStart) 0L else startAtMs
        if (inBackground) {
            // Nothing starts in the background (a page that resolved its title late): the start
            // waits for the foreground and comes paused, like a playback that went there.
            userPlay = false
            resume = Resume(at)
            return
        }
        begin(id, at, play = true, switching = false)
    }

    fun play() {
        if (mutable.value.ended) {
            engine.seekTo(0)
            mutable.update { it.copy(ended = false, countdown = null) }
            clearCountdown()
        }
        engine.play()
    }

    fun pause() = engine.pause()

    fun togglePlay() = if (wantsToPlay && !mutable.value.ended) pause() else play()

    fun seekTo(positionMs: Long) {
        val duration = duration()
        val target = if (duration > 0) positionMs.coerceIn(0, duration) else positionMs.coerceAtLeast(0)
        engine.seekTo(target)
        val outro = mutable.value.outro
        if (outro == null || target < outro.startMs) {
            clearCountdown()
            mutable.update { it.copy(countdown = null) }
        }
        mutable.update { it.copy(positionMs = target, ended = if (target < duration) false else it.ended) }
    }

    fun seekBy(deltaMs: Long) = seekTo(engine.positionMs + deltaMs)

    fun setSpeed(speed: Float) {
        engine.setSpeed(speed)
        mutable.update { it.copy(speed = speed) }
    }

    /** Another audio track: inside the player when the file plays directly and the groups match, else a new start. */
    fun selectAudio(track: Track) {
        val playing = info ?: return
        if (mutable.value.selectedAudio?.id == track.id) return
        manualChoice = true
        audioWish = track.index
        val group = if (playing.mode == PlaybackInfo.Mode.DIRECT && !narrowing.noTrackChoice) TrackMatching.group(playing.tracks.audio, engineAudio, track) else null
        // Rule 12: inside the player only a codec the start reported («Immer Stereo», «von Hand»
        // take formats out); any other the server decides on a new start.
        val reported = track.codec in mutable.value.capabilities?.audio.orEmpty()
        if (group != null && reported && engineAudio.getOrNull(group)?.supported == true) {
            engine.selectAudio(group)
            mutable.update { s -> s.copy(audio = s.audio.map { it.copy(selected = it.track.id == track.id) }) }
        } else {
            restart()
        }
    }

    /** Subtitles of [track], or none with `null`. */
    fun selectSubtitle(track: Track?) {
        val playing = info ?: return
        if (mutable.value.selectedSubtitle?.id == track?.id) return
        manualChoice = true
        if (track == null) {
            subtitleWish = SubtitleWish.Off
            if (playing.selection.subtitleDelivery == TrackSelection.SubtitleDelivery.BURN) {
                restart()
            } else {
                engine.selectText(null)
                mutable.update { it.copy(subtitlesOff = true) }
            }
            return
        }
        subtitleWish = SubtitleWish.Track(track.id)
        val direct = playing.mode == PlaybackInfo.Mode.DIRECT && !narrowing.noTrackChoice
        val group = if (direct && track.codec in CapabilityRules.SUBTITLES) TrackMatching.group(playing.tracks.subtitles, engineText, track) else null
        val sideloadedNow = playing.selection.subtitleTrack == track.id && playing.subtitleUrl != null
        when {
            group != null -> engine.selectText(group)
            sideloadedNow -> engine.selectText(TrackMatching.sideloaded(engineText))
            else -> {
                restart()
                return
            }
        }
        mutable.update { s -> s.copy(subtitlesOff = false, subtitles = s.subtitles.map { it.copy(selected = it.track.id == track.id) }) }
    }

    /** A quality of the sheet: `auto` leaves the cap of the device's settings. */
    fun selectQuality(quality: Quality) {
        if (info == null) return
        val current = qualityWish ?: mutable.value.qualities.firstOrNull { it.current }?.quality ?: Quality.AUTO
        if (quality == current) return
        qualityWish = quality
        restart()
    }

    fun skipIntro() {
        mutable.value.intro?.let { seekTo(it.endMs) }
    }

    fun skipOutro() {
        mutable.value.outro?.let { seekTo(minOf(it.endMs, duration())) }
    }

    fun cancelCountdown() {
        countdownCancelled = true
        clearCountdown()
        mutable.update { it.copy(countdown = null) }
    }

    fun playNext() {
        mutable.value.next?.let { playEpisode(it.id) }
    }

    fun playPrevious() {
        mutable.value.previous?.let { playEpisode(it.id) }
    }

    /** Another episode of the series (episode list, previous, next, countdown). */
    fun playEpisode(id: String) {
        if (id == itemId) return
        leaveCurrent()
        itemId = id
        resetChoices()
        begin(id, null, play = true, switching = false)
    }

    /** The episodes of the season of the playing episode, for the sheet «Folgen». */
    fun loadEpisodes() {
        val item = mutable.value.item ?: return
        val season = item.parentId ?: return
        if (mutable.value.episodes?.episodes != null) return
        mutable.update { it.copy(episodes = SeasonEpisodes(item.seasonNumber, null)) }
        scope.launch {
            val list = (server.episodes(season) as? ApiResult.Ok)?.value.orEmpty()
            mutable.update { it.copy(episodes = SeasonEpisodes(item.seasonNumber, list)) }
        }
    }

    /**
     * What the device reports changed while a title plays (concept 26 5.2: the sound system was
     * switched off, a headphone came): a new start where it stands, with the capabilities read
     * again, so the server decides again (rule 12). Nothing while nothing plays, in the background
     * (the next start reads them anyway) or after an error.
     */
    fun capabilitiesChanged() {
        if (inBackground || mutable.value.error != null || mutable.value.ended) return
        // Also during the first start: the facts it read may be old already (a receiver waking up).
        if (info == null && startJob?.isActive != true) return
        restart()
    }

    /** «Erneut versuchen» of an error picture. */
    fun retry() {
        val id = itemId ?: return
        begin(id, mutable.value.positionMs.takeIf { it > 0 }, play = true, switching = false)
    }

    /**
     * The app left the foreground: progress, then the session ends (concept 27 8.1, as the web
     * player at `pagehide`). [resumeFromBackground] starts again where it stood, paused.
     */
    fun moveToBackground() {
        if (inBackground) return
        inBackground = true
        // Nothing goes on in the background: no countdown, no start, no reconnection.
        clearCountdown()
        val starting = startJob?.isActive == true
        startJob?.cancel()
        reconnectJob?.cancel()
        stopLoops()
        val session = sessionId
        resume = when {
            itemId == null -> null
            session != null -> Resume(engine.positionMs.coerceAtLeast(0))
            starting -> Resume(pendingPositionMs)
            else -> null
        }
        if (session != null) {
            endSession(session, if (!mutable.value.ended) progressEvent(ProgressEvent.State.PAUSED) else null)
            sessionId = null
        }
        engine.stop()
        loadId = NO_LOAD
        mutable.update { it.copy(playing = false, countdown = null) }
    }

    fun resumeFromBackground() {
        if (!inBackground) return
        inBackground = false
        val back = resume ?: return
        resume = null
        val id = itemId ?: return
        begin(id, back.positionMs, play = false, switching = info != null)
    }

    /** The page is left: the last progress and the end of the session, even after the page is gone. */
    fun close() {
        startJob?.cancel()
        reconnectJob?.cancel()
        leaveCurrent()
        itemId = null
        engine.release()
    }

    // ---- the start ----------------------------------------------------------------------------

    private fun resetChoices() {
        audioWish = null
        subtitleWish = SubtitleWish.Server
        qualityWish = null
        manualChoice = false
        countdownCancelled = false
        clearCountdown()
        videoRetried = false
        audioDecoderRetried = false
        containerRetried = false
        narrowing = narrowing.forNextTitle()
    }

    private fun clearCountdown() {
        countdownEnd = null
        countdownLeftMs = null
    }

    /** A new start of the same title where it stands now (another track, quality, a fallback). */
    private fun restart(narrower: Narrowing? = null) {
        val id = itemId ?: return
        if (narrower != null) narrowing += narrower
        // While a start still runs the player is empty (position 0 after stop): its wish holds.
        val starting = startJob?.isActive == true || mutable.value.switching
        val position = if (starting) pendingPositionMs else engine.positionMs.coerceAtLeast(0)
        begin(id, position, play = userPlay, switching = true)
    }

    private fun begin(id: String, positionMs: Long?, play: Boolean, switching: Boolean) {
        startJob?.cancel()
        reconnectJob?.cancel()
        stopLoops()
        sessionId?.let { old ->
            // The time played since the last report goes with the end of the old session.
            val last = if (switching && !mutable.value.ended) {
                progressEvent(if (wantsToPlay) ProgressEvent.State.PLAYING else ProgressEvent.State.PAUSED)
            } else {
                null
            }
            endSession(old, last)
        }
        sessionId = null
        engine.stop()
        loadId = NO_LOAD
        pendingPositionMs = positionMs
        userPlay = play
        startedAt = clock()
        tracksApplied = false
        started = false
        mutable.update {
            if (switching) {
                it.copy(switching = true, error = null, firstFrameMs = null)
            } else {
                PlayerState(loading = true, speed = it.speed, item = it.item?.takeIf { item -> item.id == id })
            }
        }
        startJob = scope.launch {
            // The device's settings as stored, not the defaults before the store answered.
            device = settings.first()
            var attempts = 0
            while (true) {
                val caps = try {
                    capabilities(narrowing)
                } catch (e: CancellationException) {
                    throw e
                } catch (e: RuntimeException) {
                    Log.w(TAG, "capabilities unreadable", error = e)
                    fail(PlayerError.Failed)
                    return@launch
                }
                val request = request(caps, positionMs)
                when (val result = server.start(id, request)) {
                    is ApiResult.Ok -> {
                        onStarted(result.value, caps, play)
                        return@launch
                    }
                    is ApiResult.Failed -> if (result.code == TRANSCODE_BUSY && attempts < BUSY_ATTEMPTS) {
                        attempts++
                        mutable.update { it.copy(error = PlayerError.Busy(retrying = true)) }
                        delay(BUSY_RETRY_MS)
                        continue
                    } else {
                        fail(startError(result))
                        return@launch
                    }
                    else -> {
                        fail(startError(result))
                        return@launch
                    }
                }
            }
        }
    }

    private fun request(caps: PlaybackCapabilities, positionMs: Long?): PlaybackRequest {
        val subtitle = subtitleWish
        val quality = qualityWish
        return PlaybackRequest(
            capabilities = caps,
            positionMs = positionMs,
            audioTrack = audioWish,
            subtitleTrack = (subtitle as? SubtitleWish.Track)?.id,
            subtitleOff = if (subtitle == SubtitleWish.Off) true else null,
            rememberForSeries = if (manualChoice && mutable.value.isEpisode) true else null,
            quality = quality?.takeIf { it != Quality.AUTO },
            qualityCap = if (quality == null || quality == Quality.AUTO) device.cap(metered.metered()) else null,
        )
    }

    private fun startError(result: ApiResult<*>): PlayerError = when (result) {
        ApiResult.NotFound -> PlayerError.NotFound
        is ApiResult.NotAllowed -> if (result.code == WATCH_LIMIT) PlayerError.WatchLimit else PlayerError.Failed
        ApiResult.SignedOut -> PlayerError.SignedOut
        ApiResult.Offline -> PlayerError.Network(retrying = false)
        is ApiResult.Failed -> when (result.code) {
            STREAM_LIMIT -> PlayerError.StreamLimit
            DELETING -> PlayerError.NotFound
            TRANSCODE_BUSY -> PlayerError.Busy(retrying = false)
            else -> PlayerError.Failed
        }
        else -> PlayerError.Failed
    }

    private fun onStarted(started: PlaybackInfo, caps: PlaybackCapabilities, play: Boolean) {
        if (inBackground) {
            // A start that answered after the app left: its session ends at once, it never plays.
            endSession(started.playbackSessionId, null)
            return
        }
        info = started
        sessionId = started.playbackSessionId
        val selection = started.selection
        // Turning subtitles off in the player stays off across a new start of the same title.
        val off = selection.subtitleTrack == null
        mutable.update {
            it.copy(
                item = started.item,
                info = started,
                capabilities = caps,
                loading = false,
                switching = false,
                error = null,
                ended = false,
                durationMs = started.mediaFile.durationMs ?: it.durationMs,
                positionMs = started.resumePositionMs,
                audio = started.tracks.audio.map { t -> TrackChoice(t, t.index == selection.audioTrack) },
                subtitles = started.tracks.subtitles.map { t -> TrackChoice(t, t.id == selection.subtitleTrack) },
                subtitlesOff = off,
                qualities = started.qualities,
                chapters = started.chapters,
                intro = started.segments.intro,
                outro = started.segments.outro,
                next = started.nextEpisode,
                previous = started.previousEpisode,
                trickplay = started.trickplay?.let { set -> TrickplayTiles(set, server::absolute) },
                episodes = it.episodes?.takeIf { e -> e.seasonNumber == started.item.seasonNumber && it.item?.parentId == started.item.parentId },
            )
        }
        val url = server.absolute(started.streamUrl)
        if (url == null) {
            fail(PlayerError.Failed)
            return
        }
        val subtitleLanguage = started.tracks.subtitles.firstOrNull { it.id == selection.subtitleTrack }?.language
        val next = EngineMedia(
            url = url,
            hls = started.mode == PlaybackInfo.Mode.HLS,
            subtitleUrl = started.subtitleUrl?.let(server::absolute),
            subtitleLanguage = subtitleLanguage,
            title = started.item.title,
            subtitle = started.item.seriesTitle,
        )
        media = next
        seq = 1
        playedMs = 0
        playingSince = null
        loadId = ++loads
        engine.load(next, started.resumePositionMs, play, loadId)
        startLoops(started)
    }

    // ---- what the engine says ------------------------------------------------------------------

    private fun onEvent(event: EngineEvent) {
        when (event) {
            is EngineEvent.Tracks -> onTracks(event)
            EngineEvent.FirstFrame -> if (mutable.value.firstFrameMs == null) {
                val ms = clock() - startedAt
                mutable.update { it.copy(firstFrameMs = ms) }
                // The measure of stage 5 (docs/perf/K27-android-handy.md): from the start to the first picture.
                Log.i(TAG, "first picture", "ms" to ms, "mode" to info?.mode?.value, "transcoding" to info?.transcoding?.mode?.value)
            }
            is EngineEvent.Playing -> onPlaying(event.playing)
            is EngineEvent.WantsToPlay -> {
                val was = wantsToPlay
                wantsToPlay = event.play
                // Play/Pause of the page, a headphone, the media session: the wish for the next start.
                userPlay = event.play
                // The countdown stands while the picture stands (the web player's countdown runs
                // with the media time).
                val end = countdownEnd
                val left = countdownLeftMs
                if (!event.play && end != null) {
                    countdownLeftMs = (end - clock()).coerceAtLeast(0)
                    countdownEnd = null
                } else if (event.play && left != null) {
                    countdownEnd = clock() + left
                    countdownLeftMs = null
                }
                // The web player reports `paused` at the pause event; the end reports `ended` itself.
                if (was && !event.play && !mutable.value.ended) sendProgress(ProgressEvent.State.PAUSED)
            }
            is EngineEvent.Buffering -> mutable.update { it.copy(buffering = event.buffering) }
            EngineEvent.Ended -> {
                stopClock()
                sendProgress(ProgressEvent.State.ENDED)
                mutable.update { it.copy(ended = true, playing = false, positionMs = duration()) }
                maybeCountdown(atEnd = true)
            }
            is EngineEvent.Failed -> onFailed(event)
            is EngineEvent.VideoSize -> mutable.update { it.copy(videoWidth = event.width, videoHeight = event.height) }
        }
    }

    private fun onPlaying(playing: Boolean) {
        if (playing) {
            started = true
            if (playingSince == null) playingSince = clock()
        } else {
            stopClock()
        }
        mutable.update { it.copy(playing = playing, buffering = if (playing) false else it.buffering) }
    }

    private fun onTracks(tracks: EngineEvent.Tracks) {
        engineAudio = tracks.audio
        engineText = tracks.text
        val playing = info ?: return
        if (!tracksApplied) {
            tracksApplied = true
            if (!applySelection(playing)) return
        }
        checkSilent(playing)
    }

    /** Sets the server's choice as the player's selection; `false` when a new start follows. */
    private fun applySelection(playing: PlaybackInfo): Boolean {
        val selection = playing.selection
        if (playing.mode == PlaybackInfo.Mode.DIRECT) {
            val wanted = playing.tracks.audio.firstOrNull { it.index == selection.audioTrack }
            if (wanted != null && engineAudio.size > 1) {
                val group = TrackMatching.group(playing.tracks.audio, engineAudio, wanted)
                if (group == null) {
                    // The file's groups do not agree with the server's list: the server picks (remux).
                    Log.i(TAG, "groups do not match the server's audio tracks")
                    restart(Narrowing(noTrackChoice = true))
                    return false
                }
                engine.selectAudio(group)
            }
        }
        val subtitle = playing.tracks.subtitles.firstOrNull { it.id == selection.subtitleTrack }
        when {
            subtitle == null || selection.subtitleDelivery == TrackSelection.SubtitleDelivery.BURN -> engine.selectText(null)
            playing.subtitleUrl != null -> engine.selectText(TrackMatching.sideloaded(engineText))
            else -> {
                val group = TrackMatching.group(playing.tracks.subtitles, engineText, subtitle)
                if (group == null) {
                    restart(Narrowing(noTrackChoice = true))
                    return false
                }
                engine.selectText(group)
            }
        }
        return true
    }

    /**
     * Media3 plays on without sound when no decoder takes the track (concept 26 stage 0): a group
     * that cannot play, or no audio group selected at all. The app starts again without that codec;
     * the server then converts the sound.
     */
    private fun checkSilent(playing: PlaybackInfo) {
        if (engineAudio.isEmpty()) return
        val wanted = playing.tracks.audio.firstOrNull { it.index == playing.selection.audioTrack }
        val wantedGroup = if (playing.mode == PlaybackInfo.Mode.DIRECT && wanted != null && engineAudio.size > 1) {
            TrackMatching.group(playing.tracks.audio, engineAudio, wanted)?.let(engineAudio::getOrNull)
        } else {
            engineAudio.singleOrNull()
        }
        val silent = (wantedGroup != null && !wantedGroup.supported) || engineAudio.none { it.selected }
        if (!silent) return
        val group = wantedGroup?.takeIf { !it.supported } ?: engineAudio.firstOrNull { !it.supported } ?: return
        val codec = CapabilityRules.audioCodecOf(group.mime) ?: wanted?.codec ?: return
        if (codec in narrowing.audio) return
        Log.i(TAG, "audio plays silently, starting again without the codec", "codec" to codec)
        audioWish = wanted?.index ?: audioWish
        restart(Narrowing(audio = setOf(codec)))
    }

    private fun onFailed(event: EngineEvent.Failed) {
        val playing = info
        Log.i(TAG, "player failed", "failure" to event.failure.name, "status" to event.httpStatus)
        stopClock()
        when (event.failure) {
            EngineFailure.Network -> reconnect()
            EngineFailure.Refused -> if (event.httpStatus == HTTP_NOT_FOUND) {
                fail(PlayerError.NotFound)
            } else {
                val session = sessionId
                if (session == null) fail(PlayerError.StoppedElsewhere) else sessionJobs = sessionJobs + scope.launch { checkStopped(session, afterRefusal = true) }
            }
            EngineFailure.VideoDecoder -> if (!videoRetried && playing != null) {
                videoRetried = true
                val codec = playing.mediaFile.videoCodec
                restart(Narrowing(video = setOfNotNull(codec), noHdr = true))
            } else {
                fail(PlayerError.Unplayable)
            }
            EngineFailure.AudioDecoder -> {
                val codec = mutable.value.selectedAudio?.codec ?: playing?.mediaFile?.audioCodec
                if (!audioDecoderRetried && codec != null) {
                    audioDecoderRetried = true
                    restart(Narrowing(audio = setOf(codec)))
                } else {
                    fail(PlayerError.Unplayable)
                }
            }
            EngineFailure.Container -> {
                val container = playing?.mediaFile?.container
                if (!containerRetried && playing?.mode == PlaybackInfo.Mode.DIRECT && container != null) {
                    containerRetried = true
                    restart(Narrowing(containers = setOf(container)))
                } else {
                    fail(PlayerError.Unplayable)
                }
            }
            EngineFailure.Other -> fail(PlayerError.Failed)
        }
    }

    /** The network is gone: wait for the server, then load the same stream where it stood. */
    private fun reconnect() {
        val session = sessionId ?: return fail(PlayerError.Network(retrying = false))
        val position = engine.positionMs
        val play = wantsToPlay
        mutable.update { it.copy(error = PlayerError.Network(retrying = true), playing = false) }
        reconnectJob?.cancel()
        reconnectJob = scope.launch {
            while (isActive) {
                delay(RECONNECT_MS)
                val answer = server.state(session)
                if (sessionId != session) return@launch
                when (answer) {
                    ApiResult.Offline -> continue
                    is ApiResult.Ok -> if (answer.value.state == PlaybackSessionState.State.PlaybackStateStopped) {
                        stopped(session, answer.value.stoppedBy)
                        return@launch
                    }
                    ApiResult.SignedOut -> {
                        fail(PlayerError.SignedOut)
                        return@launch
                    }
                    else -> Unit
                }
                val again = media ?: return@launch
                mutable.update { it.copy(error = null) }
                loadId = ++loads
                engine.load(again, position, play, loadId)
                return@launch
            }
        }
    }

    // ---- loops: progress, session state, trickplay, marks ------------------------------------

    private fun startLoops(playing: PlaybackInfo) {
        stopLoops()
        val session = playing.playbackSessionId
        val jobs = mutableListOf<Job>()
        jobs += scope.launch {
            while (isActive) {
                delay(PROGRESS_INTERVAL_MS)
                if (mutable.value.playing) sendProgress(ProgressEvent.State.PLAYING)
            }
        }
        jobs += scope.launch {
            while (isActive) {
                delay(STATE_INTERVAL_MS)
                if (mutable.value.playing && sessionId == session) checkStopped(session, afterRefusal = false)
            }
        }
        val trickplay = playing.trickplay
        if (trickplay != null && (trickplay.state == Trickplay.State.QUEUED || trickplay.state == Trickplay.State.RUNNING)) {
            jobs += scope.launch {
                while (isActive) {
                    delay(TRICKPLAY_INTERVAL_MS)
                    val set = (server.trickplay(playing.item.id, playing.mediaFile.id) as? ApiResult.Ok)?.value ?: continue
                    mutable.update { it.copy(trickplay = TrickplayTiles(set, server::absolute)) }
                    if (set.state == Trickplay.State.COMPLETE || set.state == Trickplay.State.FAILED) break
                }
            }
        }
        sessionJobs = jobs
    }

    private fun stopLoops() {
        sessionJobs.forEach { it.cancel() }
        sessionJobs = emptyList()
    }

    /**
     * `getPlaybackSessionState` of [session]: `stopped`, 401 and 403 `profile_required` end the
     * playback; 404 and no answer mean nothing. An answer for a session that is no longer the
     * current one (a new start came in between) means nothing either.
     */
    private suspend fun checkStopped(session: String, afterRefusal: Boolean) {
        val answer = server.state(session)
        if (sessionId != session) return
        when (answer) {
            is ApiResult.Ok -> when {
                answer.value.state == PlaybackSessionState.State.PlaybackStateStopped -> stopped(session, answer.value.stoppedBy)
                afterRefusal -> fail(PlayerError.StoppedElsewhere)
            }
            ApiResult.SignedOut -> fail(PlayerError.SignedOut)
            is ApiResult.NotAllowed -> if (answer.code == PROFILE_REQUIRED) fail(PlayerError.SignedOut)
            else -> if (afterRefusal) fail(PlayerError.StoppedElsewhere)
        }
    }

    private fun stopped(session: String, by: PlaybackSessionState.StoppedBy?) {
        if (sessionId != session) return
        // The server ended the session itself: no stop of ours, no last progress.
        sessionId = null
        fail(if (by == PlaybackSessionState.StoppedBy.PlaybackStoppedByLimit) PlayerError.WatchLimit else PlayerError.StoppedElsewhere)
    }

    private fun fail(error: PlayerError) {
        stopLoops()
        reconnectJob?.cancel()
        engine.stop()
        loadId = NO_LOAD
        mutable.update { it.copy(error = error, loading = false, switching = false, playing = false, countdown = null) }
        clearCountdown()
    }

    private fun tick() {
        val playing = info ?: return
        // In the background nothing counts; during a new start the engine has no position yet.
        if (inBackground || mutable.value.error != null || mutable.value.loading || mutable.value.switching) return
        val position = engine.positionMs.coerceAtLeast(0)
        val duration = duration()
        val intro = playing.segments.intro
        val outro = playing.segments.outro
        mutable.update {
            it.copy(
                positionMs = if (it.ended) duration else position,
                durationMs = duration,
                bufferedMs = engine.bufferedMs,
                showSkipIntro = device.offerSkipIntro && intro != null && position >= intro.startMs && position < intro.endMs - SKIP_TAIL_MS,
                showSkipOutro = outro != null && it.countdown == null && position >= outro.startMs && position < outro.endMs - SKIP_TAIL_MS,
            )
        }
        maybeCountdown(atEnd = false)
        countdownLeftMs?.let { left ->
            mutable.update { s -> s.countdown?.let { s.copy(countdown = it.copy(secondsLeft = ceil(left / 1000.0).toInt())) } ?: s }
        }
        val end = countdownEnd ?: return
        val left = ceil((end - clock()) / 1000.0).toInt()
        if (left <= 0) {
            countdownEnd = null
            playNext()
        } else {
            mutable.update { s -> s.countdown?.let { s.copy(countdown = it.copy(secondsLeft = left)) } ?: s }
        }
    }

    /** From the end credits on, or at the end of the file: the card of the next episode (W-9). */
    private fun maybeCountdown(atEnd: Boolean) {
        val s = mutable.value
        val next = s.next ?: return
        if (inBackground || countdownCancelled || s.countdown != null || s.error != null) return
        val outro = s.outro
        if (!atEnd && (outro == null || s.positionMs < outro.startMs)) return
        countdownEnd = if (device.autoplayNext && wantsToPlay) clock() + AUTO_NEXT_MS else null
        countdownLeftMs = if (device.autoplayNext && !wantsToPlay) AUTO_NEXT_MS else null
        mutable.update { it.copy(countdown = Countdown(next, if (device.autoplayNext) (AUTO_NEXT_MS / 1000).toInt() else null)) }
    }

    // ---- progress and the end of a session -----------------------------------------------------

    private fun duration(): Long = engine.durationMs.takeIf { it > 0 } ?: info?.mediaFile?.durationMs ?: mutable.value.durationMs

    private fun stopClock() {
        playingSince?.let { playedMs += clock() - it }
        playingSince = null
    }

    private fun sendProgress(state: ProgressEvent.State) {
        val id = itemId ?: return
        val event = progressEvent(state) ?: return
        // Progress is best effort: a lost event is repaired by the next one.
        scope.launch { server.progress(id, event) }
    }

    /** The next progress event (and the played time it takes along); `null` before the first picture ran. */
    private fun progressEvent(state: ProgressEvent.State): ProgressEvent? {
        val playing = info ?: return null
        if (!started) return null
        val running = playingSince != null
        stopClock()
        val event = ProgressEvent(
            positionMs = engine.positionMs.coerceAtLeast(0),
            durationMs = duration(),
            playedMs = playedMs,
            seq = seq++,
            state = state,
            clientTime = now(),
            mediaFileId = playing.mediaFile.id,
            playbackSessionId = playing.playbackSessionId,
            playbackRate = mutable.value.speed.takeIf { it != 1f }?.let { BigDecimal(it.toString()) },
        )
        playedMs = 0
        if (running) playingSince = clock()
        return event
    }

    /**
     * Ends [session] after the page or the title is gone: first the [last] progress, then the stop,
     * one after the other (the server records the event before the session closes).
     */
    private fun endSession(session: String, last: ProgressEvent?) {
        val id = itemId
        finalScope.launch {
            if (last != null && id != null) server.progress(id, last)
            server.stop(session)
        }
    }

    /** Leaves the playing title: the last progress and the end of its session. */
    private fun leaveCurrent() {
        // Without a session (ended by the server, or in the background) the last progress went out already.
        sessionId?.let { endSession(it, if (!mutable.value.ended) progressEvent(ProgressEvent.State.PAUSED) else null) }
        sessionId = null
        stopLoops()
        engine.stop()
        loadId = NO_LOAD
        info = null
    }

    companion object {
        private const val TAG = "player"

        /** Progress every 10 s while playing (plan 3.4 step 7). */
        const val PROGRESS_INTERVAL_MS = 10_000L

        /** The state of the session every 4 s while playing (concept 13, 11.13). */
        const val STATE_INTERVAL_MS = 4_000L

        /** Trickplay is asked again every 10 s while it is made (concept 18 part A). */
        const val TRICKPLAY_INTERVAL_MS = 10_000L

        /** The next-episode countdown (W-9). */
        const val AUTO_NEXT_MS = 10_000L

        /** The skip button hides this close to the end of the intro. */
        const val SKIP_TAIL_MS = 500L

        const val TICK_MS = 250L
        const val RECONNECT_MS = 3_000L
        const val BUSY_RETRY_MS = 5_000L
        const val BUSY_ATTEMPTS = 6

        const val WATCH_LIMIT = "watch_limit_reached"
        const val STREAM_LIMIT = "stream_limit_reached"
        const val DELETING = "deleting"
        const val TRANSCODE_BUSY = "transcode_busy"
        const val PROFILE_REQUIRED = "profile_required"
        const val HTTP_NOT_FOUND = 404
        /** Matches no event, not even those of [PlayerEngine.stop]. */
        private const val NO_LOAD = -1L
    }
}
