package com.slouchcat.app.tv.setup

import androidx.activity.ComponentActivity
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performImeAction
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.tv.testing.Dpad
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.ui.QrContent
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
import org.robolectric.annotation.GraphicsMode

/** The setup pages with the D-pad (rule 8 for the TV): start focus, every control reached, no trap, Back. */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class SetupScreensTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val remote = Dpad(compose)
    private val context = ApplicationProvider.getApplicationContext<android.content.Context>()

    private fun s(id: Int, vararg args: Any) = context.getString(id, *args)

    @Test
    fun theChoiceStartsOnThePhoneAndReachesTheWayByHand() {
        var chosen = ""
        remote.show { SetupScreen(onPhone = { chosen = "phone" }, onHand = { chosen = "hand" }) }
        remote.assertFocused(s(R.string.tv_setup_phoneTitle))
        remote.assertEveryControlReachable()
        remote.assertFocusVisible(compose.onNode(hasClickAction() and hasText(s(R.string.tv_setup_phoneTitle), substring = true)), compose.onNode(hasClickAction() and hasText(s(R.string.tv_setup_handTitle), substring = true)))
        remote.ok()
        assertEquals("phone", chosen)
        remote.right()
        remote.assertFocused(s(R.string.tv_setup_handTitle))
        remote.ok()
        assertEquals("hand", chosen)
    }

    private val code = "https://pair.slouchcat.com/tv#v=1&i=AAAAAAAAAAAAAAAAAAAAAA&k=QCYdnDrDMuTaRSfMmHi3L0lWboCDrMRS8Qe0vcuBxnk"

    @Test
    fun theCodeWithItsStepsAndTheWaysOut() {
        val pressed = mutableListOf<String>()
        remote.show {
            PairScreen(
                state = PairUiState(PairPhase.Waiting(code, 299), relay = "pair.slouchcat.com"),
                onNewCode = { pressed += "new" },
                onHand = { pressed += "hand" },
                onRelay = { pressed += "relay" },
                onContinue = {},
                onUndo = {},
                onInput = {},
            )
        }
        compose.onNode(SemanticsMatcher.expectValue(QrContent, code)).assertExists()
        compose.onNodeWithText(s(R.string.tv_pair_validFor, "4:59")).assertExists()
        compose.onNodeWithText(s(R.string.tv_pair_step1)).assertExists()
        remote.assertFocused(s(R.string.tv_pair_newCode))
        remote.assertEveryControlReachable()
        remote.ok()
        remote.right()
        remote.ok()
        remote.down()
        remote.ok()
        assertEquals(listOf("new", "hand", "relay"), pressed)
    }

    @Test
    fun linkedShowsServerAndAccountAndBackGoesOn() {
        val pressed = mutableListOf<String>()
        remote.show {
            PairScreen(
                state = PairUiState(PairPhase.Linked("media.example.test", true, "owner@example.test", "Shield · Android TV 11", 7)),
                onNewCode = {},
                onHand = {},
                onRelay = {},
                onContinue = { pressed += "continue" },
                onUndo = { pressed += "undo" },
                onInput = { pressed += "input" },
            )
        }
        for (text in listOf("media.example.test", "owner@example.test", "Shield · Android TV 11", s(R.string.tv_hand_encrypted))) {
            compose.onNodeWithText(text).assertExists()
        }
        remote.assertFocused(s(R.string.tv_pair_continueIn, 7))
        remote.assertEveryControlReachable()
        pressed.clear()
        remote.right()
        remote.assertFocused(s(R.string.tv_pair_undo))
        remote.ok()
        assertTrue("a key stops the countdown", "input" in pressed)
        assertTrue("undo" in pressed)
        remote.back()
        assertEquals("continue", pressed.last())
    }

    @Test
    fun aServerTheTvDoesNotReachShowsItsAddressAndTheWayByHand() {
        val pressed = mutableListOf<String>()
        remote.show {
            PairScreen(
                state = PairUiState(PairPhase.Problem(PairProblem.ServerUnreachable, address = "https://100.64.0.7:8096")),
                onNewCode = { pressed += "new" },
                onHand = { pressed += "hand" },
                onRelay = {},
                onContinue = {},
                onUndo = {},
                onInput = {},
            )
        }
        compose.onNodeWithText("https://100.64.0.7:8096").assertExists()
        compose.onNodeWithText(s(R.string.tv_pair_noServerTitle)).assertExists()
        remote.assertFocused(s(R.string.tv_pair_byHand))
        remote.assertEveryControlReachable()
        remote.ok()
        remote.back()
        assertEquals(listOf("hand", "new"), pressed)
    }

    @Test
    fun aRelayThatDoesNotAnswerOffersItsAddress() {
        remote.show {
            PairScreen(
                state = PairUiState(PairPhase.Problem(PairProblem.RelayUnreachable), relay = "pair.example.test"),
                onNewCode = {}, onHand = {}, onRelay = {}, onContinue = {}, onUndo = {}, onInput = {},
            )
        }
        compose.onNodeWithText(s(R.string.tv_pair_noRelayLead, "pair.example.test")).assertExists()
        remote.assertEveryControlReachable()
        compose.onNode(hasClickAction() and hasText(s(R.string.tv_pair_relay), substring = true)).assertExists()
    }

    @Test
    fun theWayByHandGoesThroughItsStepsWithTheInputOfTheSystem() {
        var state by mutableStateOf(HandUiState())
        val calls = mutableListOf<String>()
        remote.show {
            HandSetupScreen(
                state = state,
                onAddress = { calls += "address:$it" },
                onConsent = { calls += "consent:$it" },
                onNext = { calls += "next" },
                onEmail = { calls += "email:$it" },
                onPassword = {
                    calls += "password"
                    state = state.copy(password = it, error = null)
                },
                onTogglePassword = { calls += "show" },
                onSignIn = { calls += "signin" },
                onBack = { calls += "back" },
            )
        }
        // The address: the field has the focus, «Weiter» is off until a server answered.
        remote.assertFocused(s(R.string.tv_hand_serverLabel))
        remote.ok()
        val editor = compose.onNode(hasSetTextAction() and hasContentDescription(s(R.string.tv_hand_serverLabel)))
        editor.performTextInput("http://media.example.test")
        editor.performImeAction()
        remote.settle()
        assertEquals("address:http://media.example.test", calls.last())

        // Plain HTTP outside the home network: the warning and its switch get the focus.
        val info = ServerInfo("http://media.example.test/".toHttpUrl(), "1.43.0", "1.43.0", serverKey = "key")
        state = state.copy(address = "http://media.example.test", check = AddressCheck.Found(info, needsConsent = true))
        remote.settle()
        compose.onNodeWithText(s(R.string.app_setup_insecure_body)).assertExists()
        remote.assertFocused(s(R.string.tv_hand_consent))
        remote.ok()
        assertEquals("consent:true", calls.last())
        state = state.copy(httpConsent = true)
        remote.settle()
        remote.assertFocused(s(R.string.tv_hand_next))
        remote.assertEveryControlReachable()

        // Password: field, «anzeigen», «Anmelden»; Back is one step back.
        state = state.copy(step = HandStep.Password, email = "owner@example.test", password = "pw", error = HandError.InvalidCredentials)
        remote.settle()
        compose.onNodeWithText(s(R.string.tv_hand_wrong)).assertExists()
        remote.assertFocused(s(R.string.tv_hand_passwordLabel))
        remote.assertEveryControlReachable()
        // After a wrong password: the field again, «Fertig» goes on to «Anmelden».
        state = state.copy(password = "", error = HandError.InvalidCredentials)
        remote.settle()
        remote.assertFocused(s(R.string.tv_hand_passwordLabel))
        remote.ok()
        val passwordEditor = compose.onNode(hasSetTextAction() and hasContentDescription(s(R.string.tv_hand_passwordLabel)))
        passwordEditor.performTextInput("right-password")
        passwordEditor.performImeAction()
        remote.settle()
        assertEquals("password", calls.last())
        remote.assertFocused(s(R.string.tv_hand_signIn))
        remote.back()
        assertEquals("back", calls.last())
    }
}
