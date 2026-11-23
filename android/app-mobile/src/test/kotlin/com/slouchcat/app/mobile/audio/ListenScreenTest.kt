package com.slouchcat.app.mobile.audio

import android.content.Context
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.player.LastAudio
import com.slouchcat.core.player.PlayerError
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.player.SleepChoice
import com.slouchcat.core.player.SleepState
import com.slouchcat.core.player.testing.audioInfo
import com.slouchcat.core.player.testing.ref
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

/** Records what the page asks for. */
class RecordedListen : ListenEvents {
    val calls = mutableListOf<String>()
    override fun onClose() { calls += "close" }
    override fun onTogglePlay() { calls += "toggle" }
    override fun onSeekTo(positionMs: Long) { calls += "seek:$positionMs" }
    override fun onBack() { calls += "back15" }
    override fun onForward() { calls += "forward30" }
    override fun onPreviousChapter() { calls += "previousChapter" }
    override fun onNextChapter() { calls += "nextChapter" }
    override fun onPreviousEpisode() { calls += "previousEpisode" }
    override fun onNextEpisode() { calls += "nextEpisode" }
    override fun onSpeed(speed: Float) { calls += "speed:$speed" }
    override fun onSleep(choice: SleepChoice) { calls += "sleep:$choice" }
    override fun onChapter(chapter: Chapter) { calls += "chapter:${chapter.startMs}" }
    override fun onRetry() { calls += "retry" }
}

val BOOK_CHAPTERS = (0 until 20).map { Chapter(it * 300_000L, (it + 1) * 300_000L, "Kapitel ${it + 1}") }

/** An audiobook that plays in chapter 8. */
fun bookState(playing: Boolean = true, error: PlayerError? = null) = PlayerState(
    item = ref("book", title = "The Quiet Harbour", kind = ItemKind.AUDIOBOOK).copy(author = "Mira Lindqvist"),
    info = audioInfo("book", durationMs = 6_000_000, chapters = BOOK_CHAPTERS),
    loading = false,
    playing = playing,
    positionMs = 2_266_000,
    durationMs = 6_000_000,
    chapters = BOOK_CHAPTERS,
    speed = 1f,
    error = error,
)

/** The audio player (rule 8, concept 27 section 12): two widths, 200 % font, every button, the sheets, Back. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class ListenScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = RecordedListen()

    private fun show(ui: ListenUi, sheet: ListenSheet? = null) = compose.setContent {
        SlouchcatTheme { ListenScreen(ui, events, initialSheet = sheet) }
    }

    private fun book() {
        show(ListenUi(state = bookState(), sleep = null))
        compose.onNode(hasText("The Quiet Harbour") and isHeading()).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_audio_chapterOf, "8", "20"), substring = true).assertExists()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNodeWithContentDescription(s(R.string.player_pause)).performClick()
        compose.onNodeWithContentDescription(s(R.string.player_audio_back15)).performClick()
        compose.onNodeWithContentDescription(s(R.string.player_audio_forward30)).performClick()
        compose.onNodeWithContentDescription(s(R.string.player_audio_previousChapter)).performClick()
        compose.onNodeWithContentDescription(s(R.string.player_audio_nextChapter)).performClick()
        compose.onNodeWithContentDescription(s(R.string.app_audio_close)).performClick()
        assertEquals(listOf("toggle", "back15", "forward30", "previousChapter", "nextChapter", "close"), events.calls)
    }

    @Test @Config(qualifiers = PHONE)
    fun bookPhone() = book()

    @Test @Config(qualifiers = TABLET)
    fun bookTablet() = book()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun bookPhoneLargeFont() = book()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun bookTabletLargeFont() = book()

    @Test @Config(qualifiers = PHONE)
    fun theSpeedSheetChoosesAPresetAndStepsInTenths() {
        show(ListenUi(state = bookState().copy(speed = 1.2f)), ListenSheet.Speed)
        compose.assertEveryControlLabelledAndTappable()
        compose.reveal(compose.onNode(hasText("1,5×") or hasText("1.5×"))).performClick()
        assertEquals("speed:1.5", events.calls.single())
    }

    @Test @Config(qualifiers = PHONE)
    fun theSleepSheetShowsWhatIsLeft() {
        show(ListenUi(state = bookState(), sleep = SleepState(SleepChoice.Minutes(30), 754_000)), ListenSheet.Sleep)
        compose.onAllNodesWithText(s(R.string.app_audio_sleepLeft, "12:34")).onFirst().assertExists()
        compose.reveal(compose.onNode(hasText(s(R.string.player_audio_sleep_chapter)))).performClick()
        assertEquals("sleep:EndOfChapter", events.calls.single())
    }

    @Test @Config(qualifiers = PHONE)
    fun aChapterOfTheSheetJumpsToItsStart() {
        show(ListenUi(state = bookState()), ListenSheet.Chapters)
        compose.reveal(compose.onNode(hasText("Kapitel 3"))).performClick()
        assertEquals("chapter:600000", events.calls.single())
    }

    @Test @Config(qualifiers = PHONE)
    fun anErrorOffersToTryAgain() {
        show(ListenUi(state = bookState(playing = false, error = PlayerError.Network(retrying = false))))
        compose.reveal(compose.onNode(hasText(s(R.string.app_state_retry)))).performClick()
        assertEquals("retry", events.calls.single())
        compose.onNodeWithContentDescription(s(R.string.player_audio_back15)).assertIsNotEnabled()
    }

    @Test @Config(qualifiers = PHONE)
    fun aRestoredTitleWaitsForATap() {
        show(ListenUi(restored = LastAudio("book", "The Quiet Harbour", "Mira Lindqvist", "owner")))
        compose.onNodeWithText("The Quiet Harbour").assertExists()
        compose.onNodeWithContentDescription(s(R.string.player_play)).performClick()
        assertEquals("toggle", events.calls.single())
    }

    @Test @Config(qualifiers = PHONE)
    fun anEpisodeWithoutChaptersGoesToTheNextEpisode() {
        val episode = bookState().copy(item = ref("ep", title = "Episode 7", kind = ItemKind.PODCAST_EPISODE), chapters = emptyList(), next = ref("ep8", kind = ItemKind.PODCAST_EPISODE))
        show(ListenUi(state = episode))
        compose.onNodeWithContentDescription(s(R.string.app_audio_nextEpisode)).performClick()
        assertTrue(events.calls == listOf("nextEpisode"))
    }
}

/** The small player: what plays, play or pause, the cross, the tap that opens the player. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class MiniPlayerTest {
    @get:Rule
    val compose = createComposeRule()

    private fun mini(qualifier: String) {
        val calls = mutableListOf<String>()
        compose.setContent {
            SlouchcatTheme {
                MiniPlayer(MiniUi("The Quiet Harbour", "Mira Lindqvist", com.slouchcat.core.media.ImageSet(), playing = true), onOpen = { calls += "open" }, onToggle = { calls += "toggle" }, onStop = { calls += "stop" })
            }
        }
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithContentDescription(s(R.string.player_pause)).performClick()
        compose.onNodeWithContentDescription(s(R.string.app_audio_stop)).performClick()
        compose.onNodeWithText("The Quiet Harbour").performClick()
        assertEquals(listOf("toggle", "stop", "open"), calls)
        assertTrue(qualifier.isNotEmpty())
    }

    @Test @Config(qualifiers = PHONE)
    fun phone() = mini(PHONE)

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun tabletLargeFont() = mini(TABLET)

    /** Code review M-5: after an error the button says «Erneut versuchen» and hands the tap on (AudioPlayback retries). */
    @Test @Config(qualifiers = PHONE)
    fun anErrorOffersARetry() {
        val calls = mutableListOf<String>()
        compose.setContent {
            SlouchcatTheme {
                MiniPlayer(MiniUi("The Quiet Harbour", null, com.slouchcat.core.media.ImageSet(), playing = false, retry = true), onOpen = {}, onToggle = { calls += "toggle" }, onStop = {})
            }
        }
        compose.onNodeWithContentDescription(s(R.string.radio_player_retry)).performClick()
        assertEquals(listOf("toggle"), calls)
    }
}
