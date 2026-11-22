package com.slouchcat.core.player.testing

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.PlaybackInfo
import com.slouchcat.core.api.generated.models.Progress
import com.slouchcat.core.api.generated.models.RadioHeartbeat
import com.slouchcat.core.api.generated.models.RadioListen
import com.slouchcat.core.api.generated.models.RadioNow
import com.slouchcat.core.api.generated.models.RadioStation
import com.slouchcat.core.api.generated.models.RadioStationState
import com.slouchcat.core.player.AudioMemory
import com.slouchcat.core.player.LastAudio
import com.slouchcat.core.player.RadioServer
import kotlinx.coroutines.CompletableDeferred
import java.math.BigDecimal
import java.time.OffsetDateTime

/*
 * The fakes of the audio of stage 7 (concept 27 8.3) for the tests of :core:player and both apps:
 * a radio server, a station, an audiobook as startPlayback answers it, a memory of the last title.
 */

/** A radio server that answers what the test sets and records the calls. */
class FakeRadioServer : RadioServer {
    val calls = mutableListOf<String>()
    var heartbeatSeconds = 30
    var startAnswer: (Long) -> ApiResult<RadioListen> = { id -> ApiResult.Ok(listen(id, heartbeatSeconds)) }
    var heartbeatAnswer: ApiResult<RadioHeartbeat> = ApiResult.Ok(RadioHeartbeat(expiresAt = 0))
    var nowAnswer: ApiResult<RadioNow> = ApiResult.Ok(RadioNow(live = true, propertyEntries = emptyList()))

    /** When set, a start waits for it. */
    var startGate: CompletableDeferred<Unit>? = null
    private var listens = 0

    override suspend fun start(stationId: Long): ApiResult<RadioListen> {
        calls += "start:$stationId"
        startGate?.await()
        return startAnswer(stationId)
    }

    override suspend fun heartbeat(listenId: String, playing: Boolean): ApiResult<RadioHeartbeat> {
        calls += "beat:$listenId:$playing"
        return heartbeatAnswer
    }

    override suspend fun end(listenId: String): ApiResult<Unit> {
        calls += "end:$listenId"
        return ApiResult.Ok(Unit)
    }

    override suspend fun now(stationId: Long): ApiResult<RadioNow> {
        calls += "now:$stationId"
        return nowAnswer
    }

    override fun absolute(path: String): String = "http://server.test$path"

    fun listen(stationId: Long, heartbeat: Int) = RadioListen(
        listenId = "listen-${++listens}",
        playlistUrl = "/api/v1/radio/listen/x/live.m3u8?rt=token",
        heartbeatSeconds = heartbeat,
        startOffsetSeconds = -8,
        timeShiftSeconds = 1800,
        station = station(stationId),
    )
}

fun station(id: Long, name: String = "Testradio $id") = RadioStation(
    id = id,
    name = name,
    languages = listOf("de"),
    genres = emptyList(),
    kinds = emptyList(),
    moods = emptyList(),
    hls = false,
    showsTitles = true,
    favorite = false,
    recommended = false,
    state = RadioStationState.OK,
)

/** An audiobook as `startPlayback` answers it: one HLS timeline over its parts, chapters on it. */
fun audioInfo(id: String, rate: Double? = null, resumeMs: Long = 0, durationMs: Long = 600_000, chapters: List<Chapter> = emptyList()): PlaybackInfo {
    val progress = rate?.let { Progress(positionMs = resumeMs, durationMs = durationMs, percent = BigDecimal.ZERO, completed = false, updatedAt = OffsetDateTime.now(), playbackRate = BigDecimal(it)) }
    val base = info(id, direct = false, resumeMs = resumeMs, chapters = chapters, item = ref(id, kind = ItemKind.AUDIOBOOK).copy(author = "Ann Author", durationMs = durationMs, progress = progress))
    return base.copy(mediaKind = PlaybackInfo.MediaKind.AUDIO)
}

/** [AudioMemory] in memory. */
class MemoryAudio(var last: LastAudio? = null) : AudioMemory {
    override suspend fun read() = last

    override suspend fun write(last: LastAudio?) {
        this.last = last
    }
}
