package com.slouchcat.app.mobile.media

import android.content.Context
import android.graphics.Bitmap
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHasNoClickAction
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import coil3.ImageLoader
import coil3.asImage
import coil3.intercept.Interceptor
import coil3.request.ImageResult
import coil3.request.SuccessResult
import com.slouchcat.app.mobile.testing.SERVER_IMAGES
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.posters
import com.slouchcat.app.mobile.testing.progress
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.CardShape
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.MoreState
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

/**
 * The media building blocks (mockup `card`, `hrow`, `.grid`, `fsk`, `.empty`, `pic()`): two widths,
 * 200 % font, every control labelled and 48 dp, a missing image shows the name, a loaded image draws.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class MediaComponentsTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val film = MediaItem.from(itemRef(1, title = "Night Train", year = 2004, rating = 12), SERVER_IMAGES)
    private val book = MediaItem.from(itemRef(2, title = "Quiet Book", kind = ItemKind.AUDIOBOOK, author = "A. Author"), SERVER_IMAGES)
    private val started = MediaItem.from(itemRef(3, title = "Long Film", progress = progress(40, minutes = 120)), SERVER_IMAGES)

    private fun blocks() {
        compose.setContent {
            SlouchcatTheme {
                Column {
                    MediaRow("Row title", onShowAll = { events += "all" }) {
                        items(listOf(film, book), key = { it.id }) { MediaCard(it, onClick = { events += it.id }) }
                        item { MediaCard(started, onClick = { events += "wide" }, shape = CardShape.Wide, caption = remainingText(started)) }
                    }
                    MediaCard(film, onClick = null, tag = CardTag(s(R.string.app_browse_tip), SlouchcatIcon.Heart))
                    FskBadge(12)
                }
            }
        }
        compose.assertEveryControlLabelledAndTappable()
        // Title and caption are one control; the placeholder of the missing image shows the name.
        compose.onNodeWithContentDescription("Night Train, 2004 · Film", substring = false).assertHasClickAction().performClick()
        compose.onNodeWithContentDescription("Quiet Book, A. Author").assertHasClickAction().performClick()
        val remaining = s(R.string.home_remaining, s(R.string.format_hoursMinutes, "1", "12"))
        compose.onNodeWithContentDescription("Long Film, $remaining").performClick()
        compose.onNodeWithContentDescription(s(R.string.app_browse_showAllLabel, "Row title")).assertHasClickAction().performClick()
        // Without its page the card is no control.
        compose.onNodeWithContentDescription("Night Train, 2004 · Film, ${s(R.string.app_browse_tip)}").assertHasNoClickAction()
        compose.onNodeWithContentDescription(s(R.string.rating_age, "12")).assertIsDisplayed()
        assertTrue(compose.onAllNodesWithTag(IMAGE_PLACEHOLDER_TAG, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty())
        assertEquals(listOf(film.id, book.id, "wide", "all"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun blocksPhone() = blocks()

    @Test @Config(qualifiers = TABLET)
    fun blocksTablet() = blocks()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun blocksPhoneLargeFont() = blocks()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun blocksTabletLargeFont() = blocks()

    private fun grid(columns: Int) {
        val many = (1..12).map { MediaItem.from(itemRef(it, title = "Grid $it"), SERVER_IMAGES) }
        var nearEnd = 0
        compose.setContent {
            SlouchcatTheme {
                MediaGrid(many, key = { it.id }, onNearEnd = { nearEnd++ }, header = { fullWidth { MediaEmpty("Head") } }) {
                    MediaCard(it, onClick = { events += it.title }, width = null)
                }
            }
        }
        compose.onNodeWithText("Head").assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        val first = compose.onNodeWithContentDescription("Grid 1,", substring = true).fetchSemanticsNode().boundsInRoot
        val sameLine = compose.onAllNodes(hasContentDescription("Grid ", substring = true)).fetchSemanticsNodes().count { it.boundsInRoot.top == first.top }
        assertEquals(columns, sameLine)
        compose.waitForIdle()
        assertTrue("the grid asks for more near its end", nearEnd >= 1)
    }

    // 412 - 2 × 16 = 380 dp, cells at least 104 dp with 12 dp gaps: 3 columns (mockup phone).
    @Test @Config(qualifiers = PHONE)
    fun gridPhone() = grid(3)

    // 1280 dp without the side column of the shell (here the whole width): 1232 dp, 140 dp cells, 16 dp gaps: 8.
    @Test @Config(qualifiers = TABLET)
    fun gridTablet() = grid(8)

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun gridPhoneLargeFont() = grid(3)

    private fun states() {
        compose.setContent {
            SlouchcatTheme {
                Column {
                    MediaEmpty("Nothing", body = "Body", actionLabel = "Reset", action = { events += "reset" })
                    PagingFooter(MoreState.Failed, onRetry = { events += "retry" })
                }
            }
        }
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithText("Reset").performClick()
        compose.onNodeWithText(s(R.string.app_state_retry)).performClick()
        compose.onNodeWithText(s(R.string.library_page_pageFailed)).assertIsDisplayed()
        assertEquals(listOf("reset", "retry"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun statesPhone() = states()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun statesTabletLargeFont() = states()

    /** A card with an image of the server draws the image, not the placeholder (the test server has no covers). */
    @Test @Config(qualifiers = PHONE)
    fun aCardWithAnImageDrawsIt() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        val requested = mutableListOf<String>()
        val loader = ImageLoader.Builder(context)
            .components {
                add(
                    Interceptor { chain ->
                        requested += chain.request.data.toString()
                        SuccessResult(Bitmap.createBitmap(20, 30, Bitmap.Config.ARGB_8888).asImage(), chain.request) as ImageResult
                    },
                )
            }
            .build()
        val withImage = MediaItem.from(itemRef(9, title = "Covered", images = posters(com.slouchcat.app.mobile.testing.ulid(9))), SERVER_IMAGES)
        compose.setContent {
            SlouchcatTheme {
                CompositionLocalProvider(LocalMediaImageLoader provides loader) {
                    MediaCard(withImage, onClick = {})
                }
            }
        }
        compose.waitUntil(5_000) { compose.onAllNodesWithTag(IMAGE_LOADED_TAG, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() }
        compose.onNodeWithTag(IMAGE_PLACEHOLDER_TAG, useUnmergedTree = true).assertDoesNotExist()
        // A card of 124 dp asks for the smallest poster at least as wide as its pixels.
        val px = 124 * context.resources.displayMetrics.density
        val size = if (px <= 200) "w200" else if (px <= 400) "w400" else "w800"
        assertTrue(requested.single().contains("/images/poster/$size?v=hash&p=1"))
        assertTrue(requested.single().startsWith("http://192.0.2.10:8096/api/v1/items/"))
    }
}
