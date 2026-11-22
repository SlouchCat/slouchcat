package com.slouchcat.core.player

import com.slouchcat.core.api.generated.models.PlaybackCapabilities
import com.slouchcat.core.api.generated.models.RadioStation
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull
import kotlinx.coroutines.joinAll

/** Where a title plays from: the server (`online`) or a download (stage 6), each with its own [PlaybackServer]. */
data class AudioSource(val key: String, val server: () -> PlaybackServer) {
    override fun toString() = "AudioSource($key)"
}

/** The last title of a process that died (concept 27 8.3): shown paused, plays again from the server's position. */
data class LastAudio(
    val itemId: String,
    val title: String,
    val byline: String?,
    /** The account and profile it belongs to (the app shows it only to that profile). */
    val owner: String,
    /** It played from a download: it plays again from the device (m-11). */
    val offline: Boolean = false,
)

/** Remembers [LastAudio] across the death of the process (the app keeps it in a file; tests in memory). */
interface AudioMemory {
    suspend fun read(): LastAudio?

    suspend fun write(last: LastAudio?)
}

/** What the audio of the app plays now; `null`: nothing (the mini player draws nothing). */
sealed interface AudioNow {
    /** An audiobook or a podcast episode. [offline]: from a download. */
    data class Title(val state: PlayerState, val offline: Boolean, val sleep: SleepState?) : AudioNow

    data class Radio(val state: RadioState) : AudioNow

    /** The title of a process that died: nothing loaded, «Abspielen» starts it again. */
    data class Restored(val last: LastAudio) : AudioNow
}

/**
 * The sound of the app across its pages (concept 27 8.3; the phone's main case: it plays on with the
 * screen off and another app in front): one audiobook, podcast episode or station at a time. A
 * title runs a [PlaybackController] (progress every 10 s, the session state, the speed per title),
 * a station a [RadioController] (heartbeat, `endRadioListen`). Starting one ends the other (web:
 * any title stops the radio). Each gets its own player and its own child scope, so nothing of an
 * ended one runs on (code review stage 5, «verwaiste Coroutines»).
 *
 * Nothing starts here without a call from the user's hand (a button of a page, the mini player,
 * the media session): the app never starts sound by itself, also not after the death of the
 * process ([restore] only shows the last title). The phone app lifts [engine] into its
 * `MediaSessionService`; the TV app uses it without one (no sound in the background, K26 5.6).
 *
 * [scope] is the app's scope (main thread); every call comes from the main thread.
 */
class AudioPlayback(
    private val scope: CoroutineScope,
    private val engines: (EngineKind) -> PlayerEngine,
    private val radioServer: () -> RadioServer,
    private val capabilities: suspend (Narrowing) -> PlaybackCapabilities,
    private val settings: Flow<DeviceSettings>,
    private val metered: MeteredNetwork = MeteredNetwork { false },
    private val clock: () -> Long,
    private val memory: AudioMemory? = null,
    /** Who listens now (account and profile); the [LastAudio] of another owner is not shown. */
    private val owner: () -> String? = { null },
) {
    private class Active(val scope: CoroutineScope, val engine: PlayerEngine, val title: PlaybackController?, val radio: RadioController?, val source: AudioSource?)

    private var active: Active? = null
    private val sleepTimer = SleepTimer()
    private var remembered: String? = null

    private val mutable = MutableStateFlow<AudioNow?>(null)
    val now: StateFlow<AudioNow?> = mutable.asStateFlow()

    private val engineFlow = MutableStateFlow<PlayerEngine?>(null)

    /** The player of what plays (the media session of the phone's service follows it). */
    val engine: StateFlow<PlayerEngine?> = engineFlow.asStateFlow()

    /** The controller of the title that plays; `null` for none or a station. */
    val title: PlaybackController? get() = active?.title

    /** The controller of the station that plays; `null` for none or a title. */
    val radio: RadioController? get() = active?.radio

    /** Whether a page or the mini player is on screen («Jetzt läuft» is asked only then). */
    var visible: Boolean = true
        set(value) {
            field = value
            active?.radio?.nowWanted = value
        }

    /**
     * «Abspielen» of [itemId] from [source]: at [startAtMs], from the start with [fromStart], else where
     * the server resumes. The title already loaded from the same source plays on from where it stands
     * (or jumps to [startAtMs]) without a new start, so its session, the time played and the speed stay
     * (m-1). Only a button that plays calls this; opening the audio player only shows (M-1).
     */
    fun playTitle(itemId: String, source: AudioSource, startAtMs: Long? = null, fromStart: Boolean = false) {
        val current = active
        val controller = if (current?.title != null && current.source?.key == source.key) {
            current.title
        } else {
            retire(clearEngine = false)
            val child = child()
            val engine = engines(EngineKind.Speech)
            val made = PlaybackController(child, scope, source.server(), engine, capabilities, settings, metered, clock)
            active = Active(child, engine, made, null, source)
            engineFlow.value = engine
            follow(child, made, engine, source)
            made
        }
        val state = controller.state.value
        if (state.item?.id == itemId && state.error == null && !state.loading) {
            val at = if (fromStart) 0L else startAtMs
            if (at != null) controller.seekTo(at)
            controller.play()
        } else {
            sleepTimer.cancel()
            controller.start(itemId, startAtMs, fromStart)
        }
    }

    /** Plays [station] (web `radio.play`): a title that plays ends first. */
    fun playRadio(station: RadioStation) {
        val current = active
        val controller = if (current?.radio != null) {
            current.radio
        } else {
            retire(clearEngine = false)
            val child = child()
            val engine = engines(EngineKind.Music)
            val made = RadioController(child, scope, radioServer(), engine)
            made.nowWanted = visible
            active = Active(child, engine, null, made, null)
            engineFlow.value = engine
            // At once, not one frame later with the title that ended (L-8).
            mutable.value = AudioNow.Radio(RadioState(station = station, starting = true, wantsToPlay = true))
            child.launch { made.state.collect { mutable.value = if (it.active) AudioNow.Radio(it) else null } }
            made
        }
        sleepTimer.cancel()
        controller.start(station)
    }

    /**
     * Play or pause what is loaded; a restored title starts again (from the download when it played
     * from one); a title that failed tries again (M-5).
     */
    fun togglePlay() {
        when (val now = mutable.value) {
            is AudioNow.Restored -> {
                val source = (if (now.last.offline) restoredOfflineSource?.invoke() else restoredSource) ?: return
                playTitle(now.last.itemId, source)
            }
            else -> {
                val title = active?.title
                if (title != null && title.state.value.error != null) title.retry() else title?.togglePlay()
                active?.radio?.togglePlay()
            }
        }
    }

    /** Where a restored title plays from (the app sets the server's source). */
    var restoredSource: AudioSource? = null

    /** Where a restored title of a download plays from (`null`: none for the profile now). */
    var restoredOfflineSource: (() -> AudioSource?)? = null

    /** «Schlaftimer» of the audio player. */
    fun setSleep(choice: SleepChoice) {
        val controller = active?.title ?: return
        val s = controller.state.value
        sleepTimer.set(choice, active?.engine?.positionMs ?: s.positionMs, s.chapters, s.durationMs, s.speed)
        publishTitle(s)
    }

    /** «Vorheriges Kapitel» (web `previousChapterStart`). */
    fun previousChapter() {
        val controller = active?.title ?: return
        val s = controller.state.value
        controller.seekTo(AudioRules.previousChapterStart(s.chapters, s.positionMs))
    }

    /** «Nächstes Kapitel»; nothing in the last one. */
    fun nextChapter() {
        val controller = active?.title ?: return
        val s = controller.state.value
        AudioRules.nextChapterStart(s.chapters, s.positionMs)?.let(controller::seekTo)
    }

    /**
     * Ends everything that plays (the cross of the mini player, a video, a change of the listener) and
     * forgets the last title, also one restored after the death of the process (L-4). The last
     * progress and the end of the session go out in the app's scope; [stopAndFlush] waits for them.
     * [forget] `false` (a video, the swipe of the paused app) keeps «Zuletzt gehört» for the next start.
     */
    fun stop(forget: Boolean = true) {
        finish(forget)
    }

    /**
     * [stop], then waits up to [timeoutMs] for the last progress, `stopPlayback` and `endRadioListen`
     * (stage 7 M-2): before the sign-out and before a profile change, while the token still counts.
     */
    suspend fun stopAndFlush(timeoutMs: Long = FLUSH_MS) {
        val jobs = finish()
        withTimeoutOrNull(timeoutMs) { jobs.joinAll() }
    }

    /** Ends what plays; [forget]: «Zuletzt gehört» goes too (the cross, a change of the listener). */
    private fun finish(forget: Boolean = true): List<Job> {
        val jobs = retire()
        sleepTimer.cancel()
        mutable.value = null
        remembered = null
        if (forget) memory?.let { store -> scope.launch { store.write(null) } }
        return jobs
    }

    /** After the death of the process: shows [last] paused when it belongs to the listener now and nothing plays. */
    suspend fun restore() {
        if (active != null || mutable.value != null) return
        val last = memory?.read() ?: return
        if (last.owner != owner()) return
        remembered = "${last.owner}|${last.itemId}"
        mutable.value = AudioNow.Restored(last)
    }

    private fun child() = CoroutineScope(scope.coroutineContext + SupervisorJob(scope.coroutineContext[Job]))

    private fun follow(child: CoroutineScope, controller: PlaybackController, engine: PlayerEngine, source: AudioSource) {
        child.launch {
            controller.state.collect { s ->
                publishTitle(s, source)
                s.item?.let { item -> remember(LastAudio(item.id, item.title, item.author ?: item.seriesTitle, owner() ?: return@let, offline = source.key.startsWith(OFFLINE))) }
            }
        }
        child.launch {
            while (isActive) {
                delay(SLEEP_TICK_MS)
                val s = controller.state.value
                if (sleepTimer.state == null) continue
                // The player's own position: the state lags behind by up to a tick.
                if (sleepTimer.tick(clock(), s.playing, engine.positionMs, s.chapters, s.durationMs, s.speed)) controller.pause()
                publishTitle(controller.state.value, source)
            }
        }
    }

    private fun publishTitle(s: PlayerState, source: AudioSource? = active?.source) {
        if (active?.title == null) return
        mutable.value = AudioNow.Title(s, offline = source?.key?.startsWith(OFFLINE) == true, sleep = sleepTimer.state)
    }

    /** Keeps the last title for the next process; only when it changes (one write per title). */
    private fun remember(last: LastAudio?) {
        val key = last?.let { "${it.owner}|${it.itemId}" }
        if (key == remembered) return
        remembered = key
        val store = memory ?: return
        scope.launch { store.write(last) }
    }

    /**
     * Closes what plays: the last progress or `endRadioListen` in the app's scope, then its loops end.
     * Returns the jobs of those last requests.
     */
    private fun retire(clearEngine: Boolean = true): List<Job> {
        val old = active ?: return emptyList()
        active = null
        // A change between two players keeps the flow on the old one until the new one is there (no
        // `null` in between: the media session would end and start again).
        if (clearEngine) engineFlow.value = null
        val jobs = listOfNotNull(old.title?.close(), old.radio?.close())
        old.scope.cancel()
        return jobs
    }

    companion object {
        /** The key of a download's source starts with it (`offline:<account>`). */
        const val OFFLINE = "offline"
        const val SLEEP_TICK_MS = 1_000L

        /** The longest wait for the last requests before a sign-out or a profile change. */
        const val FLUSH_MS = 3_000L
    }
}
