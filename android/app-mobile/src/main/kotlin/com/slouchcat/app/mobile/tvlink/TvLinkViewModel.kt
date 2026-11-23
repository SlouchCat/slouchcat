package com.slouchcat.app.mobile.tvlink

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.devices.DeviceSignOutError
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.pairing.PairingCode
import com.slouchcat.core.pairing.PhonePairing
import com.slouchcat.core.pairing.PhonePairingFlow
import com.slouchcat.core.pairing.ScannedCode
import com.slouchcat.core.session.DevicesRepository
import com.slouchcat.core.session.RelaySettings
import com.slouchcat.core.session.ServerAddress
import com.slouchcat.core.session.ServerRepository
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import okhttp3.HttpUrl

/** Why the scanner gave no code (code review M-2): each with a message that says what is true. */
enum class ScannerTrouble {
    /** Google's scanner module is still being downloaded: a second try in a moment works. */
    Loading,

    /** No Google Play services, or too old ones (build `play`). */
    NoPlayServices,

    /** No camera the app can use (build `foss`). */
    NoCamera,

    /** Anything else: the scanner did not open. */
    Failed,
}

/** Why a scanned code was not taken (concept 27 section 10 step 2). */
sealed interface ScanProblem {
    data object NotACode : ScanProblem

    /** A code of a newer TV app. */
    data object OtherVersion : ScanProblem

    /** A code of another relay than the one set here; its host is never offered as the one to set (security review L-4). */
    data object OtherRelay : ScanProblem

    data class Scanner(val trouble: ScannerTrouble) : ScanProblem

    /** A valid code, but no signed-in profile any more (code review L-5). */
    data object NoSession : ScanProblem
}

/** Why a pairing ended without «Verbunden». */
sealed interface TvLinkFailure {
    /** 409 or 410 and no own TV appeared: somebody laid a packet down first (section 10 step 5). */
    data object Occupied : TvLinkFailure

    /** 404 of the relay: the code ran out. */
    data object Expired : TvLinkFailure

    /** No TV under «Meine Geräte» within 2 minutes (section 10 step 6). */
    data object TimedOut : TvLinkFailure

    /** The relay did not answer (7.7): the TV can still be set up by hand. */
    data object RelayUnreachable : TvLinkFailure

    data object ServerOffline : TvLinkFailure

    /** 403 `primary_profile_required` (F-23). */
    data object PrimaryOnly : TvLinkFailure

    /** 403 otherwise. */
    data object NotAllowed : TvLinkFailure

    data class TooManyAttempts(val seconds: Long?) : TvLinkFailure

    /** Another key under the server's address (ADR 0040). */
    data object ServerUnconfirmed : TvLinkFailure

    /**
     * The page was left while it ran. [delivered]: the packet may be at the relay already (the `PUT`
     * had started, code review m-3), so the TV may be connected.
     */
    data class Left(val delivered: Boolean) : TvLinkFailure

    data object Failed : TvLinkFailure
}

/** The steps of the page (mockup `/tv`: intro, scan, confirm, run, done, fail). */
sealed interface TvLinkStep {
    data object Intro : TvLinkStep

    data object Scan : TvLinkStep

    /** «Diesen Fernseher mit deinem Konto verbinden?» with what the phone knows itself. */
    data class Confirm(val account: String, val server: String, val profile: String, val primary: Boolean, val plainHttp: Boolean) : TvLinkStep

    /** [done] of the three lines are done; the next one is busy. */
    data class Running(val done: Int) : TvLinkStep

    /** The line [at] failed. */
    data class Failed(val failure: TvLinkFailure, val at: Int) : TvLinkStep

    /**
     * A new TV is under «Meine Geräte». [device]: the one TV of this pairing, its name as the device
     * gave it; `null` when two new TVs appeared at once and the page cannot tell which is this one.
     */
    data class Done(val device: MyDevice?) : TvLinkStep
}

/** «Nicht mein Fernseher – abmelden» on «Verbunden» (security review L-2). */
data class NotMine(val device: MyDevice, val signingOut: Boolean = false, val error: DeviceSignOutError? = null)

data class TvLinkUiState(
    val step: TvLinkStep = TvLinkStep.Intro,
    /** Shown from the profile the server named primary; the server decides (403 `primary_profile_required`). */
    val primary: Boolean = false,
    val relay: HttpUrl = RelaySettings.DEFAULT,
    /** The TVs of the account («Verbunden» on the intro). */
    val tvs: List<MyDevice> = emptyList(),
    val scanProblem: ScanProblem? = null,
    /** Google's scanner module is being downloaded (the scan step says so). */
    val scannerLoading: Boolean = false,
    val notMine: NotMine? = null,
    /** The name of a TV just signed out with «Nicht mein Fernseher», for the note on the intro. */
    val signedOutTv: String? = null,
)

/**
 * «Fernseher verbinden» (concept 27 section 10; mockup `/tv`): scan, confirm, voucher, packet at the
 * relay, wait for the TV. The scanned code lives only in this ViewModel, in memory, for one pairing:
 * never in the saved state, never in a log. The pairing lives only while the page is seen
 * ([onShown], [onHidden]), as on the TV (K26 stage 2a review): leaving the page stops it.
 */
class TvLinkViewModel(
    private val session: SessionManager,
    private val relaySettings: RelaySettings,
    private val devices: DevicesRepository,
    private val health: ServerRepository,
    private val pairing: (relay: HttpUrl, server: HttpUrl) -> PhonePairingFlow,
) : ViewModel() {
    private val ready = session.state.value as? SessionState.Ready
    private val _state = MutableStateFlow(TvLinkUiState(primary = ready?.profile?.isPrimary == true))
    val state: StateFlow<TvLinkUiState> = _state.asStateFlow()

    /** The code of the confirm step; gone after the pairing or a cancel. */
    private var code: PairingCode? = null
    private var run: Job? = null

    /** From the moment the `PUT` starts: the packet may be at the relay. */
    private var delivered = false
    private var visible = false
    private val running get() = run?.isActive == true

    init {
        viewModelScope.launch { relaySettings.relay.collect { relay -> _state.update { it.copy(relay = relay) } } }
        loadTvs()
    }

    private fun loadTvs() {
        val server = session.server ?: return
        viewModelScope.launch {
            val list = (devices.devices(server) as? ApiResult.Ok)?.value ?: return@launch
            _state.update { s -> s.copy(tvs = list.filter { it.type == MyDevice.Type.TV }) }
        }
    }

    /** The page is seen (lifecycle started). Back from «Meine Geräte» after «Verbunden»: the intro again. */
    fun onShown() {
        if (visible) return
        visible = true
        if (_state.value.step is TvLinkStep.Done && _state.value.notMine == null) {
            _state.update { it.copy(step = TvLinkStep.Intro) }
            loadTvs()
        }
    }

    /** The page is hidden (another page, another app, the screen off): a running pairing stops. */
    fun onHidden() {
        if (!visible) return
        visible = false
        if (!running) return
        run?.cancel()
        run = null
        code = null
        val at = (_state.value.step as? TvLinkStep.Running)?.done ?: 0
        _state.update { it.copy(step = TvLinkStep.Failed(TvLinkFailure.Left(delivered), at)) }
    }

    fun onScan() {
        if (running) return
        code = null
        _state.update { it.copy(step = TvLinkStep.Scan, scanProblem = null, scannerLoading = false, signedOutTv = null) }
    }

    fun onScanClosed() {
        if (_state.value.step != TvLinkStep.Scan) return
        _state.update { it.copy(step = TvLinkStep.Intro, scannerLoading = false) }
    }

    /** Google's scanner module is downloading ([loading]) or ready again. */
    fun onScannerLoading(loading: Boolean) {
        if (_state.value.step != TvLinkStep.Scan) return
        _state.update { it.copy(scannerLoading = loading) }
    }

    fun onScannerTrouble(trouble: ScannerTrouble) {
        if (_state.value.step != TvLinkStep.Scan) return
        _state.update { it.copy(step = TvLinkStep.Intro, scanProblem = ScanProblem.Scanner(trouble), scannerLoading = false) }
    }

    /**
     * Whether [text] is worth ending the scanner for: a pairing code in any version and of any
     * relay. The own scanner skips other QR codes in the picture (code review m-4).
     */
    fun accepts(text: String): Boolean = ScannedCode.read(text, _state.value.relay) != ScannedCode.Read.NotACode

    /** The scanner read [text]: only a code of the relay in use goes on to the question. */
    fun onScanned(text: String) {
        val now = _state.value
        if (now.step != TvLinkStep.Scan) return
        when (val read = ScannedCode.read(text, now.relay)) {
            is ScannedCode.Read.Ok -> {
                val ready = session.state.value as? SessionState.Ready
                val server = session.savedServer
                if (ready == null || server == null) {
                    problem(ScanProblem.NoSession)
                    return
                }
                code = read.code
                _state.update {
                    it.copy(
                        step = TvLinkStep.Confirm(
                            account = ready.email,
                            server = ServerAddress.display(server.address),
                            profile = ready.profile.name,
                            primary = ready.profile.isPrimary,
                            plainHttp = !server.encrypted,
                        ),
                        scanProblem = null,
                        scannerLoading = false,
                    )
                }
            }
            ScannedCode.Read.NotACode -> problem(ScanProblem.NotACode)
            ScannedCode.Read.OtherVersion -> problem(ScanProblem.OtherVersion)
            is ScannedCode.Read.OtherRelay -> problem(ScanProblem.OtherRelay)
        }
    }

    private fun problem(problem: ScanProblem) = _state.update { it.copy(step = TvLinkStep.Intro, scanProblem = problem, scannerLoading = false) }

    /** «Abbrechen» on the question, «Verstanden» after a failure, «Fertig»: back to the start, the code is gone. */
    fun onBackToIntro() {
        if (running) return
        code = null
        _state.update { it.copy(step = TvLinkStep.Intro, scanProblem = null, notMine = null) }
    }

    /** «Verbinden»: voucher, packet, wait. */
    fun onConfirm() {
        val scanned = code ?: return
        if (_state.value.step !is TvLinkStep.Confirm || running) return
        val saved = session.savedServer ?: return
        val flow = pairing(_state.value.relay, saved.address)
        delivered = false
        _state.update { it.copy(step = TvLinkStep.Running(0)) }
        run = viewModelScope.launch {
            val issued = when (val issue = flow.issue(scanned)) {
                is PhonePairing.Issue.Ok -> issue.voucher
                else -> return@launch fail(issueFailure(issue), 0)
            }
            _state.update { it.copy(step = TvLinkStep.Running(1)) }
            val serverKey = saved.serverKey ?: (health.health(saved.address) as? ApiResult.Ok)?.value?.serverKey
                ?: return@launch fail(TvLinkFailure.Failed, 1)
            delivered = true
            when (flow.deliver(scanned, issued, saved.address, serverKey)) {
                PhonePairing.Deliver.Delivered -> Unit
                // The own packet may lie there already (a repeated request, an answer lost on the way):
                // the TV shows up within seconds then. Only without it the warning (security review L-1).
                PhonePairing.Deliver.Occupied -> {
                    if (linked(flow.awaitTv(issued, PhonePairing.LOOK_FOR_TV)) != null) fail(TvLinkFailure.Occupied, 1)
                    return@launch
                }
                PhonePairing.Deliver.RelayUnreachable -> {
                    if (linked(flow.awaitTv(issued, PhonePairing.LOOK_FOR_TV)) != null) fail(TvLinkFailure.RelayUnreachable, 1)
                    return@launch
                }
                PhonePairing.Deliver.Expired -> {
                    delivered = false
                    return@launch fail(TvLinkFailure.Expired, 1)
                }
                PhonePairing.Deliver.Failed -> return@launch fail(TvLinkFailure.Failed, 1)
            }
            code = null
            _state.update { it.copy(step = TvLinkStep.Running(2)) }
            linked(flow.awaitTv(issued))?.let { fail(it, 2) }
        }
    }

    /** «Verbunden» for [linked]; without a TV the failure to show instead. */
    private fun linked(linked: PhonePairing.Linked): TvLinkFailure? {
        code = null
        return when (linked) {
            is PhonePairing.Linked.Ok -> {
                val tv = linked.device
                _state.update { s -> s.copy(step = TvLinkStep.Done(tv), tvs = s.tvs.filterNot { it.id == tv.id } + tv) }
                null
            }
            PhonePairing.Linked.Several -> {
                _state.update { it.copy(step = TvLinkStep.Done(null)) }
                null
            }
            PhonePairing.Linked.TimedOut -> TvLinkFailure.TimedOut
            PhonePairing.Linked.Ended -> TvLinkFailure.Failed
        }
    }

    private fun fail(failure: TvLinkFailure, at: Int) {
        code = null
        _state.update { it.copy(step = TvLinkStep.Failed(failure, at)) }
    }

    private fun issueFailure(issue: PhonePairing.Issue): TvLinkFailure = when (issue) {
        is PhonePairing.Issue.Ok -> TvLinkFailure.Failed
        PhonePairing.Issue.PrimaryOnly -> TvLinkFailure.PrimaryOnly
        is PhonePairing.Issue.NotAllowed -> TvLinkFailure.NotAllowed
        is PhonePairing.Issue.TooManyAttempts -> TvLinkFailure.TooManyAttempts(issue.seconds)
        PhonePairing.Issue.Offline -> TvLinkFailure.ServerOffline
        PhonePairing.Issue.ServerUnconfirmed -> TvLinkFailure.ServerUnconfirmed
        PhonePairing.Issue.SessionEnded, PhonePairing.Issue.Failed -> TvLinkFailure.Failed
    }

    /** «Nicht mein Fernseher – abmelden»: the question for the TV of «Verbunden». */
    fun onNotMine() {
        val tv = (_state.value.step as? TvLinkStep.Done)?.device ?: return
        _state.update { it.copy(notMine = NotMine(tv)) }
    }

    fun onNotMineDismiss() {
        if (_state.value.notMine?.signingOut == true) return
        _state.update { it.copy(notMine = null) }
    }

    /** Signs exactly that TV out (`signOutMyDevice`); [unnamed] names a TV without a name. */
    fun onNotMineConfirm(unnamed: String) {
        val asking = _state.value.notMine ?: return
        if (asking.signingOut) return
        val server = session.server ?: return
        _state.update { it.copy(notMine = asking.copy(signingOut = true, error = null)) }
        viewModelScope.launch {
            val error = when (val result = devices.signOut(server, asking.device.id)) {
                is ApiResult.Ok, ApiResult.NotFound -> null
                is ApiResult.NotAllowed -> if (result.code == PRIMARY_REQUIRED) DeviceSignOutError.PrimaryOnly else DeviceSignOutError.Failed
                ApiResult.Offline -> DeviceSignOutError.Unreachable
                else -> DeviceSignOutError.Failed
            }
            _state.update { s ->
                if (error != null) {
                    s.copy(notMine = asking.copy(signingOut = false, error = error))
                } else {
                    s.copy(
                        step = TvLinkStep.Intro,
                        notMine = null,
                        tvs = s.tvs.filterNot { it.id == asking.device.id },
                        signedOutTv = asking.device.name?.takeIf { it.isNotBlank() } ?: unnamed,
                    )
                }
            }
        }
    }

    private companion object {
        const val PRIMARY_REQUIRED = "primary_profile_required"
    }
}
