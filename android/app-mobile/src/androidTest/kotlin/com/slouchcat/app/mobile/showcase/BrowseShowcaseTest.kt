package com.slouchcat.app.mobile.showcase

import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasScrollToNodeAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performScrollToNode
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.mobile.shell.features
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.strings.R
import org.junit.After
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Stage 4 with content (concept 27, visual check against the mockup): the real start page,
 * media choice, library in grid and list with every chip and sheet, and a list, over
 * [BrowseShowcaseData] and drawn pictures; no server. Runs in every emulator flow; photos with
 * `slouchcatShots` (`task android:e2e SHOTS=…`) as `c01…`.
 */
@RunWith(AndroidJUnit4::class)
class BrowseShowcaseTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private val showcase = Showcase(compose, features, BrowseShowcaseData.libraries, BrowseShowcaseData.images()) { graph ->
        BrowseShowcaseData.data(MediaData.api(graph.api, graph.http))
    }

    @After
    fun close() = showcase.close()

    private fun button(text: String) = compose.onAllNodes(hasText(text) and hasClickAction()).onLast()

    private fun startPage() {
        showcase.waitFor(hasText("Nachtzug nach Lumen") or hasContentDescription("Nachtzug nach Lumen"))
        showcase.waitFor(hasText(s(R.string.home_rows_continue)))
    }

    @Test
    fun browseWithContent() {
        showcase.start()
        startPage()
        showcase.shot("c01-start")
        val page = compose.onAllNodes(hasScrollToNodeAction()).onFirst()
        page.performScrollToNode(hasText(s(R.string.home_rows_released)))
        showcase.shot("c02-start-rows")
        page.performScrollToNode(hasText(s(R.string.home_rows_libraries)))
        showcase.shot("c03-start-more-rows")
        page.performScrollToNode(hasContentDescription(s(R.string.home_rows_spotlight)))

        // Media choice: the menu on the phone, the side column from 700 dp.
        val menu = compose.onAllNodes(hasContentDescription(s(R.string.app_shell_menuOpen)) and hasClickAction())
        if (menu.fetchSemanticsNodes().isNotEmpty()) {
            menu.onFirst().performClick()
            showcase.waitFor(hasText("Hörbücher"))
            showcase.shot("c04-media-choice")
        }
        button("Filme").performClick()
        showcase.waitFor(hasText("Filme") and isHeading())
        showcase.waitFor(hasText("Kalter Mond"))
        showcase.shot("c05-library-grid")

        button(s(R.string.library_filters_genre)).performClick()
        showcase.waitFor(hasText(s(R.string.library_filters_allGenres)))
        showcase.shot("c06-sheet-genre")
        button("Drama").performScrollTo().performClick()
        compose.waitUntil(10_000) { compose.onAllNodes(hasText(s(R.string.library_filters_allGenres))).fetchSemanticsNodes().isEmpty() }

        button(s(R.string.library_filters_year)).performClick()
        showcase.waitFor(hasText(s(R.string.app_browse_allYears)))
        showcase.shot("c07-sheet-year")
        button(s(R.string.common_close)).performScrollTo().performClick()

        button(s(R.string.library_filters_rating)).performClick()
        showcase.waitFor(hasText(s(R.string.library_filters_anyRating)))
        showcase.shot("c08-sheet-rating")
        button(s(R.string.common_close)).performScrollTo().performClick()

        compose.onAllNodes(hasContentDescription(s(R.string.app_browse_sortLabel, s(R.string.library_sort_added))) and hasClickAction()).onFirst().performClick()
        showcase.waitFor(hasText(s(R.string.app_browse_sortTitle)))
        showcase.shot("c09-sheet-sort")
        button(s(R.string.app_browse_sortTitleAz)).performClick()
        showcase.waitFor(hasContentDescription(s(R.string.app_browse_sortLabel, s(R.string.app_browse_sortTitleAz))))
        showcase.shot("c10-library-filtered")

        button(s(R.string.app_browse_all)).performClick()
        compose.onAllNodes(hasContentDescription(s(R.string.app_browse_showList)) and hasClickAction()).onFirst().performClick()
        showcase.waitFor(hasContentDescription(s(R.string.app_browse_showGrid)))
        showcase.shot("c11-library-list")
        showcase.back()
        startPage()

        // Square covers.
        val menuAgain = compose.onAllNodes(hasContentDescription(s(R.string.app_shell_menuOpen)) and hasClickAction())
        if (menuAgain.fetchSemanticsNodes().isNotEmpty()) menuAgain.onFirst().performClick()
        button("Hörbücher").performClick()
        showcase.waitFor(hasText("Der Kartograf"))
        showcase.shot("c12-library-books")
        showcase.back()
        startPage()

        // The list behind «User-Empfehlungen».
        page.performScrollToNode(hasContentDescription(s(R.string.app_browse_showAllLabel, s(R.string.home_picks))))
        compose.onAllNodes(hasContentDescription(s(R.string.app_browse_showAllLabel, s(R.string.home_picks))) and hasClickAction()).onFirst().performClick()
        showcase.waitFor(hasText(s(R.string.home_picks)) and isHeading())
        showcase.shot("c13-list-picks")
        showcase.back()
        // Back where it was: the start page scrolled to the row.
        showcase.waitFor(hasContentDescription(s(R.string.app_browse_showAllLabel, s(R.string.home_picks))))
    }
}
