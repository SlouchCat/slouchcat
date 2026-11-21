package com.slouchcat.app.mobile.player

import android.content.Context
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.isSelectable
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.SUB_DE
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.episodeState
import com.slouchcat.app.mobile.testing.filmState
import com.slouchcat.core.api.generated.models.Quality
import com.slouchcat.core.api.generated.models.Track
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.player.Countdown
import com.slouchcat.core.player.PlayerError
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.player.SeasonEpisodes
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** The player turns a phone sideways: the phone is tested in landscape (915 × 412), the tablet at 1280 dp. */
private const val PHONE = "w915dp-h412dp-land"
private const val TABLET = "w1280dp-h800dp-land"

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/** The player page (rule 8): two widths, 200 % font, every control labelled, each sheet and error picture. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class PlayerScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val recorder = object : PlayerEvents {
        override fun onBack() { events += "back" }
        override fun onTogglePlay() { events += "toggle" }
        override fun onSeekTo(positionMs: Long) { events += "seek:$positionMs" }
        override fun onSeekBy(deltaMs: Long) { events += "by:$deltaMs" }
        override fun onSpeed(speed: Float) { events += "speed:$speed" }
        override fun onAudio(track: Track) { events += "audio:${track.id}" }
        override fun onSubtitle(track: Track?) { events += "subtitle:${track?.id}" }
        override fun onQuality(quality: Quality) { events += "quality:${quality.value}" }
        override fun onSkipIntro() { events += "intro" }
        override fun onSkipOutro() { events += "outro" }
        override fun onPrevious() { events += "previous" }
        override fun onNext() { events += "next" }
        override fun onEpisode(id: String) { events += "episode:$id" }
        override fun onCancelCountdown() { events += "cancel" }
        override fun onRetry() { events += "retry" }
        override fun onEpisodesOpened() { events += "episodes" }
    }

    private fun show(state: PlayerState, sheet: PlayerSheet? = null) = compose.setContent {
        SlouchcatTheme { PlayerScreen(state, recorder, initialSheet = sheet) }
    }

    private fun button(label: String) = compose.onNode((hasText(label) or hasContentDescription(label)) and hasClickAction())

    private fun film() {
        show(filmState())
        compose.onNode(hasText("Night Train") and isHeading()).assertIsDisplayed()
        compose.onNodeWithText("0:01").assertIsDisplayed()
        compose.onNodeWithText("-2:24:59").assertIsDisplayed()
        for (tool in listOf(R.string.player_tracks_audio, R.string.player_tracks_subtitles, R.string.player_settings_quality, R.string.player_settings_speed)) {
            button(s(tool)).assertIsDisplayed()
        }
        // No episode: no «Folgen», no previous or next.
        compose.onNodeWithText(s(R.string.app_player_episodes)).assertDoesNotExist()
        compose.onNodeWithContentDescription(s(R.string.player_seek)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        button(s(R.string.player_play)).performClick()
        button(s(R.string.player_chrome_back10)).performClick()
        button(s(R.string.player_chrome_forward10)).performClick()
        button(s(R.string.app_player_close)).performClick()
        assertEquals(listOf("toggle", "by:-10000", "by:10000", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun filmPhone() = film()

    @Test @Config(qualifiers = TABLET)
    fun filmTablet() = film()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun filmPhoneLargeFont() = film()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun filmTabletLargeFont() = film()

    private fun episode() {
        show(episodeState())
        compose.onNode(hasText("S1 · E2 · Shadows of the Past") and isHeading()).assertIsDisplayed()
        compose.onNodeWithText("Quiet Ring").assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        button(s(R.string.app_player_skipIntro)).performClick()
        button(s(R.string.player_chrome_previousEpisode)).performClick()
        button(s(R.string.player_chrome_nextEpisode)).performClick()
        assertEquals(listOf("intro", "previous", "next"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun episodePhone() = episode()

    @Test @Config(qualifiers = TABLET)
    fun episodeTablet() = episode()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun episodePhoneLargeFont() = episode()

    @Test @Config(qualifiers = PHONE)
    fun theFirstEpisodeHasNoPrevious() {
        show(episodeState().copy(previous = null))
        button(s(R.string.player_chrome_previousEpisode)).assertIsNotEnabled()
    }

    private fun countdown() {
        val state = episodeState()
        show(state.copy(showSkipIntro = false, countdown = Countdown(state.next!!, 7)))
        compose.onNodeWithText(context().resources.getQuantityString(R.plurals.app_player_nextIn, 7, 7)).assertIsDisplayed()
        compose.onNodeWithText("S1 · E3 · Cowardice").assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        button(s(R.string.app_player_playNow)).performClick()
        button(s(R.string.player_next_cancel)).performClick()
        assertEquals(listOf("next", "cancel"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun countdownPhone() = countdown()

    @Test @Config(qualifiers = TABLET)
    fun countdownTablet() = countdown()

    @Test @Config(qualifiers = PHONE)
    fun withoutAutoplayTheCardWaits() {
        val state = episodeState()
        show(state.copy(showSkipIntro = false, countdown = Countdown(state.next!!, null)))
        compose.onNodeWithText(s(R.string.player_next_title)).assertIsDisplayed()
    }

    private fun context(): Context = ApplicationProvider.getApplicationContext()

    private fun audioSheet() {
        show(filmState(), PlayerSheet.Audio)
        compose.onNode(hasText(s(R.string.player_tracks_audio)) and isHeading()).assertIsDisplayed()
        compose.onNode(hasText("German · E-AC3 · 5.1") and isSelectable()).assertIsSelected()
        compose.onNodeWithText(s(R.string.player_option_default)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNode(hasText("English · DTS · 5.1", substring = true)).performClick()
        assertEquals(listOf("audio:11"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun audioSheetPhone() = audioSheet()

    @Test @Config(qualifiers = TABLET)
    fun audioSheetTablet() = audioSheet()

    @Test @Config(qualifiers = PHONE)
    fun subtitleSheet() {
        show(filmState().copy(subtitlesOff = true), PlayerSheet.Subtitles)
        compose.onNode(hasText(s(R.string.player_tracks_off)) and isSelectable()).assertIsSelected()
        compose.onNodeWithText("English · SRT · ${s(R.string.player_tracks_forced)} · ${s(R.string.player_tracks_external)}").assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNodeWithText("German · SRT").performClick()
        assertEquals(listOf("subtitle:${SUB_DE.id}"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun qualitySheet() {
        show(filmState(), PlayerSheet.Quality)
        compose.onNode(hasText(s(R.string.player_quality_autoOriginal)) and isSelectable()).assertIsSelected()
        compose.onNodeWithText("720p").assertIsDisplayed()
        // 480p is not available: not offered.
        compose.onNodeWithText("480p").assertDoesNotExist()
        compose.onNodeWithText(s(R.string.app_player_qualityNote)).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNodeWithText("720p").performClick()
        assertEquals(listOf("quality:720p"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun speedSheet() {
        show(filmState().copy(speed = 1.5f), PlayerSheet.Speed)
        compose.onNode(hasText("1.5×") and isSelectable()).assertIsSelected()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNodeWithText(s(R.string.app_player_normalSpeed)).performClick()
        assertEquals(listOf("speed:1.0"), events)
    }

    private fun infoSheet() {
        show(filmState(), PlayerSheet.Info)
        compose.onNodeWithText(s(R.string.app_player_info_direct)).assertIsDisplayed()
        compose.onNodeWithText("HEVC · 1080p · 1920 × 802", substring = true).assertIsDisplayed()
        compose.onNodeWithText("8.3 Mbit/s").performScrollTo().assertIsDisplayed()
        compose.onNodeWithText("MKV").performScrollTo().assertIsDisplayed()
        compose.onNodeWithText("Pixel 9 · Android 16").performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        button(s(R.string.common_close)).performScrollTo().performClick()
        compose.onNode(hasText(s(R.string.app_player_info_mode))).assertDoesNotExist()
    }

    @Test @Config(qualifiers = PHONE)
    fun infoSheetPhone() = infoSheet()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun infoSheetTabletLargeFont() = infoSheet()

    @Test @Config(qualifiers = PHONE)
    fun episodeSheet() {
        val state = episodeState()
        val list = listOf(state.previous!!, state.item!!, state.next!!)
        show(state.copy(episodes = SeasonEpisodes(1, list)), PlayerSheet.Episodes)
        compose.onNode(hasText(s(R.string.app_player_season, "1")) and isHeading()).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_player_nowPlaying)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNodeWithText("3. Cowardice").performClick()
        assertTrue(events.contains("episode:${state.next!!.id}"))
    }

    private fun error(error: PlayerError, title: Int, body: String, retry: Boolean) {
        show(filmState().copy(error = error))
        compose.onNode(hasText(s(title)) and isHeading()).assertIsDisplayed()
        compose.onNodeWithText(body).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        if (retry) button(s(R.string.app_state_retry)).performClick() else button(s(R.string.app_state_retry)).assertDoesNotExist()
        button(s(R.string.common_back)).performClick()
        assertEquals(if (retry) listOf("retry", "back") else listOf("back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun notFound() = error(PlayerError.NotFound, R.string.app_player_error_notFoundTitle, s(R.string.app_player_error_notFoundBody), retry = false)

    @Test @Config(qualifiers = PHONE)
    fun watchLimit() = error(PlayerError.WatchLimit, R.string.player_watchLimit_title, s(R.string.player_watchLimit_bodyUnknown), retry = false)

    @Test @Config(qualifiers = PHONE)
    fun streamLimit() = error(PlayerError.StreamLimit, R.string.player_streamLimit_title, s(R.string.player_streamLimit_bodyUnknown), retry = true)

    @Test @Config(qualifiers = TABLET)
    fun stoppedElsewhere() = error(PlayerError.StoppedElsewhere, R.string.player_ended_title, s(R.string.app_player_error_stoppedBody), retry = true)

    @Test @Config(qualifiers = PHONE)
    fun networkGone() = error(PlayerError.Network(retrying = true), R.string.app_player_error_networkTitle, s(R.string.app_player_error_networkRetrying), retry = false)

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun networkFailed() = error(PlayerError.Network(retrying = false), R.string.app_player_error_networkTitle, s(R.string.app_player_error_networkBody), retry = true)

    @Test @Config(qualifiers = PHONE)
    fun unplayable() = error(PlayerError.Unplayable, R.string.app_player_error_unplayableTitle, s(R.string.app_player_error_unplayableBody), retry = false)
}
