package com.slouchcat.app.tv.audio

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.isFocused
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.testing.TestAudio
import com.slouchcat.app.tv.testing.withoutAudio
import com.slouchcat.core.browse.testing.FakeRadio
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * «Webradio» (concept 26 5.6, photo TV 31): the star of a station of «Meine Sender» lies on its tile,
 * also while the tile has the focus and grows (verifier 4a); a station without the heart has none.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class RadioScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote
    private val audio = TestAudio()
    private val radio = FakeRadio()

    private fun s(id: Int, vararg args: Any) = pages.s(id, *args)

    private fun starOnTheFocusedTile() = compose.onAllNodes(hasTestTag(RadioKeys.STAR) and hasAnyAncestor(isFocused()), useUnmergedTree = true)

    @Test
    fun theFocusedFavoriteTileCarriesTheStar() {
        pages.show({ it.copy(radio = radio) }, features = withoutAudio + audio.feature)
        pages.openRail(s(R.string.app_radio_title))
        pages.waitFor("Radio 2")
        // «Meine Sender» first: its first station has the focus and shows its star.
        remote.assertFocused("Radio 2")
        assertEquals(1, starOnTheFocusedTile().fetchSemanticsNodes().size)
        starOnTheFocusedTile()[0].assertIsDisplayed()
        // The first station of the offer, without the heart: no star on the focused tile.
        remote.down(2)
        remote.assertFocused(s(R.string.radio_page_play, "Radio 1"))
        assertEquals(0, starOnTheFocusedTile().fetchSemanticsNodes().size)
    }
}
