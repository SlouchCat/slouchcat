package com.slouchcat.app.tv.audio

import android.content.Context
import android.view.KeyEvent
import androidx.activity.ComponentActivity
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.requestFocus
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.tv.testing.Dpad
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.testing.playedEpisode
import com.slouchcat.app.tv.ui.TvRemoteKeys
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.browse.testing.chapters
import com.slouchcat.core.browse.testing.episodeRef
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.Paged
import com.slouchcat.core.player.PlayerError
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.player.testing.ref
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/** What the audio player asked for, in order. */
private class Recorded : ListenEvents {
    val calls = mutableListOf<String>()
    override fun onTogglePlay() { calls += "toggle" }
    override fun onPlay() { calls += "play" }
    override fun onPause() { calls += "pause" }
    override fun onBack() { calls += "back" }
    override fun onForward() { calls += "forward" }
    override fun onPrevious() { calls += "previous" }
    override fun onNext() { calls += "next" }
    override fun onSpeed(speed: Float) { calls += "speed:$speed" }
    override fun onChapter(chapter: Chapter) { calls += "chapter:${chapter.startMs}" }
    override fun onEpisode(episode: ItemRef) { calls += "episode:${episode.id}" }
    override fun onRetry() { calls += "retry" }
    override fun onStop() { calls += "stop" }
    override fun onLeave() { calls += "leave" }
}

/**
 * The audio player with the remote (rule 8, concept 26 5.6, photos TV 29, 30, 58, 59): the start
 * focus on play, every control reached and the focus visible, left and right on the timeline jump,
 * the media keys of the remote, «Tempo» and «Kapitel» or «Folgen» as side menus with the current
 * choice focused, the calm screen and the first key that only wakes it, Back, the loading and
 * error states, labels for TalkBack.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class ListenScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val remote = Dpad(compose)
    private val context: Context = ApplicationProvider.getApplicationContext()
    private val events = Recorded()
    private val keys = TvRemoteKeys()
    private val book: List<Chapter> = chapters(count = 4, minutes = 10)
    private var state by mutableStateOf<PlayerState?>(null)
    private fun s(id: Int, vararg args: Any) = context.getString(id, *args)
    private val menu = hasAnyAncestor(SemanticsMatcher.keyIsDefined(SemanticsProperties.PaneTitle))

    private fun bookState(positionMs: Long = 14 * 60_000L, playing: Boolean = true) = PlayerState(
        item = ref("book", title = "The Quiet Harbour", kind = ItemKind.AUDIOBOOK).copy(author = "Mira Lindqvist"),
        loading = false,
        playing = playing,
        positionMs = positionMs,
        durationMs = book.last().endMs,
        chapters = book,
    )

    private var nearEnd = 0

    private fun show(initial: PlayerState?, episodes: Paged<ItemRef>? = null, calmAfterMs: Long = CALM_MS) {
        state = initial
        val ui = episodes?.let { EpisodesUi(it, emptySet(), onNearEnd = { nearEnd++ }, onRetry = {}, onHeard = {}) }
        remote.show { ListenScreen(state, images = null, episodes = ui, remoteKeys = keys, events = events, calmAfterMs = calmAfterMs) }
    }

    private fun media(code: Int) {
        compose.runOnUiThread { keys.dispatch(KeyEvent(KeyEvent.ACTION_DOWN, code)) }
        remote.settle()
    }

    @Test
    fun playHasTheFocusAndEveryControlIsReached() {
        show(bookState())
        remote.assertFocused(s(R.string.player_pause))
        // The page says the kind and the chapter, the title, the author, the chapter's name.
        compose.onNode(hasText(s(R.string.kind_audiobook).uppercase(), substring = true)).assertExists()
        compose.onNode(hasText(s(R.string.app_audio_chapterOf, "2", "4").uppercase(), substring = true)).assertExists()
        compose.onNode(hasText("Kapitel 2")).assertExists()
        // The timeline of the chapter: 4:00 played, 6:00 left.
        compose.onNode(hasText("4:00")).assertExists()
        compose.onNode(hasText(s(R.string.tv_player_behind, "6:00"))).assertExists()
        remote.assertFocusVisible(
            compose.onNode(hasContentDescription(s(R.string.player_audio_back15)) and hasClickAction()),
            compose.onNode(hasContentDescription(s(R.string.player_audio_forward30)) and hasClickAction()),
        )
        remote.assertEveryControlReachable()
    }

    @Test
    fun theTimelineJumpsWithLeftAndRight() {
        show(bookState())
        remote.up()
        remote.assertFocused(s(R.string.player_seek))
        val bar = compose.onNode(hasContentDescription(s(R.string.player_seek))).fetchSemanticsNode()
        assertEquals(s(R.string.player_time, "4:00", "10:00"), bar.config[SemanticsProperties.StateDescription])
        remote.left()
        remote.right()
        remote.right()
        assertEquals(listOf("back", "forward", "forward"), events.calls)
        // The focus stays on the timeline.
        remote.assertFocused(s(R.string.player_seek))
    }

    @Test
    fun theButtonsAndTheMediaKeys() {
        show(bookState())
        remote.ok()
        remote.left()
        remote.ok()
        remote.right(2)
        remote.ok()
        remote.right()
        remote.ok()
        assertEquals(listOf("toggle", "back", "forward", "next"), events.calls)
        events.calls.clear()
        media(KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE)
        media(KeyEvent.KEYCODE_MEDIA_NEXT)
        media(KeyEvent.KEYCODE_MEDIA_PREVIOUS)
        media(KeyEvent.KEYCODE_MEDIA_FAST_FORWARD)
        media(KeyEvent.KEYCODE_MEDIA_REWIND)
        media(KeyEvent.KEYCODE_MEDIA_STOP)
        assertEquals(listOf("toggle", "next", "previous", "forward", "back", "stop"), events.calls)
    }

    @Test
    fun tempoIsASideMenuWithTheCurrentSpeedFocused() {
        show(bookState())
        compose.onNode(hasTestTag(ListenKeys.SPEED)).requestFocus()
        remote.settle()
        remote.ok()
        remote.assertFocused(s(R.string.player_audio_rate, "1"))
        remote.assertFocused(s(R.string.tv_audio_normal))
        remote.assertEveryControlReachable(scope = menu)
        remote.down()
        remote.ok()
        assertEquals(listOf("speed:1.25"), events.calls)
        // The menu closed, the focus is back on «Tempo».
        remote.assertFocused(s(R.string.app_audio_speedTitle))
    }

    @Test
    fun theChaptersMenuFocusesTheChapterThatPlays() {
        show(bookState())
        compose.onNode(hasTestTag(ListenKeys.LIST)).requestFocus()
        remote.settle()
        remote.ok()
        remote.assertFocused("Kapitel 2")
        // The one before is heard, the one that plays has the time left.
        compose.onNode(hasText(s(R.string.tv_audio_heard), substring = true) and menu).assertExists()
        remote.down()
        remote.ok()
        assertEquals(listOf("chapter:${book[2].startMs}"), events.calls)
        remote.assertFocused(s(R.string.player_chapters_button))
        // Back closes a menu, not the page.
        remote.ok()
        remote.back()
        assertTrue(events.calls.none { it == "leave" })
        remote.back()
        assertEquals("leave", events.calls.last())
    }

    @Test
    fun anEpisodeShowsItsPodcastAndTheEpisodesMenu() {
        val episodes = (101..104).map { episodeRef(it) }
        val playing = episodes[1]
        val episode = bookState().copy(item = playedEpisode(playing.id, "The Letter", podcast = "Night Shift Stories", parent = "pod"), chapters = emptyList(), previous = episodes[2], next = episodes[0])
        show(episode, episodes = Paged(items = Loadable.Ready(episodes), total = episodes.size))
        compose.onNode(hasText("Night Shift Stories")).assertExists()
        compose.onNode(hasText("The Letter")).assertExists()
        compose.onNode(hasContentDescription(s(R.string.app_audio_nextEpisode))).assertExists()
        compose.onNode(hasTestTag(ListenKeys.LIST)).requestFocus()
        remote.settle()
        remote.assertFocused(s(R.string.item_episodes_title))
        remote.ok()
        remote.assertFocused(playing.title)
        remote.up()
        remote.ok()
        assertEquals(listOf("episode:${episodes[0].id}"), events.calls)
    }

    @Test
    fun theEpisodesMenuLoadsTheOlderEpisodes() {
        val episodes = (101..104).map { episodeRef(it) }
        val episode = bookState().copy(item = playedEpisode(episodes[0].id, "The Letter", podcast = "Night Shift Stories", parent = "pod"), chapters = emptyList())
        // A podcast with more episodes than one page (review m-3).
        show(episode, episodes = Paged(items = Loadable.Ready(episodes), total = 40))
        compose.onNode(hasTestTag(ListenKeys.LIST)).requestFocus()
        remote.settle()
        remote.ok()
        assertTrue("the next page is asked for", nearEnd > 0)
    }

    @Test
    fun aNextEpisodeKeepsTheFocusOnNext() {
        val episodes = (101..104).map { episodeRef(it) }
        fun on(n: Int) = bookState().copy(item = playedEpisode(episodes[n].id, "Episode $n", podcast = "Night Shift Stories", parent = "pod"), chapters = emptyList(), next = episodes[n + 1], previous = null)
        show(on(0))
        compose.onNode(hasTestTag(ListenKeys.NEXT)).requestFocus()
        remote.settle()
        remote.ok()
        // The next episode comes (review m-2): the focus stays on «Nächste Folge», a second OK goes on.
        state = on(1)
        remote.settle()
        remote.settle()
        remote.assertFocused(s(R.string.app_audio_nextEpisode))
        remote.ok()
        assertEquals(listOf("next", "next"), events.calls)
    }

    @Test
    fun theKeysPlayAndPauseDoNotToggle() {
        show(bookState(playing = true))
        // A remote with keys of their own (review m-1): Play on a title that plays changes nothing.
        media(KeyEvent.KEYCODE_MEDIA_PLAY)
        media(KeyEvent.KEYCODE_MEDIA_PAUSE)
        assertEquals(listOf("play", "pause"), events.calls)
    }

    @Test
    fun anErrorWithoutRetryFocusesBack() {
        show(null)
        // Review m-4: «Zurück» has the focus when «Erneut versuchen» cannot help.
        state = PlayerState(loading = false, error = PlayerError.NotFound)
        remote.settle()
        remote.settle()
        remote.assertFocused(s(R.string.common_back))
        compose.onNode(hasTestTag(ListenKeys.RETRY)).assertDoesNotExist()
        remote.ok()
        assertEquals(listOf("leave"), events.calls)
    }

    @Test
    fun theScreenTurnsCalmAndTheFirstKeyOnlyWakesIt() {
        show(bookState(), calmAfterMs = 2_000)
        compose.mainClock.advanceTimeBy(3_000)
        remote.settle()
        compose.onNode(hasTestTag(ListenKeys.CALM)).assertExists()
        // The first key wakes the page and does nothing else.
        remote.ok()
        assertTrue(events.calls.isEmpty())
        remote.ok()
        assertEquals(listOf("toggle"), events.calls)
        // Calm again; Back wakes it first, then leaves.
        compose.mainClock.advanceTimeBy(3_000)
        remote.settle()
        remote.back()
        assertTrue(events.calls.none { it == "leave" })
        remote.back()
        assertEquals("leave", events.calls.last())
    }

    @Test
    fun noMenuWhileItLoadsAndAnErrorOffersRetry() {
        show(null)
        compose.onNode(hasText(s(R.string.app_state_loading))).assertExists()
        state = PlayerState(loading = false, error = PlayerError.Network(retrying = false))
        remote.settle()
        remote.settle()
        remote.assertFocused(s(R.string.common_retry))
        remote.ok()
        remote.right()
        remote.ok()
        assertEquals(listOf("retry", "leave"), events.calls)
    }

    @Test
    fun talkBackHearsTheControls() {
        show(bookState(playing = false))
        compose.onNode(hasContentDescription(s(R.string.player_play)) and hasClickAction()).assertExists()
        compose.onNode(hasContentDescription(s(R.string.player_audio_previousChapter)) and hasClickAction()).assertExists()
        compose.onNode(hasContentDescription(s(R.string.player_audio_nextChapter)) and hasClickAction()).assertExists()
        compose.onNode(hasText(s(R.string.app_audio_speedTitle)) and hasClickAction()).assertExists()
    }
}
