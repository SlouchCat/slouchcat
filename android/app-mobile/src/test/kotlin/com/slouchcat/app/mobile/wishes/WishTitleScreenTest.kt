package com.slouchcat.app.mobile.wishes

import androidx.compose.material3.SnackbarHostState
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isToggleable
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.WishSeasonOffer
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.browse.testing.SERVER_IMAGES
import com.slouchcat.core.browse.testing.externalTitle
import com.slouchcat.core.browse.testing.wish
import com.slouchcat.core.browse.wishes.WishSheet
import com.slouchcat.core.browse.wishes.WishTitleUiState
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** The page of a title outside the library and its two sheets (rule 8). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class WishTitleScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val recorder = object : WishTitleEvents {
        override fun onBack() { events += "back" }
        override fun onRetry() { events += "retry" }
        override fun onRequest() { events += "request" }
        override fun onRetrySeasons() { events += "retrySeasons" }
        override fun onSeason(number: Int) { events += "season:$number" }
        override fun onAllSeasons() { events += "all" }
        override fun onSubmit() { events += "submit" }
        override fun onWithdraw() { events += "withdraw" }
        override fun onConfirmWithdraw() { events += "confirm" }
        override fun onDismiss() { events += "dismiss" }
        override val onMine: () -> Unit = { events += "mine" }
    }

    private fun show(state: WishTitleUiState) = compose.setContent { SlouchcatTheme { WishTitleScreen(state, SERVER_IMAGES, recorder, SnackbarHostState()) } }

    private fun requestPage() {
        show(WishTitleUiState(Loadable.Ready(externalTitle(106, "Alexander", ExternalMedia.SHOW))))
        compose.onNodeWithText("Alexander").assertIsDisplayed()
        compose.onNodeWithText(s(R.string.titles_missing)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.app_wishes_request)) and hasClickAction())).performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("request", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun requestPagePhone() = requestPage()

    @Test @Config(qualifiers = TABLET)
    fun requestPageTablet() = requestPage()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun requestPagePhoneLargeFont() = requestPage()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun requestPageTabletLargeFont() = requestPage()

    private fun wishPage() {
        show(WishTitleUiState(Loadable.Ready(externalTitle(79, "1984", wish = WishState.PENDING)), ownWish = wish(79, "1984")))
        compose.onNodeWithContentDescription(s(R.string.app_wishes_steps_current, s(R.string.app_wishes_steps_requested))).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.wishes_state_pending)).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.wishes_withdraw_action)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.app_wishes_toMine)) and hasClickAction())).performClick()
        assertEquals(listOf("withdraw", "mine"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun wishPagePhone() = wishPage()

    @Test @Config(qualifiers = TABLET)
    fun wishPageTablet() = wishPage()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun wishPagePhoneLargeFont() = wishPage()

    private fun seasonSheet() {
        val seasons = listOf(
            WishSeasonOffer(1, 8, WishSeasonOffer.State.AVAILABLE),
            WishSeasonOffer(2, 8, WishSeasonOffer.State.MISSING),
            WishSeasonOffer(3, 6, WishSeasonOffer.State.REQUESTED),
        )
        show(WishTitleUiState(Loadable.Ready(externalTitle(106, "Alexander", ExternalMedia.SHOW)), sheet = WishSheet.Request, seasons = Loadable.Ready(seasons), chosen = setOf(2, 3)))
        compose.onNode(hasText(s(R.string.app_wishes_allSeasons)) and isToggleable()).assertIsOn()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.wishes_dialog_season, "3")) and isToggleable())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.app_wishes_allSeasons)) and isToggleable())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.common_cancel)) and hasClickAction())).performClick()
        assertEquals(listOf("season:3", "all", "dismiss"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun seasonSheetPhone() = seasonSheet()

    @Test @Config(qualifiers = TABLET)
    fun seasonSheetTablet() = seasonSheet()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun seasonSheetPhoneLargeFont() = seasonSheet()

    @Test @Config(qualifiers = PHONE)
    fun aWishSeerrApprovedHasNoWithdraw() {
        // Approved in Seerr: the server says it cannot go any more (code review M-1).
        show(WishTitleUiState(Loadable.Ready(externalTitle(79, "1984", wish = WishState.PROCESSING)), ownWish = wish(79, "1984", WishState.PROCESSING)))
        compose.onNodeWithText(s(R.string.wishes_state_processing)).performScrollTo().assertIsDisplayed()
        assertTrue(compose.onAllNodes(hasText(s(R.string.wishes_withdraw_action)) and hasClickAction()).fetchSemanticsNodes().isEmpty())
    }

    @Test @Config(qualifiers = PHONE)
    fun aRefusedRequestSaysWhy() {
        show(WishTitleUiState(Loadable.Ready(externalTitle(79, "1984")), sheet = WishSheet.Request, seasons = Loadable.Empty, error = "quota_exceeded"))
        compose.onNodeWithText(s(R.string.errors_code_quota_exceeded)).performScrollTo().assertIsDisplayed()
    }

    @Test @Config(qualifiers = PHONE)
    fun withdrawSheetPhone() {
        show(WishTitleUiState(Loadable.Ready(externalTitle(79, "1984", wish = WishState.PENDING)), sheet = WishSheet.Withdraw))
        compose.onNodeWithText(s(R.string.app_wishes_withdrawBody, "1984")).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNode(hasText(s(R.string.app_wishes_keep)) and hasClickAction()).performClick()
        assertEquals(listOf("dismiss"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun aTitleNotThereIsNotFound() {
        show(WishTitleUiState(Loadable.Failed(LoadError.NotFound)))
        compose.onNodeWithText(s(R.string.titles_notFound_title)).assertIsDisplayed()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("back"), events)
    }
}
