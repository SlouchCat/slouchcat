package com.slouchcat.app.tv

import android.graphics.Bitmap
import android.os.Bundle
import android.view.KeyEvent
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isFocused
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.performImeAction
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.tv.common.DataStoreRelaySettings
import com.slouchcat.app.tv.ui.QrContent
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SessionCredentials
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.ProfilesApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.PinRequest
import com.slouchcat.core.api.generated.models.SelectProfileRequest
import com.slouchcat.core.api.generated.models.SetupRequest
import com.slouchcat.core.strings.R
import kotlinx.coroutines.runBlocking
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.FixMethodOrder
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.junit.runners.MethodSorters
import java.io.File

/**
 * The way of a new TV through the app against a real server (scripts/android-tv-e2e.sh, concept 26
 * stage 2a): (a) set up by hand with the D-pad and the input of the system, profile with PIN (a
 * wrong one first), «Einstellungen» with «Server und Konto», sign this TV out; (b) set up by scan:
 * the TV shows its QR code, the test hands its content out as an instrumentation status
 * (`pairCode=…`), the script lets `apicheck -pair-phone` play the phone through the local relay,
 * the TV redeems and lands in the profile choice with server and account (K-9).
 *
 * Arguments (`am instrument -e <name> …`): `slouchcatServer` (default `http://10.0.2.2:8697`),
 * `slouchcatRelay` (default `http://10.0.2.2:8180`), `slouchcatEmail`, `slouchcatPassword`,
 * `slouchcatPin`; `slouchcatShots` saves a photo of every page and menu (the app's `files/shots/`).
 */
@RunWith(AndroidJUnit4::class)
@FixMethodOrder(MethodSorters.NAME_ASCENDING)
class TvFlowTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val args = InstrumentationRegistry.getArguments()
    private val server: HttpUrl = (args.getString("slouchcatServer") ?: "http://10.0.2.2:8697").toHttpUrl()
    private val relay: HttpUrl = (args.getString("slouchcatRelay") ?: "http://10.0.2.2:8180").toHttpUrl()
    private val email = args.getString("slouchcatEmail") ?: "e2e-owner@example.test"
    private val password = args.getString("slouchcatPassword") ?: "e2e-password-0123"
    private val pin = args.getString("slouchcatPin") ?: "246810"
    private val shots = args.getString("slouchcatShots") != null
    private val context = instrumentation.targetContext
    private val identity = ClientIdentity(ClientIdentity.Kind.TV, BuildConfig.VERSION_NAME)

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    /** The account as a household has it: set up, the primary profile with a PIN. */
    private fun prepareServer() {
        val plain = SlouchcatHttp.client(identity)
        val setup = SlouchcatApi.create(::SetupApi, server, plain)
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = email, password = password))
        val login = SlouchcatApi.create(::AuthApi, server, plain).appLogin(AppLoginRequest(email, password, "E2E preparation", AppLoginRequest.Kind.TABLET))!!
        val credentials = object : SessionCredentials {
            override val server = this@TvFlowTest.server
            override val bearerToken = login.token
            override val deviceId: String? = null
        }
        val http = SlouchcatHttp.client(identity, credentials)
        val auth = SlouchcatApi.create(::AuthApi, server, http)
        val primary = auth.listMyProfiles()!!.single { it.isPrimary }
        auth.selectProfile(primary.id, if (primary.hasPin) SelectProfileRequest(pin) else null)
        SlouchcatApi.create(::ProfilesApi, server, http).setProfilePin(primary.id, PinRequest(pin))
        auth.logout()
    }

    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = 30_000, unmerged: Boolean = true) {
        compose.waitUntil(timeout) { compose.onAllNodes(matcher, useUnmergedTree = unmerged).fetchSemanticsNodes().isNotEmpty() }
    }

    private fun waitForText(text: String, timeout: Long = 30_000) = waitFor(hasText(text, substring = true), timeout)

    /** A key of the remote, as the system delivers it. */
    private fun key(code: Int, times: Int = 1) = repeat(times) {
        instrumentation.sendKeyDownUpSync(code)
        compose.waitForIdle()
    }

    /** The element with [text] has the focus (merged: the text sits on a child of the focused button). */
    private fun focused(text: String) =
        waitFor(isFocused() and (hasText(text, substring = true) or hasContentDescription(text, substring = true)), unmerged = false)

    /** A field: OK opens the input of the system, the text goes in, «Fertig» takes it. */
    private fun type(label: String, text: String) {
        focused(label)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(hasSetTextAction() and hasContentDescription(label))
        compose.onNode(hasSetTextAction() and hasContentDescription(label)).performTextInput(text)
        compose.onNode(hasSetTextAction() and hasContentDescription(label)).performImeAction()
        compose.waitForIdle()
    }

    /** From the digit 1 of the pad down to «Öffnen» (bottom right) with the D-pad, and OK. */
    private fun submitPin() {
        focused("1")
        key(KeyEvent.KEYCODE_DPAD_DOWN, 3)
        key(KeyEvent.KEYCODE_DPAD_RIGHT, 2)
        focused(s(R.string.profiles_pin_submit))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
    }

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        instrumentation.waitForIdleSync()
        compose.mainClock.advanceTimeBy(FRAME_MS)
        compose.waitForIdle()
        val bitmap = instrumentation.uiAutomation.takeScreenshot() ?: return
        // The app's own files: the TV image refuses its external files to the test (EACCES on API 34).
        val dir = File(context.filesDir, "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private fun launch(): ActivityScenario<MainActivity> {
        runBlocking { DataStoreRelaySettings(context).set(relay) }
        return ActivityScenario.launch(MainActivity::class.java)
    }

    @Test
    fun a_byHandToSigningOut() {
        prepareServer()
        val scenario = launch()

        // The choice: the phone has the focus, the way by hand is to its right.
        waitForText(s(R.string.tv_setup_welcome))
        shot("01-setup")
        focused(s(R.string.tv_setup_phoneTitle))
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        focused(s(R.string.tv_setup_handTitle))
        key(KeyEvent.KEYCODE_DPAD_CENTER)

        // Address, e-mail, password with the D-pad and the input of the system.
        waitForText(s(R.string.tv_hand_serverLead))
        shot("02-hand-server")
        type(s(R.string.tv_hand_serverLabel), server.toString().trimEnd('/'))
        waitForText(server.host)
        focused(s(R.string.tv_hand_next))
        shot("03-hand-server-found")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_hand_emailLead))
        type(s(R.string.tv_hand_emailLabel), email)
        focused(s(R.string.tv_hand_next))
        shot("04-hand-email")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_hand_passwordLead, server.host))
        type(s(R.string.tv_hand_passwordLabel), "wrong-password-0123")
        focused(s(R.string.tv_hand_signIn))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_hand_wrong))
        shot("05-hand-wrong-password")
        type(s(R.string.tv_hand_passwordLabel), password)
        focused(s(R.string.tv_hand_signIn))
        key(KeyEvent.KEYCODE_DPAD_CENTER)

        // The profile choice says where the TV is connected (K-9); a wrong PIN first.
        waitForText(s(R.string.profiles_picker_title))
        waitForText(s(R.string.tv_profiles_connected, server.host, email))
        shot("06-profiles")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_pin_prompt))
        shot("07-pin")
        "0000".forEach { key(KeyEvent.KEYCODE_0 + (it - '0')) }
        submitPin()
        waitForText(s(R.string.profiles_pin_invalid))
        shot("08-pin-wrong")
        pin.forEach { key(KeyEvent.KEYCODE_0 + (it - '0')) }
        shot("09-pin-typed")
        submitPin()

        // «Einstellungen» (the first entry of the rail while no other page exists).
        waitForText(s(R.string.tv_settings_switchPane))
        shot("10-settings")
        key(KeyEvent.KEYCODE_DPAD_LEFT)
        shot("11-rail-open")
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        focused(s(R.string.tv_shell_switchProfile))
        key(KeyEvent.KEYCODE_DPAD_DOWN)
        focused(s(R.string.tv_settings_server))
        shot("12-settings-server")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_settings_tvHint))
        shot("13-server-menu")
        key(KeyEvent.KEYCODE_DPAD_DOWN)
        focused(s(R.string.tv_settings_signOut))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_settings_signOutBody))
        focused(s(R.string.common_cancel))
        shot("14-sign-out-question")
        key(KeyEvent.KEYCODE_DPAD_LEFT)
        focused(s(R.string.tv_settings_signOutConfirm))
        key(KeyEvent.KEYCODE_DPAD_CENTER)

        // Signed out: the setup again.
        waitForText(s(R.string.tv_setup_welcome))
        waitForText(s(R.string.tv_settings_signedOut))
        shot("15-signed-out")
        scenario.close()
    }

    @Test
    fun b_byScan() {
        prepareServer()
        val scenario = launch()
        waitForText(s(R.string.tv_setup_welcome))
        focused(s(R.string.tv_setup_phoneTitle))
        key(KeyEvent.KEYCODE_DPAD_CENTER)

        // The QR code: its content goes out to the script, which plays the phone.
        val withCode = SemanticsMatcher.keyIsDefined(QrContent)
        waitFor(withCode)
        shot("20-scan-code")
        val code = compose.onNode(withCode, useUnmergedTree = true).fetchSemanticsNode().config[QrContent]
        instrumentation.sendStatus(0, Bundle().apply { putString("pairCode", code) })

        // The phone laid the packet down; the TV redeemed it and shows server and account.
        waitForText(s(R.string.tv_pair_linkedLead), timeout = 150_000)
        waitForText(email)
        waitForText(server.host)
        shot("21-scan-linked")
        focused(s(R.string.tv_pair_continue))
        key(KeyEvent.KEYCODE_DPAD_CENTER)

        waitForText(s(R.string.profiles_picker_title))
        waitForText(s(R.string.tv_profiles_connected, server.host, email))
        shot("22-scan-profiles")
        scenario.close()
    }

    private companion object {
        const val FRAME_MS = 500L
    }
}
