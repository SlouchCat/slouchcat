package com.slouchcat.app.tv.player

import android.content.Context
import android.view.KeyEvent
import android.view.accessibility.AccessibilityManager
import org.robolectric.Shadows.shadowOf
import com.slouchcat.core.browse.testing.seasonRef
import com.slouchcat.core.browse.testing.ulid
import androidx.activity.ComponentActivity
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasTestTag
import com.slouchcat.core.api.generated.models.Chapter
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performKeyInput
import androidx.compose.ui.test.pressKey
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.tv.testing.Dpad
import com.slouchcat.app.tv.testing.RecordedEvents
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.testing.countdownState
import com.slouchcat.app.tv.testing.episodeState
import com.slouchcat.app.tv.testing.filmState
import com.slouchcat.app.tv.testing.playerEpisode
import com.slouchcat.app.tv.ui.TvRemoteKeys
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
import org.robolectric.annotation.GraphicsMode

/**
 * The player page with the remote (rule 8, concept 26 9): the start focus on the bar, every control
 * reached with the D-pad and the focus visible, OK plays and pauses, Links and Rechts spool (a
 * streak and a held key), Hoch and Runter bring the hidden controls back, Zurück hides them first,
 * the menus and their choices, the card of the next episode, the error pictures with «Erneut
 * versuchen», the media keys, labels for TalkBack.
 */
@OptIn(ExperimentalTestApi::class)
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class PlayerScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val remote = Dpad(compose)
    private val context: Context = ApplicationProvider.getApplicationContext()
    private val events = RecordedEvents()
    private var state by mutableStateOf(filmState())
    private fun s(id: Int, vararg args: Any) = context.getString(id, *args)

    private val menu = hasAnyAncestor(SemanticsMatcher.keyIsDefined(SemanticsProperties.PaneTitle))

    private var seasons by mutableStateOf(SeasonsMenu())

    private fun show(initial: PlayerState = filmState(), controls: Boolean = true, keys: TvRemoteKeys? = null, autoHide: Boolean = false) {
        state = initial
        remote.show {
            PlayerScreen(state, SoundState(), seasons, events, images = null, remoteKeys = keys, autoHide = autoHide, initialControls = controls)
        }
    }

    private fun controlsShown(): Boolean = compose.onAllNodes(hasTestTag(PlayerKeys.CONTROLS_SHOWN)).fetchSemanticsNodes().isNotEmpty()

    @Test
    fun theBarHasTheFocusAndEveryButtonIsReached() {
        show()
        remote.assertFocused(s(R.string.player_seek))
        remote.down()
        remote.assertFocused(s(R.string.player_tracks_audio))
        remote.assertFocusVisible(
            compose.onNode(hasText(s(R.string.player_tracks_audio)) and hasClickAction()),
            compose.onNode(hasText(s(R.string.player_tracks_subtitles)) and hasClickAction()),
        )
        // Hoch back to the bar, Runter to the button that had the focus.
        remote.right()
        remote.up()
        remote.assertFocused(s(R.string.player_seek))
        remote.down()
        remote.assertFocused(s(R.string.player_tracks_subtitles))
        remote.up()
        remote.assertEveryControlReachable()
    }

    @Test
    fun okPlaysAndPausesAndTheBarIsASlider() {
        show()
        remote.ok()
        assertEquals(listOf("toggle"), events.calls)
        val bar = compose.onNode(hasContentDescription(s(R.string.player_seek))).fetchSemanticsNode()
        assertTrue(bar.config.contains(SemanticsProperties.ProgressBarRangeInfo))
        // The time and the chapter there (code review L-2 of stage 5).
        assertEquals(s(R.string.player_time, "0:01", "2:25:00") + s(R.string.format_listSeparator) + "Anfang", bar.config[SemanticsProperties.StateDescription])
        // Paused: the page says so.
        compose.onNode(hasText(s(R.string.tv_player_paused))).assertExists()
    }

    @Test
    fun rechtsSpoolsInGrowingStepsAndSeeksOnce() {
        show()
        // Three presses within 300 ms: 10 + 10 + 20 s from 0:01.
        compose.onRoot().performKeyInput {
            pressKey(Key.DirectionRight)
            advanceEventTime(100)
            pressKey(Key.DirectionRight)
            advanceEventTime(100)
            pressKey(Key.DirectionRight)
        }
        compose.waitForIdle()
        assertTrue(compose.onAllNodes(hasText("0:41")).fetchSemanticsNodes().isNotEmpty())
        compose.onNode(hasText(s(R.string.tv_player_ahead, s(R.string.format_seconds, "40")))).assertExists()
        assertTrue("no seek while the keys come: ${events.calls}", events.calls.none { it.startsWith("seek") })
        compose.mainClock.advanceTimeBy(Scrub.COMMIT_MS + FRAME)
        compose.waitForIdle()
        assertEquals(listOf("seek:41000"), events.calls)
    }

    @Test
    fun aHeldKeyGoesMinutesAhead() {
        show()
        compose.onRoot().performKeyInput {
            keyDown(Key.DirectionRight)
            advanceEventTime(2_000)
            keyUp(Key.DirectionRight)
        }
        compose.mainClock.advanceTimeBy(Scrub.COMMIT_MS + FRAME)
        compose.waitForIdle()
        val seeks = events.calls.filter { it.startsWith("seek") }
        assertEquals("one seek after the key is let go: $seeks", 1, seeks.size)
        val seek = seeks.single().substringAfter(':').toLong()
        assertTrue("a held key spools faster than single steps: $seek", seek >= 5 * 60_000)
    }

    @Test
    fun hiddenControlsComeBackWithTheArrowsAndBackHidesThem() {
        show(controls = false)
        // Runter: the controls on the first button; Zurück hides them again.
        remote.down()
        remote.assertFocused(s(R.string.player_tracks_audio))
        remote.back()
        assertTrue(events.calls.isEmpty())
        // OK on hidden controls plays or pauses.
        remote.ok()
        assertEquals(listOf("toggle"), events.calls)
        remote.back()
        // Links spools and shows the bar.
        remote.left()
        remote.assertFocused(s(R.string.player_seek))
    }

    @Test
    fun theMenusChooseAndCloseWithBack() {
        show()
        remote.down()
        remote.ok()
        // «Ton»: the first track is chosen and has the focus; the note and the way to the settings.
        remote.assertFocused("Dolby Digital Plus")
        compose.onNode(hasText(s(R.string.tv_player_soundSettings)) and menu).assertExists()
        remote.assertEveryControlReachable(menu)
        remote.down()
        remote.ok()
        assertEquals(listOf("audio:11"), events.calls)
        remote.assertFocused(s(R.string.player_tracks_audio))

        // «Untertitel»: «Aus», then the PGS track drawn by the TV.
        remote.right()
        remote.ok()
        remote.assertFocused(s(R.string.player_tracks_off))
        compose.onNode(hasText(s(R.string.tv_player_subtitleImage)) and menu).assertExists()
        remote.back()
        remote.assertFocused(s(R.string.player_tracks_subtitles))

        // «Qualität»; right of it only «Info» (no speed on the TV, mockup 22-play-0).
        remote.right()
        remote.ok()
        remote.down(2)
        remote.ok()
        remote.right()
        remote.assertFocused(s(R.string.tv_player_info))
        assertEquals(listOf("audio:11", "quality:1080p"), events.calls)
    }

    @Test
    fun theEpisodesOfTheSeasonAndPreviousAndNext() {
        val episodes = (1..4).map { playerEpisode(it) }
        show(episodeState().copy(episodes = SeasonEpisodes(1, episodes)))
        // The skip button floats above the picture.
        compose.onNode(hasText(s(R.string.app_player_skipIntro)) and hasClickAction()).assertExists()
        remote.down()
        remote.right(3)
        remote.assertFocused(s(R.string.app_player_episodes))
        remote.ok()
        assertEquals("episodes", events.calls.single())
        remote.assertFocused("2. Episode 2")
        remote.down()
        remote.ok()
        assertEquals("episode:${episodes[2].id}", events.calls.last())
        remote.right()
        remote.ok()
        remote.right()
        remote.ok()
        assertEquals(listOf("previous", "next"), events.calls.takeLast(2))
    }

    @Test
    fun theCardOfTheNextEpisodeTakesTheFocus() {
        show(countdownState())
        remote.assertFocused(s(R.string.app_player_playNow))
        compose.onNode(hasText(context.resources.getQuantityString(R.plurals.app_player_nextIn, 7, 7))).assertExists()
        remote.right()
        remote.assertFocused(s(R.string.player_next_cancel))
        remote.ok()
        remote.left()
        remote.ok()
        assertEquals(listOf("cancelCountdown", "next"), events.calls)
    }

    @Test
    fun anErrorOffersAnotherTryOrTheWayBack() {
        show(filmState().copy(error = PlayerError.Failed))
        remote.assertFocused(s(R.string.common_retry))
        remote.ok()
        remote.right()
        remote.ok()
        assertEquals(listOf("retry", "back"), events.calls)
        // A title that is gone: only the way back.
        state = filmState().copy(error = PlayerError.NotFound)
        remote.settle()
        remote.settle()
        remote.assertFocused(s(R.string.common_back))
        compose.onNode(hasText(s(R.string.common_retry))).assertDoesNotExist()
        compose.onNode(hasText(s(R.string.app_player_error_notFoundTitle))).assertExists()
    }

    @Test
    fun theMediaKeysOfTheRemote() {
        val keys = TvRemoteKeys()
        show(keys = keys)
        fun press(code: Int) {
            compose.runOnUiThread { keys.dispatch(KeyEvent(KeyEvent.ACTION_DOWN, code)) }
            remote.settle()
        }
        press(KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE)
        press(KeyEvent.KEYCODE_MEDIA_NEXT)
        press(KeyEvent.KEYCODE_MEDIA_STOP)
        assertEquals(listOf("toggle", "next", "back"), events.calls)
    }

    @Test
    fun theInfoPanelNamesModeVideoHdrAndSound() {
        show()
        remote.down()
        remote.right(3)
        remote.assertFocused(s(R.string.tv_player_info))
        remote.ok()
        compose.onNode(hasText(s(R.string.app_player_info_direct))).assertExists()
        compose.onNode(hasText(s(R.string.tv_player_hdrNone))).assertExists()
        // Zurück closes the panel first.
        remote.back()
        compose.onNode(hasText(s(R.string.app_player_info_direct))).assertDoesNotExist()
    }

    @Test
    fun whileItPlaysTheControlsHideAfterFourSecondsAndAMediaKeyBringsThemBack() {
        val keys = TvRemoteKeys()
        show(filmState().copy(playing = true), keys = keys, autoHide = true)
        assertTrue(controlsShown())
        compose.mainClock.advanceTimeBy(HIDE_MS + FRAME)
        remote.settle()
        assertTrue("hidden after 4 s", !controlsShown())
        // The focus stays on the bar (invisible), nothing is lost.
        remote.assertFocused(s(R.string.player_seek))
        compose.runOnUiThread { keys.dispatch(KeyEvent(KeyEvent.ACTION_DOWN, KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE)) }
        remote.settle()
        assertEquals(listOf("toggle"), events.calls)
        assertTrue("a media key shows them", controlsShown())
        // A key keeps them: 2.5 s, a key, 2.5 s more (5 s after the media key), still there.
        compose.mainClock.advanceTimeBy(2_500)
        remote.down()
        compose.mainClock.advanceTimeBy(2_500)
        remote.settle()
        assertTrue("a key keeps them", controlsShown())
    }

    @Test
    fun withTalkBackTheControlsStay() {
        val manager = context.getSystemService(AccessibilityManager::class.java)
        assertTrue(!touchExploration(manager))
        shadowOf(manager).setEnabled(true)
        shadowOf(manager).setTouchExplorationEnabled(true)
        assertTrue(touchExploration(manager))
        // The page then gets autoHide = false: nothing hides while it plays.
        show(filmState().copy(playing = true), autoHide = !touchExploration(manager))
        compose.mainClock.advanceTimeBy(HIDE_MS * 2)
        remote.settle()
        assertTrue(controlsShown())
    }

    @Test
    fun backWhileSpoolingDropsTheTarget() {
        show()
        remote.right()
        remote.back()
        compose.mainClock.advanceTimeBy(Scrub.COMMIT_MS + FRAME)
        remote.settle()
        assertTrue("no jump after Zurück: ${events.calls}", events.calls.none { it.startsWith("seek") })
        assertTrue("the controls stay for the next Zurück", controlsShown())
    }

    @Test
    fun theSeasonChipsShowTheEpisodesOfAnotherSeason() {
        val season1 = (1..3).map { playerEpisode(it) }
        val season2 = listOf(playerEpisode(1, "Return").copy(id = ulid(401), seasonNumber = 2, parentId = ulid(202)))
        seasons = SeasonsMenu(seasons = listOf(seasonRef(1), seasonRef(2).copy(id = ulid(202))))
        show(episodeState().copy(episodes = SeasonEpisodes(1, season1)))
        remote.down()
        remote.right(3)
        remote.ok()
        remote.assertFocused("2. Episode 2")
        // Up to the chips, right to season 2.
        remote.up(3)
        remote.right()
        remote.ok()
        assertEquals("season:${ulid(202)}", events.calls.last())
        seasons = seasons.copy(shown = ulid(202), episodes = mapOf(ulid(202) to season2))
        remote.settle()
        remote.settle()
        remote.assertFocused("Return")
        remote.ok()
        assertEquals("episode:${ulid(401)}", events.calls.last())
    }

    @Test
    fun aBusyServerShowsTheWaitingThenTheWayOn() {
        show(filmState().copy(error = PlayerError.Busy(retrying = true)))
        compose.onNode(hasText(s(R.string.player_busy_body))).assertExists()
        compose.onNode(hasText(s(R.string.common_retry))).assertDoesNotExist()
        state = filmState().copy(error = PlayerError.Busy(retrying = false))
        remote.settle()
        remote.settle()
        remote.assertFocused(s(R.string.common_retry))
    }

    @Test
    fun theBarMarksTheChaptersAndSpoolingNamesTheChapter() {
        val end = filmState().durationMs
        val chapters = listOf(Chapter(0, 30_000, "Opening"), Chapter(30_000, 600_000), Chapter(600_000, end, "Finale"))
        // The rules: the chapter at a position, and the marks without the very start.
        assertEquals(-1, chapterIndexAt(emptyList(), 5_000))
        assertEquals(0, chapterIndexAt(chapters, 29_999))
        assertEquals(1, chapterIndexAt(chapters, 30_000))
        assertEquals(listOf(30_000L, 600_000L), chapterMarks(chapters, end))
        assertEquals(emptyList<Long>(), chapterMarks(chapters, 0))
        // Marks closer than the gap to the one before are left out (many chapters).
        assertEquals(listOf(30_000L), chapterMarks(chapters, end, minGapMs = 600_000))
        show(filmState().copy(chapters = chapters))
        assertEquals(2, compose.onAllNodes(hasTestTag(PlayerKeys.CHAPTER_MARK)).fetchSemanticsNodes().size)
        // One step: 0:11 lies in «Opening»; the bubble names it.
        compose.onRoot().performKeyInput { pressKey(Key.DirectionRight) }
        compose.waitForIdle()
        compose.onNode(hasText("Opening")).assertExists()
        // Two more within the streak: 0:41 lies in the second chapter, which has no title.
        compose.onRoot().performKeyInput {
            advanceEventTime(100)
            pressKey(Key.DirectionRight)
            advanceEventTime(100)
            pressKey(Key.DirectionRight)
        }
        compose.waitForIdle()
        compose.onNode(hasText(s(R.string.player_chapters_untitled, "2"))).assertExists()
        // Without chapters neither marks nor a name.
        state = filmState().copy(chapters = emptyList())
        remote.settle()
        assertTrue(compose.onAllNodes(hasTestTag(PlayerKeys.CHAPTER_MARK)).fetchSemanticsNodes().isEmpty())
    }

    private companion object {
        const val FRAME = 100L
    }
}
