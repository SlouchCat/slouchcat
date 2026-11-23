package com.slouchcat.app.mobile.wishes

import android.content.Context
import androidx.compose.material3.SnackbarHostState
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.core.api.generated.models.DiscoverRowKey
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.WishCounts
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.browse.testing.SERVER_IMAGES
import com.slouchcat.core.browse.testing.externalRef
import com.slouchcat.core.browse.testing.rights
import com.slouchcat.core.browse.testing.wish
import com.slouchcat.core.browse.wishes.DiscoverRowCards
import com.slouchcat.core.browse.wishes.WishesTab
import com.slouchcat.core.browse.wishes.WishesUiState
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.Paged
import com.slouchcat.core.media.WishCard
import com.slouchcat.core.media.data.WishFilter
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

internal const val PHONE = "w412dp-h915dp"
internal const val TABLET = "w1280dp-h800dp-land"

internal fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

internal fun discoverState() = WishesUiState(
    rights = Loadable.Ready(rights()),
    rows = Loadable.Ready(
        listOf(
            DiscoverRowCards(DiscoverRowKey.TRENDING, listOf(externalRef(1, "Trend One"), externalRef(2, "Trend Two", ExternalMedia.SHOW, wish = WishState.PENDING)).map { ExternalCard.from(it, SERVER_IMAGES) }),
            DiscoverRowCards(DiscoverRowKey.POPULAR_MOVIES, listOf(ExternalCard.from(externalRef(3, "Classic"), SERVER_IMAGES, itemId = "x"))),
        ),
    ),
    counts = WishCounts(3, 2, 1, 0, 1, 0),
)

internal fun mineState() = discoverState().copy(
    tab = WishesTab.Mine,
    mine = Paged(
        Loadable.Ready(
            listOf(
                WishCard.from(wish(1, "1984"), SERVER_IMAGES),
                WishCard.from(wish(2, "Alexander", WishState.PARTIAL, ExternalMedia.SHOW, seasons = listOf(1, 2)), SERVER_IMAGES),
                WishCard.from(wish(3, "Verity", WishState.AVAILABLE), SERVER_IMAGES),
            ),
        ),
        total = 3,
    ),
)

/** «Wünsche» (rule 8): both tabs, two widths, 200 % font, every control labelled, the withdraw sheet, Seerr off. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class WishesScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val recorder = object : WishesEvents {
        override fun onTab(tab: WishesTab) { events += "tab:$tab" }
        override fun onText(text: String) { events += "text:$text" }
        override fun onClear() { events += "clear" }
        override fun onSubmit() { events += "submit" }
        override fun onRetry() { events += "retry" }
        override fun onMoreResults() { events += "more" }
        override fun onFilter(filter: WishFilter) { events += "filter:$filter" }
        override fun onMoreWishes() { events += "moreWishes" }
        override fun onOpenTitle(card: ExternalCard) { events += "open:${card.title}" }
        override fun onOpenWish(card: WishCard) { events += "wish:${card.title}" }
        override fun onAskWithdraw(card: WishCard) { events += "ask:${card.title}" }
        override fun onConfirmWithdraw() { events += "confirm" }
        override fun onDismissWithdraw() { events += "dismiss" }
    }

    private fun show(state: WishesUiState) = compose.setContent { SlouchcatTheme { WishesScreen(state, recorder, SnackbarHostState()) } }

    private fun discover() {
        show(discoverState())
        compose.onNodeWithText(s(R.string.app_wishes_discoverIntro)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.wishes_discover_rows_trending)).assertIsDisplayed()
        compose.onNode(hasText(s(R.string.wishes_area_discover)) and hasClickAction()).assertIsSelected()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasContentDescription("Trend One", substring = true) and hasClickAction())).performClick()
        compose.onNode(hasText(s(R.string.wishes_area_mine)) and hasClickAction()).performClick()
        assertEquals(listOf("open:Trend One", "tab:Mine"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun discoverPhone() = discover()

    @Test @Config(qualifiers = TABLET)
    fun discoverTablet() = discover()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun discoverPhoneLargeFont() = discover()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun discoverTabletLargeFont() = discover()

    private fun mine() {
        show(mineState())
        compose.onNodeWithText(s(R.string.wishes_state_pending)).assertIsDisplayed()
        compose.onNodeWithText("Verity").performScrollTo().assertIsDisplayed()
        // The row names the wished seasons, not their number.
        val list = "1" + s(R.string.format_listSeparator) + "2"
        val seasons = ApplicationProvider.getApplicationContext<Context>().resources.getQuantityString(R.plurals.wishes_seasons, 2, list)
        compose.onNode(hasText(seasons, substring = true)).assertExists()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.wishes_page_filters_open)) and hasClickAction())).performClick()
        compose.reveal(compose.onNodeWithContentDescription(s(R.string.wishes_row_withdrawLabel, "1984"))).performClick()
        compose.reveal(compose.onNode(hasText("Verity", substring = true) and hasClickAction())).performClick()
        assertEquals(listOf("filter:Open", "ask:1984", "wish:Verity"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun minePhone() = mine()

    @Test @Config(qualifiers = TABLET)
    fun mineTablet() = mine()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun minePhoneLargeFont() = mine()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun mineTabletLargeFont() = mine()

    @Test @Config(qualifiers = PHONE)
    fun withdrawAsksFirst() {
        val state = mineState()
        show(state.copy(withdrawing = state.mine.loaded.first()))
        compose.onNodeWithText(s(R.string.wishes_withdraw_title)).assertIsDisplayed()
        compose.onNode(hasText(s(R.string.app_wishes_keep)) and hasClickAction()).performClick()
        compose.onNode(hasText(s(R.string.wishes_withdraw_confirm)) and hasClickAction()).performClick()
        assertEquals(listOf("dismiss", "confirm"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun seerrNotSetUpSaysSo() {
        show(WishesUiState(rights = Loadable.Empty))
        compose.onNodeWithText(s(R.string.app_wishes_off_title)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_wishes_off_body)).assertIsDisplayed()
    }

    @Test @Config(qualifiers = PHONE)
    fun seerrOutOfReachOffersARetry() {
        show(discoverState().copy(rows = Loadable.Failed(LoadError.Failed)))
        compose.reveal(compose.onNode(hasText(s(R.string.app_state_retry)) and hasClickAction())).performClick()
        assertEquals(listOf("retry"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun withoutDiscoverThereAreNoTabs() {
        show(mineState().copy(rights = Loadable.Ready(rights(canRequest = false, discover = false))))
        assertTrue(compose.onAllNodes(hasText(s(R.string.wishes_area_discover))).fetchSemanticsNodes().isEmpty())
        compose.onNodeWithText("1984").assertIsDisplayed()
    }

    @Test @Config(qualifiers = PHONE)
    fun theSearchShowsItsHits() {
        val hits = listOf(externalRef(7, "Found Seven")).map { ExternalCard.from(it, SERVER_IMAGES) }
        show(discoverState().copy(text = "found", results = Paged(Loadable.Ready(hits), 1)))
        compose.onNodeWithText(s(R.string.wishes_search_hits)).assertIsDisplayed()
        compose.onNode(hasText("Found Seven", substring = true) and hasClickAction()).performClick()
        compose.onNodeWithContentDescription(s(R.string.app_search_clear)).performClick()
        assertEquals(listOf("open:Found Seven", "clear"), events)
    }
}
