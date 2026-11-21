package com.slouchcat.app.mobile.profiles

import android.content.Context
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.testing.EMAIL
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.app.mobile.testing.summary
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private const val PHONE = "w412dp-h915dp"
private const val TABLET = "w1280dp-h800dp-land"

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/** «Wer schaut?» and the PIN pad (rule 8): two widths, 200 % font, every control labelled. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class ProfilesScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val primary = summary(1, "Primary", primary = true, accent = Accent.EMBER)
    private val child = summary(2, "Child", hasPin = true, accent = Accent.SKY)

    private fun show(state: ProfilesUiState) = compose.setContent {
        SlouchcatTheme {
            ProfilesScreen(
                state = state,
                onPick = { events += "pick:${it.id}" },
                onDigit = { events += "digit:$it" },
                onDelete = { events += "delete" },
                onSubmitPin = { events += "submit" },
                onCancelPin = { events += "cancel" },
                onRetry = { events += "retry" },
                onSignOut = { events += "signout" },
            )
        }
    }

    private fun picker() {
        show(ProfilesUiState(Loadable.Ready(listOf(primary, child)), current = 1, email = EMAIL, host = "192.0.2.10"))
        compose.onNodeWithText(s(R.string.profiles_picker_title)).assert(SemanticsMatcher.keyIsDefined(SemanticsProperties.Heading))
        compose.assertEveryControlLabelledAndTappable()
        val primaryLabel = listOf("Primary", s(R.string.profiles_picker_primary), s(R.string.profiles_picker_active)).joinToString(", ")
        compose.onNodeWithContentDescription(primaryLabel).performClick()
        compose.onNodeWithContentDescription(listOf("Child", s(R.string.profiles_picker_locked)).joinToString(", ")).performClick()
        compose.onNodeWithText(s(R.string.app_profiles_account, EMAIL, "192.0.2.10")).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(s(R.string.nav_logout)).performScrollTo().performClick()
        assertEquals(listOf("pick:1", "pick:2", "signout"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun pickerPhone() = picker()

    @Test @Config(qualifiers = TABLET)
    fun pickerTablet() = picker()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun pickerPhoneLargeFont() = picker()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun pickerTabletLargeFont() = picker()

    private fun pin() {
        show(ProfilesUiState(Loadable.Ready(listOf(primary, child)), pin = PinState(child, digits = "1234", message = PinMessage.Invalid)))
        compose.onNodeWithText(s(R.string.profiles_pin_title, "Child")).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.profiles_pin_invalid)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithText("7").performClick()
        compose.onNodeWithContentDescription(s(R.string.app_profiles_pinDelete)).performClick()
        compose.onNodeWithContentDescription(s(R.string.profiles_pin_submit)).performClick()
        compose.onNodeWithText(s(R.string.common_cancel)).performScrollTo().performClick()
        assertEquals(listOf("digit:7", "delete", "submit", "cancel"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun pinPhone() = pin()

    @Test @Config(qualifiers = TABLET)
    fun pinTablet() = pin()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun pinPhoneLargeFont() = pin()

    private fun locked() {
        show(ProfilesUiState(Loadable.Ready(listOf(child)), pin = PinState(child, message = PinMessage.Locked, lockedSeconds = 25)))
        compose.onNodeWithText(s(R.string.app_profiles_pinLocked, "25")).assertIsDisplayed()
        compose.onNodeWithText("5").assertIsNotEnabled()
        compose.onNodeWithContentDescription(s(R.string.profiles_pin_submit)).assertIsNotEnabled()
    }

    @Test @Config(qualifiers = PHONE)
    fun lockedPhone() = locked()

    @Test @Config(qualifiers = TABLET)
    fun lockedTablet() = locked()

    private fun failed() {
        show(ProfilesUiState(Loadable.Failed(LoadError.Offline)))
        compose.onNodeWithText(s(R.string.app_state_offline)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_state_retry)).performClick()
        assertEquals(listOf("retry"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun failedPhone() = failed()

    @Test @Config(qualifiers = TABLET)
    fun failedTablet() = failed()
}
