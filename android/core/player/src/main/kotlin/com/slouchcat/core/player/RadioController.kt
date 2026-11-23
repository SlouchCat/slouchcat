package com.slouchcat.core.player

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.RadioApi
import com.slouchcat.core.api.generated.models.RadioHeartbeat
import com.slouchcat.core.api.generated.models.RadioHeartbeatRequest
import com.slouchcat.core.api.generated.models.RadioListen
import com.slouchcat.core.api.generated.models.RadioNow
import com.slouchcat.core.api.generated.models.RadioNowEntry
import com.slouchcat.core.api.generated.models.RadioStation
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.withContext
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import okhttp3.Call
import okhttp3.HttpUrl

/**
 * The operations of listening to a station (concept 16 7.2, the seam of the tests): the start
 * hands a playlist with the listen token `rt` (bound to that station), the heartbeat keeps the
 * listen alive and counts the watch time (K-5, the server decides), the end revokes the token.
 */
interface RadioServer {
    /** `startRadioListen`. */
    suspend fun start(stationId: Long): ApiResult<RadioListen>

    /** `radioListenHeartbeat`. */
    suspend fun heartbeat(listenId: String, playing: Boolean): ApiResult<RadioHeartbeat>

    /** `endRadioListen`. */
    suspend fun end(listenId: String): ApiResult<Unit>

    /** `getRadioNow`: what the server read from the stream (the client never parses it). */
    suspend fun now(stationId: Long): ApiResult<RadioNow>

    /** A path of the server (the playlist, a logo) as an absolute address on its own origin. */
    fun absolute(path: String): String?
}

class ApiRadioServer(
    private val api: ApiCaller,
    private val http: Call.Factory,
    /** The server of the session; `null` once signed out. */
    private val server: () -> HttpUrl?,
) : RadioServer {
    private suspend fun <T : Any> radio(block: (RadioApi) -> T?): ApiResult<T> {
        val base = server() ?: return ApiResult.SignedOut
        return api.call("radio") { block(SlouchcatApi.create(::RadioApi, base, http)) }
    }

    override suspend fun start(stationId: Long) = radio { it.startRadioListen(stationId) }

    override suspend fun heartbeat(listenId: String, playing: Boolean) = radio { it.radioListenHeartbeat(listenId, RadioHeartbeatRequest(playing)) }

    override suspend fun end(listenId: String) = radio { it.endRadioListen(listenId) }

    override suspend fun now(stationId: Long) = radio { it.getRadioNow(stationId) }

    override fun absolute(path: String): String? = server()?.let { ServerPaths.absolute(it, path) }
}

/** Why the radio stopped (web `RadioNotice`); each has its text in the app. */
enum class RadioError {
    /** 403 `watch_limit_reached` at the start or at a heartbeat. */
    WatchLimit,

    /** 409 `radio_capacity`: the server relays as many stations as it may. */
    Capacity,

    /** 502 `radio_unreachable` at the start, 410 at a heartbeat: the station does not answer. */
    Unreachable,

    /** 422 `radio_not_audio`: what the station sends is no sound. */
    NotAudio,

    /** The listen is gone (401, 404, the token refused): signed out, another profile, stopped by the administration. */
    Ended,

    /** No connection. */
    Network,

    /** Anything else. */
    Failed,
}

/** What the radio plays: the station, its state, what it plays now (`null`: no title known). */
data class RadioState(
    val station: RadioStation? = null,
    val starting: Boolean = false,
    val playing: Boolean = false,
    /** The user wants it to play (a pause by the user, a headphone or another app sets it false). */
    val wantsToPlay: Boolean = false,
    val buffering: Boolean = false,
    val error: RadioError? = null,
    /** The entry of `getRadioNow` that plays now. */
    val now: RadioNowEntry? = null,
) {
    val active: Boolean get() = station != null
}

/**
 * One station at a time (web `RadioProvider`): the start with `startRadioListen`, the live HLS
 * playlist with the token `rt` from its live edge, a heartbeat every `heartbeatSeconds` (also while
 * paused) and once at each play or pause, `endRadioListen` at a change of station, a stop and the
 * end. A start that answers after a newer start or a stop is ended at once. «Jetzt läuft» comes from
 * the server every [NOW_INTERVAL_MS] while the station plays and [nowWanted] says so.
 *
 * [scope] lives as long as the controller (its loops end with it); [finalScope] outlives it for the
 * last `endRadioListen`.
 */
class RadioController(
    private val scope: CoroutineScope,
    private val finalScope: CoroutineScope,
    private val server: RadioServer,
    private val engine: PlayerEngine,
    /**
     * Wall clock in milliseconds (also through deep sleep): a pause longer than a heartbeat starts a
     * new listen instead of resuming behind the live window or on an ended listen (m-2).
     */
    private val clock: () -> Long = System::currentTimeMillis,
) {
    private val mutable = MutableStateFlow(RadioState())

    /** When the player was paused; `null` while it plays. */
    private var pausedAt: Long? = null
    val state: StateFlow<RadioState> = mutable.asStateFlow()

    /** Whether «Jetzt läuft» is worth asking (a page or the mini player shows it). */
    @Volatile var nowWanted: Boolean = true

    private var listen: RadioListen? = null
    private var attempt = 0L
    private var loadId = NO_LOAD
    private var loads = 0L
    private var startJob: Job? = null
    private var loops: List<Job> = emptyList()

    init {
        scope.launch {
            engine.events.collect { signal -> if (signal.load == loadId) onEvent(signal.event) }
        }
    }

    /** Plays [station]; the station already chosen plays on (or again after an error). */
    fun start(station: RadioStation) {
        val current = mutable.value
        if (current.station?.id == station.id && current.error == null && (listen != null || current.starting)) {
            play()
            return
        }
        endListen()
        val mine = ++attempt
        mutable.value = RadioState(station = station, starting = true, wantsToPlay = true)
        pausedAt = null
        startJob = scope.launch {
            // To its end even when the controller goes: the late answer ends its listen (below).
            val result = withContext(NonCancellable) { server.start(station.id) }
            if (mine != attempt || !isActive) {
                // A newer start or a stop came first: this listen ends at once.
                (result as? ApiResult.Ok)?.value?.let { late -> finalScope.launch { server.end(late.listenId) } }
                return@launch
            }
            when (result) {
                is ApiResult.Ok -> onStarted(result.value)
                else -> fail(startError(result))
            }
        }
    }

    fun play() {
        val current = listen
        val station = mutable.value.station
        if (current == null) {
            station?.takeIf { mutable.value.error != null }?.let(::start)
            return
        }
        val paused = pausedAt
        if (station != null && paused != null && clock() - paused > current.heartbeatSeconds * 1000L) {
            // A long pause: the heartbeats stood (deep sleep), the live window moved on.
            endListen()
            start(station)
            return
        }
        engine.play()
    }

    fun pause() = engine.pause()

    fun togglePlay() = if (mutable.value.wantsToPlay && mutable.value.error == null) pause() else play()

    /** «Radio beenden»: the end of the listen, nothing plays. */
    fun stop() {
        attempt++
        endListen()
        mutable.value = RadioState()
    }

    /**
     * The controller is gone: stop and release the player. Returns the job of `endRadioListen`
     * (`null` without a listen), so a sign-out can wait for it (stage 7 M-2).
     */
    fun close(): Job? {
        attempt++
        val end = endListen()
        mutable.value = RadioState()
        engine.release()
        return end
    }

    private fun onStarted(started: RadioListen) {
        val url = server.absolute(started.playlistUrl)
        listen = started
        if (url == null) {
            fail(RadioError.Failed)
            return
        }
        mutable.update { it.copy(station = started.station, starting = false, error = null) }
        loadId = ++loads
        engine.load(EngineMedia(url = url, hls = true, title = started.station.name, artworkUrl = started.station.logo?.url?.let(server::absolute)), PlayerEngine.LIVE_EDGE, play = true, load = loadId)
        startLoops(started)
    }

    private fun startLoops(started: RadioListen) {
        loops.forEach { it.cancel() }
        val every = started.heartbeatSeconds.coerceAtLeast(MIN_HEARTBEAT_S) * 1000L
        loops = listOf(
            scope.launch {
                while (isActive) {
                    delay(every)
                    beat()
                }
            },
            scope.launch {
                while (isActive) {
                    if (nowWanted && mutable.value.playing && started.station.showsTitles) readNow(started.station.id)
                    delay(NOW_INTERVAL_MS)
                }
            },
        )
    }

    private suspend fun beat() {
        val current = listen ?: return
        val answer = server.heartbeat(current.listenId, mutable.value.wantsToPlay)
        if (listen !== current) return
        when (answer) {
            is ApiResult.Ok -> Unit
            is ApiResult.NotAllowed -> if (answer.code == WATCH_LIMIT) dropListen(RadioError.WatchLimit)
            is ApiResult.Failed -> if (answer.status == HTTP_GONE) dropListen(RadioError.Unreachable)
            ApiResult.NotFound, ApiResult.SignedOut -> dropListen(RadioError.Ended)
            // No answer: the next beat tries again (the server keeps the listen for its buffer).
            else -> Unit
        }
    }

    private suspend fun readNow(stationId: Long) {
        val answer = server.now(stationId) as? ApiResult.Ok ?: return
        if (mutable.value.station?.id != stationId) return
        // The newest entry: the app plays from the live edge (no time shift in this release).
        val entry = answer.value.propertyEntries.lastOrNull()?.takeIf { answer.value.live }
        mutable.update { it.copy(now = entry) }
    }

    private fun onEvent(event: EngineEvent) {
        when (event) {
            is EngineEvent.Playing -> mutable.update { it.copy(playing = event.playing, buffering = if (event.playing) false else it.buffering) }
            is EngineEvent.WantsToPlay -> {
                val changed = mutable.value.wantsToPlay != event.play
                mutable.update { it.copy(wantsToPlay = event.play) }
                pausedAt = if (event.play) null else pausedAt ?: clock()
                // A play or a pause tells the server at once (web: one beat on every change).
                if (changed && listen != null) scope.launch { beat() }
            }
            is EngineEvent.Buffering -> mutable.update { it.copy(buffering = event.buffering) }
            is EngineEvent.Failed -> {
                Log.i(TAG, "radio failed", "failure" to event.failure.name, "status" to event.httpStatus)
                when (event.failure) {
                    EngineFailure.Network -> fail(RadioError.Network)
                    EngineFailure.Refused -> fail(if (event.httpStatus == HTTP_GONE) RadioError.Unreachable else RadioError.Ended)
                    else -> fail(RadioError.Failed)
                }
            }
            EngineEvent.Ended -> fail(RadioError.Unreachable)
            else -> Unit
        }
    }

    private fun startError(result: ApiResult<*>): RadioError = when (result) {
        is ApiResult.NotAllowed -> if (result.code == WATCH_LIMIT) RadioError.WatchLimit else RadioError.Failed
        ApiResult.Offline -> RadioError.Network
        ApiResult.NotFound, ApiResult.SignedOut -> RadioError.Ended
        is ApiResult.Failed -> when (result.code) {
            "radio_capacity" -> RadioError.Capacity
            "radio_unreachable" -> RadioError.Unreachable
            "radio_not_audio" -> RadioError.NotAudio
            else -> RadioError.Failed
        }
        else -> RadioError.Failed
    }

    /** The listen ended on the server's side: no `endRadioListen` of ours. */
    private fun dropListen(error: RadioError) {
        listen = null
        fail(error)
    }

    private fun fail(error: RadioError) {
        endListen()
        mutable.update { it.copy(starting = false, playing = false, buffering = false, wantsToPlay = false, error = error) }
    }

    /**
     * Ends the current listen (if any) after its loops; the player stops. A start on its way is not
     * cancelled: the server may have opened the listen already, its answer ends it (see [start]).
     */
    private fun endListen(): Job? {
        startJob = null
        loops.forEach { it.cancel() }
        loops = emptyList()
        val end = listen?.let { old -> finalScope.launch { server.end(old.listenId) } }
        listen = null
        pausedAt = null
        engine.stop()
        loadId = NO_LOAD
        return end
    }

    companion object {
        private const val TAG = "radio"
        const val WATCH_LIMIT = "watch_limit_reached"
        const val HTTP_GONE = 410

        /** «Jetzt läuft» every 10 s (web `RADIO_NOW_INTERVAL_MS`). */
        const val NOW_INTERVAL_MS = 10_000L

        /** A heartbeat never more often than this, whatever the server names. */
        const val MIN_HEARTBEAT_S = 5

        /** Matches no event, not even those of [PlayerEngine.stop]. */
        private const val NO_LOAD = -1L
    }
}
