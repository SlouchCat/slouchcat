package com.slouchcat.core.offline

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.MediaFileInfo
import com.slouchcat.core.api.generated.models.OfflineEvent
import com.slouchcat.core.api.generated.models.PlaybackInfo
import com.slouchcat.core.api.generated.models.PlaybackRequest
import com.slouchcat.core.api.generated.models.PlaybackSessionState
import com.slouchcat.core.api.generated.models.Progress
import com.slouchcat.core.api.generated.models.ProgressEvent
import com.slouchcat.core.api.generated.models.Segments
import com.slouchcat.core.api.generated.models.Trickplay
import com.slouchcat.core.api.generated.models.TrackList
import com.slouchcat.core.api.generated.models.TrackSelection
import com.slouchcat.core.api.generated.models.TrickplaySheet
import com.slouchcat.core.api.generated.models.WatchTime as ServerWatchTime
import com.slouchcat.core.player.PlaybackController
import com.slouchcat.core.player.PlaybackServer
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import java.io.File
import java.math.BigDecimal
import java.math.RoundingMode

/**
 * The offline player's server (concept 27 9.1 and 9.5, stage 5 handover): [PlaybackServer] answered
 * on the device from the downloads of [profileId]. `start` plays the local HLS playlist built from
 * the fragment index (stage 0 point 5) with the subtitles and preview sheets of the download;
 * progress goes into the queue of offline events (uploaded at the next contact, the server merges
 * and books them) and into the position and watch time on the device. Regel 12: only downloads whose
 * `allowedProfileIds` name the profile; the watch time is the server's snapshot minus what plays here.
 */
class OfflinePlaybackServer(
    private val engine: OfflineEngine,
    /**
     * The owner of the downloads now and whether [profileId] may play now (the session's profile or
     * the one opened offline with its PIN, security review L-6); `null`: nothing plays.
     */
    private val owner: suspend () -> String?,
    private val profileId: Long,
    private val clock: () -> Long = System::currentTimeMillis,
) : PlaybackServer {
    private val lock = Mutex()

    /** The playing download per playback session (the controller's `playbackSessionId`). */
    private val sessions = mutableMapOf<String, String>()

    private var account: String = ""

    private suspend fun playable(itemId: String): StoredDownload? = engine.allDownloads().firstOrNull {
        it.account == account && !it.pendingDelete && it.complete && profileId in it.allowedProfileIds && (it.item?.id ?: it.manifest?.itemId) == itemId
    }

    private suspend fun profile(): StoredProfile? = engine.profile(account, profileId)

    private fun remainingMs(profile: StoredProfile?, offset: Long): Long? =
        profile?.watch?.let { WatchTime.remainingMs(it, clock() + offset) }

    override suspend fun start(itemId: String, request: PlaybackRequest): ApiResult<PlaybackInfo> {
        account = owner() ?: return ApiResult.NotFound
        val download = playable(itemId) ?: return ApiResult.NotFound
        val manifest = download.manifest ?: return ApiResult.NotFound
        val item = download.item ?: return ApiResult.NotFound
        val offset = engine.serverOffsetMs()
        if (remainingMs(profile(), offset) == 0L) return ApiResult.NotAllowed(PlaybackController.WATCH_LIMIT)
        val files = engine.files
        val subtitles = manifest.subtitles.filter { it.track.id in download.subtitlesStored }.map { it.track }
        val chosen = when {
            request.subtitleOff == true -> null
            request.subtitleTrack != null -> subtitles.firstOrNull { it.id == request.subtitleTrack }
            else -> null
        }
        val key = profileId.toString()
        val local = download.positions[key]
        val fromServer = download.serverPositions[key]?.takeUnless { it.completed }
        val resume = request.positionMs ?: listOfNotNull(local?.let { it.at to it.positionMs }, fromServer?.let { it.at to it.positionMs })
            .maxByOrNull { it.first }?.second ?: 0
        val neighbours = download.neighbours[key]
        val session = Ulid.next(clock())
        lock.withLock { sessions[session] = download.id }
        val info = PlaybackInfo(
            mode = PlaybackInfo.Mode.HLS,
            mediaKind = if (manifest.mimeType.startsWith("audio/")) PlaybackInfo.MediaKind.AUDIO else PlaybackInfo.MediaKind.VIDEO,
            item = item,
            mediaFile = MediaFileInfo(
                id = download.id,
                container = "mp4",
                sizeBytes = manifest.sizeBytes,
                available = true,
                mimeType = manifest.mimeType,
                width = manifest.width,
                height = manifest.height,
                durationMs = manifest.durationMs,
            ),
            streamUrl = uri(files.playlist(download.id)),
            playbackSessionId = session,
            resumePositionMs = resume.coerceIn(0, manifest.durationMs),
            tracks = TrackList(mediaFileId = download.id, audio = listOfNotNull(manifest.audio), subtitles = subtitles),
            selection = TrackSelection(audioTrack = manifest.audio?.index, subtitleTrack = chosen?.id),
            qualities = emptyList(),
            chapters = manifest.chapters,
            segments = Segments(),
            subtitleUrl = chosen?.let { uri(files.subtitle(download.id, it.id)) },
            nextEpisode = neighbours?.next?.let { onDevice(it) },
            previousEpisode = neighbours?.previous?.let { onDevice(it) },
            trickplay = manifest.trickplay?.let { set -> localSheets(set, download) },
        )
        return ApiResult.Ok(info)
    }

    /** A neighbour episode only when it lies complete on the device for this profile (the server named it). */
    private suspend fun onDevice(itemId: String): ItemRef? = playable(itemId)?.item

    private fun localSheets(set: Trickplay, download: StoredDownload): Trickplay? {
        val sheets = set.sheets.filter { it.index in download.trickplayStored }
        if (sheets.size != set.sheets.size) return null
        return set.copy(sheets = sheets.map { TrickplaySheet(it.index, uri(engine.files.trickplay(download.id, it.index))) })
    }

    override suspend fun progress(itemId: String, event: ProgressEvent): ApiResult<Progress> {
        if (account.isEmpty()) account = owner() ?: return ApiResult.NotFound
        val downloadId = lock.withLock { event.playbackSessionId?.let(sessions::get) } ?: playable(itemId)?.id ?: return ApiResult.NotFound
        val now = clock()
        val state = when (event.state) {
            ProgressEvent.State.PLAYING -> OfflineEvent.State.PLAYING
            ProgressEvent.State.ENDED -> OfflineEvent.State.ENDED
            else -> OfflineEvent.State.PAUSED
        }
        engine.queueEvent(
            account,
            OfflineEvent(
                profileId = profileId,
                itemId = itemId,
                playbackId = event.playbackSessionId ?: Ulid.next(now),
                positionMs = event.positionMs,
                durationMs = event.durationMs.coerceAtLeast(1),
                playedMs = event.playedMs,
                seq = event.seq,
                state = state,
                clientTime = event.clientTime,
                downloadId = downloadId,
                playbackRate = event.playbackRate,
            ),
        )
        engine.updateDownload(downloadId) { it?.copy(positions = it.positions + (profileId.toString() to LocalPosition(event.positionMs, now))) }
        val offset = engine.serverOffsetMs()
        val profile = engine.updateProfile(account, profileId) { stored ->
            stored?.copy(watch = stored.watch?.let { WatchTime.addPlayed(it, event.playedMs, now + offset) })
        }
        val left = remainingMs(profile, offset)
        val percent = if (event.durationMs > 0) BigDecimal(event.positionMs * 100.0 / event.durationMs).setScale(1, RoundingMode.HALF_UP) else BigDecimal.ZERO
        return ApiResult.Ok(
            Progress(
                positionMs = event.positionMs,
                durationMs = event.durationMs,
                percent = percent,
                completed = state == OfflineEvent.State.ENDED,
                updatedAt = engine.now(),
                watchTime = profile?.watch?.let { w ->
                    val remaining = ((left ?: 0) / MINUTE).toInt()
                    ServerWatchTime(limitMinutes = w.limitMinutes, usedMinutes = (w.limitMinutes - remaining).coerceAtLeast(0), remainingMinutes = remaining)
                },
            ),
        )
    }

    /**
     * Offline nobody else stops the playback; the watch time of the profile does (AN-18), and so does a
     * download that went meanwhile (revoked, deleted: code review m-14).
     */
    override suspend fun state(sessionId: String): ApiResult<PlaybackSessionState> {
        val downloadId = lock.withLock { sessions[sessionId] }
        if (downloadId != null && engine.download(downloadId).let { it == null || it.pendingDelete }) {
            return ApiResult.Ok(PlaybackSessionState(state = PlaybackSessionState.State.PlaybackStateStopped, stoppedBy = PlaybackSessionState.StoppedBy.PlaybackStoppedByAdmin))
        }
        val left = remainingMs(profile(), engine.serverOffsetMs())
        return ApiResult.Ok(
            if (left == 0L) {
                PlaybackSessionState(state = PlaybackSessionState.State.PlaybackStateStopped, stoppedBy = PlaybackSessionState.StoppedBy.PlaybackStoppedByLimit)
            } else {
                PlaybackSessionState(state = PlaybackSessionState.State.PlaybackStateActive)
            },
        )
    }

    override suspend fun stop(sessionId: String): ApiResult<Unit> {
        lock.withLock { sessions.remove(sessionId) }
        engine.scheduleSync()
        return ApiResult.Ok(Unit)
    }

    /** The preview sheets of a download are complete or absent; nothing to ask again. */
    override suspend fun trickplay(itemId: String, mediaFileId: String): ApiResult<Trickplay> = ApiResult.NotFound

    /** The episodes of a season that lie on the device for this profile. */
    override suspend fun episodes(seasonId: String): ApiResult<List<ItemRef>> = ApiResult.Ok(
        engine.allDownloads().takeIf { account.isNotEmpty() || owner()?.also { account = it } != null }.orEmpty()
            .filter { it.account == account && it.complete && !it.pendingDelete && profileId in it.allowedProfileIds && it.item?.parentId == seasonId }
            .mapNotNull { it.item }
            .sortedBy { it.episodeNumber ?: 0 },
    )

    /** Local files pass as they are; the offline player asks no server. */
    override fun absolute(path: String): String? = path.takeIf { it.startsWith("file:") }

    private fun uri(file: File): String = "file://" + file.absolutePath

    private companion object {
        const val MINUTE = 60_000L
    }
}
