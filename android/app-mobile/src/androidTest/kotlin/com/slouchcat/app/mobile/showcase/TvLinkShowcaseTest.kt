package com.slouchcat.app.mobile.showcase

import android.accessibilityservice.AccessibilityService
import android.content.Intent
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry
import androidx.test.runner.lifecycle.Stage
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.mobile.HandedScanner
import com.slouchcat.app.mobile.MainActivity
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.features
import com.slouchcat.app.mobile.tvlink.MemoryRelaySettings
import com.slouchcat.app.mobile.tvlink.RelaySettings
import com.slouchcat.app.mobile.tvlink.TvLinkFeature
import com.slouchcat.app.mobile.tvlink.TvLinkRoute
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.pairing.IssuedVoucher
import com.slouchcat.core.pairing.PairingCode
import com.slouchcat.core.pairing.PhonePairing
import com.slouchcat.core.pairing.PhonePairingFlow
import com.slouchcat.core.strings.R
import kotlinx.coroutines.CompletableDeferred
import okhttp3.HttpUrl
import org.junit.After
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.time.Instant
import java.time.OffsetDateTime

/**
 * «Fernseher verbinden» with every answer the page knows (concept 27 section 10), for the visual
 * check against the mockup: the steps of a pairing that works, then each failure the server or the
 * page itself brings (the relay's own answers come from [com.slouchcat.app.mobile.TvLinkFlowTest]
 * with a real relay). No server, no relay: the steps are scripted. Photos with `slouchcatShots` as `p…`.
 */
@RunWith(AndroidJUnit4::class)
class TvLinkShowcaseTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    /** The scripted steps: what [issue] answers, a [gate] the wait holds on, what [linked] answers. */
    private class Script : PhonePairingFlow {
        var issue: PhonePairing.Issue = PhonePairing.Issue.Ok(IssuedVoucher("v".repeat(43), Instant.EPOCH, emptySet()))
        var linked: PhonePairing.Linked = PhonePairing.Linked.Ok(
            MyDevice("tv-2", OffsetDateTime.now(), 0, current = false, name = "Schlafzimmer-TV", type = MyDevice.Type.TV),
        )
        var gate = CompletableDeferred<Unit>().apply { complete(Unit) }

        override suspend fun issue(code: PairingCode) = issue

        override suspend fun deliver(code: PairingCode, voucher: IssuedVoucher, serverAddress: HttpUrl, serverKey: String) = PhonePairing.Deliver.Delivered

        override suspend fun awaitTv(voucher: IssuedVoucher): PhonePairing.Linked {
            gate.await()
            return linked
        }
    }

    private val script = Script()
    private val scanner = HandedScanner()
    private val code = PairingCode(RelaySettings.DEFAULT, Base64Url.encode(ByteArray(16) { 3 }), Base64Url.encode(ByteArray(32) { 5 })).text

    private val pages = features.map { feature ->
        if (feature is TvLinkFeature) TvLinkFeature(scanner, relaySettings = { MemoryRelaySettings() }, pairing = { _, _, _ -> script }) else feature
    }

    private val images = ShowcaseImages(BrowseShowcaseData.names + PagesShowcaseData.names).also { it.squareIds = BrowseShowcaseData.squareIds }

    private val showcase = Showcase(compose, pages, BrowseShowcaseData.libraries, images, PagesShowcaseData.auth, PagesShowcaseData.devices) { graph ->
        PagesShowcaseData.data(MediaData.api(graph.api, graph.http))
    }

    @After
    fun close() = showcase.close()

    private fun open(target: String, vararg routes: Any) =
        InstrumentationRegistry.getInstrumentation().runOnMainSync { showcase.graph.links.request(AppLink(target, routes.toList())) }

    private fun waitForText(text: String) = showcase.waitFor(hasText(text, substring = true))

    private fun button(text: String) = compose.onAllNodes(hasText(text) and hasClickAction()).onLast()

    /** «Scanner öffnen», the code, «Verbinden». */
    private fun scanAndConnect() {
        scanner.next = CompletableDeferred()
        button(s(R.string.app_tv_intro_scan)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_scan_aim))
        scanner.next.complete(code)
        waitForText(s(R.string.app_tv_confirm_title))
        button(s(R.string.app_tv_confirm_go)).performScrollTo().performClick()
    }

    private fun failure(issue: PhonePairing.Issue, text: String, name: String, dismiss: Int = R.string.app_tv_fail_ok) {
        script.issue = issue
        scanAndConnect()
        waitForText(text)
        showcase.shot(name)
        button(s(dismiss)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_intro_title))
    }

    @Test
    fun everyStepAndEveryMessage() {
        showcase.start()
        open("more", TvLinkRoute)
        waitForText(s(R.string.app_tv_intro_title))
        waitForText("Wohnzimmer-TV")
        showcase.shot("p01-intro")

        // The scanner, the question (a server over HTTP: the warning), the run, «Verbunden».
        scanner.next = CompletableDeferred()
        button(s(R.string.app_tv_intro_scan)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_scan_aim))
        showcase.shot("p02-scanner")
        scanner.next.complete(code)
        waitForText(s(R.string.app_tv_confirm_title))
        showcase.shot("p03-confirm")
        script.gate = CompletableDeferred()
        button(s(R.string.app_tv_confirm_go)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_run_waiting))
        showcase.shot("p04-running")
        script.gate.complete(Unit)
        waitForText(s(R.string.app_tv_done_title))
        waitForText("Schlafzimmer-TV")
        showcase.shot("p05-connected")
        button(s(R.string.app_tv_done_finish)).performScrollTo().performClick()
        open("more", TvLinkRoute)
        waitForText(s(R.string.app_tv_intro_title))

        // The server's answers.
        failure(PhonePairing.Issue.PrimaryOnly, s(R.string.app_tv_intro_primaryOnly), "p06-primary-only")
        failure(PhonePairing.Issue.Offline, s(R.string.app_tv_fail_server), "p07-server-offline", dismiss = R.string.common_cancel)
        failure(PhonePairing.Issue.TooManyAttempts(40), s(R.string.errors_code_too_many_attempts), "p08-too-many", dismiss = R.string.common_cancel)
        failure(PhonePairing.Issue.ServerUnconfirmed, s(R.string.app_serverKey_unconfirmedBody), "p09-server-unconfirmed")
        failure(PhonePairing.Issue.NotAllowed("app_session_required"), s(R.string.app_tv_fail_notAllowed), "p10-not-allowed")

        // No TV within two minutes.
        script.issue = PhonePairing.Issue.Ok(IssuedVoucher("v".repeat(43), Instant.EPOCH, emptySet()))
        script.linked = PhonePairing.Linked.TimedOut
        scanAndConnect()
        waitForText(s(R.string.app_tv_fail_timedOut))
        showcase.shot("p11-timed-out")
        button(s(R.string.common_cancel)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_intro_title))

        // Leaving the app while the page waits for the TV: the home screen, then the app again.
        script.gate = CompletableDeferred()
        scanAndConnect()
        waitForText(s(R.string.app_tv_run_waiting))
        val stages = ActivityLifecycleMonitorRegistry.getInstance()
        InstrumentationRegistry.getInstrumentation().uiAutomation.performGlobalAction(AccessibilityService.GLOBAL_ACTION_HOME)
        compose.waitUntil(20_000) {
            var stopped = false
            InstrumentationRegistry.getInstrumentation().runOnMainSync { stopped = stages.getActivitiesInStage(Stage.STOPPED).isNotEmpty() }
            stopped
        }
        context.startActivity(Intent(context, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_REORDER_TO_FRONT))
        waitForText(s(R.string.app_tv_fail_leftAfter))
        showcase.shot("p12-left")
        button(s(R.string.app_tv_fail_ok)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_intro_title))

        // No scanner on the device, and the sheet of the relay.
        scanner.next = CompletableDeferred()
        button(s(R.string.app_tv_intro_scan)).performScrollTo().performClick()
        scanner.next.complete(null)
        waitForText(s(R.string.app_tv_problem_unavailable))
        showcase.shot("p13-no-scanner")
        button(s(R.string.app_tv_relay_change)).performScrollTo().performClick()
        waitForText(s(R.string.tv_relay_lead))
        showcase.shot("p14-relay")
    }
}
