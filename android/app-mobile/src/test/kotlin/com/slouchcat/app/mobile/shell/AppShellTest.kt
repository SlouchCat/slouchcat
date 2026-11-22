package com.slouchcat.app.mobile.shell

import android.content.Context
import androidx.compose.material3.Text
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.core.browse.testing.PRIMARY
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private const val PHONE = "w412dp-h915dp"
private const val TABLET = "w1280dp-h800dp-land"

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/** The frame of the app: bar, side column and media menu as the registry and the back stack say. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class AppShellTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val start = TopDestination("start", 10, "start", R.string.nav_home, SlouchcatIcon.Home)
    private val search = TopDestination("search", 20, "search", R.string.nav_search, SlouchcatIcon.Search)
    private val more = TopDestination("more", 50, "more", R.string.app_more_title, SlouchcatIcon.Dots)
    private val films = MediaEntry("films", MediaSection.Media, 10, Label.Plain("Films"), SlouchcatIcon.Film, "films", count = 1846)
    private val watchlist = MediaEntry("watchlist", MediaSection.MyMedia, 10, Label.Res(R.string.nav_watchlist), SlouchcatIcon.Bookmark, "watchlist", count = 4)

    private fun show(frame: ShellFrame) = compose.setContent {
        SlouchcatTheme {
            AppShell(
                frame = frame,
                profile = PRIMARY,
                onTarget = { events += "target:${it.key}" },
                onMedia = { events += "media:${it.key}" },
                onSwitchProfile = { events += "profiles" },
            ) {
                ShellTopBar("Page title")
            }
        }
    }

    private fun frame(targets: List<TopDestination>, media: List<MediaEntry>, selected: String? = targets.firstOrNull()?.key, selectedMedia: String? = null) =
        ShellFrame(fullScreen = false, targets = targets, selectedTarget = selected, media = media, selectedMedia = selectedMedia)

    private fun oneTarget() {
        show(frame(listOf(more), emptyList()))
        compose.onNodeWithText("Page title").assertIsDisplayed()
        // No bar with a single target and no menu without media entries: no control without effect.
        compose.onNodeWithContentDescription(s(R.string.app_more_title)).assertDoesNotExist()
        compose.onNodeWithContentDescription(s(R.string.app_shell_menuOpen)).assertDoesNotExist()
    }

    @Test @Config(qualifiers = PHONE)
    fun oneTargetPhone() = oneTarget()

    @Test @Config(qualifiers = TABLET)
    fun oneTargetTablet() = oneTarget()

    private fun everythingOnAPhone() {
        show(frame(listOf(start, search, more), listOf(films, watchlist), selected = "more"))
        compose.assertEveryControlLabelledAndTappable()
        // A tab of the bar is read as one label (with its badge, if any).
        compose.onNodeWithContentDescription(s(R.string.app_more_title)).assert(SemanticsMatcher.expectValue(SemanticsProperties.Selected, true))
        compose.onNodeWithContentDescription(s(R.string.nav_search)).performClick()
        // The left edge belongs to the system: the menu opens by its button.
        compose.onNodeWithContentDescription(s(R.string.app_shell_menuOpen)).performClick()
        compose.waitForIdle()
        compose.onNodeWithText("Films").assertIsDisplayed()
        compose.onNodeWithText(s(R.string.nav_libraries).uppercase(), substring = true, ignoreCase = true).assertIsDisplayed()
        compose.onNodeWithText("Films").performClick()
        compose.onNodeWithContentDescription(s(R.string.app_shell_menuOpen)).performClick()
        compose.waitForIdle()
        compose.onNodeWithText(s(R.string.app_more_switchProfile)).performClick()
        assertEquals(listOf("target:search", "media:films", "profiles"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun phoneWithEverything() = everythingOnAPhone()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun phoneWithEverythingLargeFont() = everythingOnAPhone()

    private fun sideColumn() {
        show(frame(listOf(start, search, more), listOf(films, watchlist), selected = "start", selectedMedia = "films"))
        compose.assertEveryControlLabelledAndTappable()
        // One entry marked: the media entry wins over the target.
        compose.onNodeWithText("Films").assert(SemanticsMatcher.expectValue(SemanticsProperties.Selected, true))
        compose.onNodeWithText(s(R.string.nav_home)).assert(SemanticsMatcher.expectValue(SemanticsProperties.Selected, false))
        compose.onNodeWithContentDescription(s(R.string.app_shell_menuOpen)).assertDoesNotExist()
        compose.onNodeWithText(s(R.string.nav_watchlist)).performClick()
        compose.onNodeWithText(s(R.string.app_more_title)).performClick()
        assertEquals(listOf("media:watchlist", "target:more"), events)
    }

    @Test @Config(qualifiers = TABLET)
    fun sideColumnTablet() = sideColumn()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun sideColumnTabletLargeFont() = sideColumn()

    @Test @Config(qualifiers = PHONE)
    fun fullScreenPagesStandAlone() {
        compose.setContent {
            SlouchcatTheme {
                AppShell(ShellFrame(true, listOf(start, more), "start", listOf(films), null), PRIMARY, {}, {}, {}) { Text("Alone") }
            }
        }
        compose.onNodeWithText("Alone").assertIsDisplayed()
        compose.onNodeWithContentDescription(s(R.string.nav_home)).assertDoesNotExist()
    }
}
