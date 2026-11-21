package com.slouchcat.core.design

import androidx.compose.foundation.layout.Column
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHeightIsAtLeast
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsOff
import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.assertTouchHeightIsEqualTo
import androidx.compose.ui.test.assertWidthIsAtLeast
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.dp
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** The building blocks: tap targets of 48 dp (K27 3.2), labels for the screen reader, states. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36], qualifiers = "w412dp-h915dp")
class ComponentsTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun buttonsAreAtLeast48dp() {
        var clicks = 0
        compose.setContent {
            SlouchcatTheme {
                Column {
                    for (kind in ButtonKind.entries) SlouchcatButton("Button $kind", onClick = { clicks++ }, kind = kind)
                    SlouchcatButton("Small", onClick = {}, small = true)
                    SlouchcatIconButton(SlouchcatIcon.Back, contentDescription = "Back", onClick = { clicks++ })
                    SlouchcatChip("Chip", selected = false, onClick = {})
                }
            }
        }
        for (kind in ButtonKind.entries) {
            compose.onNodeWithText("Button $kind").assertHeightIsAtLeast(48.dp).assertHasClickAction()
        }
        compose.onNodeWithText("Small").assertTouchHeightIsEqualTo(48.dp)
        compose.onNodeWithContentDescription("Back").assertHeightIsAtLeast(48.dp).assertWidthIsAtLeast(48.dp).performClick()
        compose.onNodeWithText("Chip").assertTouchHeightIsEqualTo(48.dp)
        compose.onNodeWithText("Button Primary").performClick()
        assertEquals(2, clicks)
    }

    @Test
    fun aBusyButtonTakesNoSecondTap() {
        var clicks = 0
        compose.setContent { SlouchcatTheme { SlouchcatButton("Wait", onClick = { clicks++ }, busy = true) } }
        compose.onNodeWithText("Wait").performClick()
        assertEquals(0, clicks)
    }

    @Test
    fun theTextFieldIsNamedByItsLabelAndTellsItsError() {
        compose.setContent {
            SlouchcatTheme {
                var value by remember { mutableStateOf("") }
                Column {
                    SlouchcatTextField(value, { value = it }, label = "Address", hint = "The hint")
                    SlouchcatTextField("x", {}, label = "Broken", error = "Wrong")
                }
            }
        }
        compose.onNodeWithText("The hint").assertIsDisplayed()
        // The label belongs to the field's node: no description over the content.
        compose.onNode(hasSetTextAction() and hasText("Address")).assertHeightIsAtLeast(48.dp)
            .assert(SemanticsMatcher.keyNotDefined(SemanticsProperties.ContentDescription))
        compose.onNode(hasSetTextAction() and hasText("Broken")).assert(SemanticsMatcher.expectValue(SemanticsProperties.Error, "Wrong"))
        compose.onNodeWithText("Wrong").assertIsDisplayed()
    }

    @Test
    fun aSwitchRowIsOneSwitch() {
        compose.setContent {
            SlouchcatTheme {
                var on by remember { mutableStateOf(false) }
                SwitchRow("Wifi only", checked = on, onCheckedChange = { on = it }, subtitle = "Downloads")
            }
        }
        compose.onNodeWithText("Wifi only", useUnmergedTree = false).assertIsOff().assertHeightIsAtLeast(48.dp).performClick()
        compose.onNodeWithText("Wifi only").assertIsOn()
    }

    @Test
    fun listRowsNotesBadgesAndTheBar() {
        var back = false
        compose.setContent {
            SlouchcatTheme {
                Column {
                    TopBar("Title", navigationLabel = "Back", onNavigation = { back = true })
                    SectionTitle("Section")
                    ListRow("Row", icon = SlouchcatIcon.Server, subtitle = "Second line", onClick = {})
                    Note("A note", warning = true)
                    Badge("Found", tone = BadgeTone.Ok, icon = SlouchcatIcon.Check)
                    EmptyState("Nothing here", body = "Add something")
                    SlouchcatCard { Wordmark("Slouchcat") }
                }
            }
        }
        compose.onNodeWithContentDescription("Back").performClick()
        assertTrue(back)
        compose.onNodeWithText("SECTION").assertIsDisplayed()
        compose.onNodeWithText("Row").assertHeightIsAtLeast(22.dp)
        compose.onNodeWithText("A note").assertIsDisplayed()
        compose.onNodeWithText("Found").assertIsDisplayed()
        compose.onNodeWithText("Nothing here").assertIsDisplayed()
        compose.onNodeWithText("Slouchcat").assertIsDisplayed()
    }

    @Test
    fun everyIconOfTheMockupBuilds() {
        for (icon in SlouchcatIcon.entries) {
            assertEquals(icon.name, 24f, icon.vector.viewportWidth)
        }
    }
}
