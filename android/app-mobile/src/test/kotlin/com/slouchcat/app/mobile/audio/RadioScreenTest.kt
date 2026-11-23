package com.slouchcat.app.mobile.audio

import android.content.Context
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.core.api.generated.models.AudioSeries
import com.slouchcat.core.api.generated.models.RadioFacetCount
import com.slouchcat.core.api.generated.models.RadioStation
import com.slouchcat.core.browse.radio.RadioFilter
import com.slouchcat.core.browse.radio.RadioUiState
import com.slouchcat.core.browse.testing.SERVER_IMAGES
import com.slouchcat.core.browse.testing.audioSeries
import com.slouchcat.core.browse.testing.radioStation
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.Paged
import com.slouchcat.core.player.RadioState
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

/** Records what the radio page asks for. */
class RecordedRadio : RadioEvents {
    val calls = mutableListOf<String>()
    override fun onBack() { calls += "back" }
    override fun onRetry() { calls += "retry" }
    override fun onNearEnd() = Unit
    override fun onSearch(text: String) { calls += "search:$text" }
    override fun onCountry(code: String?) { calls += "country:$code" }
    override fun onCountrySheet(open: Boolean) { calls += "sheet:$open" }
    override fun onFavoritesOnly() { calls += "favorites" }
    override fun onFavorite(station: RadioStation) { calls += "heart:${station.id}" }
    override fun onStation(station: RadioStation) { calls += "station:${station.id}" }
}

val STATIONS = listOf(radioStation(1, "Radio Lakeside"), radioStation(2, "Radio Hilltop", favorite = true), radioStation(3, "Radio Riverbank"))

fun radioUi(favorites: Loadable<List<RadioStation>> = Loadable.Ready(listOf(STATIONS[1]))) = RadioUiState(
    filter = RadioFilter(country = "CH"),
    myCountry = "CH",
    countries = listOf(RadioFacetCount("CH", 3), RadioFacetCount("DE", 9)),
    favorites = favorites,
)

/** «Webradio» (rule 8): two widths, 200 % font, the chips, a station, the heart, the country sheet, Back. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class RadioScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = RecordedRadio()

    private fun show(ui: RadioUiState = radioUi(), stations: Paged<RadioStation> = Paged(Loadable.Ready(STATIONS), 3), playing: RadioState? = null) = compose.setContent {
        SlouchcatTheme { RadioScreen(ui, stations, playing, { ImageSet() }, events) }
    }

    private fun radio() {
        show(playing = RadioState(station = STATIONS[0], wantsToPlay = true, playing = true))
        compose.onNode(hasText(s(R.string.app_radio_title)) and isHeading()).assertIsDisplayed()
        compose.onAllNodesWithContentDescription(s(R.string.app_radio_favoriteOff, "Radio Lakeside")).onFirst().performClick()
        compose.onNode(hasText(s(R.string.radio_page_quick_favorites))).performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        // A lazy grid: the controls on screen (rows off screen are not composed).
        compose.assertEveryControlLabelledAndTappable()
        assertEquals(listOf("heart:1", "favorites", "back"), events.calls)
    }

    @Test @Config(qualifiers = PHONE)
    fun radioPhone() = radio()

    @Test @Config(qualifiers = TABLET)
    fun radioTablet() = radio()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun radioPhoneLargeFont() = radio()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun radioTabletLargeFont() = radio()

    @Test @Config(qualifiers = PHONE)
    fun theCountrySheetOffersEveryCountry() {
        show(radioUi().copy(countrySheet = true))
        compose.reveal(compose.onNode(hasText(s(R.string.app_radio_allCountries)))).performClick()
        assertEquals("country:null", events.calls.last())
    }

    @Test @Config(qualifiers = PHONE)
    fun withoutFavouritesTheHint() {
        show(radioUi(favorites = Loadable.Empty), stations = Paged(Loadable.Empty))
        compose.reveal(compose.onNode(hasText(s(R.string.radio_page_rows_noFavorites)))).assertExists()
        compose.reveal(compose.onNode(hasText(s(R.string.radio_page_empty_title)))).assertExists()
    }

    @Test @Config(qualifiers = PHONE)
    fun aTapOnAStationPlaysIt() {
        show()
        compose.onAllNodes(hasText("Radio Lakeside")).onFirst().performClick()
        assertTrue("station:1" in events.calls)
    }
}

/** The series of an audiobook (rule 8): two widths, 200 % font, «Weiterhören», a volume, Back, not found. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class AudioSeriesScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val calls = mutableListOf<String>()

    private fun show(state: Loadable<AudioSeries>) = compose.setContent {
        SlouchcatTheme {
            AudioSeriesScreen(state, SERVER_IMAGES, SeriesLinks(back = { calls += "back" }, retry = { calls += "retry" }, item = { calls += "item:$it" }, play = { calls += "play:$it" }))
        }
    }

    private fun series() {
        show(Loadable.Ready(audioSeries()))
        compose.onNode(hasText("Harbour Tales") and isHeading()).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.audioSeries_continue, "2")))).performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(2, calls.size)
        assertEquals("back", calls.last())
    }

    @Test @Config(qualifiers = PHONE)
    fun seriesPhone() = series()

    @Test @Config(qualifiers = TABLET)
    fun seriesTablet() = series()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun seriesPhoneLargeFont() = series()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun seriesTabletLargeFont() = series()

    @Test @Config(qualifiers = PHONE)
    fun aSeriesTheProfileDoesNotSee() {
        show(Loadable.Failed(LoadError.NotFound))
        compose.onNodeWithText(s(R.string.audioSeries_notFound_title)).assertExists()
    }
}
