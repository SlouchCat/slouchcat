package com.slouchcat.app.mobile.wishes

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.slouchcat.app.mobile.browse.NotificationsRoute
import com.slouchcat.app.mobile.home.HomeRoute
import com.slouchcat.app.mobile.more.MoreRoute
import com.slouchcat.app.mobile.notifications.NotificationSettingsRoute
import com.slouchcat.app.mobile.notifications.NotificationsFeature
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.core.browse.testing.SERVER_IMAGES
import com.slouchcat.core.browse.testing.externalRef
import com.slouchcat.core.browse.testing.rights
import com.slouchcat.core.browse.wishes.MissingTitlesUiState
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** «Noch nicht in der Bibliothek?» under the search's hits (code review m-7) and the in-app links of stage 8. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36], qualifiers = PHONE)
class MissingTitlesTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()

    private fun show(state: MissingTitlesUiState) = compose.setContent {
        SlouchcatTheme {
            MissingTitlesSection(state, "glas", onOpen = { events += "open:${it.title}" }, onSearch = { events += "search" }, onRetry = { events += "retry" })
        }
    }

    @Test
    fun hitsAndTheCardToTheWishesSearch() {
        show(MissingTitlesUiState(rights(), "glas", Loadable.Ready(listOf(ExternalCard.from(externalRef(1, "Glasvogel"), SERVER_IMAGES)))))
        compose.onNodeWithText(s(R.string.app_wishes_missingTitle)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNode(hasText("Glasvogel", substring = true) and hasClickAction()).performClick()
        compose.onNode(hasText(s(R.string.app_wishes_searchCard, "glas")) and hasClickAction()).performClick()
        assertEquals(listOf("open:Glasvogel", "search"), events)
    }

    @Test
    fun nothingFoundWithoutTheWishesSearchShowsNothing() {
        show(MissingTitlesUiState(rights(discover = false), "glas", Loadable.Empty))
        assertTrue(compose.onAllNodes(hasText(s(R.string.app_wishes_missingTitle))).fetchSemanticsNodes().isEmpty())
    }

    @Test
    fun aFailureSaysSoAndOffersARetry() {
        show(MissingTitlesUiState(rights(discover = false), "glas", Loadable.Failed(LoadError.Failed)))
        compose.onNodeWithText(s(R.string.app_wishes_failed)).assertIsDisplayed()
        compose.onNode(hasText(s(R.string.app_state_retry)) and hasClickAction()).performClick()
        assertEquals(listOf("retry"), events)
    }

    @Test
    fun theLinksOfStage8() {
        val wishes = WishesFeature()
        assertEquals(AppLink(WishesFeature.TARGET, listOf(WishesRoute(tab = WishesFeature.MINE))), wishes.link("/wishes?focus=4"))
        assertEquals(AppLink(WishesFeature.TARGET, listOf(WishesRoute(), WishTitleRoute("show", 9100))), wishes.link("titles/show/9100"))
        assertNull(wishes.link("titles/book/1"))
        assertNull(wishes.link("titles/movie/0"))
        val notifications = NotificationsFeature(devicePush = { null })
        assertEquals(AppLink("start", listOf(HomeRoute, NotificationsRoute)), notifications.link("/notifications"))
        assertEquals(AppLink("more", listOf(MoreRoute, NotificationSettingsRoute)), notifications.link("settings/notifications"))
        assertNull(notifications.link("settings/other"))
    }
}
