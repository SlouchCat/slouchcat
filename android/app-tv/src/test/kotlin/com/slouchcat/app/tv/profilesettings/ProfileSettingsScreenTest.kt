package com.slouchcat.app.tv.profilesettings

import androidx.activity.ComponentActivity
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.browse.testing.CHILD
import com.slouchcat.core.browse.testing.FakeHome
import com.slouchcat.core.browse.testing.FakeProfileSettings
import com.slouchcat.core.browse.testing.profile
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * «Profile verwalten» with the remote (mockup `/settings/profiles`): from the settings, the list
 * with the recommendation of a PIN, the side menu of a profile with its colour, rating and PIN,
 * saving, deleting with a question; another profile reads why and may switch. No offline PIN.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class ProfileSettingsScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote
    private val menu = hasAnyAncestor(SemanticsMatcher.keyIsDefined(SemanticsProperties.PaneTitle))

    private val repo = FakeProfileSettings(profile(1, "Primary", primary = true), profile(2, "Kid", rating = 12))

    private fun open(profile: com.slouchcat.core.session.ActiveProfile = com.slouchcat.core.browse.testing.PRIMARY) {
        pages.show({ it.copy(home = FakeHome(), profiles = repo) }, profile = profile)
        pages.openRail(pages.s(R.string.tv_settings_title))
        pages.waitFor(pages.s(R.string.app_profiles_manage_title))
        remote.down()
        remote.assertFocused(pages.s(R.string.app_profiles_manage_title))
        remote.ok()
        pages.waitFor(pages.s(if (profile.isPrimary) R.string.profiles_settings_add else R.string.profiles_settings_onlyPrimary_action))
    }

    @Test
    fun theListAdvisesAPinAndEveryControlIsReached() {
        open()
        remote.assertFocused("Primary")
        compose.waitUntil(5_000) { compose.onAllNodes(androidx.compose.ui.test.hasText(pages.s(R.string.profiles_pinAdvice_title))).fetchSemanticsNodes().isNotEmpty() }
        remote.assertEveryControlReachable(pages.page)
        // Back leaves the page for the settings, on its row.
        remote.back()
        remote.assertFocused(pages.s(R.string.app_profiles_manage_title))
    }

    @Test
    fun aProfileIsChangedInItsMenuAndSaved() {
        open()
        remote.down()
        remote.assertFocused("Kid")
        remote.ok()
        remote.assertFocused(pages.s(R.string.profiles_settings_form_name))
        remote.assertEveryControlReachable(menu)
        // A colour, then «Speichern».
        remote.down()
        remote.right()
        remote.ok()
        repeat(MENU_ROWS) { remote.down() }
        remote.assertFocused(pages.s(R.string.profiles_settings_form_save))
        remote.ok()
        compose.waitUntil(5_000) { repo.calls.contains("update:2") }
        remote.settle()
        remote.assertFocused("Kid")
        assertTrue(repo.calls.none { it.startsWith("offline") })
    }

    @Test
    fun deletingAsksFirst() {
        open()
        remote.down()
        remote.ok()
        repeat(MENU_ROWS) { remote.down() }
        remote.right()
        remote.assertFocused(pages.s(R.string.profiles_settings_delete_action))
        remote.ok()
        remote.assertFocused(pages.s(R.string.app_profiles_manage_keep))
        remote.left()
        remote.ok()
        compose.waitUntil(5_000) { repo.calls.contains("delete:2") }
        assertEquals(listOf("Primary"), repo.profiles.map { it.name })
    }

    @Test
    fun anotherProfileReadsWhyAndMaySwitch() {
        repo.listAnswer = ApiResult.NotAllowed("primary_profile_required")
        open(CHILD)
        remote.assertFocused(pages.s(R.string.profiles_settings_onlyPrimary_action))
        remote.assertEveryControlReachable(pages.page)
        remote.ok()
        pages.waitFor(pages.s(R.string.profiles_picker_title))
    }

    private companion object {
        /** Down from the colours to «Speichern»: rating, PIN, the buttons. */
        const val MENU_ROWS = 4
    }
}
