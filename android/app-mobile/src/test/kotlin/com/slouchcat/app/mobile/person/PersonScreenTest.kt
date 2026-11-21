package com.slouchcat.app.mobile.person

import android.content.Context
import android.graphics.Bitmap
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
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
import com.slouchcat.app.mobile.testing.SERVER_IMAGES
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.app.mobile.testing.person
import com.slouchcat.app.mobile.testing.ulid
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

/** The person page (rule 8): two widths, 200 % font, portrait, facts, biography, titles with roles. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class PersonScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()

    private val loader by lazy {
        ImageLoader.Builder(ApplicationProvider.getApplicationContext<Context>())
            .components { add(Interceptor { chain -> SuccessResult(Bitmap.createBitmap(40, 60, Bitmap.Config.ARGB_8888).asImage(), chain.request) as ImageResult }) }
            .build()
    }

    private fun page(open: Boolean = false): PersonUiState {
        val p = person()
        return PersonUiState(Loadable.Ready(PersonPage(p, p.credits.groupBy { it.item.id }.values.map { PersonTitle(it.first().item, it) })), biographyOpen = open)
    }

    private fun show(state: PersonUiState) = compose.setContent {
        SlouchcatTheme {
            CompositionLocalProvider(LocalMediaImageLoader provides loader) {
                PersonScreen(
                    state = state,
                    images = SERVER_IMAGES,
                    onBack = { events += "back" },
                    onRetry = { events += "retry" },
                    onToggleBiography = { events += "bio" },
                    onOpen = { id -> { events += "item:$id" } },
                )
            }
        }
    }

    private fun aPerson() {
        show(page(open = true))
        compose.onNode(hasText("Ann Actor") and isHeading()).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.person_department_acting)).assertIsDisplayed()
        compose.onNodeWithText(ApplicationProvider.getApplicationContext<Context>().resources.getQuantityString(R.plurals.app_person_inLibrary, 2, 2)).assertIsDisplayed()
        compose.reveal(compose.onNodeWithText("Example Town")).assertIsDisplayed()
        compose.waitUntil(5_000) { compose.onAllNodesWithTag(IMAGE_LOADED_TAG, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() }
        compose.reveal(compose.onNode(hasText(s(R.string.app_person_knownFrom)) and isHeading())).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.person_biography_less)) and hasClickAction())).performClick()
        // One card per title with every role in it.
        val roles = listOf(s(R.string.person_as, "The Hero"), s(R.string.item_cast_role_director)).joinToString(s(R.string.format_listSeparator))
        compose.reveal(compose.onNodeWithContentDescription("Night Train" + s(R.string.format_listSeparator) + roles)).performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("bio", "item:${ulid(1)}", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun personPhone() = aPerson()

    @Test @Config(qualifiers = TABLET)
    fun personTablet() = aPerson()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun personPhoneLargeFont() = aPerson()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun personTabletLargeFont() = aPerson()

    private fun notFound() {
        show(PersonUiState(Loadable.Failed(LoadError.NotFound)))
        compose.onNodeWithText(s(R.string.person_notFound_title)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
    }

    @Test @Config(qualifiers = PHONE)
    fun notFoundPhone() = notFound()

    @Test @Config(qualifiers = TABLET)
    fun notFoundTablet() = notFound()

    private fun failed() {
        show(PersonUiState(Loadable.Failed(LoadError.Offline)))
        compose.onNode(hasText(s(R.string.app_state_retry)) and hasClickAction()).performClick()
        assertEquals(listOf("retry"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun failedPhone() = failed()

    @Test @Config(qualifiers = TABLET)
    fun failedTablet() = failed()
}
