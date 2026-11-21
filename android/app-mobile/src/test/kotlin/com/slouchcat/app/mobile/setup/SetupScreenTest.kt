package com.slouchcat.app.mobile.setup

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHeightIsAtLeast
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.unit.dp
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.MainActivity
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.session.ServerInfo
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private const val PHONE = "w412dp-h915dp"
private const val TABLET = "w1280dp-h800dp-land"

private fun s(id: Int, vararg args: Any): String =
    ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/**
 * The page «Serveradresse» (rule 8 for the phone app): every control reachable and labelled, in
 * both widths of the mockup (412 dp and 1280 dp).
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class SetupScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()

    private fun show(state: SetupUiState) {
        compose.setContent {
            SlouchcatTheme {
                SetupScreen(
                    state = state,
                    onAddressChange = { events += "address:$it" },
                    onConnect = { events += "connect" },
                    onConfirmInsecure = { events += "confirm" },
                    onDismissInsecure = { events += "dismiss" },
                    onChangeAddress = { events += "change" },
                    onNext = { events += "next" },
                )
            }
        }
    }

    private fun emptyPage() {
        show(SetupUiState())
        compose.onNodeWithText(s(R.string.app_setup_title)).assertIsDisplayed()
            .assert(SemanticsMatcher.keyIsDefined(SemanticsProperties.Heading))
        compose.onNodeWithText(s(R.string.app_setup_tagline)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithText(s(R.string.app_setup_addressHint)).assertIsDisplayed()
        compose.onNode(hasSetTextAction() and hasText(s(R.string.app_setup_addressLabel)))
            .assertHeightIsAtLeast(48.dp)
            .performTextInput("medien.example.test")
        compose.onNodeWithText(s(R.string.app_setup_connect))
            .performScrollTo()
            .assertHeightIsAtLeast(48.dp)
            .assertHasClickAction()
            .performClick()
        assertEquals(listOf("address:medien.example.test", "connect"), events)
    }

    @Test
    @Config(qualifiers = PHONE)
    fun emptyPagePhone() = emptyPage()

    @Test
    @Config(qualifiers = TABLET)
    fun emptyPageTablet() = emptyPage()

    private fun foundPage() {
        show(
            SetupUiState(
                address = "http://192.0.2.10:8096",
                found = ServerInfo("http://192.0.2.10:8096/".toHttpUrl(), "1.42.0", "1.42.0"),
            ),
        )
        compose.onNodeWithText(s(R.string.app_setup_found)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithText(s(R.string.app_setup_notEncrypted)).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_setup_next)).performScrollTo().assertHeightIsAtLeast(48.dp).performClick()
        compose.onNodeWithText(s(R.string.app_setup_otherAddress)).performScrollTo().assertHeightIsAtLeast(48.dp).performClick()
        assertEquals(listOf("next", "change"), events)
    }

    @Test
    @Config(qualifiers = PHONE, fontScale = 2f)
    fun emptyPagePhoneLargeFont() = emptyPage()

    @Test
    @Config(qualifiers = TABLET, fontScale = 2f)
    fun emptyPageTabletLargeFont() = emptyPage()

    @Test
    @Config(qualifiers = PHONE, fontScale = 2f)
    fun foundPhoneLargeFont() = foundPage()

    @Test
    @Config(qualifiers = TABLET, fontScale = 2f)
    fun foundTabletLargeFont() = foundPage()

    private fun busy() {
        show(SetupUiState(address = "https://media.example.test", busy = true))
        compose.onNodeWithText(s(R.string.app_setup_connecting)).assertIsDisplayed().performClick()
        compose.assertEveryControlLabelledAndTappable()
        assertEquals(emptyList<String>(), events)
    }

    @Test
    @Config(qualifiers = PHONE)
    fun busyPhone() = busy()

    @Test
    @Config(qualifiers = TABLET)
    fun busyTablet() = busy()

    @Test
    @Config(qualifiers = PHONE)
    fun foundPhone() = foundPage()

    @Test
    @Config(qualifiers = TABLET)
    fun foundTablet() = foundPage()

    private fun errors() {
        show(SetupUiState(address = "http://192.0.2.10", error = SetupError.TooOld("1.41.3", "1.42.0")))
        val text = s(R.string.app_setup_error_tooOld, "1.41.3", "1.42.0")
        compose.onNodeWithText(text).assertIsDisplayed()
        compose.onNode(hasSetTextAction() and hasText(s(R.string.app_setup_addressLabel)))
            .assert(SemanticsMatcher.expectValue(SemanticsProperties.Error, text))
    }

    @Test
    @Config(qualifiers = PHONE)
    fun errorPhone() = errors()

    @Test
    @Config(qualifiers = TABLET)
    fun errorTablet() = errors()

    private fun insecureWarning() {
        show(SetupUiState(address = "198.51.100.7", insecureAddress = "http://198.51.100.7"))
        compose.onNodeWithText(s(R.string.app_setup_insecure_title)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_setup_insecure_body)).assertIsDisplayed()
        compose.onNodeWithText("http://198.51.100.7", useUnmergedTree = true).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_setup_insecure_cancel)).assertHeightIsAtLeast(48.dp).performClick()
        compose.onNodeWithText(s(R.string.app_setup_insecure_confirm)).assertHeightIsAtLeast(48.dp).performClick()
        assertEquals(listOf("dismiss", "confirm"), events)
    }

    @Test
    @Config(qualifiers = PHONE)
    fun insecureWarningPhone() = insecureWarning()

    @Test
    @Config(qualifiers = TABLET)
    fun insecureWarningTablet() = insecureWarning()
}

/** Back on the first page leaves the app (rule 8: «Zurück führt hinaus»). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class SetupBackTest {
    @get:Rule
    val compose = createAndroidComposeRule<MainActivity>()

    private fun backLeaves() {
        // The first page waits for the stored server (DataStore, off the main thread).
        compose.waitUntil(5_000) {
            compose.onAllNodesWithText(s(R.string.app_setup_title)).fetchSemanticsNodes().isNotEmpty()
        }
        compose.onNodeWithText(s(R.string.app_setup_title)).assertExists()
        compose.activityRule.scenario.onActivity { (it as ComponentActivity).onBackPressedDispatcher.onBackPressed() }
        compose.activityRule.scenario.onActivity { assertTrue(it.isFinishing) }
    }

    @Test
    @Config(qualifiers = PHONE)
    fun backLeavesPhone() = backLeaves()

    @Test
    @Config(qualifiers = TABLET)
    fun backLeavesTablet() = backLeaves()
}
