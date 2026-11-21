package com.slouchcat.app.mobile

import android.graphics.Bitmap
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
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
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/**
 * The way of a new phone through the app against a real server (task android:e2e, concept 27 12
 * «Gerät»): server address, sign-in (a wrong password first), profile choice with PIN (a wrong PIN
 * first), «Mehr», «Meine Geräte» with this device and signing out another one, «Konto», sign-out.
 *
 * Arguments (`-Pandroid.testInstrumentationRunnerArguments.<name>=…`): `slouchcatServer` (default
 * `http://10.0.2.2:8597`), `slouchcatEmail`, `slouchcatPassword`, `slouchcatPin`; `slouchcatShots`
 * saves a photo of every page and sheet to the app's external files (`shots/`) for the visual check.
 */
@RunWith(AndroidJUnit4::class)
class SignInFlowTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val args = InstrumentationRegistry.getArguments()
    private val server: HttpUrl = (args.getString("slouchcatServer") ?: "http://10.0.2.2:8597").toHttpUrl()
    private val email = args.getString("slouchcatEmail") ?: "e2e-owner@example.test"
    private val password = args.getString("slouchcatPassword") ?: "e2e-password-0123"
    private val pin = args.getString("slouchcatPin") ?: "246810"
    private val shots = args.getString("slouchcatShots") != null
    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, BuildConfig.VERSION_NAME)

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    /**
     * The account as a household has it: set up, a primary profile with a PIN, and a second
     * device that stays signed in (a TV) so the list has one to sign out.
     */
    private fun prepareServer() {
        val plain = SlouchcatHttp.client(identity)
        val setup = SlouchcatApi.create(::SetupApi, server, plain)
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = email, password = password))
        val login = SlouchcatApi.create(::AuthApi, server, plain)
            .appLogin(AppLoginRequest(email, password, TV_NAME, AppLoginRequest.Kind.TV))!!
        val credentials = object : SessionCredentials {
            override val server = this@SignInFlowTest.server
            override val bearerToken = login.token
            override val deviceId: String? = null
        }
        val http = SlouchcatHttp.client(identity, credentials)
        val auth = SlouchcatApi.create(::AuthApi, server, http)
        val primary = auth.listMyProfiles()!!.single { it.isPrimary }
        auth.selectProfile(primary.id, if (primary.hasPin) SelectProfileRequest(pin) else null)
        SlouchcatApi.create(::ProfilesApi, server, http).setProfilePin(primary.id, PinRequest(pin))
    }

    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = 20_000) {
        compose.waitUntil(timeout) { compose.onAllNodes(matcher, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() }
    }

    private fun waitForText(text: String, timeout: Long = 20_000) = waitFor(hasText(text, substring = true), timeout)

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        // The frame on screen may lag behind the semantics tree by a frame or two.
        InstrumentationRegistry.getInstrumentation().waitForIdleSync()
        compose.mainClock.advanceTimeBy(FRAME_MS)
        compose.waitForIdle()
        val bitmap = InstrumentationRegistry.getInstrumentation().uiAutomation.takeScreenshot() ?: return
        val dir = File(context.getExternalFilesDir(null), "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private fun field(label: String) = compose.onNode(hasSetTextAction() and hasText(label))

    private fun button(text: String) = compose.onAllNodes(hasText(text) and hasClickAction()).onLast()

    private fun back() = InstrumentationRegistry.getInstrumentation().runOnMainSync {
        // The system's back: the activity's dispatcher, as the gesture does.
        resumed?.onBackPressedDispatcher?.onBackPressed()
    }

    private var resumed: MainActivity? = null

    @Test
    fun fromTheAddressToSigningOut() {
        prepareServer()
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        scenario.onActivity { resumed = it }

        // Server address (part 1): a server in the home network needs no warning.
        waitForText(s(R.string.app_setup_title))
        field(s(R.string.app_setup_addressLabel)).performTextInput(server.toString().trimEnd('/'))
        button(s(R.string.app_setup_connect)).performScrollTo().performClick()
        waitForText(s(R.string.app_setup_found))
        shot("01-setup-found")
        button(s(R.string.app_setup_next)).performScrollTo().performClick()

        // Sign-in: a wrong password first.
        waitFor(hasText(s(R.string.login_title)) and isHeading())
        shot("02-signin")
        field(s(R.string.login_email)).performTextInput(email)
        field(s(R.string.login_password)).performTextInput("wrong-password-0123")
        button(s(R.string.login_submit)).performScrollTo().performClick()
        waitForText(s(R.string.errors_code_invalid_credentials))
        shot("03-signin-wrong-password")
        field(s(R.string.login_password)).performTextInput(password)
        button(s(R.string.login_submit)).performScrollTo().performClick()

        // Profile choice with PIN: a wrong PIN first.
        waitForText(s(R.string.profiles_picker_title))
        waitFor(hasContentDescription(s(R.string.profiles_picker_primary), substring = true))
        compose.waitUntil(20_000) { compose.onAllNodes(hasContentDescription(s(R.string.app_state_loading))).fetchSemanticsNodes().isEmpty() }
        shot("04-profiles")
        compose.onAllNodes(hasContentDescription(s(R.string.profiles_picker_primary), substring = true) and hasClickAction()).onLast().performClick()
        waitForText(s(R.string.profiles_pin_hint))
        shot("05-pin")
        "0000".forEach { button(it.toString()).performClick() }
        compose.onAllNodes(hasContentDescription(s(R.string.profiles_pin_submit))).onLast().performClick()
        waitForText(s(R.string.profiles_pin_invalid))
        shot("06-pin-wrong")
        pin.forEach { button(it.toString()).performClick() }
        compose.onAllNodes(hasContentDescription(s(R.string.profiles_pin_submit))).onLast().performClick()

        // «Mehr», the server sheet.
        waitFor(hasText(s(R.string.app_more_title)) and isHeading())
        shot("07-more")
        button(server.host).performScrollTo().performClick()
        waitForText(s(R.string.app_more_serverSheet_hint))
        shot("08-more-server-sheet")
        button(s(R.string.common_close)).performClick()

        // «Meine Geräte»: this device is in the list; the TV of the fixture is signed out.
        button(s(R.string.nav_devices)).performScrollTo().performClick()
        waitForText(s(R.string.settings_devices_thisDevice))
        waitForText(describeDevice(context).name)
        shot("09-devices")
        compose.onAllNodes(hasContentDescription(s(R.string.settings_devices_signOut_label, TV_NAME)) and hasClickAction()).onLast().performClick()
        waitForText(s(R.string.settings_devices_signOut_confirm))
        shot("10-devices-sign-out-sheet")
        button(s(R.string.settings_devices_signOut_confirm)).performClick()
        waitForText(s(R.string.settings_devices_signedOut, TV_NAME))
        compose.waitUntil(20_000) { compose.onAllNodes(hasText(TV_NAME)).fetchSemanticsNodes().isEmpty() }
        shot("11-devices-after")
        back()

        // «Konto» and its sheets.
        waitFor(hasText(s(R.string.app_more_title)) and isHeading())
        button(s(R.string.nav_account)).performScrollTo().performClick()
        waitForText(s(R.string.app_account_adminHint))
        shot("12-account")
        button(s(R.string.app_account_password)).performClick()
        waitForText(s(R.string.app_account_passwordWarning))
        shot("13-account-password-sheet")
        button(s(R.string.common_cancel)).performScrollTo().performClick()
        button(email).performClick()
        waitForText(s(R.string.app_account_newEmail))
        shot("14-account-email-sheet")
        button(s(R.string.common_cancel)).performScrollTo().performClick()
        back()

        // Sign out: the sign-in comes back, the address stays.
        waitFor(hasText(s(R.string.app_more_title)) and isHeading())
        button(s(R.string.nav_logout)).performScrollTo().performClick()
        waitForText(s(R.string.app_signOut_hint))
        shot("15-sign-out-sheet")
        button(s(R.string.nav_logout)).performClick()
        waitFor(hasText(s(R.string.login_title)) and isHeading())
        waitForText(server.host)
        shot("16-signed-out")
        scenario.close()
    }

    private companion object {
        const val TV_NAME = "E2E living room TV"
        const val FRAME_MS = 500L
    }
}
