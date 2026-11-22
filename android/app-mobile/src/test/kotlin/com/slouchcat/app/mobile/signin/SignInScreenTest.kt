package com.slouchcat.app.mobile.signin

import android.content.Context
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.HTTP_SERVER
import com.slouchcat.app.mobile.testing.HTTPS_SERVER
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.core.design.SlouchcatTheme
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

/** «Anmelden» (rule 8 for the phone app): every control reachable and labelled, two widths, 200 % font. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class SignInScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()

    private fun show(state: SignInUiState) = compose.setContent {
        SlouchcatTheme {
            SignInScreen(
                state = state,
                onEmailChange = { events += "email:$it" },
                onPasswordChange = { events += "password" },
                onTogglePassword = { events += "toggle" },
                onSubmit = { events += "submit" },
                onChangeServer = { events += "server" },
                onCheckKey = { events += "check" },
                onOwnNetwork = { events += "own:$it" },
                onConfirmKey = { events += "confirm" },
                onDismissKey = { events += "dismiss" },
            )
        }
    }

    private fun form() {
        show(SignInUiState(server = HTTPS_SERVER, deviceName = "Pixel 9 · Android 16"))
        compose.onNode(hasText(s(R.string.login_title)) and isHeading()).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_signIn_deviceHint, "Pixel 9 · Android 16")).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNode(hasSetTextAction() and hasText(s(R.string.login_email))).performTextInput("viewer@example.test")
        compose.onNode(hasSetTextAction() and hasText(s(R.string.login_password))).performTextInput("secret")
        compose.onNodeWithContentDescription(s(R.string.app_signIn_showPassword)).performClick()
        compose.onNode(hasText(s(R.string.login_submit)) and hasClickAction()).performScrollTo().performClick()
        compose.onNodeWithContentDescription(s(R.string.app_signIn_serverLabel, "media.example.test")).performScrollTo().performClick()
        // The stateless field reports an empty value once it loses the focus; only the order counts.
        assertEquals(listOf("email:viewer@example.test", "password", "toggle", "submit", "server"), events.filterNot { it == "email:" })
    }

    @Test @Config(qualifiers = PHONE)
    fun formPhone() = form()

    @Test @Config(qualifiers = TABLET)
    fun formTablet() = form()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun formPhoneLargeFont() = form()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun formTabletLargeFont() = form()

    private fun errors() {
        show(
            SignInUiState(
                server = HTTP_SERVER,
                email = "viewer@example",
                emailError = FieldError.EmailInvalid,
                passwordError = FieldError.PasswordEmpty,
                error = SignInError.TooManyAttempts(42),
            ),
        )
        compose.onNode(hasSetTextAction() and hasText(s(R.string.login_email)))
            .assert(SemanticsMatcher.expectValue(SemanticsProperties.Error, s(R.string.app_signIn_error_emailInvalid)))
        compose.onNodeWithText(s(R.string.app_signIn_error_passwordEmpty)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_signIn_error_tooMany, "42")).performScrollTo().assertIsDisplayed()
        // Not encrypted: said in the server row.
        compose.onNodeWithContentDescription(s(R.string.app_signIn_serverLabel, "192.0.2.10")).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
    }

    @Test @Config(qualifiers = PHONE)
    fun errorsPhone() = errors()

    @Test @Config(qualifiers = TABLET)
    fun errorsTablet() = errors()

    private fun tooOld() {
        show(SignInUiState(server = HTTP_SERVER, error = SignInError.ServerTooOld("1.41.3", "1.42.0")))
        compose.onNodeWithText(s(R.string.app_setup_error_tooOld, "1.41.3", "1.42.0")).performScrollTo().assertIsDisplayed()
        compose.onNode(hasText(s(R.string.login_submit)) and hasClickAction()).performScrollTo().assertIsNotEnabled()
    }

    @Test @Config(qualifiers = PHONE)
    fun serverTooOldPhone() = tooOld()

    @Test @Config(qualifiers = TABLET)
    fun serverTooOldTablet() = tooOld()

    private fun confirmKey() {
        val key = com.slouchcat.core.api.crypto.Base64Url.encode(ByteArray(32))
        show(SignInUiState(server = HTTP_SERVER, confirmKey = true, newKey = key, busy = true))
        compose.onNodeWithText(s(R.string.app_serverKey_confirming, "6668 7aad f862 bd77 6c8f c18b 8e9f 8e20")).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.login_submitting)).performScrollTo().performClick()
        assertTrue("busy takes no second tap", events.isEmpty())
    }

    @Test @Config(qualifiers = PHONE)
    fun confirmKeyPhone() = confirmKey()

    @Test @Config(qualifiers = TABLET)
    fun confirmKeyTablet() = confirmKey()
}
