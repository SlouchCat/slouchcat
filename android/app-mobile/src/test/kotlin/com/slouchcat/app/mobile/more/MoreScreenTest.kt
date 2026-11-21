package com.slouchcat.app.mobile.more

import android.content.Context
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsOff
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.common.SignOutQuestion
import com.slouchcat.app.mobile.common.SignOutStep
import com.slouchcat.app.mobile.shell.MoreSection
import com.slouchcat.app.mobile.testing.EMAIL
import com.slouchcat.app.mobile.testing.HTTPS_SERVER
import com.slouchcat.app.mobile.testing.HTTP_SERVER
import com.slouchcat.app.mobile.testing.PRIMARY
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.core.design.SlouchcatIcon
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

/** «Mehr» with its sheets (rule 8): two widths, 200 % font, every control labelled. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class MoreScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()

    private val rows = listOf(
        MoreRow("devices", MoreSection.Devices, SlouchcatIcon.Phone, "Devices row", "2 signed in") { events += "devices" },
        MoreRow("account", MoreSection.Settings, SlouchcatIcon.User, "Account row", EMAIL) { events += "account" },
    )

    private fun show(state: MoreUiState) = compose.setContent {
        SlouchcatTheme {
            MoreScreen(
                state = state,
                rows = rows,
                appVersion = "0.1.0",
                onSwitchProfile = { events += "switch" },
                onServer = { events += "server" },
                onSignOutAsk = { events += "ask" },
                onChoice = { i, v -> events += "choice:$i:$v" },
                onSignOut = { events += "signout" },
                onDismissSheet = { events += "dismiss" },
            )
        }
    }

    private fun page(server: com.slouchcat.core.session.SavedServer) {
        show(MoreUiState(server = server, email = EMAIL, profile = PRIMARY))
        compose.onNodeWithText(s(R.string.app_more_title)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithText(s(R.string.app_more_switchProfile)).performClick()
        compose.onNodeWithText("Devices row").performScrollTo().performClick()
        compose.onNodeWithText("Account row").performScrollTo().performClick()
        compose.onNodeWithText(server.address.host).performScrollTo().performClick()
        compose.onNodeWithText(s(R.string.nav_logout)).performScrollTo().performClick()
        compose.onNodeWithText(s(R.string.app_more_footer, "0.1.0")).performScrollTo().assertIsDisplayed()
        assertEquals(listOf("switch", "devices", "account", "server", "ask"), events)
    }

    private fun insecure() {
        page(HTTP_SERVER)
        // The warning stays as long as the connection is unencrypted (concept 27 section 6).
        compose.onNodeWithText(s(R.string.app_more_insecureTitle), substring = true).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_more_plain)).performScrollTo().assertIsDisplayed()
    }

    @Test @Config(qualifiers = PHONE)
    fun insecurePhone() = insecure()

    @Test @Config(qualifiers = TABLET)
    fun insecureTablet() = insecure()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun insecurePhoneLargeFont() = insecure()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun insecureTabletLargeFont() = insecure()

    private fun encrypted() {
        page(HTTPS_SERVER)
        compose.onNodeWithText(s(R.string.app_more_insecureTitle), substring = true).assertDoesNotExist()
        compose.onNodeWithText(s(R.string.app_setup_encrypted)).performScrollTo().assertIsDisplayed()
    }

    @Test @Config(qualifiers = PHONE)
    fun encryptedPhone() = encrypted()

    @Test @Config(qualifiers = TABLET)
    fun encryptedTablet() = encrypted()

    private fun serverSheet() {
        show(MoreUiState(server = HTTP_SERVER, email = EMAIL, profile = PRIMARY, sheet = MoreSheet.Server))
        compose.onNodeWithText(s(R.string.app_more_serverSheet_notEncrypted)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_more_serverSheet_hint)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_more_serverSheet_change)).performScrollTo().performClick()
        compose.onNodeWithText(s(R.string.common_close)).performScrollTo().performClick()
        assertEquals(listOf("ask", "dismiss"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun serverSheetPhone() = serverSheet()

    @Test @Config(qualifiers = TABLET)
    fun serverSheetTablet() = serverSheet()

    private val downloads = object : SignOutStep {
        override suspend fun beforeSignOut(answer: Boolean) = Unit
    }

    private fun signOutSheet() {
        val choice = SignOutChoice(downloads, SignOutQuestion(R.string.nav_downloads), answer = false)
        show(MoreUiState(server = HTTP_SERVER, email = EMAIL, profile = PRIMARY, sheet = MoreSheet.SignOut, choices = listOf(choice)))
        compose.onNodeWithText(s(R.string.app_signOut_title)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_signOut_hint)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.nav_downloads)).assertIsOff().performClick()
        compose.onNodeWithText(s(R.string.common_cancel)).performScrollTo().performClick()
        assertEquals(listOf("choice:0:true", "dismiss"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun signOutSheetPhone() = signOutSheet()

    @Test @Config(qualifiers = TABLET)
    fun signOutSheetTablet() = signOutSheet()
}
