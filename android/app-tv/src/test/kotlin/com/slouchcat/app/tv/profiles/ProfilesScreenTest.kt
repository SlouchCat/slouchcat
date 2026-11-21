package com.slouchcat.app.tv.profiles

import androidx.activity.ComponentActivity
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.tv.common.LoadError
import com.slouchcat.app.tv.common.Loadable
import com.slouchcat.app.tv.testing.Dpad
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.testing.summary
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/** «Wer schaut?» with the D-pad: tiles, the PIN pad with arrows and digit keys, K-9 at the foot. */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class ProfilesScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val remote = Dpad(compose)
    private val context = ApplicationProvider.getApplicationContext<android.content.Context>()
    private val profiles = listOf(summary(1, "Marco", primary = true, accent = Accent.EMBER), summary(2, "Lea", hasPin = true, accent = Accent.SKY), summary(3, "Kinder"))
    private val calls = mutableListOf<String>()

    private fun s(id: Int, vararg args: Any) = context.getString(id, *args)

    private fun show(state: () -> ProfilesUiState, manage: Boolean = false) = remote.show {
        ProfilesScreen(
            state = state(),
            onPick = { calls += "pick:${it.name}" },
            onDigit = { calls += "digit:$it" },
            onDelete = { calls += "delete" },
            onSubmitPin = { calls += "submit" },
            onCancelPin = { calls += "cancel" },
            onRetry = { calls += "retry" },
            onSignOut = { calls += "signout" },
            onManage = if (manage) ({ calls += "manage" }) else null,
        )
    }

    @Test
    fun theTilesTheConnectionAndThePickedProfile() {
        show({ ProfilesUiState(Loadable.Ready(profiles), current = 2, email = "owner@example.test", server = "https://media.example.test") })
        compose.onNodeWithText(s(R.string.tv_profiles_connected, "https://media.example.test", "owner@example.test")).assertExists()
        // The profile the TV ran as last has the focus; the lock is said.
        remote.assertFocused("Lea")
        compose.onNode(hasContentDescription("Lea, ${s(R.string.profiles_picker_locked)}")).assertExists()
        remote.assertEveryControlReachable()
        remote.left()
        remote.assertFocused("Marco")
        remote.ok()
        assertEquals("pick:Marco", calls.last())
        remote.assertFocusVisible(compose.onNode(hasContentDescription("Marco")), compose.onNode(hasContentDescription("Kinder")))
        // «Profile verwalten» only once that page exists (stage 2b).
        assertTrue(compose.onAllNodes(hasClickAction() and hasContentDescription(s(R.string.profiles_picker_manage))).fetchSemanticsNodes().isEmpty())
    }

    @Test
    fun theManagementAppearsOnceItsPageIsThere() {
        show({ ProfilesUiState(Loadable.Ready(profiles), email = "owner@example.test", server = "https://media.example.test") }, manage = true)
        remote.down()
        remote.assertFocused(s(R.string.profiles_picker_manage))
        remote.ok()
        assertEquals("manage", calls.last())
    }

    @Test
    fun thePinPadTakesArrowsAndDigitKeysAndCatchesTheFocus() {
        var state by mutableStateOf(ProfilesUiState(Loadable.Ready(profiles), email = "owner@example.test", server = "https://media.example.test", pin = PinState(profiles[1])))
        show({ state })
        compose.onNodeWithText(s(R.string.profiles_pin_title, "Lea")).assertExists()
        remote.assertFocused("1")
        remote.ok()
        remote.right()
        remote.ok()
        // A digit key of the remote goes in as well.
        remote.press(Key.Seven)
        remote.press(Key.Backspace)
        assertEquals(listOf("digit:1", "digit:2", "digit:7", "delete"), calls)
        val inPad = hasAnyAncestor(SemanticsMatcher.keyIsDefined(SemanticsProperties.PaneTitle))
        state = state.copy(pin = PinState(profiles[1], digits = "2468"))
        remote.settle()
        remote.assertEveryControlReachable(scope = inPad)
        // Caught: up from the first row stays in the pad.
        remote.up(3)
        assertTrue(remote.focusedLabel(), remote.focusedLabel().matches(Regex("[1-3]")))
        remote.down(3)
        remote.right(2)
        remote.assertFocused(s(R.string.profiles_pin_submit))
        remote.ok()
        assertEquals("submit", calls.last())
        remote.back()
        assertEquals("cancel", calls.last())
    }

    @Test
    fun aWrongPinAndTheLockAreSaid() {
        show({ ProfilesUiState(Loadable.Ready(profiles), pin = PinState(profiles[1], message = PinMessage.Invalid)) })
        compose.onNodeWithText(s(R.string.profiles_pin_invalid)).assertExists()
        remote.assertFocused("1")
    }

    @Test
    fun theLockedPadSaysHowLongItWaits() {
        show({ ProfilesUiState(Loadable.Ready(profiles), pin = PinState(profiles[1], message = PinMessage.Locked, lockedSeconds = 25)) })
        compose.onNodeWithText(s(R.string.app_profiles_pinLocked, 25L)).assertExists()
    }

    @Test
    fun withoutProfilesTheTvOffersRetryAndSigningOut() {
        show({ ProfilesUiState(Loadable.Failed(LoadError.Offline), email = "owner@example.test", server = "https://media.example.test") })
        remote.assertFocused(s(R.string.app_state_retry))
        remote.assertEveryControlReachable()
        remote.ok()
        remote.down()
        remote.assertFocused(s(R.string.tv_settings_signOut))
        remote.ok()
        assertEquals(listOf("retry", "signout"), calls)
    }
}
