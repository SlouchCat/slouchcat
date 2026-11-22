package com.slouchcat.app.tv.audio

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.requestFocus
import com.slouchcat.app.tv.shell.ShellTags
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.testing.TestAudio
import com.slouchcat.app.tv.testing.withoutAudio
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.HomePage
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.browse.testing.FakeAudio
import com.slouchcat.core.browse.testing.FakeDetails
import com.slouchcat.core.browse.testing.FakeHome
import com.slouchcat.core.browse.testing.FakeLibrary
import com.slouchcat.core.browse.testing.FakeRadio
import com.slouchcat.core.browse.testing.audiobook
import com.slouchcat.core.browse.testing.homeRow
import com.slouchcat.core.browse.testing.itemRef
import com.slouchcat.core.browse.testing.library
import com.slouchcat.core.browse.testing.podcast
import com.slouchcat.core.browse.testing.posters
import com.slouchcat.core.browse.testing.progress
import com.slouchcat.core.browse.testing.radioStation
import com.slouchcat.core.browse.testing.ulid
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * The pages of stage 4 in the real shell with the remote (rule 8, concept 26 3.3, 9; photos TV 08,
 * 25 to 28, 31): the sound libraries and the radio in the rail, the page of an audiobook (every
 * control reached, a chapter plays from its start, Back returns to it), a podcast (new and heard,
 * the check mark, an episode plays), the series, the radio (OK starts and pauses, «Jetzt läuft», the
 * heart, the search with the keyboard of the TV's search), «Weiterhören» on Start, the empty and
 * error states.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class AudioPagesTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote
    private val audio = TestAudio()
    private val book = audiobook(40, chapters = audio.chapters)
    private val show = podcast(60)
    private val details = FakeDetails(book, show)
    private val sounds = FakeAudio()
    private val radio = FakeRadio()
    private val bookRef = itemRef(40, title = book.title, kind = ItemKind.AUDIOBOOK, author = "Mira Lindqvist", images = posters(ulid(40)))
    private val shelf = FakeLibrary(ApiResult.Ok(listOf(bookRef, itemRef(60, title = show.title, kind = ItemKind.PODCAST, images = posters(ulid(60))))))
    private val home = FakeHome(ApiResult.Ok(HomePage(rows = listOf(homeRow(HomeRowKey.CONTINUE, listOf(bookRef.copy(progress = progress(30, minutes = 40))))))))
    private val libraries = listOf(library(7, "Filme"), library(4, "Hörbücher", kind = LibraryKind.AUDIOBOOKS), library(5, "Podcasts", kind = LibraryKind.PODCASTS))

    private fun s(id: Int, vararg args: Any) = pages.s(id, *args)

    private fun open() = pages.show(
        { it.copy(home = home, detail = details, library = shelf, audio = sounds, radio = radio) },
        libraries = libraries,
        features = withoutAudio + audio.feature,
    )

    private fun focus(tag: String) {
        compose.onNode(hasTestTag(tag)).requestFocus()
        remote.settle()
    }

    private fun openBook() {
        open()
        pages.openRail("Hörbücher")
        pages.waitFor(book.title)
        remote.assertFocused(book.title)
        remote.ok()
        pages.waitFor(s(R.string.item_audio_chapters))
    }

    @Test
    fun theSoundLibrariesAndTheRadioAreInTheRail() {
        open()
        compose.onNode(hasTestTag(ShellTags.entry("library-4"))).assertExists()
        compose.onNode(hasTestTag(ShellTags.entry("library-5"))).assertExists()
        compose.onNode(hasTestTag(ShellTags.entry(AudioFeature.RADIO))).assertExists()
        // Nothing plays: no «Läuft gerade».
        compose.onNode(hasTestTag(ShellTags.entry(AudioFeature.NOW))).assertDoesNotExist()
        pages.openRail("Hörbücher")
        pages.waitFor(book.title)
    }

    @Test
    fun theBookPageReachesEveryControlAndPlaysAChapterFromItsStart() {
        openBook()
        remote.assertFocused(s(R.string.tv_audio_listen))
        compose.onNode(hasText("Mira Lindqvist")).assertExists()
        compose.onNode(hasText(s(R.string.item_audio_volume, "Harbour Tales", "2"))).assertExists()
        remote.assertEveryControlReachable(scope = pages.page)
        focus(AudioItemKeys.chapter(1))
        remote.ok()
        pages.waitFor(s(R.string.player_seek))
        assertEquals(ulid(40), audio.titles.starts.single().first)
        assertEquals(audio.chapters[1].startMs, audio.titles.starts.single().second.positionMs)
        // Back returns to the chapter; the book plays on («Läuft gerade»).
        remote.back()
        pages.waitFor(s(R.string.item_audio_chapters))
        remote.assertFocused("Kapitel 2")
        assertTrue(audio.titles.stops.isEmpty())
        compose.onNode(hasTestTag(ShellTags.entry(AudioFeature.NOW))).assertExists()
    }

    @Test
    fun theSeriesPageLeadsToTheNextVolume() {
        sounds.series = ApiResult.Ok(com.slouchcat.core.browse.testing.audioSeries())
        openBook()
        focus(AudioItemKeys.SERIES)
        remote.ok()
        pages.waitFor("Harbour Tales 3")
        remote.assertFocused(s(R.string.audioSeries_continue, "2"))
        remote.assertEveryControlReachable(scope = pages.page)
        remote.ok()
        pages.waitFor(s(R.string.player_seek))
        assertEquals(ulid(42), audio.titles.starts.single().first)
        remote.back()
        pages.waitFor("Harbour Tales 3")
        remote.back()
        pages.waitFor(s(R.string.item_audio_chapters))
    }

    @Test
    fun aSeriesThatIsGoneSaysSo() {
        sounds.series = ApiResult.NotFound
        openBook()
        focus(AudioItemKeys.SERIES)
        remote.ok()
        pages.waitFor(s(R.string.app_state_notFound))
        remote.back()
        pages.waitFor(s(R.string.item_audio_chapters))
    }

    @Test
    fun aPodcastShowsNewAndHeardEpisodesAndPlaysOne() {
        open()
        pages.openRail("Podcasts")
        pages.waitFor(show.title)
        remote.right()
        remote.assertFocused(show.title)
        remote.ok()
        pages.waitFor("Episode 101")
        remote.assertFocused(s(R.string.app_audio_newest))
        assertTrue(compose.onAllNodes(hasText(s(R.string.tv_audio_new), substring = true)).fetchSemanticsNodes().isNotEmpty())
        remote.assertEveryControlReachable(scope = pages.page)
        // The check at the end of a row marks the episode heard at the server.
        focus(AudioItemKeys.episode(ulid(101)))
        remote.right()
        remote.assertFocused(s(R.string.app_audio_episodeHeard, "Episode 101"))
        remote.ok()
        assertTrue(details.changes.toString(), "watched:${ulid(101)}:true" in details.changes)
        pages.waitFor(s(R.string.tv_audio_heard))
        // OK on an episode plays it.
        focus(AudioItemKeys.episode(ulid(102)))
        remote.ok()
        pages.waitFor(s(R.string.player_seek))
        assertEquals(ulid(102), audio.titles.starts.single().first)
    }

    @Test
    fun episodesThatDoNotLoadOfferRetry() {
        sounds.episodesFailure = ApiResult.Offline
        open()
        pages.openRail("Podcasts")
        pages.waitFor(show.title)
        remote.right()
        remote.ok()
        pages.waitFor(s(R.string.app_state_offline))
        val asked = sounds.asked.size
        sounds.episodesFailure = null
        focus(AudioItemKeys.RETRY)
        remote.ok()
        pages.waitFor("Episode 101")
        assertTrue(sounds.asked.size > asked)
    }

    @Test
    fun weiterhoerenOnStartOpensTheAudioPlayer() {
        open()
        pages.waitFor(book.title)
        remote.assertFocused(book.title)
        remote.ok()
        pages.waitFor(s(R.string.player_seek))
        assertEquals(ulid(40), audio.titles.starts.single().first)
        // Not the video player: the audio player's own buttons.
        compose.onNode(hasTestTag(ListenKeys.SPEED)).assertExists()
    }

    @Test
    fun theRadioStartsAndPausesWithOkAndKeepsTheHeart() {
        open()
        pages.openRail(s(R.string.app_radio_title))
        pages.waitFor("Radio 2")
        compose.onNode(hasText(s(R.string.tv_radio_hint))).assertExists()
        // «Meine Sender» first: its first station has the focus.
        remote.assertFocused("Radio 2")
        remote.assertEveryControlReachable(scope = pages.page)
        // Down from a tile is its heart, Down again the next row on the tile it had last (the walk
        // above ended there).
        remote.down()
        remote.assertFocused(s(R.string.app_radio_favoriteOn, "Radio 2"))
        remote.down()
        remote.assertFocused(s(R.string.radio_page_play, "Radio 5"))
        remote.down()
        remote.assertFocused(s(R.string.app_radio_favoriteOff, "Radio 5"))
        // Up into «Meine Sender» returns to its tile.
        remote.up(2)
        remote.assertFocused(s(R.string.radio_page_play, "Radio 2"))
        remote.ok()
        assertTrue(audio.radio.calls.toString(), "start:2" in audio.radio.calls)
        audio.engine.ready()
        pages.waitFor(s(R.string.radio_player_open, "Testradio 2"))
        assertTrue(compose.onAllNodes(hasText(s(R.string.radio_page_playing))).fetchSemanticsNodes().isNotEmpty())
        // OK again pauses the station.
        remote.assertFocused("Radio 2")
        remote.ok()
        assertTrue(!audio.engine.wantsPlay)
        // The heart below a tile.
        focus(RadioKeys.heart("fav", 2))
        remote.ok()
        assertTrue(radio.calls.toString(), "favorite:2:false" in radio.calls)
        pages.waitFor(s(R.string.app_radio_removed, "Radio 2"))
    }

    @Test
    fun theSearchUsesTheKeyboardOfTheTvSearch() {
        radio.stations = listOf(radioStation(1), radioStation(9, name = "Quellfunk"))
        open()
        pages.openRail(s(R.string.app_radio_title))
        pages.waitFor("Radio 2")
        focus(RadioKeys.SEARCH)
        remote.ok()
        remote.assertFocused("q")
        remote.ok()
        focus(RadioKeys.SEARCH_APPLY)
        remote.ok()
        pages.waitFor(s(R.string.radio_page_filters_search, "q"))
        assertEquals("q", radio.queries.last().text)
        pages.waitFor("Quellfunk")
        compose.onNode(hasText("Radio 1")).assertDoesNotExist()
        // «Alle Filter zurücksetzen» shows every station again.
        focus(RadioKeys.CLEAR)
        remote.ok()
        pages.waitFor("Radio 2")
        assertEquals("", radio.queries.last().text.orEmpty())
    }

    @Test
    fun noStationSaysSoAndTheSearchStaysReachable() {
        radio.stations = emptyList()
        radio.favorites = ApiResult.Ok(emptyList())
        open()
        pages.openRail(s(R.string.app_radio_title))
        pages.waitFor(s(R.string.radio_page_empty_title))
        remote.assertFocused(s(R.string.radio_page_searchLabel))
    }

    @Test
    fun theCountryMenuFilters() {
        radio.stations = listOf(radioStation(1), radioStation(2, country = "AT"))
        open()
        pages.openRail(s(R.string.app_radio_title))
        pages.waitFor("Radio 2")
        focus(RadioKeys.COUNTRY)
        remote.ok()
        // The profile's own country is chosen and has the focus.
        remote.assertFocused(s(R.string.radio_page_quick_myCountry, java.util.Locale.Builder().setRegion("CH").build().getDisplayCountry(java.util.Locale.ENGLISH)))
        focus(RadioKeys.country("AT"))
        remote.ok()
        pages.waitFor(java.util.Locale.Builder().setRegion("AT").build().getDisplayCountry(java.util.Locale.ENGLISH))
        assertEquals("AT", radio.queries.last().country)
    }
}
