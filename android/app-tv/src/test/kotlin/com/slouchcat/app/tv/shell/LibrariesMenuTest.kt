package com.slouchcat.app.tv.shell

import androidx.activity.ComponentActivity
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import com.slouchcat.app.tv.testing.FakeHome
import com.slouchcat.app.tv.testing.FakeLibrary
import com.slouchcat.app.tv.testing.FakeMyLists
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.testing.library
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * The rail with many libraries and the menu «Bibliotheken» (code review M-3): the rail holds as
 * many as fit, «Alle Bibliotheken …» is reached and visible, the menu lists every library and
 * «Meine Listen», a list opens from there and marks «Alle Bibliotheken …».
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class LibrariesMenuTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote
    private val many = (1L..10L).map { library(it, "Bibliothek $it") }

    private fun show() = pages.show({ it.copy(home = FakeHome(), library = FakeLibrary(), myLists = FakeMyLists()) }, libraries = many)

    @Test
    fun theRailHoldsWhatFitsAndTheLastEntryIsSeen() {
        show()
        val shown = compose.onAllNodes(hasTestTag(ShellTags.RAIL).let { hasAnyAncestor(it) } and SemanticsMatcher("library") { node ->
            node.config.getOrNull(SemanticsProperties.TestTag)?.startsWith(ShellTags.entry("library-")) == true
        }).fetchSemanticsNodes()
        assertEquals(railLibraryCapacity(androidx.compose.ui.unit.Dp(540f)), shown.size)
        // Down to the last entry of «Medien» and on to the settings: each one is on the screen.
        pages.openRail(pages.s(R.string.tv_shell_allLibraries))
        compose.onNode(hasTestTag(ShellTags.entry(LIBRARIES_MENU))).assertIsDisplayed()
        remote.assertFocused("Bibliothek 1")
    }

    @Test
    fun theMenuListsEveryLibraryAndMyLists() {
        show()
        pages.openRail(pages.s(R.string.tv_shell_allLibraries))
        val menu = hasAnyAncestor(SemanticsMatcher.keyIsDefined(SemanticsProperties.PaneTitle))
        pages.waitFor(pages.s(R.string.tv_shell_myLists))
        // Every library, the tenth too (not in the rail), and the lists.
        pages.waitFor("Bibliothek 10")
        remote.assertEveryControlReachable(menu)
        remote.back()
        pages.openList(pages.s(R.string.watched_page_title))
        pages.waitFor(pages.s(R.string.watched_page_empty_title))
        // A list marks «Alle Bibliotheken …» in the rail.
        val marked = compose.onAllNodes(hasTestTag(ShellTags.entry(LIBRARIES_MENU)) and SemanticsMatcher.expectValue(SemanticsProperties.Selected, true)).fetchSemanticsNodes()
        assertTrue(marked.isNotEmpty())
        // Back closes nothing more: the menu is gone, Back goes to Start.
        remote.back()
        pages.waitFor(pages.s(R.string.home_empty_title))
    }

    @Test
    fun aMenuClosedWithBackGivesTheFocusBack() {
        show()
        pages.openRail(pages.s(R.string.tv_shell_allLibraries))
        pages.waitFor(pages.s(R.string.tv_shell_myLists))
        remote.assertFocused("Bibliothek 1")
        remote.back()
        assertTrue(compose.onAllNodes(SemanticsMatcher.keyIsDefined(SemanticsProperties.PaneTitle)).fetchSemanticsNodes().isEmpty())
        assertTrue(remote.focusedLabel() != "nothing")
    }
}
