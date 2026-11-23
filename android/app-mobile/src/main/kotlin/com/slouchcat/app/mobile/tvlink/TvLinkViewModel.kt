package com.slouchcat.app.mobile.tvlink

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.pairing.PairingCode
import com.slouchcat.core.pairing.PhonePairing
import com.slouchcat.core.pairing.PhonePairingFlow
import com.slouchcat.core.pairing.ScannedCode
import com.slouchcat.core.session.DevicesRepository
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

/** Why a scanned code was not taken (concept 27 section 10 step 2). */
sealed interface ScanProblem {
    data object NotACode : ScanProblem

    /** A code of a newer TV app. */
    data object OtherVersion : ScanProblem

    /** A code of another relay than the one set here; [host] only to show it. */
    data class OtherRelay(val host: String) : ScanProblem

    /** No scanner on this device (no camera, no Google Play services). */
    data object Unavailable : ScanProblem
}

/** Why a pairing ended without «Verbunden». */
sealed interface TvLinkFailure {
    /** 409 or 410: somebody laid a packet down first (section 10 step 5). */
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

    /** The page was left while it ran; [delivered]: the packet was at the relay already. */
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

    /** The TV appeared under «Meine Geräte»; [name] as it named itself. */
    data class Done(val name: String?) : TvLinkStep
}

data class TvLinkUiState(
    val step: TvLinkStep = TvLinkStep.Intro,
    /** Shown from the profile the server named primary; the server decides (403 `primary_profile_required`). */
    val primary: Boolean = false,
    val relay: HttpUrl = RelaySettings.DEFAULT,
    /** The TVs of the account («Verbunden» on the intro). */
    val tvs: List<MyDevice> = emptyList(),
    val scanProblem: ScanProblem? = null,
    val editingRelay: Boolean = false,
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
    private var delivered = false
    private var visible = false

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
        if (_state.value.step is TvLinkStep.Done) {
            _state.update { it.copy(step = TvLinkStep.Intro) }
            loadTvs()
        }
    }

    /** The page is hidden (another page, another app, the screen off): a running pairing stops. */
    fun onHidden() {
        if (!visible) return
        visible = false
        val running = run ?: return
        if (!running.isActive) return
        running.cancel()
        run = null
        code = null
        val at = (_state.value.step as? TvLinkStep.Running)?.done ?: 0
        _state.update { it.copy(step = TvLinkStep.Failed(TvLinkFailure.Left(delivered), at)) }
    }

    fun onScan() {
        if (run?.isActive == true) return
        code = null
        _state.update { it.copy(step = TvLinkStep.Scan, scanProblem = null) }
    }

    fun onScanClosed() {
        if (_state.value.step != TvLinkStep.Scan) return
        _state.update { it.copy(step = TvLinkStep.Intro) }
    }

    fun onScannerUnavailable() {
        if (_state.value.step != TvLinkStep.Scan) return
        _state.update { it.copy(step = TvLinkStep.Intro, scanProblem = ScanProblem.Unavailable) }
    }

    /** The scanner read [text]: only a code of the relay in use goes on to the question. */
    fun onScanned(text: String) {
        val now = _state.value
        if (now.step != TvLinkStep.Scan) return
        when (val read = ScannedCode.read(text, now.relay)) {
            is ScannedCode.Read.Ok -> {
                val ready = session.state.value as? SessionState.Ready
                val server = session.savedServer
                if (ready == null || server == null) {
                    _state.update { it.copy(step = TvLinkStep.Intro) }
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
                    )
                }
            }
            ScannedCode.Read.NotACode -> problem(ScanProblem.NotACode)
            ScannedCode.Read.OtherVersion -> problem(ScanProblem.OtherVersion)
            is ScannedCode.Read.OtherRelay -> problem(ScanProblem.OtherRelay(read.relay))
        }
    }

    private fun problem(problem: ScanProblem) = _state.update { it.copy(step = TvLinkStep.Intro, scanProblem = problem) }

    /** «Abbrechen» on the question, «Verstanden» after a failure: back to the start, the code is gone. */
    fun onBackToIntro() {
        if (run?.isActive == true) return
        code = null
        _state.update { it.copy(step = TvLinkStep.Intro, scanProblem = null) }
    }

    /** «Verbinden»: voucher, packet, wait. */
    fun onConfirm() {
        val scanned = code ?: return
        if (_state.value.step !is TvLinkStep.Confirm || run?.isActive == true) return
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
            when (flow.deliver(scanned, issued, saved.address, serverKey)) {
                PhonePairing.Deliver.Delivered -> delivered = true
                PhonePairing.Deliver.Occupied -> return@launch fail(TvLinkFailure.Occupied, 1)
                PhonePairing.Deliver.Expired -> return@launch fail(TvLinkFailure.Expired, 1)
                PhonePairing.Deliver.RelayUnreachable -> return@launch fail(TvLinkFailure.RelayUnreachable, 1)
                PhonePairing.Deliver.Failed -> return@launch fail(TvLinkFailure.Failed, 1)
            }
            code = null
            _state.update { it.copy(step = TvLinkStep.Running(2)) }
            when (val linked = flow.awaitTv(issued)) {
                is PhonePairing.Linked.Ok -> {
                    val tv = linked.device
                    _state.update { s -> s.copy(step = TvLinkStep.Done(tv.name?.takeIf { it.isNotBlank() }), tvs = s.tvs.filterNot { it.id == tv.id } + tv) }
                }
                PhonePairing.Linked.TimedOut -> fail(TvLinkFailure.TimedOut, 2)
            }
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

    fun onEditRelay(open: Boolean) {
        if (run?.isActive == true) return
        _state.update { it.copy(editingRelay = open) }
    }

    /** A new relay (`null`: the built-in one); a code read before belongs to the old one. */
    fun onRelay(relay: HttpUrl?) {
        viewModelScope.launch { relaySettings.set(relay) }
        code = null
        _state.update { it.copy(editingRelay = false, step = if (it.step is TvLinkStep.Confirm) TvLinkStep.Intro else it.step, scanProblem = null) }
    }
}
