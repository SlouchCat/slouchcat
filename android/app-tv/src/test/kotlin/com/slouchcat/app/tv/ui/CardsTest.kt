package com.slouchcat.app.tv.ui

import androidx.activity.ComponentActivity
import androidx.compose.foundation.layout.Column
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import com.slouchcat.app.tv.testing.Dpad
import com.slouchcat.app.tv.testing.TV
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * The cards of stage 2b with the D-pad: a row comes back to its card, the info line names the
 * focused card, the focus of a card is visible, a grid comes back to its tile.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class CardsTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val remote = Dpad(compose)

    @Test
    fun aRowComesBackToItsCardAndNamesItBelow() {
        var opened = 0
        remote.show {
            val focus = rememberFocusMemory()
            Column {
                TvButton("Oben", onClick = {}, modifier = Modifier.focusKey(focus, "top"))
                TvCardRow(
                    title = "Neu",
                    items = (1..6).toList(),
                    key = { it },
                    focus = focus,
                    rowKey = "new",
                    info = { CardInfo("Film $it", "Jahr 200$it") },
                ) { item, modifier -> TvCard("Film $item", image = null, onClick = { opened = item }, modifier = modifier) }
            }
            focus.StartFocus("top")
        }
        remote.assertFocused("Oben")
        // The info line is empty while the row has no focus.
        compose.onAllNodesWithText("Jahr", substring = true).assertCountEquals(0)
        remote.down()
        remote.assertFocused("Film 1")
        remote.right(2)
        remote.assertFocused("Film 3")
        compose.onAllNodesWithText("Jahr 2003").assertCountEquals(1)
        // Out of the row and back: the card that had the focus, not the first one.
        remote.up()
        remote.assertFocused("Oben")
        remote.down()
        remote.assertFocused("Film 3")
        remote.ok()
        assertEquals(3, opened)
        remote.assertFocusVisible(compose.onNode(hasContentDescription("Film 3")), compose.onNode(hasContentDescription("Film 2")))
        remote.assertEveryControlReachable()
    }

    @Test
    fun aGridComesBackToItsTile() {
        remote.show {
            val focus = rememberFocusMemory()
            Column {
                TvButton("Filter", onClick = {}, modifier = Modifier.focusKey(focus, "filter"))
                TvCardGrid(items = (1..6).toList(), key = { it }, focus = focus, gridKey = "all", columns = 3) { item, modifier ->
                    TvCard("Kachel $item", image = null, onClick = {}, modifier = modifier, shape = CardShape.Wide)
                }
            }
            focus.StartFocus("filter")
        }
        remote.down()
        remote.assertFocused("Kachel 1")
        remote.right()
        remote.down()
        remote.assertFocused("Kachel 5")
        remote.up()
        remote.assertFocused("Kachel 2")
        remote.up()
        remote.assertFocused("Filter")
        remote.down()
        remote.assertFocused("Kachel 2")
        remote.assertEveryControlReachable()
    }
}
