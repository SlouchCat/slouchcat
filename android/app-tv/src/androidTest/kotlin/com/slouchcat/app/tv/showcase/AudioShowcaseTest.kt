package com.slouchcat.app.tv.showcase

import android.content.Context
import android.graphics.Bitmap
import android.os.SystemClock
import android.view.KeyEvent
import androidx.activity.compose.setContent
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isFocused
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.tv.MainActivity
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.audio.AudioFeature
import com.slouchcat.app.tv.navigation.TvNavHost
import com.slouchcat.app.tv.player.TvFactsReader
import com.slouchcat.app.tv.ui.LocalTvImageLoader
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.AudioSeries
import com.slouchcat.core.api.generated.models.AudioSeriesVolume
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.HomePage
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemImages
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.LibraryFacets
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.api.generated.models.Progress
import com.slouchcat.core.api.generated.models.RadioLogo
import com.slouchcat.core.api.generated.models.RadioNow
import com.slouchcat.core.api.generated.models.RadioNowEntry
import com.slouchcat.core.api.generated.models.RadioStation
import com.slouchcat.core.api.generated.models.RootContent
import com.slouchcat.core.browse.testing.FakeDetails
import com.slouchcat.core.browse.testing.FakeRadio
import com.slouchcat.core.browse.testing.mediaFile
import com.slouchcat.core.media.data.AudioRepository
import com.slouchcat.core.media.data.HomeRepository
import com.slouchcat.core.media.Page
import com.slouchcat.core.media.data.LibraryPage
import com.slouchcat.core.media.data.LibraryQuery
import com.slouchcat.core.media.data.LibraryRepository
import com.slouchcat.core.player.AudioPlayback
import com.slouchcat.core.player.AudioSource
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.player.testing.FakeEngine
import com.slouchcat.core.player.testing.FakeRadioServer
import com.slouchcat.core.player.testing.FakeServer
import com.slouchcat.core.player.testing.audioInfo
import com.slouchcat.core.session.LibrariesRepository
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.ServerProbe
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File
import java.math.BigDecimal
import java.time.OffsetDateTime

/**
 * Made-up titles of stage 4 for the showcase (invented names, drawn covers, no real books, podcasts
 * or stations): eight audiobooks, a book in a series with sixteen chapters and a third heard, a
 * podcast with episodes heard, begun and new, stations of three countries with their logos.
 */
private object AudioShowcaseData {
    private var n = 0
    private fun id(): String = "01JSOUND000000000000000" + (++n).toString().padStart(3, '0')

    private fun images(id: String) = ItemImages(
        posterSmall = BrowseShowcaseData.image(id, "poster", "w200", 200, 200),
        poster = BrowseShowcaseData.image(id, "poster", "w400", 400, 400),
        posterLarge = BrowseShowcaseData.image(id, "poster", "w800", 800, 800),
    )

    private fun ref(title: String, kind: ItemKind, author: String?, year: Int? = null, minutes: Int? = null) = id().let { id ->
        ItemRef(id, kind, title, inWatchlist = false, images = images(id), year = year, author = author, runtimeMinutes = minutes)
    }

    val books = listOf(
        ref("Das Salz der Gezeiten", ItemKind.AUDIOBOOK, "Liv Andersen", 2023, 964),
        ref("Die Vermessung der Stille", ItemKind.AUDIOBOOK, "Jon Halvorsen", 2022, 456),
        ref("Gezeiten I: Ebbe", ItemKind.AUDIOBOOK, "Liv Andersen", 2021, 812),
        ref("Gezeiten III: Flut", ItemKind.AUDIOBOOK, "Liv Andersen", 2025, 890),
        ref("Der Uhrmacher von Brenn", ItemKind.AUDIOBOOK, "Mira Talin", 2020, 702),
        ref("Kleine Philosophie des Wartens", ItemKind.AUDIOBOOK, "Ada Kessler", 2024, 318),
        ref("Zwölf Nächte am Fjord", ItemKind.AUDIOBOOK, "Jon Halvorsen", 2019, 588),
        ref("Atlas der leisen Orte", ItemKind.AUDIOBOOK, "Noor Ellis", 2026, 402),
    )
    val book = books[0]

    /** Sixteen chapters of 30 to 75 minutes. */
    val chapters: List<Chapter> = run {
        var start = 0L
        (1..16).map { i ->
            val length = (30 + (i * 37) % 46) * 60_000L
            Chapter(startMs = start, endMs = start + length, title = "Kapitel $i").also { start += length }
        }
    }
    private val length = chapters.last().endMs
    private val position = chapters[6].startMs + 14 * 60_000L + 31_000L

    val bookDetail = ItemDetail(
        id = book.id, kind = ItemKind.AUDIOBOOK, title = book.title, inWatchlist = true, images = book.images,
        year = 2023, author = "Liv Andersen", narrator = "Paul Brandt", durationMs = length, runtimeMinutes = (length / 60_000).toInt(),
        progress = Progress(position, length, BigDecimal(38), completed = false, updatedAt = OffsetDateTime.parse("2026-11-20T20:00:00Z")),
        chapters = chapters, seriesId = "gezeiten", seriesName = "Gezeiten", seriesIndex = BigDecimal(2), libraryId = 3,
        mediaFiles = listOf(mediaFile("f-${book.id}")), genres = emptyList(),
        recommendedByMe = false, canRecommend = false, collections = emptyList(),
    )

    val series = AudioSeries(
        id = "gezeiten", libraryId = 3, name = "Gezeiten", durationMs = (812 + 964 + 890) * 60_000L,
        volumes = listOf(books[2], book, books[3]).mapIndexed { i, v -> AudioSeriesVolume(v.copy(durationMs = (v.runtimeMinutes ?: 0) * 60_000L, progress = if (i == 0) Progress(1, 1, BigDecimal(100), true, OffsetDateTime.parse("2026-10-01T20:00:00Z"), watchedAt = OffsetDateTime.parse("2026-10-01T20:00:00Z")) else null), BigDecimal(i + 1), "Paul Brandt") },
        spellings = emptyList(), author = "Liv Andersen", narrator = "Paul Brandt", cover = book, next = book,
    )

    val podcast = ref("Nachtschicht", ItemKind.PODCAST, "Studio Nord", 2026)
    private val titles = listOf(
        "Die Stimme aus dem Funkturm", "Ein Sommer ohne Strom", "Was Archive verschweigen", "Die letzte Fähre",
        "Hundert Briefe an niemand", "Der Fall der grünen Tür", "Spuren im Schnee", "Das Echo der Halle",
    )
    val episodes: List<ItemRef> = titles.mapIndexed { i, title ->
        val minutes = 32 + (i * 7) % 18
        val heard = i >= 3
        val progress = when {
            heard -> Progress(minutes * 60_000L, minutes * 60_000L, BigDecimal(100), true, OffsetDateTime.parse("2026-11-01T20:00:00Z"), watchedAt = OffsetDateTime.parse("2026-11-01T20:00:00Z"))
            i == 1 -> Progress(20 * 60_000L, minutes * 60_000L, BigDecimal(40), false, OffsetDateTime.parse("2026-11-19T20:00:00Z"))
            else -> null
        }
        ItemRef(id(), ItemKind.PODCAST_EPISODE, title, inWatchlist = false, images = podcast.images, durationMs = minutes * 60_000L, runtimeMinutes = minutes,
            publishedAt = OffsetDateTime.parse("2026-11-17T06:00:00Z").minusDays(i * 7L), parentId = podcast.id, seriesTitle = podcast.title, progress = progress)
    }
    val podcastDetail = ItemDetail(
        id = podcast.id, kind = ItemKind.PODCAST, title = podcast.title, inWatchlist = false, images = podcast.images, author = "Studio Nord",
        episodes = episodes, episodeCount = episodes.size, mediaFiles = emptyList(), genres = emptyList(), recommendedByMe = false, canRecommend = false, collections = emptyList(),
    )
    val podcasts = listOf(podcast, ref("Klangwerkstatt", ItemKind.PODCAST, "Kollektiv Hall", 2025), ref("Zeitreisen", ItemKind.PODCAST, "Studio Nord", 2024))

    private fun station(id: Long, name: String, country: String, genres: List<String>, favorite: Boolean = false) = RadioStation(
        id = id, name = name, languages = listOf("de"), genres = genres, kinds = emptyList(), moods = emptyList(), hls = false, showsTitles = true,
        favorite = favorite, recommended = false, state = com.slouchcat.core.api.generated.models.RadioStationState.OK, country = country,
        logo = RadioLogo("/api/v1/radio/station-$id/logo"),
    )
    val stations = listOf(
        station(1, "Welle Seeblick", "CH", listOf("pop", "rock"), favorite = true),
        station(2, "Radio Bergtal", "CH", listOf("pop")),
        station(3, "Klassik am Morgen", "CH", listOf("classical"), favorite = true),
        station(4, "Funkhaus Nord", "DE", listOf("news")),
        station(5, "Donauwelle", "AT", listOf("schlager")),
        station(6, "Jazzkeller FM", "CH", listOf("jazz")),
    )

    val names: Map<String, String> = (books + podcasts).associate { it.id to it.title } + stations.associate { "station-${it.id}" to it.name }
    val squareIds: Set<String> = names.keys

    val libraries = listOf(
        LibraryNav(1, "Filme", LibraryKind.VIDEO, listOf(RootContent.MOVIE), 9),
        LibraryNav(2, "Serien", LibraryKind.VIDEO, listOf(RootContent.SERIES), 3),
        LibraryNav(3, "Hörbücher", LibraryKind.AUDIOBOOKS, listOf(RootContent.AUDIOBOOK), books.size),
        LibraryNav(4, "Podcasts", LibraryKind.PODCASTS, listOf(RootContent.PODCAST), podcasts.size),
    )

    fun data(base: com.slouchcat.core.media.MediaData): com.slouchcat.core.media.MediaData = base.copy(
        home = object : HomeRepository {
            override suspend fun home(server: HttpUrl) = ApiResult.Ok(HomePage(emptyList()))
            override suspend fun homeRow(server: HttpUrl, key: com.slouchcat.core.api.generated.models.HomeListKey, offset: Int, limit: Int) = ApiResult.Ok(Page(emptyList<ItemRef>(), 0))
            override suspend fun picks(server: HttpUrl, offset: Int, limit: Int) = ApiResult.Ok(Page(emptyList<ItemRef>(), 0))
        },
        library = object : LibraryRepository {
            override suspend fun items(server: HttpUrl, library: Long, query: LibraryQuery, offset: Int, limit: Int): ApiResult<LibraryPage> {
                val list = if (library == 4L) podcasts else books
                return ApiResult.Ok(LibraryPage(list.drop(offset).take(limit), list.size, LibraryFacets(letters = emptyList(), genres = emptyList(), ratings = emptyList())))
            }
            override suspend fun genres(server: HttpUrl) = ApiResult.Ok(emptyList<com.slouchcat.core.api.generated.models.GenreRef>())
        },
        detail = FakeDetails(bookDetail, podcastDetail),
        audio = object : AudioRepository {
            override suspend fun series(server: HttpUrl, libraryId: Long, seriesId: String) = ApiResult.Ok(series)
            override suspend fun episodes(server: HttpUrl, podcastId: String, offset: Int, limit: Int) = ApiResult.Ok(Page(episodes.drop(offset).take(limit), episodes.size))
        },
        radio = FakeRadio().also { radio ->
            radio.stations = stations
            radio.favorites = ApiResult.Ok(stations.filter { it.favorite })
            radio.now = ApiResult.Ok(RadioNow(true, listOf(RadioNowEntry(1, "Die Seeländer – Abendlicht", "Die Seeländer", "Abendlicht"))))
        },
    )

    /** The sound without a server: players that report playing, the book and the episodes as the server would describe them. */
    fun audio(context: Context): AudioFeature {
        val titles = FakeServer().apply {
            answer = { id, request ->
                val episode = episodes.firstOrNull { it.id == id }
                val info = audioInfo(id, resumeMs = request.positionMs ?: position, durationMs = episode?.durationMs ?: length, chapters = if (episode == null) chapters else emptyList())
                ApiResult.Ok(info.copy(item = episode?.let { it.copy(seriesTitle = podcast.title) } ?: book.copy(author = "Liv Andersen", durationMs = length)))
            }
        }
        val radio = FakeRadioServer().apply {
            startAnswer = { id -> ApiResult.Ok(listen(id, heartbeatSeconds).copy(station = stations.first { it.id == id })) }
            nowAnswer = ApiResult.Ok(RadioNow(true, listOf(RadioNowEntry(1, "Die Seeländer – Abendlicht", "Die Seeländer", "Abendlicht"))))
        }
        return AudioFeature(
            playback = { _, _, scope ->
                AudioPlayback(
                    scope = scope,
                    engines = { FakeEngine(SystemClock::elapsedRealtime).also { engine -> engine.duration = length; engines += engine } },
                    radioServer = { radio },
                    capabilities = { CapabilityRules.build(TvFactsReader(context) { "Showcase" }.read(), narrowing = it) },
                    settings = MemoryDeviceSettingsStore().settings,
                    clock = SystemClock::elapsedRealtime,
                )
            },
            source = { AudioSource("online") { titles } },
        )
    }

    /** The players made, to report them ready once their media is loaded. */
    val engines = mutableListOf<FakeEngine>()
}

/**
 * The showcase of the sound of stage 4 (androidTest only, as [TvShowcaseTest]): the real shell and
 * pages with the made-up titles of [AudioShowcaseData] and players without sound, driven with the
 * keys of the remote, photographed for the comparison with `mockup-ref/tv` (photos 08, 25 to 31).
 * Photos only with `slouchcatShots`, into the app's files (`shots/a…`).
 */
@RunWith(AndroidJUnit4::class)
class AudioShowcaseTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val context: Context = instrumentation.targetContext
    private val shots = InstrumentationRegistry.getArguments().getString("slouchcatShots") != null
    private lateinit var scenario: ActivityScenario<MainActivity>

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private fun start() {
        val libraries = AudioShowcaseData.libraries
        val repository = object : LibrariesRepository {
            override suspend fun libraries(server: HttpUrl): ApiResult<List<LibraryNav>> = ApiResult.Ok(libraries)
        }
        val audio = AudioShowcaseData.audio(context)
        lateinit var graph: TvGraph
        instrumentation.runOnMainSync {
            graph = TvGraph(
                context,
                features = com.slouchcat.app.tv.shell.features.filterNot { it is AudioFeature } + audio,
                serverStore = Servers(),
                tokenStore = Tokens(),
                accountStore = Accounts(),
                relaySettings = Relay(),
                serverProbe = object : ServerProbe {
                    override suspend fun probe(address: HttpUrl) = ProbeResult.Unreachable
                },
                auth = PagesShowcaseData.auth,
                librariesRepository = repository,
                mediaData = { g -> AudioShowcaseData.data(com.slouchcat.core.media.MediaData.api(g.api, g.http)) },
            )
        }
        val loader = ShowcaseImages(AudioShowcaseData.names).also { it.squareIds = AudioShowcaseData.squareIds }.loader(context)
        scenario = ActivityScenario.launch(MainActivity::class.java)
        scenario.onActivity { activity ->
            activity.setContent { CompositionLocalProvider(LocalTvImageLoader provides loader) { TvNavHost(graph, onExit = {}) } }
        }
    }

    /** The player made last reports its media loaded and playing (the attrappe needs the word). */
    private fun playing() {
        compose.waitUntil(TIMEOUT) { AudioShowcaseData.engines.isNotEmpty() }
        instrumentation.runOnMainSync { AudioShowcaseData.engines.last().ready() }
        compose.waitForIdle()
    }

    private fun key(code: Int, times: Int = 1) = repeat(times) {
        instrumentation.sendKeyDownUpSync(code)
        compose.waitForIdle()
    }

    private fun waitFor(text: String) = compose.waitUntil(TIMEOUT) {
        compose.onAllNodes(hasText(text, substring = true) or hasContentDescription(text, substring = true), useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    }

    private fun focusedLabel(): String = compose.onAllNodes(isFocused()).fetchSemanticsNodes().joinToString { node ->
        (node.config.getOrNull(SemanticsProperties.ContentDescription).orEmpty() + node.config.getOrNull(SemanticsProperties.Text).orEmpty().map { it.text }).joinToString(" ")
    }

    private fun focused(text: String) = try {
        compose.waitUntil(TIMEOUT) { focusedLabel().contains(text) }
    } catch (e: androidx.compose.ui.test.ComposeTimeoutException) {
        throw AssertionError("«$text» should have the focus, it is on «${focusedLabel()}»", e)
    }

    private fun rail(label: String) {
        key(KeyEvent.KEYCODE_DPAD_LEFT, LEFT_MAX)
        key(KeyEvent.KEYCODE_DPAD_UP, RAIL_MAX)
        repeat(RAIL_MAX) {
            if (focusedLabel().contains(label)) {
                key(KeyEvent.KEYCODE_DPAD_CENTER)
                return
            }
            key(KeyEvent.KEYCODE_DPAD_DOWN)
        }
        throw AssertionError("no entry «$label» in the rail")
    }

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        instrumentation.waitForIdleSync()
        SystemClock.sleep(SETTLE_MS)
        val bitmap = instrumentation.uiAutomation.takeScreenshot() ?: return
        val dir = File(context.filesDir, "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    @Test
    fun pagesOfStage4() {
        start()
        // The library of audiobooks: square covers (mockup 08).
        rail("Hörbücher")
        waitFor(AudioShowcaseData.books[1].title)
        shot("a01-library-books")

        // A book with its chapters, a third heard (mockup 25).
        focused(AudioShowcaseData.book.title)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(s(R.string.item_audio_chapters))
        focused(s(R.string.item_action_resumeListening))
        shot("a02-book")

        // Its series.
        key(KeyEvent.KEYCODE_DPAD_DOWN, 2)
        focused("Gezeiten")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(s(R.string.audioSeries_list))
        shot("a03-series")
        key(KeyEvent.KEYCODE_BACK)

        // The audio player of the book (mockup 29), its menus «Tempo» and «Kapitel» (58, 59).
        waitFor(s(R.string.item_audio_chapters))
        key(KeyEvent.KEYCODE_DPAD_UP, 2)
        focused(s(R.string.item_action_resumeListening))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        playing()
        focused(s(R.string.player_pause))
        waitFor("Kapitel 7")
        shot("a04-listen-book")
        key(KeyEvent.KEYCODE_DPAD_DOWN)
        focused(s(R.string.app_audio_speedTitle))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        shot("a05-listen-speed")
        key(KeyEvent.KEYCODE_BACK)
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        focused(s(R.string.player_chapters_button))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        shot("a06-listen-chapters")
        key(KeyEvent.KEYCODE_BACK)

        // Back: the book plays on, «Läuft gerade» at the top right and in the rail.
        key(KeyEvent.KEYCODE_BACK)
        waitFor(s(R.string.tv_audio_nowPlaying))
        shot("a07-now-playing")

        // A podcast with its episodes (mockup 27), an episode in the player (30).
        rail("Podcasts")
        waitFor(AudioShowcaseData.podcast.title)
        focused(AudioShowcaseData.podcast.title)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(AudioShowcaseData.episodes[0].title)
        shot("a08-podcast")
        focused(s(R.string.app_audio_newest))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        playing()
        waitFor(AudioShowcaseData.episodes[0].title)
        shot("a09-listen-episode")
        key(KeyEvent.KEYCODE_BACK)

        // «Webradio»: «Meine Sender», the stations, the countries (mockup 31 as meant).
        rail(s(R.string.app_radio_title))
        waitFor(AudioShowcaseData.stations[1].name)
        shot("a10-radio")
        key(KeyEvent.KEYCODE_DPAD_UP)
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        focused(s(R.string.radio_page_quick_myCountry, "").trim())
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        shot("a11-radio-countries")
        key(KeyEvent.KEYCODE_BACK)
        // A station plays: «Läuft» on its tile, «Jetzt läuft» above.
        key(KeyEvent.KEYCODE_DPAD_DOWN)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        playing()
        waitFor(s(R.string.radio_player_nowPlaying))
        shot("a12-radio-playing")
        key(KeyEvent.KEYCODE_DPAD_UP, 2)
        focused(s(R.string.radio_player_nowPlaying))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(s(R.string.radio_player_pause))
        waitFor("Abendlicht")
        shot("a13-radio-listen")
        scenario.close()
    }

    private companion object {
        const val TIMEOUT = 20_000L
        const val SETTLE_MS = 900L
        const val RAIL_MAX = 14
        const val LEFT_MAX = 8
    }
}
