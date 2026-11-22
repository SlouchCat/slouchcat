package com.slouchcat.app.mobile.showcase

import android.os.SystemClock
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.performClick
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.audio.AudioData
import com.slouchcat.app.mobile.browse.AudioSeriesRoute
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.ListenRoute
import com.slouchcat.app.mobile.browse.RadioRoute
import com.slouchcat.app.mobile.home.HomeRoute
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.features
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.browse.testing.FakeAudio
import com.slouchcat.core.browse.testing.FakeRadio
import com.slouchcat.core.browse.testing.audioSeries
import com.slouchcat.core.browse.testing.audiobook
import com.slouchcat.core.browse.testing.podcast
import com.slouchcat.core.browse.testing.progress
import com.slouchcat.core.browse.testing.radioStation
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.media.data.DetailRepository
import com.slouchcat.core.player.AudioPlayback
import com.slouchcat.core.player.AudioSource
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.DecoderInfo
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DeviceType
import com.slouchcat.core.player.DisplayInfo
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.player.testing.FakeEngine
import com.slouchcat.core.player.testing.FakeRadioServer
import com.slouchcat.core.player.testing.FakeServer
import com.slouchcat.core.player.testing.audioInfo
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl
import org.junit.After
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Stage 7 with made-up data (visual check against the mockup photos 14, 15, 33 to 39, 60 to 62, 66):
 * the audiobook with chapters and series, the podcast with its episodes, the series, the audio
 * player with its sheets, the small player over the start page, the web radio. No server; the
 * players are fakes that play when told. Photos with `slouchcatShots` as `s…`.
 */
@RunWith(AndroidJUnit4::class)
class AudioShowcaseTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private val book = audiobook(progress = progress(38, minutes = 360))
    private val show = podcast()
    private val details = mapOf<String, ItemDetail>(book.id to book, show.id to show)
    private val engines = mutableListOf<FakeEngine>()

    private val images = ShowcaseImages(
        BrowseShowcaseData.names + PagesShowcaseData.names + mapOf(book.id to book.title, show.id to show.title) +
            show.episodes.orEmpty().associate { it.id to it.title } + audioSeries().volumes.associate { it.item.id to it.item.title },
    ).also { it.squareIds = BrowseShowcaseData.squareIds + setOf(book.id, show.id) + audioSeries().volumes.map { v -> v.item.id } }

    private val facts = DeviceFacts(listOf(DecoderInfo("audio/mp4a-latm")), emptySet(), 2, DisplayInfo(1080, 2400), DeviceType.Phone, "Showcase")

    private fun audio(graph: AppGraph): AudioData {
        val clock = { SystemClock.elapsedRealtime() }
        val titles = FakeServer().apply {
            answer = { id, _ ->
                val detail = details[id] ?: book
                ApiResult.Ok(
                    audioInfo(id, durationMs = detail.durationMs ?: 2_520_000, chapters = detail.chapters.orEmpty(), resumeMs = 7_000_000).let { info ->
                        info.copy(item = info.item.copy(title = detail.title, author = detail.author, images = detail.images, kind = detail.kind))
                    },
                )
            }
        }
        val playback = AudioPlayback(
            scope = graph.scope,
            engines = { FakeEngine(clock).apply { duration = book.durationMs ?: 0 }.also { engines += it } },
            radioServer = { FakeRadioServer() },
            capabilities = { CapabilityRules.build(facts, narrowing = it) },
            settings = MemoryDeviceSettingsStore().settings,
            clock = clock,
        )
        return AudioData(playback, AudioSource("online") { titles })
    }

    private val showcase = Showcase(
        compose, features, BrowseShowcaseData.libraries, images, PagesShowcaseData.auth, PagesShowcaseData.devices,
        audio = ::audio,
        media = { graph ->
            val base = PagesShowcaseData.data(MediaData.api(graph.api, graph.http))
            base.copy(
                detail = object : DetailRepository by base.detail {
                    override suspend fun item(server: HttpUrl, id: String) = details[id]?.let { ApiResult.Ok(it) } ?: base.detail.item(server, id)
                },
                audio = FakeAudio().apply { episodes = show.episodes.orEmpty() },
                radio = FakeRadio().apply {
                    stations = listOf(radioStation(1, "Radio Seeblick"), radioStation(2, "Radio Bergwelt", favorite = true), radioStation(3, "Radio Flussufer", genres = listOf("jazz")), radioStation(4, "Radio Talgrund", genres = listOf("news")))
                    favorites = ApiResult.Ok(listOf(radioStation(2, "Radio Bergwelt", favorite = true)))
                },
            )
        },
    )

    @After
    fun close() = showcase.close()

    private fun open(target: String, vararg routes: Any) =
        InstrumentationRegistry.getInstrumentation().runOnMainSync { showcase.graph.links.request(AppLink(target, routes.toList())) }

    private fun click(text: String) = compose.onAllNodes((hasText(text) or hasContentDescription(text)) and hasClickAction()).onFirst().performClick()

    /** The fake player of what plays starts (it buffers until told). */
    private fun playing() {
        compose.waitUntil(10_000) { engines.isNotEmpty() && engines.last().loads.isNotEmpty() }
        InstrumentationRegistry.getInstrumentation().runOnMainSync { engines.last().ready() }
        compose.waitForIdle()
    }

    @Test
    fun soundPages() {
        showcase.start()
        showcase.waitFor(hasText(s(R.string.home_rows_continue)))

        open("start", ItemRoute(book.id))
        showcase.waitFor(hasText(book.title) and isHeading())
        showcase.shot("s01-book")

        open("start", ItemRoute(show.id))
        showcase.waitFor(hasText(show.title) and isHeading())
        showcase.waitFor(hasText(show.episodes!!.first().title))
        showcase.shot("s02-podcast")

        open("start", AudioSeriesRoute(4, "series-1"))
        showcase.waitFor(hasText("Harbour Tales") and isHeading())
        showcase.shot("s03-series")

        open("start", ListenRoute(book.id))
        playing()
        showcase.waitFor(hasContentDescription(s(R.string.app_audio_close)))
        showcase.shot("s04-listen")
        click(s(R.string.app_audio_sleepTitle))
        showcase.waitFor(hasText(s(R.string.player_audio_sleep_off)))
        showcase.shot("s05-listen-sleep")
        showcase.back()
        click(s(R.string.player_chapters_button))
        showcase.waitFor(hasText("Kapitel 12"))
        showcase.shot("s06-listen-chapters")
        showcase.back()

        // The small player over the start page.
        showcase.back()
        open("start", HomeRoute)
        showcase.waitFor(hasContentDescription(s(R.string.app_audio_stop)))
        showcase.shot("s07-start-mini")

        open("more", RadioRoute)
        showcase.waitFor(hasText(s(R.string.app_radio_title)) and isHeading())
        showcase.waitFor(hasText("Radio Seeblick"))
        showcase.shot("s08-radio")

        open("more", com.slouchcat.app.mobile.browse.AudiobooksRoute)
        showcase.waitFor(hasText("Hörbücher") and isHeading())
        showcase.shot("s09-library-books")
        require(book.kind == ItemKind.AUDIOBOOK)
    }
}
