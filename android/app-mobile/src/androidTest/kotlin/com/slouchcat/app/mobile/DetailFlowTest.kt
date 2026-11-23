package com.slouchcat.app.mobile

import android.graphics.Bitmap
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.apis.DiscoverApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.models.ItemWatchedRequest
import com.slouchcat.core.api.generated.models.SetupRequest
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import okhttp3.Cookie
import okhttp3.CookieJar
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.junit.After
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/**
 * The detail pages against a real server (task android:e2e, concept 27 stage 4): the script
 * writes a small media tree (scripts/android-test-media.sh) and names it as `slouchcatMedia`; the
 * test adds it as a library, signs the app in if it is not, and opens the pages through the
 * app's own jump (`graph.links.request("item/<id>")`, the start page is another branch's): film
 * with Merken, Gesehen, Empfehlen and the file, a person, the series with a season change, an
 * episode, the question before clearing a season, a title that is not there. With
 * `slouchcatShots` it photographs each page and sheet (prefix `b`).
 */
@RunWith(AndroidJUnit4::class)
class DetailFlowTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val args = InstrumentationRegistry.getArguments()
    private val server: HttpUrl = (args.getString("slouchcatServer") ?: "http://10.0.2.2:8597").toHttpUrl()
    private val media = args.getString("slouchcatMedia")
    private val email = args.getString("slouchcatEmail") ?: "e2e-owner@example.test"
    private val password = args.getString("slouchcatPassword") ?: "e2e-password-0123"
    private val pin = args.getString("slouchcatPin") ?: "246810"
    private val shots = args.getString("slouchcatShots") != null
    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, BuildConfig.VERSION_NAME)

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = 20_000) = compose.waitForNode(matcher, timeout)

    private fun waitForText(text: String, timeout: Long = 20_000) = waitFor(hasText(text, substring = true), timeout)

    private fun shown(matcher: SemanticsMatcher) = compose.isShown(matcher, unmerged = true)

    private fun button(text: String) = compose.onAllNodes((hasText(text) or hasContentDescription(text)) and hasClickAction()).onLast()

    private fun field(label: String) = compose.onNode(hasSetTextAction() and hasText(label))

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        InstrumentationRegistry.getInstrumentation().waitForIdleSync()
        compose.mainClock.advanceTimeBy(FRAME_MS)
        compose.waitForIdle()
        val bitmap = InstrumentationRegistry.getInstrumentation().uiAutomation.takeScreenshot() ?: return
        val dir = File(context.getExternalFilesDir(null), "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private var resumed: MainActivity? = null

    private fun back() = InstrumentationRegistry.getInstrumentation().runOnMainSync { resumed?.onBackPressedDispatcher?.onBackPressed() }

    private val graph: AppGraph get() = (context.applicationContext as SlouchcatApplication).graph

    private fun jump(address: String) = InstrumentationRegistry.getInstrumentation().runOnMainSync { assertTrue(graph.links.request(address)) }

    /** The ids the flow opens, read as the primary profile. */
    private class Titles(val film: String, val series: String, val seasons: List<String>)

    private fun prepareServer(): Titles {
        val setup = SlouchcatApi.create(::SetupApi, server, SlouchcatHttp.client(identity))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = email, password = password))
        val admin = Admin(server, email, password, pin)
        admin.ensureLibrary(requireNotNull(media) { "run through scripts/android-e2e.sh, which writes the media" })
        // Read and reset through the shared web session of the primary profile: no further sign-in
        // (the server allows 10 a minute and address, and the app's own sign-ins of the flows count too).
        val http = admin.http
        val items = SlouchcatApi.create(::ItemsApi, server, http)
        val library = items.listVisibleLibraries()!!.single { it.name == LIBRARY }
        val titles = items.listLibraryItems(library.id)!!.items
        val series = titles.single { it.title == "Quiet Ring" }.id
        val seasons = items.getItem(series)!!.seasons!!.map { it.title }
        val film = titles.single { it.title == "Night Train" }.id
        // The titles as a fresh library has them, whichever flow ran before on this server: not on the
        // watchlist, nothing watched, no pick of the profile (the flow sets each of them).
        val discover = SlouchcatApi.create(::DiscoverApi, server, http)
        runCatching { discover.removeFromWatchlist(film) }
        runCatching { discover.unrecommendItem(film) }
        items.setItemWatched(film, ItemWatchedRequest(false))
        items.setItemWatched(series, ItemWatchedRequest(false))
        return Titles(film, series, seasons)
    }

    /** From wherever the app stands (address, sign-in, profile choice) to a chosen profile. */
    private fun signIn() {
        compose.waitUntil(20_000) {
            ready() || shown(hasText(s(R.string.app_setup_title))) || shown(hasText(s(R.string.login_title)) and isHeading())
        }
        if (ready()) return
        if (shown(hasText(s(R.string.app_setup_title)))) {
            field(s(R.string.app_setup_addressLabel)).performTextInput(server.toString().trimEnd('/'))
            button(s(R.string.app_setup_connect)).performScrollTo().performClick()
            waitForText(s(R.string.app_setup_found))
            button(s(R.string.app_setup_next)).performScrollTo().performClick()
            waitFor(hasText(s(R.string.login_title)) and isHeading())
        }
        field(s(R.string.login_email)).performTextInput(email)
        field(s(R.string.login_password)).performTextInput(password)
        button(s(R.string.login_submit)).performScrollTo().performClick()
        compose.waitUntil(20_000) { ready() || shown(hasContentDescription(s(R.string.profiles_picker_primary), substring = true) and hasClickAction()) }
        if (!ready()) {
            compose.onAllNodes(hasContentDescription(s(R.string.profiles_picker_primary), substring = true) and hasClickAction()).onLast().performClick()
            compose.waitUntil(20_000) { ready() || shown(hasText(s(R.string.profiles_pin_hint))) }
            if (!ready()) {
                pin.forEach { button(it.toString()).performClick() }
                compose.onAllNodes(hasContentDescription(s(R.string.profiles_pin_submit))).onLast().performClick()
            }
        }
        compose.waitUntil(20_000) { ready() }
    }

    private fun ready(): Boolean = (graph.session.state.value as? SessionState.Ready)?.profile != null

    /**
     * Leaves the app as a fresh install: signed out and without a server, so SignInFlowTest (whose
     * order against this test is open) starts at the address.
     */
    @After
    fun forgetTheServer() = runBlocking {
        if (graph.session.state.value is SessionState.Ready) graph.sessionActions.signOut()
        graph.serverStore.clear()
    }

    @Test
    fun filmPersonSeriesEpisode() {
        val titles = prepareServer()
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        scenario.onActivity { resumed = it }
        signIn()

        // The film: Merken, Gesehen, Empfehlen with its sheet, the file.
        jump("item/${titles.film}")
        waitFor(hasText("Night Train") and isHeading())
        shot("b01-film")
        button(s(R.string.item_watchlist_add)).performScrollTo().performClick()
        waitFor(hasText(s(R.string.item_watchlist_added)))
        button(s(R.string.watched_button)).performScrollTo().performClick()
        button(s(R.string.item_recommend_add)).performScrollTo().performClick()
        waitForText(s(R.string.item_recommend_hint))
        shot("b02-film-recommend-sheet")
        button(s(R.string.app_item_recommendConfirm)).performClick()
        waitFor(hasText(s(R.string.item_recommend_done)))
        button(s(R.string.app_item_tech)).performScrollTo().performClick()
        waitForText(s(R.string.app_item_techVersions))
        shot("b03-film-actions-tech")

        // A person the server does not know (no cast without TMDB): the calm «not found», back.
        jump("person/987654")
        waitForText(s(R.string.person_notFound_title))
        shot("b04-person-not-found")
        back()

        // The series: the second season, its episode, the episode's page, back.
        jump("item/${titles.series}")
        waitFor(hasText("Quiet Ring") and isHeading())
        waitForText(s(R.string.app_item_episodeTitle, "1", "Arrival"))
        shot("b05-series")
        button(titles.seasons[1]).performScrollTo().performClick()
        val returned = s(R.string.app_item_episodeTitle, "1", "Return")
        waitForText(returned)
        shot("b06-series-season-2")
        // The row's own action: a tap at its centre may land on the back arrow over the page's top.
        compose.onAllNodes(hasText(returned) and hasClickAction()).onLast().performScrollTo().performSemanticsAction(SemanticsActions.OnClick)
        waitFor(hasText("Return") and isHeading())
        shot("b07-episode")
        back()
        waitFor(hasText("Quiet Ring") and isHeading())

        // The season as watched, then the question before its checks go.
        button(s(R.string.app_item_seasonWatched)).performScrollTo().performClick()
        waitForText(s(R.string.app_item_seasonUnwatched))
        button(s(R.string.app_item_seasonUnwatched)).performScrollTo().performClick()
        waitForText(s(R.string.watched_ask_removeAll))
        shot("b08-season-watched-sheet")
        button(s(R.string.watched_ask_removeAll)).performClick()
        waitForText(s(R.string.app_item_seasonWatched))

        // A title that is not there (or not for this profile): the calm state, no retry.
        jump("item/01J0000000000000000000ZZZZ")
        waitForText(s(R.string.item_notFound_title))
        shot("b09-item-not-found")
        assertTrue(compose.onAllNodes(hasText(s(R.string.app_state_retry))).fetchSemanticsNodes().isEmpty())
        scenario.close()
    }

    /** The administration as a browser uses it, only to add the test library. */
    internal class Admin(private val base: HttpUrl, private val email: String, private val password: String, private val pin: String) {
        /**
         * The web session of the account, shared by every flow of one run (one process): each sign-in
         * and profile choice counts against the server's limit of 10 attempts a minute per address,
         * which the app's own sign-ins of the flows need.
         */
        private val cookies: MutableList<Cookie> = synchronized(SESSIONS) { SESSIONS.getOrPut("$base|$email") { mutableListOf() } }

        /** The client of that web session, also for the generated API (the header the server asks of a browser). */
        val http: OkHttpClient = OkHttpClient.Builder()
            .cookieJar(object : CookieJar {
                override fun saveFromResponse(url: HttpUrl, cookies: List<Cookie>) {
                    synchronized(this@Admin.cookies) {
                        this@Admin.cookies.removeAll { old -> cookies.any { it.name == old.name } }
                        this@Admin.cookies += cookies
                    }
                }

                override fun loadForRequest(url: HttpUrl) = synchronized(cookies) { cookies.toList() }
            })
            .addInterceptor { chain -> chain.proceed(chain.request().newBuilder().header("X-Requested-With", "Slouchcat").build()) }
            .build()

        /** Signs in with the primary profile, unless the shared web session still has one. */
        fun signIn() {
            if (cookies.isNotEmpty() && runCatching { call("GET", "admin/libraries") }.isSuccess) return
            call("POST", "auth/login", """{"email":${Json.encodeToString(email)},"password":${Json.encodeToString(password)}}""")
            val primary = call("GET", "auth/profiles")!!.jsonArray.map { it.jsonObject }.single { it["isPrimary"]!!.jsonPrimitive.boolean }
            val withPin = primary["hasPin"]?.jsonPrimitive?.boolean == true
            call("POST", "auth/profiles/${primary["id"]!!.jsonPrimitive.long}/select", if (withPin) """{"pin":"$pin"}""" else null)
        }

        private fun call(method: String, path: String, body: String? = null): JsonElement? {
            val request = Request.Builder()
                .url(base.newBuilder().addPathSegments(SlouchcatApi.BASE_PATH).addPathSegments(path).build())
                .header("X-Requested-With", "Slouchcat")
                .method(method, if (method == "GET") null else (body ?: "").toRequestBody("application/json".toMediaType()))
                .build()
            http.newCall(request).execute().use { response ->
                val text = response.body.string()
                check(response.isSuccessful) { "$method $path: ${response.code} $text" }
                return text.takeIf { it.isNotBlank() }?.let(Json::parseToJsonElement)
            }
        }

        private var current = LIBRARY

        private fun library(): JsonObject? =
            call("GET", "admin/libraries")!!.jsonArray.map { it.jsonObject }.firstOrNull { it["name"]?.jsonPrimitive?.content == current }

        fun ensureLibrary(media: String, name: String = LIBRARY, folder: String = "") {
            signIn()
            current = name
            if (library() == null) {
                val movies = Json.encodeToString("$media$folder/Movies")
                val tv = Json.encodeToString("$media$folder/TV")
                call(
                    "POST",
                    "admin/libraries",
                    """{"name":"$name","kind":"video","roots":[{"path":$movies,"content":"movie","providers":["tmdb"]},{"path":$tv,"content":"series","providers":["tmdb"]}]}""",
                )
            }
            val deadline = System.nanoTime() + SCAN_TIMEOUT_NS
            while (true) {
                val state = (library()!!["scan"] as JsonObject)["state"]!!.jsonPrimitive.content
                check(state != "failed" && state != "aborted") { "the scan of the test library $state" }
                if (state == "done") return
                check(System.nanoTime() < deadline) { "the scan of the test library did not finish" }
                Thread.sleep(POLL_MS)
            }
        }
    }

    private companion object {
        /** The cookies of the web session per server and account (see [Admin]). */
        val SESSIONS = mutableMapOf<String, MutableList<Cookie>>()

        const val LIBRARY = "Android test media"
        const val FRAME_MS = 500L
        const val SCAN_TIMEOUT_NS = 120_000_000_000L
        const val POLL_MS = 250L
    }
}
