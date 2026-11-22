package com.slouchcat.app.mobile

import android.graphics.Bitmap
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.features
import com.slouchcat.app.mobile.tvlink.CodeScanner
import com.slouchcat.app.mobile.tvlink.MemoryRelaySettings
import com.slouchcat.app.mobile.tvlink.TvLinkFeature
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SessionEvents
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.apis.SystemApi
import com.slouchcat.core.api.generated.infrastructure.ClientError
import com.slouchcat.core.api.generated.infrastructure.ClientException
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.SelectProfileRequest
import com.slouchcat.core.api.generated.models.SetupRequest
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.pairing.ApiRedeemer
import com.slouchcat.core.pairing.PairingCode
import com.slouchcat.core.pairing.RelayClient
import com.slouchcat.core.pairing.TvPairing
import com.slouchcat.core.session.AccountStore
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.session.TokenStore
import com.slouchcat.core.strings.R
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.runBlocking
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File
import java.util.concurrent.atomic.AtomicReference
import kotlin.concurrent.thread

/**
 * The scanner of the emulator flows (no camera there): what the TV shows, handed over when the flow
 * says so; [next] completed with null reports «no scanner».
 */
internal class HandedScanner : CodeScanner {
    @Volatile
    var next = CompletableDeferred<String?>()

    override val asksForCamera: Boolean = false

    @Composable
    override fun Camera(onCode: (String) -> Unit, onClose: () -> Unit, onUnavailable: () -> Unit, modifier: Modifier) {
        val waiting = next
        LaunchedEffect(waiting) { waiting.await()?.let(onCode) ?: onUnavailable() }
        Box(modifier)
    }
}

/**
 * «Fernseher verbinden» against a real server and a real relay (task android:e2e, concept 27 section
 * 10 and 13 «9 Fernseher verbinden»): the phone app scans, asks, fetches the voucher, seals the
 * packet and lays it down; the TV is played in this test by :core:pairing, the code the TV app runs
 * (`TvPairing`): it opens the meeting point, takes the packet, redeems the voucher at the address
 * the phone sent and opens its token. Then the refusals of the relay with real answers: 410 (the
 * code used up), 404 (an unknown meeting point), no relay at all; and two codes the phone refuses.
 *
 * The camera is the only seam: the scanner of the test hands over the content of the TV's code
 * (no camera in the emulator). Arguments as [SignInFlowTest], and `slouchcatRelay` (the relay the
 * emulator reaches, `http://10.0.2.2:<port>`, started by scripts/android-e2e.sh).
 */
@RunWith(AndroidJUnit4::class)
class TvLinkFlowTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val args = InstrumentationRegistry.getArguments()
    private val server: HttpUrl = (args.getString("slouchcatServer") ?: "http://10.0.2.2:8597").toHttpUrl()
    private val relay: HttpUrl = requireNotNull(args.getString("slouchcatRelay")) { "the flow needs a relay: -e slouchcatRelay http://10.0.2.2:<port>" }.toHttpUrl()
    private val email = args.getString("slouchcatEmail") ?: "e2e-owner@example.test"
    private val password = args.getString("slouchcatPassword") ?: "e2e-password-0123"
    private val pin = args.getString("slouchcatPin") ?: "246810"
    private val shots = args.getString("slouchcatShots") != null
    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, BuildConfig.VERSION_NAME)

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private class Servers(server: SavedServer) : ServerStore {
        private val flow = MutableStateFlow<SavedServer?>(server)
        override val server: Flow<SavedServer?> = flow
        override suspend fun save(server: SavedServer) {
            flow.value = server
        }
        override suspend fun confirmKey(serverKey: String) = Unit
        override suspend fun updateVersion(version: String) = Unit
        override suspend fun clear() {
            flow.value = null
        }
    }

    private class Tokens(private var stored: StoredCredentials?) : TokenStore {
        override suspend fun read() = stored
        override suspend fun write(credentials: StoredCredentials) {
            stored = credentials
        }
    }

    private class Accounts(private var email: String?, private var profile: ActiveProfile?) : AccountStore {
        override suspend fun read() = email to profile
        override suspend fun write(email: String?, profile: ActiveProfile?) {
            this.email = email
            this.profile = profile
        }
    }

    private fun <T> withinTheLimit(call: () -> T): T = try {
        call()
    } catch (e: ClientException) {
        if (e.statusCode != 429) throw e
        val pause = (e.response as? ClientError<*>)?.headers?.entries?.firstOrNull { it.key.equals("Retry-After", ignoreCase = true) }
            ?.value?.firstOrNull()?.trim()?.toLongOrNull() ?: DEFAULT_PAUSE_S
        Thread.sleep(pause * 1000)
        call()
    }

    /** The phone signed in with the account's primary profile, as after «Anmelden» and the profile choice. */
    private fun signIn(): AppGraph {
        val plain = SlouchcatHttp.client(identity)
        val setup = SlouchcatApi.create(::SetupApi, server, plain)
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = email, password = password))
        val health = SlouchcatApi.create(::SystemApi, server, plain).getHealth()!!
        val login = withinTheLimit { SlouchcatApi.create(::AuthApi, server, plain).appLogin(AppLoginRequest(email, password, "Flow phone", AppLoginRequest.Kind.PHONE))!! }
        val credentials = object : com.slouchcat.core.api.SessionCredentials {
            override val server = this@TvLinkFlowTest.server
            override val bearerToken = login.token
            override val deviceId: String? = null
        }
        val auth = SlouchcatApi.create(::AuthApi, server, SlouchcatHttp.client(identity, credentials))
        val primary = auth.listMyProfiles()!!.single { it.isPrimary }
        withinTheLimit { auth.selectProfile(primary.id, if (primary.hasPin) SelectProfileRequest(pin) else null) }
        val feature = TvLinkFeature(scanner = scanner, relaySettings = { relaySettings })
        val list = features.map { if (it is TvLinkFeature) feature else it }
        lateinit var graph: AppGraph
        InstrumentationRegistry.getInstrumentation().runOnMainSync {
            graph = AppGraph(
                context,
                list,
                Servers(SavedServer(server, health.version, serverKey = health.serverKey)),
                Tokens(StoredCredentials(login.token, login.deviceId)),
                Accounts(email, ActiveProfile(primary.id, primary.name, isPrimary = true)),
            )
        }
        return graph
    }

    private val scanner = HandedScanner()
    private val relaySettings by lazy { MemoryRelaySettings(relay) }

    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = 30_000) = compose.waitForNode(matcher, timeout)

    private fun waitForText(text: String, timeout: Long = 30_000) = waitFor(hasText(text, substring = true), timeout)

    private fun button(text: String) = compose.onAllNodes(hasText(text) and hasClickAction()).onLast()

    private var activity: MainActivity? = null

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        InstrumentationRegistry.getInstrumentation().runOnMainSync { activity?.window?.decorView?.invalidate() }
        InstrumentationRegistry.getInstrumentation().waitForIdleSync()
        compose.waitForIdle()
        runCatching { InstrumentationRegistry.getInstrumentation().uiAutomation.waitForIdle(500, 5_000) }
        val bitmap = InstrumentationRegistry.getInstrumentation().uiAutomation.takeScreenshot() ?: return
        val dir = File(context.getExternalFilesDir(null), "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    /** The TV of the flow: the TV app's own pairing code over the same relay, redeeming at the packet's address. */
    private fun tv(relayAddress: HttpUrl = relay): TvPairing {
        val tvIdentity = ClientIdentity(ClientIdentity.Kind.TV, BuildConfig.VERSION_NAME)
        val redeemer = ApiRedeemer(ApiCaller(SessionEvents())) { _, _ -> SlouchcatHttp.client(tvIdentity, builder = OkHttpClient.Builder().followRedirects(false)) }
        return TvPairing(RelayClient(OkHttpClient(), relayAddress), redeemer)
    }

    /** «Scanner öffnen», the code, the question. */
    private fun scan(text: String) {
        scanner.next = CompletableDeferred()
        button(s(R.string.app_tv_intro_scan)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_scan_aim))
        scanner.next.complete(text)
    }

    private fun connectAndExpect(text: String, shotName: String, dismiss: Int = R.string.app_tv_fail_ok) {
        waitForText(s(R.string.app_tv_confirm_title))
        button(s(R.string.app_tv_confirm_go)).performScrollTo().performClick()
        waitForText(text, timeout = 60_000)
        shot(shotName)
        button(s(dismiss)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_intro_title))
    }

    @Test
    fun aTvConnectsThroughTheRelay() {
        val graph = signIn()
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        scenario.onActivity { resumed ->
            activity = resumed
            resumed.setContent { SlouchcatTheme { SlouchcatNavHost(graph) } }
        }

        // «Mehr › Fernseher verbinden».
        // The target «Mehr» as a jump of the app (bottom bar and side column name it differently).
        waitFor(hasContentDescription(s(R.string.app_more_title)) or hasText(s(R.string.app_more_title)))
        InstrumentationRegistry.getInstrumentation().runOnMainSync { graph.links.request(AppLink("more", emptyList())) }
        waitForText(s(R.string.app_tv_moreHint))
        shot("t01-more")
        button(s(R.string.app_tv_title)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_intro_title))
        shot("t02-intro")

        // The TV shows its code; the phone scans it.
        val tv = tv()
        val meeting = (runBlocking { tv.start() } as TvPairing.Started.Ok).meeting
        scanner.next = CompletableDeferred()
        button(s(R.string.app_tv_intro_scan)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_scan_aim))
        shot("t03-scanner")
        scanner.next.complete(meeting.code.text)
        waitForText(s(R.string.app_tv_confirm_title))
        waitForText(email)
        shot("t04-confirm")

        // The TV waits for the packet and redeems it, as the TV app does.
        val redeemed = AtomicReference<Any?>(null)
        val tvSide = thread(name = "tv") {
            runBlocking {
                val waited = tv.awaitPacket(meeting)
                redeemed.set(
                    if (waited is TvPairing.Waited.Packet) tv.redeem(meeting, waited.packet, waited.packet.server.toHttpUrl(), "Flow TV", "Android TV 14") else waited,
                )
            }
        }
        button(s(R.string.app_tv_confirm_go)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_run_title))
        shot("t05-running")
        tvSide.join(120_000)
        val result = redeemed.get()
        assertTrue("the TV did not redeem: $result", result is TvPairing.Redeem.Ok)
        assertEquals(email, (result as TvPairing.Redeem.Ok).answer.accountEmail)
        waitForText(s(R.string.app_tv_done_title), timeout = 60_000)
        waitForText("Flow TV")
        shot("t06-connected")
        button(s(R.string.app_tv_done_finish)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_moreHint))
        button(s(R.string.app_tv_title)).performScrollTo().performClick()
        waitForText("Flow TV")
        shot("t07-intro-linked")

        // The same code again: the relay answers 410 (picked up), the phone warns.
        scan(meeting.code.text)
        connectAndExpect(s(R.string.app_tv_fail_occupied), "t08-occupied")

        // A meeting point the relay does not know (404): the code ran out.
        scan(PairingCode(relay, Base64Url.encode(ByteArray(16) { 7 }), meeting.code.tvKey).text)
        connectAndExpect(s(R.string.app_tv_fail_expired), "t09-expired", dismiss = R.string.common_cancel)

        // No relay at all (7.7): the way by hand on the TV.
        val dead = relay.newBuilder().port(1).build()
        relaySettings.flow.value = dead
        scan(PairingCode(dead, meeting.code.id, meeting.code.tvKey).text)
        connectAndExpect(s(R.string.app_tv_fail_relay), "t10-relay-down")
        relaySettings.flow.value = relay

        // Codes the phone refuses before it asks anything.
        scan("https://example.test/some/page")
        waitForText(s(R.string.app_tv_problem_notACode))
        shot("t11-not-a-code")
        scan(PairingCode("https://pair.example.test".toHttpUrl(), meeting.code.id, meeting.code.tvKey).text)
        waitForText(s(R.string.app_tv_problem_otherRelay, "pair.example.test"))
        shot("t12-other-relay")
        scenario.close()
    }

    private companion object {
        const val DEFAULT_PAUSE_S = 6L
    }
}
