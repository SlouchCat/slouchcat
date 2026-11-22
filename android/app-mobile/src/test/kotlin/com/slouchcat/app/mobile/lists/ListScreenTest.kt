package com.slouchcat.app.mobile.lists

import android.content.Context
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.core.api.generated.models.HomeListKey
import com.slouchcat.core.browse.lists.ListKind
import com.slouchcat.core.browse.testing.SERVER_IMAGES
import com.slouchcat.core.browse.testing.itemRef
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.MoreState
import com.slouchcat.core.media.Paged
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

/** The lists behind the rows (rule 8): two widths, 200 % font, every control labelled, back leaves. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class ListScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val items = (1..3).map { MediaItem.from(itemRef(it, title = "Pick $it", year = 2016), SERVER_IMAGES) }

    private fun show(kind: ListKind?, state: Paged<MediaItem>) = compose.setContent {
        SlouchcatTheme {
            ListScreen(kind, state, onBack = { events += "back" }, onRetry = { events += "retry" }, onNearEnd = { events += "more" }, onItem = { events += it.title })
        }
    }

    private fun picks() {
        show(ListKind.Picks, Paged(Loadable.Ready(items), 3))
        compose.onNodeWithText(s(R.string.home_picks)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.picks_page_intro)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithContentDescription("Pick 1, 2016 · ${s(R.string.kind_movie)}, ${s(R.string.app_browse_tip)}").performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("Pick 1", "back"), events.filter { it != "more" })
    }

    @Test @Config(qualifiers = PHONE)
    fun picksPhone() = picks()

    @Test @Config(qualifiers = TABLET)
    fun picksTablet() = picks()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun picksPhoneLargeFont() = picks()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun picksTabletLargeFont() = picks()

    @Test @Config(qualifiers = PHONE)
    fun aRowWithItsCountAndAFailedNextPage() {
        show(ListKind.Row(HomeListKey.RECENT), Paged(Loadable.Ready(items), 80, MoreState.Failed))
        compose.onNodeWithText(s(R.string.home_rows_recent)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_state_retry)).performClick()
        compose.assertEveryControlLabelledAndTappable()
    }

    @Test @Config(qualifiers = TABLET)
    fun aListThatIsGone() {
        show(ListKind.Row(HomeListKey.SPOTLIGHT), Paged(Loadable.Failed(LoadError.NotFound)))
        compose.onNodeWithText(s(R.string.home_list_notFound_body)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
    }

    @Test @Config(qualifiers = PHONE)
    fun anEmptyList() {
        show(ListKind.Row(HomeListKey.RELEASED), Paged(Loadable.Empty))
        compose.onNodeWithText(s(R.string.home_list_empty)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
    }
}
