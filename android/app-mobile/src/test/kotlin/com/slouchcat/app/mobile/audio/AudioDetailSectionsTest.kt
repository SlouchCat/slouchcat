package com.slouchcat.app.mobile.audio

import android.content.Context
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.core.browse.testing.audiobook
import com.slouchcat.core.browse.testing.podcast
import com.slouchcat.core.browse.testing.progress
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.Paged
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

/** The audio parts of a detail page: chapters with their count and length, the series, the episodes with play and «gehört». */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class AudioDetailSectionsTest {
    @get:Rule
    val compose = createComposeRule()

    private val calls = mutableListOf<String>()

    private val links = AudioDetailLinks(
        playAt = { id, at -> calls += "play:$id@$at" },
        play = { calls += "play:$it" },
        item = { id -> { calls += "open:$id" } },
        series = { library, series -> calls += "series:$library/$series" },
    )

    private fun book() {
        val book = audiobook(progress = progress(30, minutes = 360))
        compose.setContent {
            SlouchcatTheme { Column(Modifier.verticalScroll(rememberScrollState())) { AudioDetailSections(book, links, null) } }
        }
        compose.onNode(hasText(s(R.string.item_audio_chapters))).assertExists()
        compose.onAllNodes(hasText("Kapitel 4") and hasClickAction()).onFirst().performClick()
        compose.onAllNodes(hasText("Harbour Tales", substring = true) and hasClickAction()).onFirst().performClick()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        assertEquals(listOf("play:${book.id}@5400000", "series:4/series-1"), calls)
    }

    @Test @Config(qualifiers = PHONE)
    fun bookPhone() = book()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun bookTabletLargeFont() = book()

    private fun episodes() {
        val show = podcast()
        val ui = EpisodesUi(Paged(Loadable.Ready(show.episodes!!), show.episodes!!.size), emptySet(), {}, {}, { calls += "heard:${it.id}" })
        compose.setContent {
            SlouchcatTheme { Column(Modifier.verticalScroll(rememberScrollState())) { AudioDetailSections(show, links, ui) } }
        }
        val first = show.episodes!!.first()
        compose.onNode(hasContentDescription(s(R.string.item_podcast_play, first.title)) and hasClickAction()).performClick()
        compose.onNode(hasContentDescription(s(R.string.app_audio_episodeHeard, first.title))).performClick()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        assertTrue(calls.containsAll(listOf("play:${first.id}", "heard:${first.id}")))
    }

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun episodesPhoneLargeFont() = episodes()

    @Test @Config(qualifiers = TABLET)
    fun episodesTablet() = episodes()

    /** Code review M-4: more episodes only on request, never because a page arrived. */
    @Test @Config(qualifiers = PHONE)
    fun moreEpisodesLoadOnlyOnRequest() {
        val show = podcast()
        var asked = 0
        val list = show.episodes!!
        var paged by androidx.compose.runtime.mutableStateOf(Paged(Loadable.Ready(list.take(1)), list.size + 50))
        compose.setContent {
            val ui = EpisodesUi(paged, emptySet(), { asked++ }, {}, {})
            SlouchcatTheme { Column(Modifier.verticalScroll(rememberScrollState())) { AudioDetailSections(show, links, ui) } }
        }
        compose.waitForIdle()
        paged = Paged(Loadable.Ready(list), list.size + 50)
        compose.waitForIdle()
        assertEquals(0, asked)
        compose.onNode(hasText(s(R.string.app_audio_loadMore)) and hasClickAction()).performScrollTo().performClick()
        assertEquals(1, asked)
    }
}
