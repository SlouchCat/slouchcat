package com.slouchcat.app.tv.setup

import com.slouchcat.app.tv.testing.FakeProbe
import com.slouchcat.app.tv.testing.FakeServers
import com.slouchcat.app.tv.testing.TV_DEVICE
import com.slouchcat.app.tv.testing.session
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.pairing.PairingCode
import com.slouchcat.core.pairing.PairingFlow
import com.slouchcat.core.pairing.PairingPacket
import com.slouchcat.core.pairing.PickupSecret
import com.slouchcat.core.pairing.RedeemAnswer
import com.slouchcat.core.pairing.TvKeyPair
import com.slouchcat.core.pairing.TvPairing
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.ServerInfo
import com.slouchcat.core.session.SessionState
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelStore
import androidx.lifecycle.viewmodel.CreationExtras
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.time.Instant
import kotlin.reflect.KClass

@OptIn(ExperimentalCoroutinesApi::class)
class PairViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val relay = "https://pair.example.test".toHttpUrl()
    private val serverKey = Base64Url.encode(ByteArray(32) { 4 })
    private val server = "http://192.0.2.10:8096/".toHttpUrl()
    private val packet = PairingPacket(server = "http://192.0.2.10:8096", voucher = "V".repeat(43), serverKey = serverKey)
    private val answer = RedeemAnswer("device-token", "dev_1", "owner@example.test")

    @Before
    fun setUp() = Dispatchers.setMain(dispatcher)

    @After
    fun tearDown() = Dispatchers.resetMain()

    /** The steps of the pairing as a test plays them: each answer waits until the test gives it. */
    private inner class FakeFlow : PairingFlow {
        val started = mutableListOf<TvPairing.Meeting>()
        val closed = mutableListOf<TvPairing.Meeting>()
        var startAnswer: TvPairing.Started? = null
        var waited = CompletableDeferred<TvPairing.Waited>()
        var redeem: TvPairing.Redeem = TvPairing.Redeem.Ok(answer)
        /** When set, the redeem waits for it (a slow server). */
        var redeemGate: CompletableDeferred<Unit>? = null
        val polls = mutableListOf<TvPairing.Meeting>()
        val redeemed = mutableListOf<HttpUrl>()
        val discarded = mutableListOf<String>()
        var lastUntil: Instant? = null

        fun meeting(index: Int) = TvPairing.Meeting(
            PairingCode(relay, Base64Url.encode(ByteArray(16) { index.toByte() }), Base64Url.encode(ByteArray(32) { 1 })),
            START.plusSeconds(300),
            TvKeyPair.generate(),
            PickupSecret.generate(),
        )

        override suspend fun start(): TvPairing.Started = startAnswer ?: TvPairing.Started.Ok(meeting(started.size).also { started += it })

        override suspend fun awaitPacket(meeting: TvPairing.Meeting, until: Instant): TvPairing.Waited {
            lastUntil = until
            polls += meeting
            return waited.await().also { waited = CompletableDeferred() }
        }

        override suspend fun redeem(meeting: TvPairing.Meeting, packet: PairingPacket, server: HttpUrl, deviceName: String, systemVersion: String): TvPairing.Redeem {
            redeemGate?.await()
            redeemed += server
            return redeem
        }

        override suspend fun discard(server: HttpUrl, serverKey: String, token: String): Boolean {
            discarded += token
            return true
        }

        override suspend fun close(meeting: TvPairing.Meeting) {
            closed += meeting
        }
    }

    private val flow = FakeFlow()

    /** Runs what is due now, without moving the clock (the code's countdown would run to its end). */
    private fun TestScope.step() = repeat(STEPS) { testScheduler.runCurrent() }
    private val found = ProbeResult.Found(ServerInfo(server, "1.43.0", "1.43.0", serverKey = serverKey))

    private fun TestScope.model(probe: FakeProbe = FakeProbe(found), servers: FakeServers = FakeServers(null), renewals: Int = 2): Triple<PairViewModel, FakeServers, com.slouchcat.core.session.SessionManager> {
        val session = session(backgroundScope, server = null, token = null, servers = servers)
        val vm = PairViewModel(
            relay = { relay },
            flowFor = { flow },
            probe = probe,
            servers = servers,
            session = session,
            device = TV_DEVICE,
            systemVersion = "Android TV 11",
            cleanup = backgroundScope,
            now = { START.plusMillis(testScheduler.currentTime) },
            renewals = renewals,
        )
        vm.onVisible()
        return Triple(vm, servers, session)
    }

    @Test
    fun showsTheCodeCountsDownAndConnectsWithThePacket() = runTest(dispatcher) {
        val (vm, servers, session) = model()
        step()
        val waiting = vm.state.value.phase as PairPhase.Waiting
        assertEquals(flow.started.single().code.text, waiting.code)
        assertEquals(300L, waiting.secondsLeft)
        assertEquals("pair.example.test", vm.state.value.relay)
        // The code is renewed 30 s before it runs out.
        assertEquals(START.plusSeconds(270), flow.lastUntil)
        testScheduler.advanceTimeBy(5_000)
        step()
        assertEquals(295L, (vm.state.value.phase as PairPhase.Waiting).secondsLeft)

        flow.waited.complete(TvPairing.Waited.Packet(packet))
        step()
        val linked = vm.state.value.phase as PairPhase.Linked
        // K-9: the whole address with scheme and port.
        assertEquals(PairPhase.Linked("http://192.0.2.10:8096", false, "owner@example.test", TV_DEVICE.name, 10), linked)
        assertEquals(listOf(server), flow.redeemed)
        // Nothing is stored before «Weiter»; the meeting point is given up.
        assertEquals(null, servers.flow.value)
        assertEquals(flow.started, flow.closed)

        vm.onContinue()
        step()
        assertEquals(serverKey, servers.flow.value?.serverKey)
        val state = session.state.value as SessionState.ChooseProfile
        assertEquals("owner@example.test", state.email)
        assertEquals("device-token", session.bearerToken)
    }

    @Test
    fun goesOnByItselfAfterTenSecondsUnlessAKeyWasPressed() = runTest(dispatcher) {
        val (vm, servers, _) = model()
        step()
        flow.waited.complete(TvPairing.Waited.Packet(packet))
        step()
        vm.onInput()
        assertEquals(null, (vm.state.value.phase as PairPhase.Linked).autoSeconds)
        testScheduler.advanceTimeBy(20_000)
        step()
        assertEquals(null, servers.flow.value)

        // Another pairing without a key: after ten seconds the session is stored.
        val (other, otherServers, _) = model()
        step()
        flow.waited.complete(TvPairing.Waited.Packet(packet))
        step()
        assertTrue(other.state.value.phase is PairPhase.Linked)
        testScheduler.advanceTimeBy(11_000)
        step()
        assertEquals(server, otherServers.flow.value?.address)
    }

    @Test
    fun notMyAccountEndsTheDeviceSessionAndShowsANewCode() = runTest(dispatcher) {
        val (vm, servers, _) = model()
        step()
        flow.waited.complete(TvPairing.Waited.Packet(packet))
        step()
        var confirmed: Boolean? = null
        vm.onUndo { confirmed = it }
        step()
        assertEquals(listOf("device-token"), flow.discarded)
        assertEquals(true, confirmed)
        assertEquals(null, servers.flow.value)
        assertTrue(vm.state.value.phase is PairPhase.Waiting)
        assertEquals(2, flow.started.size)
    }

    @Test
    fun renewsTheCodeTwiceThenItExpires() = runTest(dispatcher) {
        val (vm, _, _) = model(renewals = 2)
        step()
        repeat(2) {
            flow.waited.complete(TvPairing.Waited.Expired)
            step()
            assertTrue(vm.state.value.phase is PairPhase.Waiting)
        }
        assertEquals(3, flow.started.size)
        // The last code runs to its end.
        assertEquals(START.plusSeconds(300), flow.lastUntil)
        flow.waited.complete(TvPairing.Waited.Expired)
        step()
        assertEquals(PairPhase.Expired, vm.state.value.phase)
        assertEquals(3, flow.closed.size)
        vm.onNewCode()
        step()
        assertTrue(vm.state.value.phase is PairPhase.Waiting)
    }

    @Test
    fun theProblemsSayWhatHappened() = runTest(dispatcher) {
        flow.startAnswer = TvPairing.Started.RelayUnreachable
        val (unreachable, _, _) = model()
        step()
        assertEquals(PairPhase.Problem(PairProblem.RelayUnreachable), unreachable.state.value.phase)
        flow.startAnswer = TvPairing.Started.RelayBusy
        unreachable.onNewCode()
        step()
        assertEquals(PairPhase.Problem(PairProblem.RelayBusy), unreachable.state.value.phase)
        flow.startAnswer = null

        // The server of the packet does not answer: its address and the way by hand.
        val (noServer, _, _) = model(probe = FakeProbe(ProbeResult.Unreachable))
        step()
        flow.waited.complete(TvPairing.Waited.Packet(packet))
        step()
        assertEquals(PairPhase.Problem(PairProblem.ServerUnreachable, address = "http://192.0.2.10:8096"), noServer.state.value.phase)
        assertTrue(flow.redeemed.isEmpty())

        // Another key under the address than the phone vouched for: no redeem at all.
        val (foreign, _, _) = model(probe = FakeProbe(ProbeResult.Found(ServerInfo(server, "1.43.0", "1.43.0", serverKey = Base64Url.encode(ByteArray(32) { 9 })))))
        step()
        flow.waited.complete(TvPairing.Waited.Packet(packet))
        step()
        assertEquals(PairPhase.Problem(PairProblem.Untrusted), foreign.state.value.phase)
        assertTrue(flow.redeemed.isEmpty())

        val (old, _, _) = model(probe = FakeProbe(ProbeResult.TooOld("1.41.0", "1.42.0")))
        step()
        flow.waited.complete(TvPairing.Waited.Packet(packet))
        step()
        assertEquals(PairPhase.Problem(PairProblem.ServerTooOld, "http://192.0.2.10:8096", "1.41.0", "1.42.0"), old.state.value.phase)

        flow.redeem = TvPairing.Redeem.Untrusted
        val (untrusted, servers, _) = model()
        step()
        flow.waited.complete(TvPairing.Waited.Packet(packet))
        step()
        assertEquals(PairPhase.Problem(PairProblem.Untrusted), untrusted.state.value.phase)
        assertEquals(null, servers.flow.value)
    }

    @Test
    fun aVoucherThatRanOutBringsANewCode() = runTest(dispatcher) {
        flow.redeem = TvPairing.Redeem.VoucherInvalid
        val (vm, _, _) = model()
        step()
        flow.waited.complete(TvPairing.Waited.Packet(packet))
        step()
        assertTrue(vm.state.value.phase is PairPhase.Waiting)
        assertEquals(2, flow.started.size)
    }

    @Test
    fun aHiddenPageStopsPollingGivesTheMeetingUpAndRedeemsNothing() = runTest(dispatcher) {
        val (vm, servers, _) = model()
        step()
        assertTrue(vm.state.value.phase is PairPhase.Waiting)
        val first = flow.started.single()
        vm.onHidden()
        step()
        assertEquals(listOf(first), flow.closed)
        assertEquals(PairPhase.Starting, vm.state.value.phase)
        // A packet laid down now comes to nobody: no poll is waiting, nothing is redeemed.
        flow.waited.complete(TvPairing.Waited.Packet(packet))
        testScheduler.advanceTimeBy(15 * 60_000L)
        step()
        assertEquals(1, flow.started.size)
        assertEquals(1, flow.polls.size)
        assertTrue(flow.redeemed.isEmpty())
        assertEquals(null, servers.flow.value)
        // Back on the page: a new code with a new meeting point.
        flow.waited = CompletableDeferred()
        vm.onVisible()
        step()
        assertEquals(2, flow.started.size)
        assertEquals(flow.started[1].code.text, (vm.state.value.phase as PairPhase.Waiting).code)
    }

    @Test
    fun aHiddenPageEndsALinkedSessionAndNeverGoesOnByItself() = runTest(dispatcher) {
        val (vm, servers, session) = model()
        step()
        flow.waited.complete(TvPairing.Waited.Packet(packet))
        step()
        assertTrue(vm.state.value.phase is PairPhase.Linked)
        vm.onHidden()
        testScheduler.advanceTimeBy(30_000)
        step()
        assertEquals(listOf("device-token"), flow.discarded)
        assertEquals(null, servers.flow.value)
        assertEquals(null, session.bearerToken)
        vm.onContinue()
        step()
        assertEquals(null, servers.flow.value)
    }

    @Test
    fun aPageHiddenWhileRedeemingKeepsNoSession() = runTest(dispatcher) {
        val gate = CompletableDeferred<Unit>()
        flow.redeemGate = gate
        val (vm, servers, _) = model()
        step()
        flow.waited.complete(TvPairing.Waited.Packet(packet))
        step()
        assertEquals(PairPhase.Connecting, vm.state.value.phase)
        vm.onHidden()
        gate.complete(Unit)
        testScheduler.advanceTimeBy(30_000)
        step()
        assertTrue(vm.state.value.phase !is PairPhase.Linked)
        assertEquals(null, servers.flow.value)
    }

    @Test
    fun aClearedViewModelGivesUpTheMeetingAndAnUnconfirmedSession() = runTest(dispatcher) {
        val (vm, _, _) = model()
        val store = ViewModelStore()
        ViewModelProvider.create(store, object : ViewModelProvider.Factory {
            @Suppress("UNCHECKED_CAST")
            override fun <T : ViewModel> create(modelClass: KClass<T>, extras: CreationExtras): T = vm as T
        })[PairViewModel::class]
        step()
        flow.waited.complete(TvPairing.Waited.Packet(packet))
        step()
        assertTrue(vm.state.value.phase is PairPhase.Linked)
        store.clear()
        step()
        assertEquals(listOf("device-token"), flow.discarded)
    }

    @Test
    fun aServerThatAlwaysRefusesTheVoucherDoesNotKeepTheTvPolling() = runTest(dispatcher) {
        flow.redeem = TvPairing.Redeem.VoucherInvalid
        val (vm, _, _) = model(renewals = 2)
        step()
        repeat(3) {
            flow.waited.complete(TvPairing.Waited.Packet(packet))
            step()
        }
        assertEquals(PairPhase.Expired, vm.state.value.phase)
        assertEquals(3, flow.started.size)
    }

    @Test
    fun theWaitingStateKeepsTheCodeOutOfItsText() {
        val waiting = PairPhase.Waiting("https://pair.example.test/tv#v=1&i=AAAA&k=BBBB", 12)
        assertEquals("Waiting(secondsLeft=12)", waiting.toString())
    }

    @Test
    fun aSilentRelayDuringTheWaitIsAProblem() = runTest(dispatcher) {
        val (vm, _, _) = model()
        step()
        flow.waited.complete(TvPairing.Waited.RelayUnreachable)
        step()
        assertEquals(PairPhase.Problem(PairProblem.RelayUnreachable), vm.state.value.phase)
        assertEquals(1, flow.closed.size)
    }

    private companion object {
        const val STEPS = 6
        val START: Instant = Instant.parse("2026-11-21T10:00:00Z")
    }
}
