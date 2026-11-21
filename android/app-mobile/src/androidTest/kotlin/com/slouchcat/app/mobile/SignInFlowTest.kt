package com.slouchcat.app.mobile

import android.graphics.Bitmap
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SessionCredentials
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.apis.AccountApi
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.ProfilesApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.infrastructure.ClientError
import com.slouchcat.core.api.generated.infrastructure.ClientException
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.api.generated.models.PinRequest
import com.slouchcat.core.api.generated.models.SelectProfileRequest
import com.slouchcat.core.api.generated.models.SetupRequest
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/**
 * The way of a new phone through the app against a real server (task android:e2e, concept 27 12
 * «Gerät»): server address, sign-in (a wrong password first), profile choice with PIN (a wrong PIN
 * first), the start page and a library with filter and order (stage 4), «Mehr», «Meine Geräte» with
 * this device and signing out another one, «Konto», sign-out.
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
        val http = signedIn(TV_NAME, password)
        val auth = SlouchcatApi.create(::AuthApi, server, http)
        val primary = auth.listMyProfiles()!!.single { it.isPrimary }
        auth.selectProfile(primary.id, if (primary.hasPin) SelectProfileRequest(pin) else null)
        SlouchcatApi.create(::ProfilesApi, server, http).setProfilePin(primary.id, PinRequest(pin))
        // What stage 4 browses, read before the wrong PIN of the flow (it slows the profile down).
        val items = SlouchcatApi.create(::ItemsApi, server, http)
        libraries = items.listVisibleLibraries()!!
        filmGenre = libraries.firstOrNull { it.kind == LibraryKind.VIDEO }?.let { items.listLibraryItems(it.id, limit = 1)!!.facets.genres.firstOrNull()?.name }
    }

    private var libraries: List<LibraryNav> = emptyList()
    private var filmGenre: String? = null

    /** A client of another device (a TV) signed in to the account. */
    private fun signedIn(name: String, secret: String, kind: AppLoginRequest.Kind = AppLoginRequest.Kind.TV): okhttp3.OkHttpClient {
        val auth = SlouchcatApi.create(::AuthApi, server, SlouchcatHttp.client(identity))
        val request = AppLoginRequest(email, secret, name, kind)
        val login = try {
            auth.appLogin(request)!!
        } catch (e: ClientException) {
            if (e.statusCode != 429) throw e
            // The flows of one run share the server's pot of sign-in and PIN attempts (10 per minute and
            // address): once, the pause the server names in Retry-After, then the same sign-in again.
            val pause = (e.response as? ClientError<*>)?.headers?.entries?.firstOrNull { it.key.equals("Retry-After", ignoreCase = true) }
                ?.value?.firstOrNull()?.trim()?.toLongOrNull() ?: DEFAULT_PAUSE_S
            Thread.sleep(pause * 1000)
            auth.appLogin(request)!!
        }
        val credentials = object : SessionCredentials {
            override val server = this@SignInFlowTest.server
            override val bearerToken = login.token
            override val deviceId: String? = null
        }
        return SlouchcatHttp.client(identity, credentials)
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

    /** A media entry: through the menu on the phone, in the side column from 700 dp. */
    private fun openMedia(name: String) {
        val menu = compose.onAllNodes(hasContentDescription(s(R.string.app_shell_menuOpen)) and hasClickAction()).fetchSemanticsNodes()
        if (menu.isNotEmpty()) {
            compose.onAllNodes(hasContentDescription(s(R.string.app_shell_menuOpen)) and hasClickAction()).onFirst().performClick()
            compose.waitForIdle()
        }
        compose.onAllNodes(hasText(name) and hasClickAction()).onLast().performClick()
        waitFor(hasText(name) and isHeading())
        waitFor(hasContentDescription(s(R.string.app_browse_sortLabel, "").trim(), substring = true))
    }

    /**
     * The start page with its content: a row or the empty state, no spinner, and (with libraries,
     * on the phone) the menu button of the media choice. Checked twice: after the profile choice a
     * new session epoch may still replace the page that was already there.
     */
    private fun waitForStart() {
        val content = hasText(s(R.string.home_rows_recent)) or hasText(s(R.string.home_empty_title))
        val loading = hasContentDescription(s(R.string.app_state_loading))
        val phone = context.resources.configuration.screenWidthDp < 700
        repeat(2) {
            compose.waitUntil(30_000) {
                compose.onAllNodes(content, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() &&
                    compose.onAllNodes(loading, useUnmergedTree = true).fetchSemanticsNodes().isEmpty() &&
                    (!phone || libraries.isEmpty() || compose.onAllNodes(hasContentDescription(s(R.string.app_shell_menuOpen))).fetchSemanticsNodes().isNotEmpty())
            }
            compose.waitForIdle()
        }
    }

    /**
     * Stage 4 (concept 27, 13): the start page; with the test media of `task testdata:media` (the
     * server seeded by apicheck) the menu, a library filtered by a genre and a decade and sorted, the
     * square covers of audiobooks and podcasts, and the list behind «Neu hinzugefügt». A server without
     * media shows the empty start page.
     */
    private fun browse() {
        waitForStart()
        shot("b01-start")
        if (libraries.isEmpty()) {
            waitForText(s(R.string.home_empty_title))
            return
        }
        if (compose.onAllNodes(hasContentDescription(s(R.string.app_shell_menuOpen))).fetchSemanticsNodes().isNotEmpty()) {
            compose.onAllNodes(hasContentDescription(s(R.string.app_shell_menuOpen)) and hasClickAction()).onFirst().performClick()
            waitForText(libraries.first().name)
            shot("b02-media-choice")
            back()
        }
        val films = libraries.first { it.kind == LibraryKind.VIDEO }
        openMedia(films.name)
        shot("b03-library")

        // Genre: the facets of the server name what the sheet offers.
        val genre = filmGenre
        if (genre != null) {
            button(s(R.string.library_filters_genre)).performClick()
            waitForText(s(R.string.library_filters_allGenres))
            shot("b04-sheet-genre")
            button(genre).performClick()
            compose.waitUntil(20_000) { compose.onAllNodes(hasText(s(R.string.library_filters_allGenres))).fetchSemanticsNodes().isEmpty() }
            waitFor(hasText(genre) and hasClickAction())
            shot("b05-library-genre")
        }
        button(s(R.string.library_filters_year)).performClick()
        waitForText(s(R.string.app_browse_allYears))
        shot("b06-sheet-year")
        button(s(R.string.common_close)).performScrollTo().performClick()

        compose.onNode(hasContentDescription(s(R.string.app_browse_sortLabel, s(R.string.library_sort_added))) and hasClickAction()).performClick()
        waitForText(s(R.string.app_browse_sortTitle))
        shot("b07-sheet-sort")
        button(s(R.string.app_browse_sortTitleAz)).performClick()
        waitFor(hasContentDescription(s(R.string.app_browse_sortLabel, s(R.string.app_browse_sortTitleAz))))
        if (genre != null) {
            button(s(R.string.app_browse_all)).performClick()
            waitFor(hasText(s(R.string.library_filters_genre)) and hasClickAction())
        }
        shot("b08-library-sorted")
        back()
        waitForStart()

        // Square covers.
        libraries.firstOrNull { it.kind == LibraryKind.AUDIOBOOKS }?.let {
            openMedia(it.name)
            shot("b09-library-books")
            back()
            waitForStart()
        }
        libraries.firstOrNull { it.kind == LibraryKind.PODCASTS }?.let {
            openMedia(it.name)
            shot("b10-library-podcasts")
            back()
            waitForStart()
        }

        // «Alle» of a row: the whole list.
        compose.onNode(hasContentDescription(s(R.string.app_browse_showAllLabel, s(R.string.home_rows_recent))) and hasClickAction()).performScrollTo().performClick()
        waitFor(hasText(s(R.string.home_rows_recent)) and isHeading())
        shot("b11-list-recent")
        back()
        waitForStart()
    }

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

        // Stage 4: the start page, a library with filter and order, the list behind a row.
        browse()

        // «Mehr», the server sheet. With more targets than «Mehr» (stage 4) the app lands on the first one, and a
        // row scrolled to the bottom edge lies under the bar: rows of «Mehr» get their own action, not a tap.
        val more = (hasText(s(R.string.app_more_title)) or hasContentDescription(s(R.string.app_more_title), substring = true)) and hasClickAction()
        compose.waitUntil(20_000) { compose.onAllNodes(more).fetchSemanticsNodes().isNotEmpty() }
        compose.onAllNodes(more).onLast().performSemanticsAction(SemanticsActions.OnClick)
        waitFor(hasText(s(R.string.app_more_title)) and isHeading())
        shot("07-more")
        button(server.host).performScrollTo().performSemanticsAction(SemanticsActions.OnClick)
        waitForText(s(R.string.app_more_serverSheet_hint))
        shot("08-more-server-sheet")
        button(s(R.string.common_close)).performClick()

        // «Meine Geräte»: this device is in the list; the TV of the fixture is signed out.
        button(s(R.string.nav_devices)).performScrollTo().performSemanticsAction(SemanticsActions.OnClick)
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
        button(s(R.string.nav_account)).performScrollTo().performSemanticsAction(SemanticsActions.OnClick)
        waitForText(s(R.string.app_account_adminHint))
        shot("12-account")
        button(s(R.string.app_account_password)).performClick()
        waitForText(s(R.string.app_account_passwordWarning))
        shot("13-account-password-sheet")
        // A new password and back: this session stays signed in (code review minor 32).
        val changed = password + "-new"
        for ((old, new) in listOf(password to changed, changed to password)) {
            if (old == changed) button(s(R.string.app_account_password)).performClick()
            field(s(R.string.app_account_currentPassword)).performTextInput(old)
            field(s(R.string.app_account_newPassword)).performTextInput(new)
            field(s(R.string.app_account_repeatPassword)).performTextInput(new)
            button(s(R.string.app_account_save)).performScrollTo().performClick()
            waitForText(s(R.string.app_account_passwordChanged))
        }
        button(email).performClick()
        waitForText(s(R.string.app_account_newEmail))
        shot("14-account-email-sheet")
        button(s(R.string.common_cancel)).performScrollTo().performClick()
        back()

        // Sign out: the sign-in comes back, the address stays.
        waitFor(hasText(s(R.string.app_more_title)) and isHeading())
        button(s(R.string.nav_logout)).performScrollTo().performSemanticsAction(SemanticsActions.OnClick)
        waitForText(s(R.string.app_signOut_hint))
        shot("15-sign-out-sheet")
        button(s(R.string.nav_logout)).performClick()
        waitFor(hasText(s(R.string.login_title)) and isHeading())
        waitForText(server.host)
        waitForText(s(R.string.app_signOut_done))
        shot("16-signed-out")
        scenario.close()

        // The server ended the session of this phone: another device no longer lists it
        // (a tablet session: a TV may not read the device list).
        val devices = SlouchcatApi.create(::AccountApi, server, signedIn("E2E check", password, AppLoginRequest.Kind.TABLET)).listMyDevices()!!
        assertTrue("the phone is still signed in", devices.none { it.name == describeDevice(context).name })
    }

    private companion object {
        const val TV_NAME = "E2E living room TV"
        const val FRAME_MS = 500L
        const val DEFAULT_PAUSE_S = 6L
    }
}
