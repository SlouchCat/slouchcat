package com.slouchcat.app.tv.ui

import androidx.activity.ComponentActivity
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isFocusable
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performImeAction
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTextReplacement
import com.slouchcat.app.tv.testing.Dpad
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.core.design.SlouchcatIcon
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * Every building block of the TV with the D-pad (rule 8 for the TV, K26 stage 0 point 7): reached
 * with the arrows, focus visible, OK does what it says, Back closes menus, focus caught in menus.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class ComponentsTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val remote = Dpad(compose)

    @Test
    fun buttonsAreReachedWithTheArrowsAndOkPressesThem() {
        var pressed by mutableStateOf("")
        remote.show {
            val first = remember { FocusRequester() }
            Row {
                TvButton("Eins", onClick = { pressed = "Eins" }, modifier = Modifier.focusRequester(first), kind = TvButtonKind.Primary)
                TvButton("Aus", onClick = { pressed = "Aus" }, enabled = false)
                TvButton("Zwei", onClick = { pressed = "Zwei" }, icon = SlouchcatIcon.Play.vector, kind = TvButtonKind.Ghost)
                TvButton("Drei", onClick = { pressed = "Drei" }, small = true)
            }
            androidx.compose.runtime.LaunchedEffect(Unit) { first.requestFocus() }
        }
        remote.assertFocused("Eins")
        remote.right()
        // The switched-off button is visible but skipped.
        remote.assertFocused("Zwei")
        remote.ok()
        assertEquals("Zwei", pressed)
        remote.right()
        remote.assertFocused("Drei")
        remote.assertFocusVisible(compose.onNodeWithText("Drei"), compose.onNodeWithText("Eins"))
        remote.assertEveryControlReachable()
        compose.onNodeWithText("Aus").assert(SemanticsMatcher.keyIsDefined(SemanticsProperties.Disabled))
    }

    @Test
    fun aFieldOpensTheInputWithOkTakesTheTextWithDoneAndForgetsItWithBack() {
        var value by mutableStateOf("")
        var done by mutableIntStateOf(0)
        remote.show {
            val first = remember { FocusRequester() }
            Column {
                TvTextField(value, onValueChange = { value = it }, label = "Serveradresse", placeholder = "z. B. https://…", onDone = {
                    done++
                    false
                }, focusRequester = first)
                TvButton("Weiter", onClick = {})
            }
            androidx.compose.runtime.LaunchedEffect(Unit) { first.requestFocus() }
        }
        remote.assertFocused("Serveradresse")
        // At rest the field shows its placeholder and has no input of its own.
        compose.onNodeWithText("z. B. https://…").assertExists()
        assertTrue(compose.onAllNodes(hasSetTextAction()).fetchSemanticsNodes().isEmpty())
        remote.ok()
        val editor = compose.onNode(hasSetTextAction() and hasContentDescription("Serveradresse"))
        editor.performTextInput("media.example.test")
        editor.performImeAction()
        remote.settle()
        assertEquals("media.example.test", value)
        assertEquals(1, done)
        remote.assertFocused("Serveradresse")
        compose.onNodeWithText("media.example.test").assertExists()

        // Back closes the input without a change.
        remote.ok()
        compose.onNode(hasSetTextAction()).performTextReplacement("other.example.test")
        remote.back()
        assertEquals("media.example.test", value)
        assertTrue(compose.onAllNodes(hasSetTextAction()).fetchSemanticsNodes().isEmpty())
        remote.assertFocused("Serveradresse")
        remote.down()
        remote.assertFocused("Weiter")
    }

    @Test
    fun aPasswordShowsDotsUntilItIsShown() {
        var shown by mutableStateOf(false)
        remote.show { TvTextField("geheim", onValueChange = {}, label = "Passwort", placeholder = "", password = true, showPassword = shown) }
        compose.onNodeWithText("•".repeat(6)).assertExists()
        assertTrue(compose.onAllNodes(hasText("geheim")).fetchSemanticsNodes().isEmpty())
        shown = true
        remote.settle()
        compose.onNodeWithText("geheim").assertExists()
    }

    @Test
    fun aSwitchTogglesWithOkAndSaysItsState() {
        var on by mutableStateOf(false)
        remote.show {
            val first = remember { FocusRequester() }
            TvSwitch("Trotzdem verbinden", checked = on, onCheckedChange = { on = it }, modifier = Modifier.focusRequester(first))
            androidx.compose.runtime.LaunchedEffect(Unit) { first.requestFocus() }
        }
        val node = compose.onNode(SemanticsMatcher.expectValue(SemanticsProperties.ToggleableState, ToggleableState.Off))
        node.assertExists()
        remote.ok()
        assertTrue(on)
        compose.onNode(SemanticsMatcher.expectValue(SemanticsProperties.ToggleableState, ToggleableState.On)).assertExists()
    }

    @Test
    fun notesAndBadgesAreReadNotFocused() {
        remote.show {
            Column {
                TvNote("Nicht verschlüsselt", tone = NoteTone.Warn)
                TvNote("Kurz", tone = NoteTone.Bad, small = true)
                TvBadge("Empfohlen", tone = BadgeTone.Accent)
                ConnectionBadge(encrypted = false, encryptedText = "verschlüsselt", plainText = "nicht verschlüsselt")
            }
        }
        for (text in listOf("Nicht verschlüsselt", "Kurz", "Empfohlen", "nicht verschlüsselt")) compose.onNodeWithText(text).assertExists()
        assertTrue(compose.onAllNodes(isFocusable()).fetchSemanticsNodes().isEmpty())
    }

    @Test
    fun aSideMenuCatchesTheFocusAndBackClosesIt() {
        var open by mutableStateOf(false)
        remote.show {
            val focus = rememberFocusMemory()
            Column {
                TvButton("Öffnen", onClick = { open = true }, modifier = Modifier.focusKey(focus, "open"))
                TvButton("Daneben", onClick = {}, modifier = Modifier.focusKey(focus, "other"))
            }
            focus.StartFocus("open")
            if (open) {
                val first = remember { FocusRequester() }
                TvSideMenu("Server und Konto", onDismiss = { open = false; focus.restore() }, initialFocus = first) {
                    TvMenuItem("Erste", onClick = {}, icon = SlouchcatIcon.Server.vector, subtitle = "Zeile", modifier = Modifier.focusRequester(first))
                    TvMenuItem("Zweite", onClick = {})
                    TvActions(vertical = true) { TvButton("Schliessen", onClick = { open = false; focus.restore() }) }
                }
            }
        }
        remote.assertFocused("Öffnen")
        remote.ok()
        remote.assertFocused("Erste")
        // Caught: no arrow leads out of the menu.
        for (key in listOf(Key.DirectionLeft, Key.DirectionUp)) {
            remote.press(key)
            assertTrue(remote.focusedLabel(), remote.focusedLabel().contains("Erste"))
        }
        val inMenu = hasAnyAncestor(SemanticsMatcher.keyIsDefined(SemanticsProperties.PaneTitle))
        remote.assertEveryControlReachable(scope = inMenu)
        remote.down(2)
        remote.assertFocused("Schliessen")
        remote.down()
        remote.assertFocused("Schliessen")
        remote.back()
        assertFalse(open)
        remote.assertFocused("Öffnen")
    }

    @Test
    fun aDialogCatchesTheFocusOnItsDefault() {
        var open by mutableStateOf(true)
        var confirmed by mutableStateOf(false)
        remote.show {
            TvButton("Hinter dem Dialog", onClick = {})
            if (open) {
                val cancel = remember { FocusRequester() }
                TvDialog("Diesen Fernseher abmelden?", onDismiss = { open = false }, initialFocus = cancel) {
                    TvActions {
                        TvButton("Abmelden", onClick = { confirmed = true })
                        TvButton("Abbrechen", onClick = { open = false }, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(cancel))
                    }
                }
            }
        }
        remote.assertFocused("Abbrechen")
        remote.right()
        remote.assertFocused("Abbrechen")
        remote.down()
        remote.assertFocused("Abbrechen")
        remote.left()
        remote.assertFocused("Abmelden")
        remote.ok()
        assertTrue(confirmed)
        remote.back()
        assertFalse(open)
    }

    @Test
    fun aMessageShowsAndGoes() {
        val toasts = TvToasts()
        remote.show { TvToastHost(toasts) }
        compose.mainClock.autoAdvance = false
        toasts.show("Verbindung getrennt")
        compose.mainClock.advanceTimeBy(FOCUS_MS)
        compose.onNodeWithText("Verbindung getrennt").assertExists()
        compose.mainClock.advanceTimeBy(TvMotion.TOAST_MS.toLong() + FOCUS_MS * 2)
        compose.waitForIdle()
        assertNull(toasts.current.value)
        compose.mainClock.autoAdvance = true
    }

    @Test
    fun theQrCodeCarriesItsTextAndTheBackdropDrawsWithoutAnImage() {
        val text = "https://pair.slouchcat.com/tv#v=1&i=AAAAAAAAAAAAAAAAAAAAAA&k=QCYdnDrDMuTaRSfMmHi3L0lWboCDrMRS8Qe0vcuBxnk"
        remote.show {
            Row {
                TvBackdrop(null, mode = BackdropMode.Dim)
                QrCode(text, "QR-Code zum Verbinden")
            }
        }
        compose.onNode(SemanticsMatcher.expectValue(QrContent, text)).assertExists()
        compose.onNode(hasContentDescription("QR-Code zum Verbinden")).assertExists()
        // Version and error correction are ZXing's choice; it has to be square and readable.
        val modules = qrModules(text)
        assertEquals(modules.width, modules.height)
        assertTrue(modules.width >= MIN_QR_MODULES)
    }

    @Test
    fun aMenuItemShowsItsFocus() {
        remote.show {
            Column {
                TvMenuItem("Profil wechseln", onClick = {}, icon = SlouchcatIcon.Users.vector, subtitle = "Aktiv: Lea")
                TvMenuItem("Über", onClick = {}, icon = SlouchcatIcon.Info.vector)
            }
        }
        remote.assertFocusVisible(compose.onNodeWithText("Profil wechseln"), compose.onNodeWithText("Über"))
    }

    private companion object {
        const val FOCUS_MS = 200L
        const val MIN_QR_MODULES = 21
    }
}
