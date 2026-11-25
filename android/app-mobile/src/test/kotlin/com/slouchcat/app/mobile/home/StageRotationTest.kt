package com.slouchcat.app.mobile.home

import android.content.Context
import android.view.accessibility.AccessibilityManager
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveableStateHolder
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performSemanticsAction
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.core.browse.home.HomeSection
import com.slouchcat.core.browse.home.HomeUiState
import com.slouchcat.core.browse.testing.SERVER_IMAGES
import com.slouchcat.core.browse.testing.itemRef
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.strings.R
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import kotlin.math.abs

/**
 * The stage moves on by itself, but holds still for a screen reader and while it has the focus
 * (WCAG 2.2.2, code review m-5 of part a).
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36], qualifiers = "w412dp-h915dp")
class StageRotationTest {
    @get:Rule
    val compose = createComposeRule()

    private val context = ApplicationProvider.getApplicationContext<Context>()

    private fun card(n: Int, title: String) = MediaItem.from(itemRef(n, title = title, runtimeMinutes = 100), SERVER_IMAGES)

    private val stage = HomeSection.Spotlight("spotlight", listOf(card(1, "Stage One"), card(2, "Stage Two")), start = 0, intervalSeconds = 7)

    @Composable
    private fun Home() = SlouchcatTheme {
        HomeScreen(HomeUiState(Loadable.Ready(listOf(stage))), HomeLinks(), onRetry = {}, onRefresh = {}, onRowEnd = {}, onWatchlist = {}, rotate = true)
    }

    private fun show() = compose.setContent { Home() }

    /** [title] on the screen, not on the page of the stage beside it. */
    private fun shown(title: String): Boolean {
        val width = compose.onRoot().fetchSemanticsNode().size.width
        return compose.onAllNodes(hasText(title)).fetchSemanticsNodes().any { node ->
            val bounds = node.boundsInRoot
            bounds.width > 0f && bounds.left >= 0f && bounds.right <= width
        }
    }

    /** The stage rests on [title]: its text sits in the middle of the screen, not between two titles. */
    private fun restsOn(title: String): Boolean {
        val width = compose.onRoot().fetchSemanticsNode().size.width
        return compose.onAllNodes(hasText(title)).fetchSemanticsNodes().any { node ->
            val bounds = node.boundsInRoot
            bounds.width > 0f && abs((bounds.left + bounds.right) / 2f - width / 2f) <= 1f
        }
    }

    /** Past the interval and the animation of the change. */
    private fun waitAnInterval() {
        compose.mainClock.advanceTimeBy(8_000)
        compose.waitForIdle()
        compose.mainClock.advanceTimeBy(2_000)
        compose.waitForIdle()
    }

    @Test
    fun theStageMovesOnByItself() {
        compose.mainClock.autoAdvance = false
        show()
        compose.waitForIdle()
        assertTrue(shown("Stage One"))
        waitAnInterval()
        assertTrue(shown("Stage Two"))
    }

    /** A change ends on the next title; it used to stop halfway, with half of each picture. */
    @Test
    fun aChangeEndsOnTheNextTitle() {
        compose.mainClock.autoAdvance = false
        show()
        compose.waitForIdle()
        assertTrue(restsOn("Stage One"))
        waitAnInterval()
        assertTrue(restsOn("Stage Two"))
    }

    /** A hold that begins during a change lets the change end; only the next one waits. */
    @Test
    fun aHoldDuringAChangeLetsItEnd() {
        compose.mainClock.autoAdvance = false
        show()
        compose.waitForIdle()
        compose.mainClock.advanceTimeBy(7_050)
        compose.waitForIdle()
        assertFalse(restsOn("Stage One"))
        assertFalse(restsOn("Stage Two"))
        shadowOf(context.getSystemService(AccessibilityManager::class.java)).setTouchExplorationEnabled(true)
        compose.waitForIdle()
        compose.mainClock.advanceTimeBy(2_000)
        compose.waitForIdle()
        assertTrue(restsOn("Stage Two"))
        waitAnInterval()
        assertTrue(restsOn("Stage Two"))
    }

    /** A change cut short, because the stage left the screen in the middle of it, comes back on one title. */
    @Test
    fun aChangeCutShortComesBackOnOneTitle() {
        compose.mainClock.autoAdvance = false
        var onScreen by mutableStateOf(true)
        compose.setContent {
            // As a list does with a row that leaves the screen: its state is kept, the row is not.
            val kept = rememberSaveableStateHolder()
            if (onScreen) kept.SaveableStateProvider("stage") { Home() }
        }
        compose.waitForIdle()
        compose.mainClock.advanceTimeBy(7_050)
        compose.waitForIdle()
        assertFalse(restsOn("Stage One"))
        assertFalse(restsOn("Stage Two"))
        onScreen = false
        compose.waitForIdle()
        compose.mainClock.advanceTimeByFrame()
        compose.waitForIdle()
        assertTrue(compose.onAllNodes(hasText("Stage One") or hasText("Stage Two")).fetchSemanticsNodes().isEmpty())
        onScreen = true
        compose.waitForIdle()
        compose.mainClock.advanceTimeBy(500)
        compose.waitForIdle()
        assertTrue(restsOn("Stage One") || restsOn("Stage Two"))
    }

    @Test
    fun aScreenReaderHoldsTheStage() {
        shadowOf(context.getSystemService(AccessibilityManager::class.java)).setTouchExplorationEnabled(true)
        compose.mainClock.autoAdvance = false
        show()
        compose.waitForIdle()
        waitAnInterval()
        assertTrue(shown("Stage One"))
        assertFalse(shown("Stage Two"))
    }

    @Test
    fun theFocusOnTheStageHoldsIt() {
        compose.mainClock.autoAdvance = false
        show()
        compose.waitForIdle()
        compose.onAllNodes(hasText(context.getString(R.string.item_watchlist_add)) and hasClickAction()).onFirst()
            .performSemanticsAction(SemanticsActions.RequestFocus)
        compose.waitForIdle()
        waitAnInterval()
        assertTrue(shown("Stage One"))
        assertFalse(shown("Stage Two"))
    }
}
