package com.slouchcat.app.mobile.tvlink

import com.slouchcat.app.mobile.testing.FakeDevices
import com.slouchcat.app.mobile.testing.device
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.generated.models.Health
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.api.generated.models.SessionKind
import com.slouchcat.core.browse.testing.CHILD
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.HTTPS_SERVER
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.settle
import com.slouchcat.core.pairing.IssuedVoucher
import com.slouchcat.core.pairing.PairingCode
import com.slouchcat.core.pairing.PhonePairing
import com.slouchcat.core.pairing.PhonePairingFlow
import com.slouchcat.core.session.MemoryRelaySettings
import com.slouchcat.core.session.RelaySettings
import com.slouchcat.core.session.ServerRepository
import java.time.Duration
import java.time.Instant
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
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/** The page «Fernseher verbinden» (concept 27 section 10): every answer of server and relay, cancel, leaving. */
@OptIn(ExperimentalCoroutinesApi::class)
class TvLinkViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val relay = RelaySettings.DEFAULT
    private val code = PairingCode(relay, Base64Url.encode(ByteArray(16) { 1 }), Base64Url.encode(ByteArray(32) { 2 }))
    private val oldTv = device("tv-old", "Kitchen", type = MyDevice.Type.TV, kind = SessionKind.DEVICE)
    private val devices = FakeDevices(ApiResult.Ok(listOf(device("me", "Pixel", current = true), oldTv)))

    /** Scripted steps; [delivered] holds the deliver until the test lets it go. */
    private val newTv = device("tv-new", "Living room", type = MyDevice.Type.TV, kind = SessionKind.DEVICE)

    private inner class FakeFlow : PhonePairingFlow {
        var issue: PhonePairing.Issue = PhonePairing.Issue.Ok(IssuedVoucher("v".repeat(43), Instant.EPOCH, setOf("me")))
        var deliver: PhonePairing.Deliver = PhonePairing.Deliver.Delivered
        var linked: PhonePairing.Linked = PhonePairing.Linked.Ok(newTv)

        /** What the short look after a refusal of the relay finds. */
        var looked: PhonePairing.Linked = PhonePairing.Linked.TimedOut
        var gate: CompletableDeferred<Unit>? = null
        var deliverGate: CompletableDeferred<Unit>? = null
        val calls = mutableListOf<String>()

        override suspend fun issue(code: PairingCode) = issue.also { calls += "issue:${code.tvKey}" }

        override suspend fun deliver(code: PairingCode, voucher: IssuedVoucher, serverAddress: HttpUrl, serverKey: String): PhonePairing.Deliver {
            calls += "deliver:$serverAddress:$serverKey"
            deliverGate?.await()
            return deliver
        }

        override suspend fun awaitTv(voucher: IssuedVoucher, within: Duration): PhonePairing.Linked {
            if (within != PhonePairing.WAIT_FOR_TV) {
                calls += "look"
                return looked
            }
            calls += "await"
            gate?.await()
            return linked
        }
    }

    private val flow = FakeFlow()
    private val relays = mutableListOf<Pair<HttpUrl, HttpUrl>>()

    private class FakeHealth(private val key: String?) : ServerRepository {
        override suspend fun health(server: HttpUrl): ApiResult<Health> =
            if (key == null) ApiResult.Offline else ApiResult.Ok(Health(status = Health.Status.OK, version = "1.42.0", contractVersion = "1.42.0", serverKey = key, uptimeSeconds = 1))
    }

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun TestScope.vm(profile: com.slouchcat.core.session.ActiveProfile = com.slouchcat.core.browse.testing.PRIMARY, server: com.slouchcat.core.session.SavedServer = com.slouchcat.core.browse.testing.HTTP_SERVER, health: ServerRepository = FakeHealth("health-key")): Pair<TvLinkViewModel, MemoryRelaySettings> {
        val s = session(backgroundScope, server = server, profile = profile)
        settle()
        val settings = MemoryRelaySettings()
        val vm = TvLinkViewModel(s, settings, devices, health) { r, srv -> relays += r to srv; flow }
        vm.onShown()
        settle()
        return vm to settings
    }

    private fun TvLinkViewModel.scan(text: String = code.text) {
        onScan()
        onScanned(text)
    }

    @Test
    fun scanConfirmAndConnect() = runTest(dispatcher) {
        val (vm, _) = vm()
        assertTrue(vm.state.value.primary)
        assertEquals(listOf(oldTv), vm.state.value.tvs)
        vm.onScan()
        assertEquals(TvLinkStep.Scan, vm.state.value.step)
        vm.onScanned(code.text)
        assertEquals(TvLinkStep.Confirm(EMAIL, "http://192.0.2.10:8096", "Primary", primary = true, plainHttp = true), vm.state.value.step)
        // Nothing went anywhere before the yes (K-5).
        assertTrue(flow.calls.isEmpty())
        vm.onConfirm()
        settle()
        assertEquals(TvLinkStep.Done(newTv), vm.state.value.step)
        assertEquals(listOf("issue:${code.tvKey}", "deliver:http://192.0.2.10:8096/:server-key", "await"), flow.calls)
        assertEquals(listOf(RelaySettings.DEFAULT to "http://192.0.2.10:8096/".toHttpUrl()), relays)
        assertEquals(listOf("tv-old", "tv-new"), vm.state.value.tvs.map { it.id })
    }

    @Test
    fun theLinesFollowTheSteps() = runTest(dispatcher) {
        val (vm, _) = vm()
        flow.gate = CompletableDeferred()
        vm.scan()
        vm.onConfirm()
        assertEquals(TvLinkStep.Running(0), vm.state.value.step)
        settle()
        assertEquals(TvLinkStep.Running(2), vm.state.value.step)
        // A second «Verbinden» or a scan do nothing while it runs.
        vm.onConfirm()
        vm.onScan()
        assertEquals(TvLinkStep.Running(2), vm.state.value.step)
        flow.gate!!.complete(Unit)
        settle()
        assertTrue(vm.state.value.step is TvLinkStep.Done)
    }

    @Test
    fun aServerOverHttpsWithoutRememberedKeyTakesTheKeyOfItsHealth() = runTest(dispatcher) {
        val (vm, _) = vm(server = HTTPS_SERVER)
        vm.scan()
        assertEquals(false, (vm.state.value.step as TvLinkStep.Confirm).plainHttp)
        vm.onConfirm()
        settle()
        assertTrue(flow.calls.contains("deliver:https://media.example.test/:health-key"))
    }

    @Test
    fun withoutAnyKeyOfTheServerNothingIsSealed() = runTest(dispatcher) {
        val (vm, _) = vm(server = HTTPS_SERVER, health = FakeHealth(null))
        vm.scan()
        vm.onConfirm()
        settle()
        assertEquals(TvLinkStep.Failed(TvLinkFailure.Failed, 1), vm.state.value.step)
        assertFalse(flow.calls.any { it.startsWith("deliver") })
    }

    @Test
    fun whatIsNoCodeOfTheRelayInUseNeverReachesTheQuestion() = runTest(dispatcher) {
        val (vm, _) = vm()
        vm.scan("https://example.test/")
        assertEquals(TvLinkStep.Intro, vm.state.value.step)
        assertEquals(ScanProblem.NotACode, vm.state.value.scanProblem)
        vm.scan(code.text.replace("#v=1&", "#v=3&"))
        assertEquals(ScanProblem.OtherVersion, vm.state.value.scanProblem)
        vm.scan(PairingCode("https://pair.example.test".toHttpUrl(), code.id, code.tvKey).text)
        assertEquals(ScanProblem.OtherRelay, vm.state.value.scanProblem)
        // A scan that finds a good code clears the old message; «Verbinden» without a code does nothing.
        vm.onConfirm()
        assertTrue(flow.calls.isEmpty())
        vm.scan()
        assertNull(vm.state.value.scanProblem)
        assertTrue(vm.state.value.step is TvLinkStep.Confirm)
    }

    @Test
    fun anotherRelaySetHereIsTheOneTheCodeMustName() = runTest(dispatcher) {
        val (vm, settings) = vm()
        val own = "https://pair.example.test".toHttpUrl()
        settings.set(own)
        settle()
        assertEquals(own, vm.state.value.relay)
        vm.scan()
        assertEquals(ScanProblem.OtherRelay, vm.state.value.scanProblem)
        vm.scan(PairingCode(own, code.id, code.tvKey).text)
        vm.onConfirm()
        settle()
        assertEquals(own, relays.single().first)
        settings.set(null)
        settle()
        assertEquals(RelaySettings.DEFAULT, vm.state.value.relay)
    }

    @Test
    fun closingTheScannerOrNoScannerReturnsToTheStart() = runTest(dispatcher) {
        val (vm, _) = vm()
        vm.onScan()
        vm.onScanClosed()
        assertEquals(TvLinkStep.Intro, vm.state.value.step)
        assertNull(vm.state.value.scanProblem)
        vm.onScan()
        vm.onScannerTrouble(ScannerTrouble.NoPlayServices)
        assertEquals(ScanProblem.Scanner(ScannerTrouble.NoPlayServices), vm.state.value.scanProblem)
        // A late answer of a scanner that is closed already counts for nothing.
        vm.onScanned(code.text)
        assertEquals(TvLinkStep.Intro, vm.state.value.step)
    }

    @Test
    fun cancelOnTheQuestionForgetsTheCode() = runTest(dispatcher) {
        val (vm, _) = vm()
        vm.scan()
        vm.onBackToIntro()
        assertEquals(TvLinkStep.Intro, vm.state.value.step)
        vm.onConfirm()
        settle()
        assertTrue(flow.calls.isEmpty())
    }

    @Test
    fun everyAnswerOfTheServerHasItsMessage() = runTest(dispatcher) {
        val cases = listOf(
            PhonePairing.Issue.PrimaryOnly to TvLinkFailure.PrimaryOnly,
            PhonePairing.Issue.NotAllowed("app_session_required") to TvLinkFailure.NotAllowed,
            PhonePairing.Issue.TooManyAttempts(30) to TvLinkFailure.TooManyAttempts(30),
            PhonePairing.Issue.Offline to TvLinkFailure.ServerOffline,
            PhonePairing.Issue.ServerUnconfirmed to TvLinkFailure.ServerUnconfirmed,
            PhonePairing.Issue.SessionEnded to TvLinkFailure.Failed,
            PhonePairing.Issue.Failed to TvLinkFailure.Failed,
        )
        val (vm, _) = vm()
        for ((issue, failure) in cases) {
            flow.issue = issue
            vm.scan()
            vm.onConfirm()
            settle()
            assertEquals(issue.toString(), TvLinkStep.Failed(failure, 0), vm.state.value.step)
            vm.onBackToIntro()
        }
        assertFalse(flow.calls.any { it.startsWith("deliver") })
    }

    @Test
    fun everyAnswerOfTheRelayHasItsMessage() = runTest(dispatcher) {
        val cases = listOf(
            PhonePairing.Deliver.Occupied to TvLinkFailure.Occupied,
            PhonePairing.Deliver.Expired to TvLinkFailure.Expired,
            PhonePairing.Deliver.RelayUnreachable to TvLinkFailure.RelayUnreachable,
            PhonePairing.Deliver.Failed to TvLinkFailure.Failed,
        )
        val (vm, _) = vm()
        for ((deliver, failure) in cases) {
            flow.deliver = deliver
            vm.scan()
            vm.onConfirm()
            settle()
            assertEquals(deliver.toString(), TvLinkStep.Failed(failure, 1), vm.state.value.step)
            vm.onBackToIntro()
        }
        assertFalse(flow.calls.contains("await"))
        assertEquals(2, flow.calls.count { it == "look" })
    }

    @Test
    fun ownPacketAlreadyThereIsNoForeignPacket() = runTest(dispatcher) {
        // A repeated PUT (lost answer) finds the own packet: the TV appears, no warning (security review L-1).
        val (vm, _) = vm()
        flow.deliver = PhonePairing.Deliver.Occupied
        flow.looked = PhonePairing.Linked.Ok(newTv)
        vm.scan()
        vm.onConfirm()
        settle()
        assertEquals(TvLinkStep.Done(newTv), vm.state.value.step)
        flow.deliver = PhonePairing.Deliver.RelayUnreachable
        vm.onBackToIntro()
        vm.scan()
        vm.onConfirm()
        settle()
        assertEquals(TvLinkStep.Done(newTv), vm.state.value.step)
    }

    @Test
    fun twoNewTvsOrASessionThatEndedAreNoGuess() = runTest(dispatcher) {
        val (vm, _) = vm()
        flow.linked = PhonePairing.Linked.Several
        vm.scan()
        vm.onConfirm()
        settle()
        assertEquals(TvLinkStep.Done(null), vm.state.value.step)
        vm.onNotMine()
        assertNull(vm.state.value.notMine)
        vm.onBackToIntro()
        flow.linked = PhonePairing.Linked.Ended
        vm.scan()
        vm.onConfirm()
        settle()
        assertEquals(TvLinkStep.Failed(TvLinkFailure.Failed, 2), vm.state.value.step)
    }

    @Test
    fun notMyTvSignsExactlyThatTvOut() = runTest(dispatcher) {
        val (vm, _) = vm()
        vm.scan()
        vm.onConfirm()
        settle()
        vm.onNotMine()
        assertEquals(newTv, vm.state.value.notMine?.device)
        devices.signOut = ApiResult.Offline
        vm.onNotMineConfirm("unnamed")
        settle()
        assertEquals(com.slouchcat.app.mobile.devices.DeviceSignOutError.Unreachable, vm.state.value.notMine?.error)
        devices.signOut = ApiResult.Ok(Unit)
        vm.onNotMineConfirm("unnamed")
        settle()
        assertEquals(listOf("tv-new"), devices.signedOut)
        assertEquals(TvLinkStep.Intro, vm.state.value.step)
        assertEquals("Living room", vm.state.value.signedOutTv)
        assertFalse(vm.state.value.tvs.any { it.id == "tv-new" })
    }

    @Test
    fun leavingWhileThePacketGoesOutMayHaveConnected() = runTest(dispatcher) {
        // Code review m-3: hidden during the PUT, the packet may be at the relay already.
        val (vm, _) = vm()
        flow.deliverGate = CompletableDeferred()
        vm.scan()
        vm.onConfirm()
        settle()
        assertEquals(TvLinkStep.Running(1), vm.state.value.step)
        vm.onHidden()
        assertEquals(TvLinkStep.Failed(TvLinkFailure.Left(delivered = true), 1), vm.state.value.step)
    }

    @Test
    fun theScannerLoadsAndTheOwnScannerSkipsOtherCodes() = runTest(dispatcher) {
        val (vm, _) = vm()
        vm.onScan()
        vm.onScannerLoading(true)
        assertTrue(vm.state.value.scannerLoading)
        vm.onScannerLoading(false)
        assertFalse(vm.state.value.scannerLoading)
        vm.onScannerTrouble(ScannerTrouble.Loading)
        assertEquals(ScanProblem.Scanner(ScannerTrouble.Loading), vm.state.value.scanProblem)
        // What the own scanner ends for: pairing codes of any relay and version, nothing else.
        assertTrue(vm.accepts(code.text))
        assertTrue(vm.accepts(PairingCode("https://pair.example.test".toHttpUrl(), code.id, code.tvKey).text))
        assertFalse(vm.accepts("WIFI:S:home;T:WPA;P:x;;"))
    }

    @Test
    fun noTvWithinTwoMinutes() = runTest(dispatcher) {
        val (vm, _) = vm()
        flow.linked = PhonePairing.Linked.TimedOut
        vm.scan()
        vm.onConfirm()
        settle()
        assertEquals(TvLinkStep.Failed(TvLinkFailure.TimedOut, 2), vm.state.value.step)
        // «Nochmals scannen» opens the scanner again.
        vm.onScan()
        assertEquals(TvLinkStep.Scan, vm.state.value.step)
    }

    @Test
    fun leavingThePageStopsThePairing() = runTest(dispatcher) {
        val (vm, _) = vm()
        flow.gate = CompletableDeferred()
        vm.scan()
        vm.onConfirm()
        settle()
        assertEquals(TvLinkStep.Running(2), vm.state.value.step)
        vm.onHidden()
        assertEquals(TvLinkStep.Failed(TvLinkFailure.Left(delivered = true), 2), vm.state.value.step)
        // The wait does not go on behind the page.
        flow.gate!!.complete(Unit)
        settle()
        assertEquals(TvLinkStep.Failed(TvLinkFailure.Left(delivered = true), 2), vm.state.value.step)
        vm.onShown()
        assertTrue(vm.state.value.step is TvLinkStep.Failed)
    }

    @Test
    fun leavingBeforeThePacketSaysSo() = runTest(dispatcher) {
        val (vm, _) = vm()
        vm.scan()
        vm.onConfirm()
        // Hidden before the first step ran: nothing reached the relay.
        vm.onHidden()
        settle()
        assertEquals(TvLinkStep.Failed(TvLinkFailure.Left(delivered = false), 0), vm.state.value.step)
        assertTrue(flow.calls.isEmpty())
    }

    @Test
    fun hidingOnTheScannerOrTheQuestionChangesNothing() = runTest(dispatcher) {
        // Google's scanner is a screen of its own: the page is hidden while it reads.
        val (vm, _) = vm()
        vm.onScan()
        vm.onHidden()
        vm.onShown()
        assertEquals(TvLinkStep.Scan, vm.state.value.step)
        vm.onScanned(code.text)
        vm.onHidden()
        vm.onShown()
        assertTrue(vm.state.value.step is TvLinkStep.Confirm)
    }

    @Test
    fun backFromMyDevicesAfterConnectedIsTheStartAgain() = runTest(dispatcher) {
        val (vm, _) = vm()
        vm.scan()
        vm.onConfirm()
        settle()
        assertTrue(vm.state.value.step is TvLinkStep.Done)
        vm.onHidden()
        vm.onShown()
        settle()
        assertEquals(TvLinkStep.Intro, vm.state.value.step)
    }

    @Test
    fun anotherProfileThanThePrimaryOneSeesTheNote() = runTest(dispatcher) {
        val (vm, _) = vm(profile = CHILD)
        assertFalse(vm.state.value.primary)
        // The server decides: a child's scan still goes to the question, its voucher fails with 403.
        flow.issue = PhonePairing.Issue.PrimaryOnly
        vm.scan()
        assertEquals("Child", (vm.state.value.step as TvLinkStep.Confirm).profile)
        vm.onConfirm()
        settle()
        assertEquals(TvLinkStep.Failed(TvLinkFailure.PrimaryOnly, 0), vm.state.value.step)
    }
}
