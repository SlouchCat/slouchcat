package com.slouchcat.app.mobile.profilesettings

import android.content.Context
import androidx.compose.material3.SnackbarHostState
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsOff
import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.isToggleable
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.api.generated.models.Language
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibrarySummary
import com.slouchcat.core.api.generated.models.Profile
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.browse.profilesettings.PinError
import com.slouchcat.core.browse.profilesettings.PinKind
import com.slouchcat.core.browse.profilesettings.ProfileEditor
import com.slouchcat.core.browse.profilesettings.ProfileSettingsUiState
import com.slouchcat.core.browse.testing.profile
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private const val PHONE = "w412dp-h915dp"
private const val TABLET = "w1280dp-h800dp-land"

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/** «Profile verwalten» (rule 8): two widths, 200 % font, the advice, the rows, the sheet with every control, read-only. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class ProfileSettingsScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()

    private val recorder = object : ProfileSettingsEvents {
        override fun onBack() { events += "back" }
        override fun onRetry() { events += "retry" }
        override fun onSwitchProfile() { events += "switch" }
        override fun onAdvice() { events += "advice" }
        override fun onAdd() { events += "add" }
        override fun onEdit(profile: Profile) { events += "edit:${profile.id}" }
        override fun onDismiss() { events += "dismiss" }
        override fun onName(name: String) { events += "name:$name" }
        override fun onAccent(accent: Accent) { events += "accent:${accent.value}" }
        override fun onRating(age: Int) { events += "rating:$age" }
        override fun onToggleLanguages() { events += "languages" }
        override fun onLanguage(language: Language) { events += "language:${language.value}" }
        override fun onAllLibraries(all: Boolean) { events += "all:$all" }
        override fun onLibrary(id: Long, on: Boolean) { events += "library:$id:$on" }
        override fun onWatchLimit(text: String) { events += "limit:$text" }
        override fun onCanRequest(on: Boolean) { events += "request:$on" }
        override fun onPinForm(kind: PinKind?) { events += "form:$kind" }
        override fun onPinFirst(digits: String) { events += "first:${digits.length}" }
        override fun onPinSecond(digits: String) { events += "second:${digits.length}" }
        override fun onPinApply() { events += "apply" }
        override fun onPinRemove(kind: PinKind) { events += "remove:$kind" }
        override fun onDeleteAsk(ask: Boolean) { events += "deleteAsk:$ask" }
        override fun onDelete() { events += "delete" }
        override fun onSave() { events += "save" }
    }

    private val profiles = listOf(profile(1, "Marco", primary = true), profile(2, "Lea", rating = 16, hasPin = true), profile(3, "Kinder", rating = 6))

    private fun state(editor: ProfileEditor? = null) = ProfileSettingsUiState(
        profiles = Loadable.Ready(profiles),
        libraries = listOf(LibrarySummary(1, "Filme", LibraryKind.VIDEO), LibrarySummary(2, "Serien", LibraryKind.VIDEO)),
        editor = editor,
    )

    private fun show(state: ProfileSettingsUiState) = compose.setContent { SlouchcatTheme { ProfileSettingsScreen(state, SnackbarHostState(), recorder) } }

    private fun list() {
        show(state())
        compose.onNode(hasText(s(R.string.app_profiles_manage_title)) and isHeading()).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.profiles_pinAdvice_body)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNode(hasText(s(R.string.profiles_pinAdvice_action)) and hasClickAction()).performClick()
        compose.onNode(hasContentDescription(s(R.string.app_profiles_manage_editLabel, "Lea"), substring = true) and hasClickAction()).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.profiles_settings_add)) and hasClickAction())).performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("advice", "edit:2", "add", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun listPhone() = list()

    @Test @Config(qualifiers = TABLET)
    fun listTablet() = list()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun listPhoneLargeFont() = list()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun listTabletLargeFont() = list()

    private fun editor() {
        val lea = ProfileEditor(id = 2, name = "Lea", accent = Accent.SKY, rating = 16, hasPin = true, allLibraries = false, libraryIds = setOf(1), languagesOpen = true)
        show(state(lea))
        compose.onNode(hasText(s(R.string.profiles_settings_form_editTitle)) and isHeading()).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_profiles_manage_ratingUpTo, "16")).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNodeWithContentDescription(s(R.string.accent_rose))).performClick()
        compose.reveal(compose.onNode(hasText("12") and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText("English") and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText("Serien") and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.app_profiles_manage_change)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.app_profiles_manage_remove)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.app_profiles_manage_set)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.profiles_settings_delete_confirm)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.profiles_settings_form_save)) and hasClickAction())).performClick()
        assertEquals(
            listOf("accent:rose", "rating:12", "language:en", "library:2:true", "form:Pin", "remove:Pin", "form:Offline", "deleteAsk:true", "save"),
            events,
        )
    }

    @Test @Config(qualifiers = PHONE)
    fun editorPhone() = editor()

    @Test @Config(qualifiers = TABLET)
    fun editorTablet() = editor()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun editorPhoneLargeFont() = editor()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun editorTabletLargeFont() = editor()

    private val requestSwitch get() = compose.onNode(hasText(s(R.string.profiles_settings_form_canRequest)) and isToggleable())

    private fun request() {
        show(state(ProfileEditor(id = 2, name = "Lea", rating = 16, canRequest = true)).copy(requestShown = true))
        compose.reveal(compose.onNode(hasText(s(R.string.profiles_settings_form_wishes)) and isHeading())).assertIsDisplayed()
        compose.reveal(compose.onNodeWithText(s(R.string.profiles_settings_form_canRequestHelp))).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(requestSwitch).assertIsOn().performClick()
        assertEquals(listOf("request:false"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun requestPhone() = request()

    @Test @Config(qualifiers = TABLET)
    fun requestTablet() = request()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun requestPhoneLargeFont() = request()

    private fun requestCapped(state: ProfileSettingsUiState) {
        show(state)
        compose.reveal(compose.onNodeWithText(s(R.string.profiles_settings_form_canRequestCapped))).assertIsDisplayed()
        assertTrue(compose.onAllNodes(hasText(s(R.string.profiles_settings_form_canRequestHelp))).fetchSemanticsNodes().isEmpty())
        compose.reveal(requestSwitch).assertIsOff().assertIsNotEnabled().performClick()
        assertTrue(events.isEmpty())
    }

    @Test @Config(qualifiers = PHONE)
    fun requestCappedByTheAccountPhone() = requestCapped(state(ProfileEditor(id = 2, name = "Lea", canRequest = true)).copy(requestShown = true, requestCapped = true))

    @Test @Config(qualifiers = TABLET)
    fun requestRefusedByTheServerTablet() = requestCapped(state(ProfileEditor(id = 2, name = "Lea", canRequest = true, requestRefused = true)).copy(requestShown = true))

    private fun requestHidden() {
        show(state(ProfileEditor(id = 2, name = "Lea", canRequest = true)))
        compose.reveal(compose.onNode(hasText(s(R.string.profiles_settings_form_save)) and hasClickAction())).assertIsDisplayed()
        assertTrue(compose.onAllNodes(hasText(s(R.string.profiles_settings_form_canRequest))).fetchSemanticsNodes().isEmpty())
        assertTrue(compose.onAllNodes(hasText(s(R.string.profiles_settings_form_wishes))).fetchSemanticsNodes().isEmpty())
    }

    @Test @Config(qualifiers = PHONE)
    fun requestHiddenPhone() = requestHidden()

    @Test @Config(qualifiers = TABLET)
    fun requestHiddenTablet() = requestHidden()

    private fun pinForm() {
        val form = ProfileEditor(name = "Neu", pinForm = PinKind.Offline, pinError = PinError.Mismatch)
        // The list behind the sheet still loads: its «Profil hinzufügen» does not stand next to the sheet's button.
        show(state(form).copy(profiles = Loadable.Loading))
        compose.reveal(compose.onNodeWithText(s(R.string.app_profiles_manage_offlineExplain))).assertIsDisplayed()
        compose.reveal(compose.onNodeWithText(s(R.string.app_profiles_manage_mismatch))).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasSetTextAction() and hasText(s(R.string.profiles_settings_pin_label)))).performTextInput("123456")
        compose.reveal(compose.onNode(hasText(s(R.string.app_profiles_manage_apply)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.profiles_settings_form_create)) and hasClickAction())).performClick()
        assertEquals(listOf("first:6", "apply", "save"), events)
        assertTrue(compose.onAllNodes(hasText(s(R.string.profiles_settings_delete_confirm))).fetchSemanticsNodes().isEmpty())
    }

    @Test @Config(qualifiers = PHONE)
    fun pinFormPhone() = pinForm()

    @Test @Config(qualifiers = TABLET)
    fun pinFormTablet() = pinForm()

    private fun deleteAsks() {
        show(state(ProfileEditor(id = 3, name = "Kinder", rating = 6, confirmDelete = true, error = "validation_failed")))
        compose.reveal(compose.onNodeWithText(s(R.string.app_profiles_manage_deleteWarning, "Kinder"))).assertIsDisplayed()
        compose.reveal(compose.onNodeWithText(s(R.string.errors_code_validation_failed))).assertIsDisplayed()
        compose.reveal(compose.onNode(hasText(s(R.string.app_profiles_manage_keep)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.profiles_settings_delete_action)) and hasClickAction())).performClick()
        assertEquals(listOf("deleteAsk:false", "delete"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun deleteAsksPhone() = deleteAsks()

    @Test @Config(qualifiers = TABLET)
    fun deleteAsksTablet() = deleteAsks()

    private fun readOnly() {
        show(ProfileSettingsUiState(canManage = false, summaries = Loadable.Ready(listOf(ProfileSummary(1, "Marco", false, true), ProfileSummary(2, "Lea", true, false)))))
        compose.onNodeWithText(s(R.string.profiles_settings_onlyPrimary_title), substring = true).assertIsDisplayed()
        compose.onNodeWithText("Lea").assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        // The rows are no controls; the only way is «Profil wechseln».
        compose.onNode(hasText(s(R.string.profiles_settings_onlyPrimary_action)) and hasClickAction()).performClick()
        assertEquals(listOf("switch"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun readOnlyPhone() = readOnly()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun readOnlyTabletLargeFont() = readOnly()

    private fun failed() {
        show(ProfileSettingsUiState(profiles = Loadable.Failed(LoadError.Offline)))
        compose.onNode(hasText(s(R.string.app_state_retry)) and hasClickAction()).performClick()
        assertEquals(listOf("retry"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun failedPhone() = failed()

    @Test @Config(qualifiers = TABLET)
    fun failedTablet() = failed()
}
