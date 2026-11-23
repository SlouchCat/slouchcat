package com.slouchcat.app.tv.testing

import android.content.Context
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import androidx.test.core.app.ApplicationProvider
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.browse.PlayerRoute
import com.slouchcat.core.session.MemoryRelaySettings
import com.slouchcat.app.tv.navigation.TvNavHost
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeAccounts
import com.slouchcat.core.browse.testing.FakeLibraries
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import com.slouchcat.core.browse.testing.HTTPS_SERVER
import com.slouchcat.core.browse.testing.PRIMARY
import com.slouchcat.core.browse.testing.library
import com.slouchcat.core.browse.testing.profile
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.settle
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.session.StoredCredentials
import org.robolectric.shadows.ShadowLooper
import java.util.concurrent.TimeUnit

/** The real features without the player of stage 3: the pages of stage 2b with or without [TestPlayerFeature]. */
val withoutPlayer: List<TvFeature> = com.slouchcat.app.tv.shell.features.filterNot { it is com.slouchcat.app.tv.player.PlayerFeature }

/** A stand-in for the player of stage 3: registers [PlayerRoute] and remembers what it was asked to play. */
class TestPlayerFeature : TvFeature {
    val played = mutableListOf<PlayerRoute>()
    override val fullScreen = setOf(PlayerRoute::class)

    override fun NavGraphBuilder.register(app: TvScope) {
        composable<PlayerRoute> { entry ->
            val route = entry.toRoute<PlayerRoute>()
            androidx.compose.runtime.LaunchedEffect(route) { played += route }
        }
    }
}

/**
 * The pages of stage 2b in the real shell and NavHost, signed in with [profile], the data of
 * [data] instead of a server (the graph's `mediaData`, as the showcase does), the [libraries] in the
 * rail. The app starts on «Start».
 */
class Pages(private val compose: TvRule) {
    val remote = Dpad(compose)
    val context: Context = ApplicationProvider.getApplicationContext()
    var exited = 0
    lateinit var graph: TvGraph
        private set

    fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    /**
     * The controls of the page, not of the rail: entering the rail always lands on its marked entry
     * (2a), so a walk that jumps into it cannot reach the others; `TvNavHostTest` checks the rail.
     */
    val page = androidx.compose.ui.test.hasAnyAncestor(androidx.compose.ui.test.hasTestTag(com.slouchcat.app.tv.shell.ShellTags.CONTENT))

    fun show(
        data: (MediaData) -> MediaData,
        libraries: List<LibraryNav> = listOf(library(7, "Filme"), library(8, "Serien", contents = listOf(com.slouchcat.core.api.generated.models.RootContent.SERIES))),
        profile: ActiveProfile = PRIMARY,
        features: List<TvFeature> = com.slouchcat.app.tv.shell.features,
    ): TvGraph {
        graph = TvGraph(
            context,
            features = features,
            serverStore = FakeServers(HTTPS_SERVER),
            tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
            accountStore = FakeAccounts(EMAIL, profile),
            relaySettings = MemoryRelaySettings(),
            serverProbe = FakeProbe(ProbeResult.Unreachable),
            auth = FakeAuth(),
            librariesRepository = FakeLibraries(ApiResult.Ok(libraries)),
            mediaData = { g -> data(MediaData.api(g.api, g.http)) },
        )
        InstrumentationRegistry.getInstrumentation().setInTouchMode(false)
        compose.setContent { TvNavHost(graph, onExit = { exited++ }) }
        compose.waitUntil(5_000) { graph.session.state.value is SessionState.Ready }
        remote.settle()
        remote.settle()
        return graph
    }

    /** Waits until something with [text] (shown or said) is on the screen. */
    fun waitFor(text: String) {
        val matcher = hasText(text, substring = true) or hasContentDescription(text, substring = true)
        // A ViewModel's delay (the search waits 200 ms after a key) runs on the main looper's clock.
        repeat(WAIT_STEPS) {
            if (compose.onAllNodes(matcher, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()) {
                remote.settle()
                return
            }
            ShadowLooper.idleMainLooper(STEP_MS, TimeUnit.MILLISECONDS)
            remote.settle()
        }
        error("«$text» did not come")
    }

    /** The rail entry [label] from anywhere: Left into the rail, down to it, OK. */
    fun openRail(label: String) {
        remote.left()
        remote.up(MAX_RAIL)
        repeat(MAX_RAIL) {
            if (remote.focusedLabel().contains(label)) {
                remote.ok()
                return
            }
            remote.down()
        }
        error("no rail entry «$label»")
    }

    /** A page of «Meine Listen» (or a library) from the menu «Bibliotheken»: its rail entry, then down to [label], OK. */
    fun openList(label: String) {
        openRail(s(com.slouchcat.core.strings.R.string.tv_shell_allLibraries))
        repeat(MAX_RAIL) {
            if (remote.focusedLabel().contains(label)) {
                remote.ok()
                return
            }
            remote.down()
        }
        error("no entry «$label» in the menu")
    }

    private companion object {
        const val MAX_RAIL = 14
        const val WAIT_STEPS = 50
        const val STEP_MS = 100L
    }
}
