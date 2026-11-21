package com.slouchcat.app.mobile.item

import android.content.Context
import android.graphics.Bitmap
import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import coil3.ImageLoader
import coil3.asImage
import coil3.intercept.Interceptor
import coil3.request.ImageResult
import coil3.request.SuccessResult
import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.media.IMAGE_LOADED_TAG
import com.slouchcat.app.mobile.media.LocalMediaImageLoader
import com.slouchcat.app.mobile.testing.SEASON_1
import com.slouchcat.app.mobile.testing.SEASON_2
import com.slouchcat.app.mobile.testing.SERVER_IMAGES
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.app.mobile.testing.film
import com.slouchcat.app.mobile.testing.progress
import com.slouchcat.app.mobile.testing.seasonDetail
import com.slouchcat.app.mobile.testing.series
import com.slouchcat.app.mobile.testing.ulid
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private const val PHONE = "w412dp-h915dp"
private const val TABLET = "w1280dp-h800dp-land"

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

private fun p(id: Int, n: Int): String = ApplicationProvider.getApplicationContext<Context>().resources.getQuantityString(id, n, n)

/**
 * The detail page (rule 8): film and series in two widths and with 200 % font, every control
 * labelled and 48 dp, each control reaches its event, images and cast with data the test server
 * lacks, not found, failure, the two sheets.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class ItemScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()

    private val recorder = object : ItemEvents {
        override fun retry() { events += "retry" }
        override fun onWatchlist() { events += "watchlist" }
        override fun onWatched() { events += "watched" }
        override fun onSeasonWatched() { events += "seasonWatched" }
        override fun onSeason(id: String) { events += "season:$id" }
        override fun onAskChoice(watched: Boolean) { events += "ask:$watched" }
        override fun onRecommendAsk() { events += "recommend" }
        override fun onRecommendConfirm() { events += "recommendConfirm" }
        override fun onDismissAsk() { events += "dismiss" }
        override fun onToggleOverview() { events += "overview" }
        override fun onToggleTech() { events += "tech" }
        override fun onVersion(id: String) { events += "version:$id" }
    }

    private fun links(player: Boolean = true) = ItemLinks(
        back = { events += "back" },
        item = { id -> { events += "item:$id" } },
        person = { id -> { events += "person:$id" } },
        collection = { id -> { events += "collection:$id" } },
        search = { events += "search" },
        play = if (player) { id -> events += "play:$id" } else null,
        trailer = { events += "trailer:$it" },
    )

    private val loader by lazy {
        ImageLoader.Builder(ApplicationProvider.getApplicationContext<Context>())
            .components { add(Interceptor { chain -> SuccessResult(Bitmap.createBitmap(40, 60, Bitmap.Config.ARGB_8888).asImage(), chain.request) as ImageResult }) }
            .build()
    }

    private fun show(state: ItemUiState, player: Boolean = true) = compose.setContent {
        SlouchcatTheme {
            CompositionLocalProvider(LocalMediaImageLoader provides loader) {
                ItemScreen(state, SERVER_IMAGES, links(player), recorder, SnackbarHostState())
            }
        }
    }

    /** The control named [label]; in an open sheet the sheet's (it lies above the page). */
    private fun control(label: String) = compose.onAllNodes((hasText(label) or hasContentDescription(label)) and hasClickAction()).onLast()

    private fun aFilm() {
        show(ItemUiState(Loadable.Ready(film()), trailer = "abcDEF12345"))
        // The logo stands for the title (said as the title); the images draw.
        compose.onNodeWithContentDescription("Night Train").assertExists()
        compose.waitUntil(5_000) { compose.onAllNodesWithTag(IMAGE_LOADED_TAG, useUnmergedTree = true).fetchSemanticsNodes().size >= 3 }
        compose.onNodeWithText("A new day begins.").assertExists()
        compose.onNodeWithText("Action, Drama", substring = true).assertExists()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(control(s(R.string.app_item_play))).performClick()
        compose.reveal(control(s(R.string.item_watchlist_add))).performClick()
        compose.reveal(control(s(R.string.watched_button))).performClick()
        compose.reveal(control(s(R.string.item_recommend_add))).performClick()
        compose.reveal(compose.onNodeWithContentDescription(s(R.string.app_item_trailerLabel))).performClick()
        compose.reveal(compose.onNodeWithContentDescription("Ann Actor", substring = true)).performClick()
        compose.reveal(control(s(R.string.app_item_tech))).performClick()
        compose.reveal(compose.onNodeWithContentDescription(s(R.string.app_browse_showAllLabel, "Night Train Collection"))).performClick()
        compose.reveal(compose.onNodeWithContentDescription("Night Train 2", substring = true)).performClick()
        compose.onNodeWithContentDescription(s(R.string.app_item_search)).performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(
            listOf(
                "play:${ulid(1)}", "watchlist", "watched", "recommend", "trailer:abcDEF12345", "person:101", "tech",
                "collection:${ulid(900)}", "item:${ulid(2)}", "search", "back",
            ),
            events,
        )
    }

    @Test @Config(qualifiers = PHONE)
    fun filmPhone() = aFilm()

    @Test @Config(qualifiers = TABLET)
    fun filmTablet() = aFilm()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun filmPhoneLargeFont() = aFilm()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun filmTabletLargeFont() = aFilm()

    private fun aStartedFilmOpened() {
        show(ItemUiState(Loadable.Ready(film(progress = progress(40, minutes = 145), inWatchlist = true, recommended = true)), overviewOpen = true, techOpen = true))
        compose.reveal(control(s(R.string.app_item_resume))).assertIsDisplayed()
        compose.reveal(compose.onNodeWithText(s(R.string.app_item_left, s(R.string.format_hoursMinutes, "1", "27")))).assertIsDisplayed()
        compose.reveal(control(s(R.string.item_watchlist_added))).assertIsDisplayed()
        compose.reveal(control(s(R.string.item_recommend_done))).assertIsDisplayed()
        // The file: both versions, the rows of the first.
        compose.reveal(compose.onNodeWithText("HEVC")).assertIsDisplayed()
        compose.reveal(compose.onNodeWithText(s(R.string.format_resolutionWithClass, s(R.string.format_resolution, "1920", "1080"), s(R.string.format_fhd)))).assertIsDisplayed()
        compose.reveal(compose.onNodeWithText(p(R.plurals.app_item_versionCount, 2))).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(control("Director's Cut")).performClick()
        compose.reveal(control(s(R.string.app_item_less))).performClick()
        assertEquals(listOf("version:f2", "overview"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun startedFilmPhone() = aStartedFilmOpened()

    @Test @Config(qualifiers = TABLET)
    fun startedFilmTablet() = aStartedFilmOpened()

    private fun withoutPlayerOrDetails() {
        show(ItemUiState(Loadable.Ready(film(overview = null, withCast = false, canRecommend = false, trailers = 0).copy(collections = emptyList()))), player = false)
        // No player yet (stage 5): no big button; no trailer, no recommend, no cast.
        control(s(R.string.app_item_play)).assertDoesNotExist()
        control(s(R.string.item_recommend_add)).assertDoesNotExist()
        compose.onNodeWithText(s(R.string.app_item_cast)).assertDoesNotExist()
        compose.reveal(compose.onNodeWithText(s(R.string.app_item_noOverview))).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
    }

    @Test @Config(qualifiers = PHONE)
    fun plainPhone() = withoutPlayerOrDetails()

    @Test @Config(qualifiers = TABLET)
    fun plainTablet() = withoutPlayerOrDetails()

    private fun aSeries() {
        show(ItemUiState(Loadable.Ready(series()), seasonId = SEASON_1, seasons = mapOf(SEASON_1 to Loadable.Ready(seasonDetail(1, watched = 2, resumeAt = 3)))))
        val code = s(R.string.home_continue_episode, "1", "3")
        compose.reveal(control(s(R.string.app_item_resumeEpisode, code))).performClick()
        compose.onNodeWithText(p(R.plurals.app_item_seasons, 2), substring = true).assertExists()
        compose.reveal(compose.onNodeWithText(s(R.string.item_episodes_title))).assertIsDisplayed()
        compose.reveal(compose.onNodeWithText(p(R.plurals.app_item_episodes, 3))).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(control("Season 2")).performClick()
        compose.reveal(control(s(R.string.app_item_seasonWatched))).performClick()
        val third = s(R.string.app_item_episodeTitle, "3", "Episode 1-3")
        compose.reveal(compose.onNode(hasText(third) and hasClickAction())).performClick()
        compose.reveal(compose.onNodeWithContentDescription(s(R.string.item_episodes_playTitle, third))).performClick()
        val episode = ulid(1000 + 100 + 3)
        assertEquals(listOf("play:$episode", "season:$SEASON_2", "seasonWatched", "item:$episode", "play:$episode"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun seriesPhone() = aSeries()

    @Test @Config(qualifiers = TABLET)
    fun seriesTablet() = aSeries()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun seriesPhoneLargeFont() = aSeries()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun seriesTabletLargeFont() = aSeries()

    private fun aSeasonLoading() {
        show(ItemUiState(Loadable.Ready(series()), seasonId = SEASON_2, seasons = mapOf(SEASON_2 to Loadable.Failed(LoadError.Offline))))
        compose.reveal(control(s(R.string.app_state_retry))).performClick()
        assertEquals(listOf("season:$SEASON_2"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun seasonFailedPhone() = aSeasonLoading()

    private fun notFound() {
        show(ItemUiState(Loadable.Failed(LoadError.NotFound)))
        compose.onNodeWithText(s(R.string.item_notFound_title)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.item_notFound_body)).assertIsDisplayed()
        control(s(R.string.app_state_retry)).assertDoesNotExist()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun notFoundPhone() = notFound()

    @Test @Config(qualifiers = TABLET)
    fun notFoundTablet() = notFound()

    private fun failed() {
        show(ItemUiState(Loadable.Failed(LoadError.Offline)))
        compose.onNodeWithText(s(R.string.app_state_offline)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        control(s(R.string.app_state_retry)).performClick()
        assertEquals(listOf("retry"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun failedPhone() = failed()

    @Test @Config(qualifiers = TABLET)
    fun failedTablet() = failed()

    private fun halfSheet() {
        show(ItemUiState(Loadable.Ready(series(watchedCount = 2)), ask = ItemAsk.Watched("Quiet Ring", audio = false, count = 2, half = true, season = false)))
        compose.onNode(hasText(s(R.string.watched_ask_halfTitle, "Quiet Ring")) and isHeading()).assertIsDisplayed()
        compose.onNodeWithText(p(R.plurals.watched_ask_halfBody, 2)).assertIsDisplayed()
        compose.reveal(control(s(R.string.watched_ask_markAll))).performClick()
        compose.reveal(control(s(R.string.watched_ask_removeAll))).performClick()
        compose.reveal(control(s(R.string.common_cancel))).performClick()
        assertEquals(listOf("ask:true", "ask:false", "dismiss"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun halfSheetPhone() = halfSheet()

    @Test @Config(qualifiers = TABLET)
    fun halfSheetTablet() = halfSheet()

    private fun recommendSheet() {
        show(ItemUiState(Loadable.Ready(film()), ask = ItemAsk.Recommend))
        compose.onNodeWithText(s(R.string.item_recommend_hint)).assertIsDisplayed()
        compose.reveal(control(s(R.string.app_item_recommendConfirm))).performClick()
        assertTrue(events.contains("recommendConfirm"))
    }

    @Test @Config(qualifiers = PHONE)
    fun recommendSheetPhone() = recommendSheet()

    @Test @Config(qualifiers = TABLET)
    fun recommendSheetTablet() = recommendSheet()

    @Test @Config(qualifiers = PHONE)
    fun aRecommendedTitleOffersTakingItBack() {
        show(ItemUiState(Loadable.Ready(film(recommended = true)), ask = ItemAsk.Recommend))
        compose.reveal(control(s(R.string.app_item_unrecommend))).performClick()
        assertEquals(listOf("recommendConfirm"), events)
    }
}
