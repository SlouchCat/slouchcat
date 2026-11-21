package com.slouchcat.app.mobile.showcase

import android.content.Context
import android.graphics.Bitmap
import androidx.activity.compose.setContent
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.junit4.ComposeTestRule
import androidx.test.core.app.ActivityScenario
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.MainActivity
import com.slouchcat.app.mobile.media.LocalMediaImageLoader
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.session.AccountStore
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.LibrariesRepository
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.DevicesRepository
import com.slouchcat.core.session.TokenStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import java.io.File

/*
 * The showcase of the androidTest source set: the real pages of the app in the real NavHost, fed
 * with made-up data and pictures drawn in the test instead of a server, to photograph them against
 * the mockup (the test server has no covers, no spotlight, nothing started). Nothing of this goes
 * into the app. Every area brings its data in a file of its own (`BrowseShowcaseData.kt` for
 * stage 4) and installs it on the graph (`MediaData.install`); [Showcase] does the rest.
 */

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

/**
 * Starts [MainActivity] and replaces its content with [SlouchcatNavHost] over a graph of
 * [features], signed in with [SHOWCASE_PROFILE], the [libraries] in the media choice, and the
 * pictures of [images]. [install] puts each area's data on the graph before the first page draws.
 */
class Showcase(
    private val compose: ComposeTestRule,
    private val features: List<Feature>,
    private val libraries: List<LibraryNav>,
    private val images: ShowcaseImages,
    /** The account's side (profile choice, «Mehr») and «Meine Geräte»; `null`: the server's (never asked by stage 4). */
    private val auth: AuthRepository? = null,
    private val devices: DevicesRepository? = null,
    private val install: (AppGraph) -> Unit,
) {
    private val context: Context = InstrumentationRegistry.getInstrumentation().targetContext
    lateinit var graph: AppGraph
        private set
    private var activity: MainActivity? = null
    private lateinit var scenario: ActivityScenario<MainActivity>

    fun start() {
        val repository = object : LibrariesRepository {
            override suspend fun libraries(server: HttpUrl): ApiResult<List<LibraryNav>> = ApiResult.Ok(libraries)
        }
        InstrumentationRegistry.getInstrumentation().runOnMainSync {
            graph = AppGraph(context, features, Servers(), Tokens(), Accounts(), librariesRepository = repository, authRepository = auth, devicesRepository = devices)
            install(graph)
        }
        val loader = images.loader(context)
        scenario = ActivityScenario.launch(MainActivity::class.java)
        scenario.onActivity { activity ->
            this.activity = activity
            activity.setContent {
                SlouchcatTheme {
                    CompositionLocalProvider(LocalMediaImageLoader provides loader) { SlouchcatNavHost(graph) }
                }
            }
        }
    }

    fun close() = scenario.close()

    /** The system's back, as the gesture gives it. */
    fun back() = InstrumentationRegistry.getInstrumentation().runOnMainSync { activity?.onBackPressedDispatcher?.onBackPressed() }

    fun waitFor(matcher: SemanticsMatcher, timeout: Long = 20_000) =
        compose.waitUntil(timeout) { compose.onAllNodes(matcher, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() }

    /** A photo of the screen to the app's files (`shots/`), taken once the page settled. */
    fun shot(name: String) {
        if (InstrumentationRegistry.getArguments().getString("slouchcatShots") == null) return
        compose.waitForIdle()
        // A frame of the replaced content, not the last one of the activity's own.
        InstrumentationRegistry.getInstrumentation().runOnMainSync { activity?.window?.decorView?.invalidate() }
        InstrumentationRegistry.getInstrumentation().waitForIdleSync()
        compose.mainClock.advanceTimeBy(FRAME_MS)
        compose.waitForIdle()
        // The window may still show an older frame (the content just replaced): wait until it is quiet.
        runCatching { InstrumentationRegistry.getInstrumentation().uiAutomation.waitForIdle(500, 5_000) }
        val bitmap = InstrumentationRegistry.getInstrumentation().uiAutomation.takeScreenshot() ?: return
        val dir = File(context.getExternalFilesDir(null), "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private companion object {
        const val FRAME_MS = 500L
    }
}
