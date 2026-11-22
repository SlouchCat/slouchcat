package com.slouchcat.app.mobile.account

import android.content.Context
import androidx.compose.material3.SnackbarHostState
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.EMAIL
import com.slouchcat.app.mobile.testing.HTTPS_SERVER
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
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

/** «Konto» with the sheets for e-mail and password (rule 8). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class AccountScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()

    private fun show(state: AccountUiState) = compose.setContent {
        SlouchcatTheme {
            AccountScreen(
                state = state,
                snackbar = SnackbarHostState(),
                onBack = { events += "back" },
                onEmailAsk = { events += "email" },
                onPasswordAsk = { events += "password" },
                onField = { key, _ -> events += "field:$key" },
                onSave = { events += "save" },
                onDismiss = { events += "dismiss" },
            )
        }
    }

    private fun page() {
        show(AccountUiState(email = EMAIL, server = HTTPS_SERVER))
        compose.onNodeWithText(s(R.string.app_account_adminHint)).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithText(EMAIL).performClick()
        compose.onNodeWithText(s(R.string.app_account_password)).performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("email", "password", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun pagePhone() = page()

    @Test @Config(qualifiers = TABLET)
    fun pageTablet() = page()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun pagePhoneLargeFont() = page()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun pageTabletLargeFont() = page()

    private fun password() {
        val form = AccountForm(errors = mapOf(AccountViewModel.REPEAT to AccountFieldError.Mismatch))
        show(AccountUiState(email = EMAIL, server = HTTPS_SERVER, sheet = AccountSheet.Password, form = form))
        compose.onNodeWithText(s(R.string.app_account_passwordWarning)).assertIsDisplayed()
        compose.onNode(hasSetTextAction() and hasText(s(R.string.app_account_repeatPassword)))
            .assert(SemanticsMatcher.expectValue(SemanticsProperties.Error, s(R.string.app_account_error_mismatch)))
        compose.onNode(hasSetTextAction() and hasText(s(R.string.app_account_currentPassword))).performTextInput("x")
        compose.onNodeWithText(s(R.string.app_account_save)).performScrollTo().performClick()
        compose.onNodeWithText(s(R.string.common_cancel)).performScrollTo().performClick()
        assertEquals(listOf("field:current", "save", "dismiss"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun passwordPhone() = password()

    @Test @Config(qualifiers = TABLET)
    fun passwordTablet() = password()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun passwordPhoneLargeFont() = password()

    private fun email() {
        val form = AccountForm(errors = mapOf(AccountViewModel.CURRENT to AccountFieldError.WrongPassword), error = AccountError.Unreachable)
        show(AccountUiState(email = EMAIL, server = HTTPS_SERVER, sheet = AccountSheet.Email, form = form))
        compose.onNodeWithText(s(R.string.app_account_error_wrongPassword)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_setup_error_unreachable)).performScrollTo().assertIsDisplayed()
        compose.onNode(hasSetTextAction() and hasText(s(R.string.app_account_newEmail))).performTextInput("x")
        assertEquals(listOf("field:newEmail"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun emailPhone() = email()

    @Test @Config(qualifiers = TABLET)
    fun emailTablet() = email()
}
