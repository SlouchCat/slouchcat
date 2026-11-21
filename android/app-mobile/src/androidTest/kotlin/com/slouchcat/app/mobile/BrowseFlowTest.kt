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
import com.slouchcat.app.mobile.browse.CollectionsRoute
import com.slouchcat.app.mobile.browse.ProfilesSettingsRoute
import com.slouchcat.app.mobile.browse.WatchedRoute
import com.slouchcat.app.mobile.browse.WatchlistRoute
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.apis.ProfilesApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.models.SetupRequest
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R
import kotlinx.coroutines.runBlocking
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/**
 * The last part of stage 4 against a real server (task android:e2e): search a title, open it,
 * put it on the watchlist, find it there and take it off; «Gesehen» and «Sammlungen»; in «Profile
 * verwalten» add a profile with a PIN (the recommendation stands while the primary profile has
 * none), choose it with its PIN in the profile choice (the page reads only there), back to the
 * primary profile, delete the new one. With `slouchcatShots` it photographs each page and sheet
 * (prefix `c`).
 */
@RunWith(AndroidJUnit4::class)
class BrowseFlowTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val args = InstrumentationRegistry.getArguments()
    private val server: HttpUrl = (args.getString("slouchcatServer") ?: "http://10.0.2.2:8597").toHttpUrl()
    private val media = args.getString("slouchcatMedia")
    private val email = args.getString("slouchcatEmail") ?: "e2e-owner@example.test"
    private val password = args.getString("slouchcatPassword") ?: "e2e-password-0123"
    private val pin = args.getString("slouchcatPin") ?: "246810"
    private val shots = args.getString("slouchcatShots") != null
    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, BuildConfig.VERSION_NAME)

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = 20_000) {
        compose.waitUntil(timeout) { compose.onAllNodes(matcher, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() }
    }

    private fun waitForText(text: String, timeout: Long = 20_000) = waitFor(hasText(text, substring = true), timeout)

    private fun gone(matcher: SemanticsMatcher, timeout: Long = 20_000) {
        compose.waitUntil(timeout) { compose.onAllNodes(matcher).fetchSemanticsNodes().isEmpty() }
    }

    private fun shown(matcher: SemanticsMatcher) = compose.onAllNodes(matcher, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    private fun button(text: String) = compose.onAllNodes((hasText(text) or hasContentDescription(text)) and hasClickAction()).onLast()

    /** A tap through the node's own action: a tap at its centre may land on a bar over the page. */
    private fun tap(matcher: SemanticsMatcher) = compose.onAllNodes(matcher and hasClickAction()).onLast().performSemanticsAction(SemanticsActions.OnClick)

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        InstrumentationRegistry.getInstrumentation().waitForIdleSync()
        compose.mainClock.advanceTimeBy(FRAME_MS)
        compose.waitForIdle()
        val bitmap = InstrumentationRegistry.getInstrumentation().uiAutomation.takeScreenshot() ?: return
        val dir = File(context.getExternalFilesDir(null), "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private var resumed: MainActivity? = null

    private fun back() = InstrumentationRegistry.getInstrumentation().runOnMainSync { resumed?.onBackPressedDispatcher?.onBackPressed() }

    private val graph: AppGraph get() = (context.applicationContext as SlouchcatApplication).graph

    private fun open(vararg routes: Any) = InstrumentationRegistry.getInstrumentation().runOnMainSync { graph.links.request(AppLink("start", routes.toList())) }

    /**
     * The account as this flow needs it: the test library (the administration's own sign-in), then,
     * through the app's own session after [signIn] (no further sign-in: the server limits them), the
     * primary profile without a PIN (so the recommendation shows) and no profile left from a run before.
     */
    private fun prepareLibrary() {
        val setup = SlouchcatApi.create(::SetupApi, server, SlouchcatHttp.client(identity))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = email, password = password))
        DetailFlowTest.Admin(server, email, password, pin).ensureLibrary(requireNotNull(media) { "run through scripts/android-e2e.sh, which writes the media" })
    }

    private fun prepareProfiles() {
        val profiles = SlouchcatApi.create(::ProfilesApi, server, graph.http)
        val primary = profiles.listProfiles()!!.single { it.isPrimary }
        profiles.removeProfilePin(primary.id)
        profiles.listProfiles()!!.filter { it.name == KID }.forEach { profiles.deleteProfile(it.id) }
    }

    private fun ready(): Boolean = (graph.session.state.value as? SessionState.Ready)?.profile != null

    private fun activeName(): String? = (graph.session.state.value as? SessionState.Ready)?.profile?.name

    /** From wherever the app stands (address, sign-in, profile choice) to the primary profile. */
    private fun signIn() {
        compose.waitUntil(20_000) {
            ready() || shown(hasText(s(R.string.app_setup_title))) || shown(hasText(s(R.string.login_title)) and isHeading())
        }
        if (ready()) return
        if (shown(hasText(s(R.string.app_setup_title)))) {
            compose.onNode(hasSetTextAction() and hasText(s(R.string.app_setup_addressLabel))).performTextInput(server.toString().trimEnd('/'))
            button(s(R.string.app_setup_connect)).performScrollTo().performClick()
            waitForText(s(R.string.app_setup_found))
            button(s(R.string.app_setup_next)).performScrollTo().performClick()
            waitFor(hasText(s(R.string.login_title)) and isHeading())
        }
        compose.onNode(hasSetTextAction() and hasText(s(R.string.login_email))).performTextInput(email)
        compose.onNode(hasSetTextAction() and hasText(s(R.string.login_password))).performTextInput(password)
        button(s(R.string.login_submit)).performScrollTo().performClick()
        compose.waitUntil(20_000) { ready() || shown(hasContentDescription(s(R.string.profiles_picker_primary), substring = true) and hasClickAction()) }
        if (!ready()) choose(s(R.string.profiles_picker_primary), pin)
        compose.waitUntil(20_000) { ready() }
    }

    /** In the profile choice: the profile whose label holds [label], with its [digits] when it asks. */
    private fun choose(label: String, digits: String?) {
        val activeBefore = activeName()
        waitFor(hasContentDescription(label, substring = true) and hasClickAction())
        compose.onAllNodes(hasContentDescription(label, substring = true) and hasClickAction()).onFirst().performClick()
        if (digits != null) {
            // A profile without PIN opens at once; with one the pad asks.
            compose.waitUntil(20_000) { shown(hasText(s(R.string.profiles_pin_hint))) || graph.session.state.value.let { it is SessionState.Ready && it.profile.name != activeBefore } }
            if (!shown(hasText(s(R.string.profiles_pin_hint)))) return
            digits.forEach { button(it.toString()).performClick() }
            compose.onAllNodes(hasContentDescription(s(R.string.profiles_pin_submit))).onLast().performClick()
        }
    }

    /** «Mehr» › «Profil wechseln». */
    private fun toProfileChoice() {
        tap(hasText(s(R.string.app_more_title)) or hasContentDescription(s(R.string.app_more_title), substring = true))
        waitFor(hasText(s(R.string.app_more_switchProfile)))
        // The media choice (closed, off screen on the phone) has the same control: its own action, not a tap.
        tap(hasText(s(R.string.app_more_switchProfile)))
    }

    @After
    fun forgetTheServer() = runBlocking {
        if (graph.session.state.value is SessionState.Ready) graph.sessionActions.signOut()
        graph.serverStore.clear()
    }

    @Test
    fun searchWatchlistProfiles() {
        prepareLibrary()
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        scenario.onActivity { resumed = it }
        signIn()
        prepareProfiles()

        // Search: the ways in, a text, the match, its page, «Merken».
        tap(hasText(s(R.string.nav_search)) or hasContentDescription(s(R.string.nav_search), substring = true))
        waitFor(hasContentDescription(s(R.string.search_label)))
        waitForText(s(R.string.app_search_genres))
        shot("c01-search")
        compose.onNode(hasSetTextAction()).performTextInput("night")
        waitFor(hasContentDescription("Night Train", substring = true) and hasClickAction())
        shot("c02-search-query")
        tap(hasContentDescription("Night Train", substring = true))
        waitFor(hasText("Night Train") and isHeading())
        if (shown(hasText(s(R.string.item_watchlist_add)))) {
            button(s(R.string.item_watchlist_add)).performScrollTo().performClick()
        }
        waitFor(hasText(s(R.string.item_watchlist_added)))

        // The watchlist holds it; «Bearbeiten», ✕, the empty list.
        open(WatchlistRoute)
        waitFor(hasText(s(R.string.nav_watchlist)) and isHeading())
        waitFor(hasContentDescription("Night Train", substring = true))
        shot("c03-watchlist")
        button(s(R.string.app_lists_edit)).performClick()
        waitFor(hasContentDescription(s(R.string.watchlist_removeLabel, "Night Train")))
        shot("c04-watchlist-edit")
        button(s(R.string.watchlist_removeLabel, "Night Train")).performClick()
        waitForText(s(R.string.watchlist_empty_title))
        shot("c05-watchlist-empty")

        // «Gesehen» and «Sammlungen» (no collection without TMDB: the calm empty state).
        open(WatchedRoute)
        waitFor(hasText(s(R.string.watched_page_title)) and isHeading())
        compose.waitUntil(20_000) { !shown(hasContentDescription(s(R.string.app_state_loading))) }
        shot("c06-watched")
        open(CollectionsRoute)
        waitFor(hasText(s(R.string.collection_all_title)) and isHeading())
        waitForText(s(R.string.collection_all_empty))
        shot("c07-collections")

        // «Profile verwalten»: the recommendation, a new profile with a PIN.
        open(ProfilesSettingsRoute)
        waitFor(hasText(s(R.string.app_profiles_manage_title)) and isHeading())
        waitForText(s(R.string.profiles_settings_add))
        shot("c08-profiles")
        button(s(R.string.profiles_settings_add)).performScrollTo().performClick()
        waitFor(hasText(s(R.string.profiles_settings_form_createTitle)) and isHeading())
        shot("c09-profile-add")
        compose.onNode(hasSetTextAction() and hasText(s(R.string.profiles_settings_form_name))).performTextInput(KID)
        tap(hasText("12") or hasText("FSK 12"))
        // The first «Setzen» is the PIN's (the offline PIN's follows).
        compose.onAllNodes(hasText(s(R.string.app_profiles_manage_set)) and hasClickAction()).onFirst().performScrollTo().performClick()
        compose.onNode(hasSetTextAction() and hasText(s(R.string.profiles_settings_pin_label))).performScrollTo().performTextInput(KID_PIN)
        compose.onNode(hasSetTextAction() and hasText(s(R.string.app_profiles_manage_repeat))).performScrollTo().performTextInput(KID_PIN)
        shot("c10-profile-pin")
        button(s(R.string.app_profiles_manage_apply)).performScrollTo().performClick()
        waitForText(s(R.string.app_profiles_manage_pendingSet))
        button(s(R.string.profiles_settings_form_create)).performScrollTo().performClick()
        waitFor(hasContentDescription(s(R.string.app_profiles_manage_editLabel, KID), substring = true))
        waitForText(s(R.string.profiles_pinAdvice_body))
        shot("c11-profiles-advice")
        tap(hasContentDescription(s(R.string.app_profiles_manage_editLabel, KID), substring = true))
        waitFor(hasText(s(R.string.profiles_settings_form_editTitle)) and isHeading())
        shot("c12-profile-edit")
        back()

        // The profile choice shows it with its lock; with the PIN the app runs as it.
        toProfileChoice()
        choose(KID, KID_PIN)
        compose.waitUntil(20_000) { activeName() == KID }
        open(ProfilesSettingsRoute)
        waitForText(s(R.string.profiles_settings_onlyPrimary_title))
        shot("c13-profiles-read-only")

        // Back to the primary profile, the new profile goes.
        toProfileChoice()
        choose(s(R.string.profiles_picker_primary), pin)
        compose.waitUntil(20_000) { ready() && activeName() != KID }
        open(ProfilesSettingsRoute)
        tap(hasContentDescription(s(R.string.app_profiles_manage_editLabel, KID), substring = true))
        button(s(R.string.profiles_settings_delete_confirm)).performScrollTo().performClick()
        waitForText(s(R.string.app_profiles_manage_deleteWarning, KID))
        shot("c14-profile-delete")
        button(s(R.string.profiles_settings_delete_action)).performScrollTo().performClick()
        gone(hasContentDescription(s(R.string.app_profiles_manage_editLabel, KID), substring = true))
        assertEquals(0, compose.onAllNodes(hasContentDescription(s(R.string.app_profiles_manage_editLabel, KID), substring = true)).fetchSemanticsNodes().size)
        scenario.close()
    }

    private companion object {
        const val KID = "E2E Kind"
        const val KID_PIN = "1357"
        const val FRAME_MS = 500L
    }
}
