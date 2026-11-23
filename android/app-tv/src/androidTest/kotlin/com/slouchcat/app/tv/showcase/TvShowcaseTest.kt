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
import com.slouchcat.core.session.RelaySettings
import com.slouchcat.app.tv.navigation.TvNavHost
import com.slouchcat.app.tv.ui.LocalTvImageLoader
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.session.AccountStore
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.LibrariesRepository
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerProbe
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.session.TokenStore
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/** The made-up server: an address of the documentation range, never asked (the pictures come from [ShowcaseImages]). */
val SHOWCASE_SERVER: HttpUrl = "http://192.0.2.10:8096/".toHttpUrl()

/** The profile the showcase is signed in with. */
val SHOWCASE_PROFILE = ActiveProfile(1, "Mara", isPrimary = true, accent = "ember")

private class Servers : ServerStore {
    private val flow = MutableStateFlow<SavedServer?>(SavedServer(SHOWCASE_SERVER, "1.42.0", serverKey = "showcase-key"))
    override val server: Flow<SavedServer?> = flow
    override suspend fun save(server: SavedServer) { flow.value = server }
    override suspend fun confirmKey(serverKey: String) = Unit
    override suspend fun updateVersion(version: String) = Unit
    override suspend fun clear() = Unit
}

private class Tokens : TokenStore {
    override suspend fun read() = StoredCredentials("showcase-token", "showcase-device")
    override suspend fun write(credentials: StoredCredentials) = Unit
}

private class Accounts : AccountStore {
    override suspend fun read(): Pair<String?, ActiveProfile?> = "mara@example.test" to SHOWCASE_PROFILE
    override suspend fun write(email: String?, profile: ActiveProfile?) = Unit
}

private class Relay : RelaySettings {
    override val relay: Flow<HttpUrl> = MutableStateFlow(RelaySettings.DEFAULT)
    override suspend fun set(address: HttpUrl?) = Unit
}

/**
 * The showcase of the TV pages of stage 2b (androidTest only): the real shell and pages with
 * made-up titles and drawn pictures instead of a server, driven with the keys of the remote, to
 * photograph them against the mockup (`mockup-ref/tv`). Photos only with `slouchcatShots`, into
 * the app's files (`shots/c…`), as TvFlowTest does. Invented names, no real films.
 */
@RunWith(AndroidJUnit4::class)
class TvShowcaseTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val context: Context = instrumentation.targetContext
    private val shots = InstrumentationRegistry.getArguments().getString("slouchcatShots") != null
    private lateinit var scenario: ActivityScenario<MainActivity>

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    /** The app over the made-up data; [wishes] brings the wishes and the inbox of stage 5. */
    private fun start(wishes: WishesShowcaseData? = null) {
        val libraries = BrowseShowcaseData.libraries
        val repository = object : LibrariesRepository {
            override suspend fun libraries(server: HttpUrl): ApiResult<List<LibraryNav>> = ApiResult.Ok(libraries)
        }
        lateinit var graph: TvGraph
        instrumentation.runOnMainSync {
            graph = TvGraph(
                context,
                serverStore = Servers(),
                tokenStore = Tokens(),
                accountStore = Accounts(),
                relaySettings = Relay(),
                serverProbe = object : ServerProbe {
                    override suspend fun probe(address: HttpUrl) = ProbeResult.Unreachable
                },
                auth = PagesShowcaseData.auth,
                librariesRepository = repository,
                mediaData = { g ->
                    val base = com.slouchcat.core.media.MediaData.api(g.api, g.http)
                    wishes?.data(base) ?: PagesShowcaseData.data(base)
                },
            )
        }
        val names = BrowseShowcaseData.names + PagesShowcaseData.names + wishes?.names.orEmpty()
        val loader = ShowcaseImages(names).also { it.squareIds = BrowseShowcaseData.squareIds }.loader(context)
        scenario = ActivityScenario.launch(MainActivity::class.java)
        scenario.onActivity { activity ->
            activity.setContent { CompositionLocalProvider(LocalTvImageLoader provides loader) { TvNavHost(graph, onExit = {}) } }
        }
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


    /** A page of «Meine Listen» from the menu «Bibliotheken»: «Alle Bibliotheken …», then down to [label], OK. */
    private fun fromMenu(label: String, shotName: String? = null) {
        rail(s(R.string.tv_shell_allLibraries))
        waitFor(s(R.string.tv_shell_myLists))
        shotName?.let(::shot)
        repeat(RAIL_MAX) {
            if (focusedLabel().contains(label)) {
                key(KeyEvent.KEYCODE_DPAD_CENTER)
                return
            }
            key(KeyEvent.KEYCODE_DPAD_DOWN)
        }
        throw AssertionError("no entry «$label» in the menu")
    }

    /** An entry of the rail: Left into it, to its top, down to [label], OK. */
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
        // The display shows a new page and its pictures a few frames after Compose drew it.
        SystemClock.sleep(SETTLE_MS)
        val bitmap = instrumentation.uiAutomation.takeScreenshot() ?: return
        val dir = File(context.filesDir, "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    @Test
    fun pagesOfStage2b() {
        start()
        val film = BrowseShowcaseData.films[0].title
        // Start: the focus on the stage, then on a row.
        waitFor(film)
        // Stage 3: «Abspielen» leads the stage (mockup 01-start), then «Merken», «Details».
        focused(s(R.string.item_play))
        shot("c01-start-stage")
        key(KeyEvent.KEYCODE_DPAD_RIGHT, 2)
        focused(s(R.string.item_details))

        // A film with its actions, then a person of the cast.
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(s(R.string.tv_item_more))
        shot("c04-film")
        key(KeyEvent.KEYCODE_DPAD_DOWN)
        waitFor("Ilka Morand")
        shot("c04b-film-cast")
        focused("Ilka Morand")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(s(R.string.app_person_knownFrom))
        shot("c06-person")
        key(KeyEvent.KEYCODE_BACK)
        key(KeyEvent.KEYCODE_BACK)
        // Down from the stage: the strip of titles, then the first row (the art layer follows).
        focused(s(R.string.item_details))
        key(KeyEvent.KEYCODE_DPAD_DOWN, 2)
        shot("c02-start-row")

        // The library of films with the genre menu.
        rail(BrowseShowcaseData.libraries[0].name)
        waitFor(s(R.string.library_filters_genre))
        key(KeyEvent.KEYCODE_DPAD_UP)
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        focused(s(R.string.library_filters_genre))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(s(R.string.library_filters_allGenres))
        key(KeyEvent.KEYCODE_DPAD_DOWN)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        key(KeyEvent.KEYCODE_DPAD_DOWN)
        shot("c03-library-filter")

        // A series with its seasons and episodes.
        rail(BrowseShowcaseData.libraries[1].name)
        val series = BrowseShowcaseData.series[0].title
        waitFor(series)
        focused(series)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(s(R.string.item_episodes_title))
        key(KeyEvent.KEYCODE_DPAD_DOWN, 2)
        shot("c05-series-episodes")

        // The search with hits, typed on a keyboard of the TV.
        rail(s(R.string.nav_search))
        focused("q")
        "nacht".forEach { key(KeyEvent.KEYCODE_A + (it - 'a')) }
        waitFor(film)
        shot("c07-search")

        // «Merkliste» and a collection.
        // The rail open, then the menu «Bibliotheken» with «Meine Listen».
        key(KeyEvent.KEYCODE_DPAD_LEFT, LEFT_MAX)
        shot("c11-rail-open")
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        fromMenu(s(R.string.watchlist_title), "c12-libraries-menu")
        waitFor(s(R.string.app_lists_edit))
        shot("c08-watchlist")
        fromMenu(s(R.string.collection_all_title))
        waitFor("Lumen")
        focused("Lumen")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(s(R.string.app_collections_members))
        shot("c09-collection")

        // The profile at the foot of the rail: the choice.
        key(KeyEvent.KEYCODE_DPAD_LEFT, LEFT_MAX)
        key(KeyEvent.KEYCODE_DPAD_DOWN, RAIL_MAX)
        focused(s(R.string.tv_shell_switchProfile))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(s(R.string.profiles_picker_title))
        shot("c10-profiles")
        scenario.close()
    }

    /** Up to the tabs of «Wünsche», then along them to [tab]. */
    private fun toTab(tab: String) {
        val tabs = listOf(R.string.wishes_area_discover, R.string.wishes_area_search, R.string.wishes_area_mine).map(::s)
        repeat(RAIL_MAX) { if (tabs.none { focusedLabel().contains(it) }) key(KeyEvent.KEYCODE_DPAD_UP) }
        repeat(tabs.size) { if (!focusedLabel().contains(tab)) key(KeyEvent.KEYCODE_DPAD_LEFT) }
        repeat(tabs.size) { if (!focusedLabel().contains(tab)) key(KeyEvent.KEYCODE_DPAD_RIGHT) }
        focused(tab)
    }

    /** Down a list or menu until [text] has the focus. */
    private fun down(text: String) {
        repeat(RAIL_MAX) { if (!focusedLabel().contains(text)) key(KeyEvent.KEYCODE_DPAD_DOWN) }
        focused(text)
    }

    /**
     * Stage 5 (mockups tv 32 to 35, 60, 61): «Wünsche» with «Entdecken», «Suche» and «Meine
     * Wünsche», a film with its wish and the question «Zurückziehen», a series with the seasons in
     * the side menu, «Benachrichtigungen» with the announcement, the rail open with the number on
     * «Einstellungen». Photos `c2x`.
     */
    @Test
    fun pagesOfStage5() {
        start(WishesShowcaseData())
        waitFor(BrowseShowcaseData.films[0].title)
        // The rail open: the number of the unread entries and the announcement on «Einstellungen».
        waitFor(context.resources.getQuantityString(R.plurals.notifications_unreadLabel, 3, 3))
        key(KeyEvent.KEYCODE_DPAD_LEFT, LEFT_MAX)
        shot("c29-rail-badge")
        key(KeyEvent.KEYCODE_DPAD_RIGHT)

        rail(s(R.string.nav_wishes))
        waitFor("Gläserne Stadt")
        focused(s(R.string.wishes_area_discover))
        shot("c20-wishes-discover")

        // «Meine Wünsche»: the limit, the filter, three wishes.
        key(KeyEvent.KEYCODE_DPAD_RIGHT, 2)
        focused(s(R.string.wishes_area_mine))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor("Signalfeuer")
        key(KeyEvent.KEYCODE_DPAD_DOWN, 2)
        focused("Gläserne Stadt")
        shot("c21-wishes-mine")

        // The film with its wish (mockup 33), the question «Zurückziehen» (mockup 60).
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(s(R.string.titles_missing).uppercase())
        focused(s(R.string.app_wishes_toMine))
        shot("c22-wish-film")
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(s(R.string.app_wishes_keep))
        shot("c23-wish-withdraw")
        key(KeyEvent.KEYCODE_BACK)
        key(KeyEvent.KEYCODE_BACK)

        // A series without a wish (mockup 34), «Anfordern» with its seasons (mockup 61).
        toTab(s(R.string.wishes_area_discover))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        key(KeyEvent.KEYCODE_DPAD_DOWN, 2)
        focused("Hafenlichter")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(s(R.string.titles_missing).uppercase())
        focused(s(R.string.app_wishes_request))
        shot("c24-wish-series")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(s(R.string.app_wishes_requestNote))
        shot("c25-wish-request-seasons")
        key(KeyEvent.KEYCODE_BACK)
        key(KeyEvent.KEYCODE_BACK)

        // «Suche» with the keyboard of the TV.
        toTab(s(R.string.wishes_area_search))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused("q")
        "licht".forEach { key(KeyEvent.KEYCODE_A + (it - 'a')) }
        waitFor("Hafenlichter")
        shot("c26-wishes-search")

        // «Benachrichtigungen» from «Einstellungen» (mockup 35): the announcement first.
        rail(s(R.string.tv_settings_title))
        waitFor(s(R.string.tv_settings_switchPane))
        down(s(R.string.notifications_title))
        shot("c27-settings-notifications")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor("Wartung am Sonntag")
        focused(s(R.string.announcements_acknowledge))
        shot("c28-notifications")

        // The main search with «Noch nicht in der Bibliothek?» and the card «„…“ wünschen».
        rail(s(R.string.nav_search))
        focused("q")
        // A title only Seerr knows: no rows of the library above it (a lazy list draws only what is seen).
        "eiszeit".forEach { key(KeyEvent.KEYCODE_A + (it - 'a')) }
        waitFor(s(R.string.app_wishes_missingTitle))
        waitFor(s(R.string.app_wishes_searchCard, "eiszeit"))
        shot("c33-search-missing")
        scenario.close()
    }

    /** Stage 5, the quiet side: no wish yet, an empty inbox (photos `c3x`). */
    @Test
    fun emptyPagesOfStage5() {
        start(WishesShowcaseData(empty = true))
        waitFor(BrowseShowcaseData.films[0].title)
        rail(s(R.string.nav_wishes))
        waitFor("Gläserne Stadt")
        key(KeyEvent.KEYCODE_DPAD_RIGHT, 2)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(s(R.string.wishes_page_empty_title))
        shot("c30-wishes-empty")
        rail(s(R.string.tv_settings_title))
        waitFor(s(R.string.tv_settings_switchPane))
        down(s(R.string.notifications_title))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(s(R.string.notifications_empty_title))
        focused(s(R.string.app_lists_toStart))
        shot("c31-notifications-empty")
        scenario.close()
    }

    /** Seerr not set up: the page says so, as the web app does (photo `c32`). */
    @Test
    fun wishesOffOfStage5() {
        start(WishesShowcaseData(off = true))
        waitFor(BrowseShowcaseData.films[0].title)
        rail(s(R.string.nav_wishes))
        waitFor(s(R.string.app_wishes_off_title))
        focused(s(R.string.app_lists_toStart))
        shot("c32-wishes-off")
        scenario.close()
    }

    private companion object {
        const val TIMEOUT = 20_000L
        const val SETTLE_MS = 700L
        const val RAIL_MAX = 14
        const val LEFT_MAX = 8
    }
}

