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
import com.slouchcat.app.tv.common.RelaySettings
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

    private fun start() {
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
                mediaData = { g -> PagesShowcaseData.data(com.slouchcat.core.media.MediaData.api(g.api, g.http)) },
            )
        }
        val loader = ShowcaseImages(BrowseShowcaseData.names + PagesShowcaseData.names).also { it.squareIds = BrowseShowcaseData.squareIds }.loader(context)
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
        rail(s(R.string.watchlist_title))
        waitFor(s(R.string.app_lists_edit))
        shot("c08-watchlist")
        rail(s(R.string.collection_all_title))
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

    private companion object {
        const val TIMEOUT = 20_000L
        const val SETTLE_MS = 700L
        const val RAIL_MAX = 14
        const val LEFT_MAX = 8
    }
}

