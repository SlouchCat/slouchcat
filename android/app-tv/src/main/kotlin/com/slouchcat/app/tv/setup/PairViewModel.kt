package com.slouchcat.app.tv.setup

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.session.RelaySettings
import com.slouchcat.app.tv.common.takeServer
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.pairing.PairingFlow
import com.slouchcat.core.pairing.PairingPacket
import com.slouchcat.core.pairing.RedeemAnswer
import com.slouchcat.core.pairing.TvPairing
import com.slouchcat.core.session.DeviceDescription
import com.slouchcat.core.session.ParsedAddress
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerAddress
import com.slouchcat.core.session.ServerInfo
import com.slouchcat.core.session.ServerProbe
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.HttpUrl
import java.time.Duration
import java.time.Instant

/** Why the setup with the phone stopped (mockup variants «Dienst nicht erreichbar», «Adresse nicht erreichbar»). */
enum class PairProblem {
    /** The relay answers nothing: no internet, or another relay address (advanced settings). */
    RelayUnreachable,

    /** The relay has too many open meeting points. */
    RelayBusy,

    /** The TV does not reach the address the phone sent (a VPN of the phone, another network). */
    ServerUnreachable,

    /** The server is older than the app needs. */
    ServerTooOld,

    /** The server's answer does not fit the packet's key: the TV does not connect (ADR 0040). */
    Untrusted,
    AppTooOld,
    TooManyAttempts,
    Failed,
}

/** Where the page «Mit dem Handy einrichten» is. */
sealed interface PairPhase {
    /** A new code is being made. */
    data object Starting : PairPhase

    /** The QR code is shown; [secondsLeft] until it runs out. */
    data class Waiting(val code: String, val secondsLeft: Long) : PairPhase {
        /** The code carries the meeting's id and the TV's key (K-8): never in a log. */
        override fun toString() = "Waiting(secondsLeft=$secondsLeft)"
    }

    /** The code ran out (also after the automatic renewals). */
    data object Expired : PairPhase

    /** A packet came: the TV checks the server and redeems the voucher. */
    data object Connecting : PairPhase

    /**
     * Paired (K-9): the whole server address (scheme, host, port), encryption, account and the TV's
     * name, before anything is stored. «Weiter» stores the session, «Nicht mein Konto» ends it at
     * once. [autoSeconds]: until it goes on by itself.
     */
    data class Linked(val server: String, val encrypted: Boolean, val account: String, val tvName: String, val autoSeconds: Long?) : PairPhase

    data class Problem(
        val kind: PairProblem,
        /** The address the phone sent ([PairProblem.ServerUnreachable]). */
        val address: String? = null,
        val serverVersion: String? = null,
        val requiredVersion: String? = null,
        val seconds: Long? = null,
    ) : PairPhase
}

data class PairUiState(
    val phase: PairPhase = PairPhase.Starting,
    /** The relay in use, as a host (shown when it does not answer). */
    val relay: String = "",
    /** «Weiter» is storing the session. */
    val storing: Boolean = false,
    /** Another relay than the built-in one is set (advanced settings): the page says so. */
    val ownRelay: Boolean = false,
)

/**
 * «Mit dem Handy einrichten» (concept 26 4.3, mockup `/setup/phone`, photo 03): one-time key and
 * pickup secret, the meeting point, the QR code, polling every two seconds; a new code shortly
 * before the five minutes run out (at most [renewals] times, then «abgelaufen»). The packet's server
 * is checked (`getHealth`, «Server weist sich aus» with the packet's key over HTTP), the voucher
 * redeemed, the answer's signature checked, the token opened. Key, secret and token live only in
 * this ViewModel; the session is stored with «Weiter».
 *
 * The pairing lives only while its page is seen (review M-1): [onVisible] makes a new code,
 * [onHidden] (way by hand, Back, Home, the app in the background) stops polling, clock and the
 * automatic «Weiter», gives the meeting point up, forgets key and secret and ends a redeemed but
 * unconfirmed device session. Nothing is redeemed or stored unless the page shows it.
 */
class PairViewModel(
    private val relay: suspend () -> HttpUrl,
    private val flowFor: (HttpUrl) -> PairingFlow,
    private val probe: ServerProbe,
    private val servers: ServerStore,
    private val session: SessionManager,
    private val device: DeviceDescription,
    private val systemVersion: String,
    /** Where the meeting point is given up when the page goes (outlives the ViewModel). */
    private val cleanup: CoroutineScope,
    private val now: () -> Instant = Instant::now,
    private val renewals: Int = RENEWALS,
) : ViewModel() {
    private val _state = MutableStateFlow(PairUiState())
    val state: StateFlow<PairUiState> = _state.asStateFlow()

    private var flow: PairingFlow? = null
    private var meeting: TvPairing.Meeting? = null
    private var running: Job? = null
    private var ticker: Job? = null
    private var auto: Job? = null
    private var visible = false

    /** New codes made since the last [start]; kept across a voucher that ran out (no endless polling). */
    private var renewed = 0

    /** What «Weiter» stores; only in memory. */
    private class Pending(val info: ServerInfo, val serverKey: String, val answer: RedeemAnswer) {
        override fun toString() = "Pending(${info.address})"
    }

    private var pending: Pending? = null

    /** The page is seen (lifecycle started): a new code, unless one is already running. */
    fun onVisible() {
        if (visible) return
        visible = true
        if (_state.value.storing) return
        start()
    }

    /** The page is gone or the app is in the background: everything of this pairing ends. */
    fun onHidden() {
        if (!visible) return
        visible = false
        if (_state.value.storing) return
        running?.cancel()
        ticker?.cancel()
        auto?.cancel()
        val open = meeting
        meeting = null
        val unconfirmed = pending
        pending = null
        val pairing = flow
        _state.update { it.copy(phase = PairPhase.Starting) }
        // The ViewModel's scope may be cancelled next: give up in the app's scope.
        cleanup.launch { pairing.giveUp(open, unconfirmed) }
    }

    /** «Neuen Code anzeigen», «Nochmals versuchen». */
    fun onNewCode() = start()

    private fun start() {
        if (!visible) return
        running?.cancel()
        auto?.cancel()
        pending = null
        renewed = 0
        running = viewModelScope.launch {
            releaseMeeting()
            val address = relay()
            _state.update { PairUiState(relay = address.host, ownRelay = address != RelaySettings.DEFAULT) }
            val pairing = flowFor(address).also { flow = it }
            run(pairing)
        }
    }

    private suspend fun run(pairing: PairingFlow) {
        while (true) {
            _state.update { it.copy(phase = PairPhase.Starting) }
            val opened = when (val started = pairing.start()) {
                is TvPairing.Started.Ok -> started.meeting
                TvPairing.Started.RelayUnreachable -> return problem(PairProblem.RelayUnreachable)
                TvPairing.Started.RelayBusy -> return problem(PairProblem.RelayBusy)
                TvPairing.Started.Failed -> return problem(PairProblem.Failed)
            }
            meeting = opened
            startTicker(opened)
            val renewAt = if (renewed < renewals) opened.expiresAt.minus(RENEW_BEFORE) else opened.expiresAt
            val waited = pairing.awaitPacket(opened, renewAt)
            ticker?.cancel()
            when (waited) {
                is TvPairing.Waited.Packet -> return connect(pairing, opened, waited.packet)
                TvPairing.Waited.Expired, TvPairing.Waited.Invalid -> {
                    releaseMeeting()
                    if (renewed >= renewals) {
                        _state.update { it.copy(phase = PairPhase.Expired) }
                        return
                    }
                    renewed++
                    Log.i(TAG, "new code", "renewal" to renewed)
                }
                TvPairing.Waited.RelayUnreachable -> {
                    releaseMeeting()
                    return problem(PairProblem.RelayUnreachable)
                }
            }
        }
    }

    private fun startTicker(opened: TvPairing.Meeting) {
        ticker?.cancel()
        ticker = viewModelScope.launch {
            while (true) {
                val left = Duration.between(now(), opened.expiresAt).seconds.coerceAtLeast(0)
                _state.update { it.copy(phase = PairPhase.Waiting(opened.code.text, left)) }
                if (left == 0L) break
                delay(SECOND)
            }
        }
    }

    private suspend fun connect(pairing: PairingFlow, opened: TvPairing.Meeting, packet: PairingPacket) {
        _state.update { it.copy(phase = PairPhase.Connecting) }
        val address = (ServerAddress.parse(packet.server) as? ParsedAddress.Valid)?.url
        if (address == null) {
            releaseMeeting()
            return problem(PairProblem.ServerUnreachable, address = packet.server)
        }
        val shown = ServerAddress.display(address)
        val info = when (val probed = probe.probe(address)) {
            is ProbeResult.Found -> probed.info
            is ProbeResult.TooOld -> {
                releaseMeeting()
                return _state.update { it.copy(phase = PairPhase.Problem(PairProblem.ServerTooOld, shown, probed.serverVersion, probed.required)) }
            }
            else -> {
                releaseMeeting()
                return problem(PairProblem.ServerUnreachable, address = shown)
            }
        }
        val key = packet.serverKey
        if (key == null || info.serverKey != key) {
            // Another device under the address than the one the phone vouched for.
            releaseMeeting()
            Log.w(TAG, "server key differs from the packet")
            return problem(PairProblem.Untrusted)
        }
        val redeemed = pairing.redeem(opened, packet, address, device.name, systemVersion)
        releaseMeeting()
        when (redeemed) {
            is TvPairing.Redeem.Ok -> {
                val done = Pending(info, key, redeemed.answer)
                if (!visible) {
                    // Hidden while redeeming: the session is not kept.
                    cleanup.launch { pairing.giveUp(null, done) }
                    return
                }
                pending = done
                linked(info, redeemed.answer)
            }
            TvPairing.Redeem.VoucherInvalid -> {
                // The phone's voucher ran out or was used: a new code (concept 26 4.3), counted
                // with the renewals, so a server that always says 410 does not keep the TV polling.
                Log.i(TAG, "voucher invalid, new code")
                if (renewed >= renewals) {
                    _state.update { it.copy(phase = PairPhase.Expired) }
                    return
                }
                renewed++
                run(pairing)
            }
            TvPairing.Redeem.ServerUnreachable -> problem(PairProblem.ServerUnreachable, address = shown)
            TvPairing.Redeem.Untrusted -> problem(PairProblem.Untrusted)
            TvPairing.Redeem.AppTooOld -> problem(PairProblem.AppTooOld)
            is TvPairing.Redeem.TooManyAttempts -> _state.update { it.copy(phase = PairPhase.Problem(PairProblem.TooManyAttempts, seconds = redeemed.seconds ?: DEFAULT_PAUSE)) }
            TvPairing.Redeem.Failed -> problem(PairProblem.Failed)
        }
    }

    private fun linked(info: ServerInfo, answer: RedeemAnswer) {
        val base = PairPhase.Linked(ServerAddress.display(info.address), info.encrypted, answer.accountEmail, device.name, AUTO_SECONDS)
        _state.update { it.copy(phase = base) }
        auto = viewModelScope.launch {
            var left = AUTO_SECONDS
            while (left > 0) {
                _state.update { s -> s.copy(phase = (s.phase as? PairPhase.Linked)?.copy(autoSeconds = left) ?: s.phase) }
                delay(SECOND)
                left--
            }
            onContinue()
        }
    }

    /** Any key on the page «Verbunden»: the automatic «Weiter» stops (mockup: `lastInput > linkedAt`). */
    fun onInput() {
        if (auto?.isActive != true) return
        auto?.cancel()
        _state.update { s -> s.copy(phase = (s.phase as? PairPhase.Linked)?.copy(autoSeconds = null) ?: s.phase) }
    }

    /** «Weiter zu den Profilen»: stores server and session; the new session epoch opens the profile choice. */
    fun onContinue() {
        val done = pending ?: return
        if (_state.value.storing || !visible) return
        auto?.cancel()
        _state.update { it.copy(storing = true) }
        viewModelScope.launch {
            val server = SavedServer(done.info.address, done.info.version, done.info.userCertificate, done.serverKey)
            // The session must see this server first: it forgets the session of any other one.
            if (!session.takeServer(servers, server)) {
                Log.e(TAG, "the session did not take the server")
                _state.update { it.copy(storing = false, phase = PairPhase.Problem(PairProblem.Failed)) }
                return@launch
            }
            runCatching { session.signedIn(done.answer.token, done.answer.deviceId, done.answer.accountEmail, null) }
                .onSuccess { pending = null }
                .onFailure {
                    Log.e(TAG, "token not stored", error = it)
                    _state.update { s -> s.copy(storing = false, phase = PairPhase.Problem(PairProblem.Failed)) }
                }
        }
    }

    /** «Nicht mein Konto, trennen» (K-9): the device session ends at once, a new code follows. */
    fun onUndo(onDone: (confirmed: Boolean) -> Unit = {}) {
        if (_state.value.storing) return
        val done = pending ?: return
        pending = null
        auto?.cancel()
        running?.cancel()
        running = viewModelScope.launch {
            val confirmed = flow?.discard(done.info.address, done.serverKey, done.answer.token) ?: false
            onDone(confirmed)
            renewed = 0
            flow?.let { run(it) }
        }
    }

    private fun problem(kind: PairProblem, address: String? = null) {
        _state.update { it.copy(phase = PairPhase.Problem(kind, address)) }
    }

    private suspend fun releaseMeeting() {
        val open = meeting ?: return
        meeting = null
        withContext(NonCancellable) { flow?.close(open) }
    }

    override fun onCleared() {
        val open = meeting
        meeting = null
        val unconfirmed = pending
        pending = null
        val pairing = flow
        // The ViewModel's scope is gone: give up in the app's scope.
        if (open != null || unconfirmed != null) cleanup.launch { pairing.giveUp(open, unconfirmed) }
    }

    /** Closes the meeting point and ends a device session nobody confirmed (review N-3). */
    private suspend fun PairingFlow?.giveUp(open: TvPairing.Meeting?, unconfirmed: Pending?) {
        val pairing = this ?: return
        open?.let { pairing.close(it) }
        unconfirmed?.let { pairing.discard(it.info.address, it.serverKey, it.answer.token) }
    }

    private companion object {
        const val TAG = "pair"
        const val RENEWALS = 2
        val RENEW_BEFORE: Duration = Duration.ofSeconds(30)
        const val SECOND = 1_000L
        const val AUTO_SECONDS = 10L
        const val DEFAULT_PAUSE = 60L
    }
}
