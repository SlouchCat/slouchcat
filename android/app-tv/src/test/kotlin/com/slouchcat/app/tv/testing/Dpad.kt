package com.slouchcat.app.tv.testing

import android.graphics.Bitmap
import androidx.activity.ComponentActivity
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.semantics.SemanticsNode
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.isFocused
import androidx.compose.ui.test.junit4.AndroidComposeTestRule
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performKeyInput
import androidx.compose.ui.test.pressKey
import androidx.compose.ui.test.requestFocus
import androidx.test.ext.junit.rules.ActivityScenarioRule
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.tv.ui.TvScreenHost
import com.slouchcat.app.tv.ui.TvTheme
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue

/** The qualifiers of a TV in Robolectric: 960 x 540 dp, landscape, television, xhdpi (K26 stage 0, point 7). */
const val TV = "w960dp-h540dp-land-television-xhdpi"

typealias TvRule = AndroidComposeTestRule<ActivityScenarioRule<ComponentActivity>, ComponentActivity>

/**
 * The remote in a JVM test (K26 stage 0, point 7): keys through the focus tree, Back through the
 * activity's dispatcher (a key Back never reaches BackHandler there), no touch mode.
 *
 * ```
 * val remote = Dpad(compose)
 * remote.show { SetupScreen(…) }
 * remote.assertFocused("Mit dem Handy einrichten")
 * remote.right(); remote.ok()
 * remote.assertEveryControlReachable()
 * ```
 */
@OptIn(ExperimentalTestApi::class)
class Dpad(private val compose: TvRule) {
    /** Sets [content] in the TV theme with its overlay layer, without touch mode. */
    fun show(content: @Composable () -> Unit) {
        InstrumentationRegistry.getInstrumentation().setInTouchMode(false)
        compose.setContent { TvTheme { TvScreenHost { content() } } }
        settle()
    }

    fun settle() {
        compose.waitForIdle()
        compose.mainClock.advanceTimeBy(FRAME_MS)
        compose.waitForIdle()
    }

    fun press(key: Key, times: Int = 1) = repeat(times) {
        compose.onRoot().performKeyInput { pressKey(key) }
        settle()
    }

    fun up(times: Int = 1) = press(Key.DirectionUp, times)
    fun down(times: Int = 1) = press(Key.DirectionDown, times)
    fun left(times: Int = 1) = press(Key.DirectionLeft, times)
    fun right(times: Int = 1) = press(Key.DirectionRight, times)

    /** OK (the centre of the D-pad). */
    fun ok() = press(Key.DirectionCenter)

    fun back() {
        compose.runOnUiThread { compose.activity.onBackPressedDispatcher.onBackPressed() }
        settle()
    }

    /** What a person reads on the focused element: its texts and descriptions, merged. */
    fun focusedLabel(): String = compose.onAllNodes(isFocused()).fetchSemanticsNodes().singleOrNull()?.let(::label) ?: "nothing"

    fun assertFocused(text: String) {
        val label = focusedLabel()
        assertTrue("«$text» should have the focus, «$label» has it", label.contains(text))
    }

    /** Nothing on the screen has the focus (it left the screen, or never came). */
    fun assertNothingFocused() = assertEquals("nothing", focusedLabel())

    /**
     * Every control of the screen (all that can be clicked) is reached with the arrow keys from the
     * start focus, and none is a trap: from each one, some arrow leads elsewhere. [scope]: only the
     * controls within it (an open menu).
     */
    fun assertEveryControlReachable(scope: SemanticsMatcher? = null, max: Int = MAX_CONTROLS) {
        val controls = compose.onAllNodes(if (scope == null) hasClickAction() else hasClickAction() and scope).fetchSemanticsNodes()
            .filter { it.isEnabled() }
        assertTrue("no control on the screen", controls.isNotEmpty())
        assertTrue("more than $max controls: give a scope", controls.size <= max)
        val start = compose.onAllNodes(isFocused()).fetchSemanticsNodes().singleOrNull()
        assertTrue("no start focus", start != null)
        val seen = linkedSetOf(start!!.id)
        val queue = ArrayDeque(listOf(start.id))
        val directions = listOf(Key.DirectionUp, Key.DirectionDown, Key.DirectionLeft, Key.DirectionRight)
        while (queue.isNotEmpty()) {
            val id = queue.removeFirst()
            var leaves = false
            for (direction in directions) {
                node(id).requestFocus()
                settle()
                press(direction)
                val now = compose.onAllNodes(isFocused()).fetchSemanticsNodes().singleOrNull() ?: continue
                if (now.id != id) leaves = true
                if (seen.add(now.id)) queue += now.id
            }
            assertTrue("«${label(id)}» is a focus trap", leaves || controls.size == 1)
        }
        val missing = controls.filter { it.id !in seen }.map(::label)
        assertTrue("not reachable with the arrow keys: $missing", missing.isEmpty())
        // Back to the start focus for what the test does next.
        node(start.id).requestFocus()
        settle()
    }

    /** The focused element looks different from the same element without focus (the focus is visible). */
    fun assertFocusVisible(target: SemanticsNodeInteraction, other: SemanticsNodeInteraction) {
        other.requestFocus()
        settle()
        val rest = pixels(target)
        target.requestFocus()
        settle()
        assertNotEquals("the focus is not visible", rest.toList(), pixels(target).toList())
    }

    private fun pixels(node: SemanticsNodeInteraction): IntArray {
        val bitmap: Bitmap = node.captureToImage().asAndroidBitmap()
        return IntArray(bitmap.width * bitmap.height).also { bitmap.getPixels(it, 0, bitmap.width, 0, 0, bitmap.width, bitmap.height) }
    }

    private fun node(id: Int): SemanticsNodeInteraction = compose.onNode(SemanticsMatcher("id $id") { it.id == id })

    private fun label(id: Int): String = label(node(id).fetchSemanticsNode())

    private fun SemanticsNode.isEnabled() = config.getOrNull(SemanticsProperties.Disabled) == null

    private companion object {
        const val FRAME_MS = 400L
        const val MAX_CONTROLS = 40
    }
}

/** The texts and descriptions of a node, merged as a screen reader says them. */
fun label(node: SemanticsNode): String = buildList {
    node.config.getOrNull(SemanticsProperties.ContentDescription)?.let { addAll(it) }
    node.config.getOrNull(SemanticsProperties.Text)?.let { texts -> addAll(texts.map { it.text }) }
}.joinToString(" ")
