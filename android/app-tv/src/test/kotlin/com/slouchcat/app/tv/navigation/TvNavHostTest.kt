package com.slouchcat.app.tv.navigation

import androidx.activity.ComponentActivity
import androidx.compose.foundation.layout.Column
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.test.core.app.ApplicationProvider
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.common.MemoryRelaySettings
import com.slouchcat.app.tv.settings.SettingsFeature
import com.slouchcat.app.tv.shell.Label
import com.slouchcat.app.tv.shell.RailEntry
import com.slouchcat.app.tv.shell.RailMedia
import com.slouchcat.app.tv.shell.RailSection
import com.slouchcat.app.tv.shell.ShellTags
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvRegistry
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.app.tv.shell.tvPage
import com.slouchcat.app.tv.testing.Dpad
import com.slouchcat.app.tv.testing.FakeAccounts
import com.slouchcat.app.tv.testing.FakeAuth
import com.slouchcat.app.tv.testing.FakeLibraries
import com.slouchcat.app.tv.testing.FakeProbe
import com.slouchcat.app.tv.testing.FakeServers
import com.slouchcat.app.tv.testing.FakeTokens
import com.slouchcat.app.tv.testing.HTTPS_SERVER
import com.slouchcat.app.tv.testing.PRIMARY
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SessionEvent
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.pairing.Redeemer
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable
import okhttp3.HttpUrl
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.shadows.ShadowLooper

@Serializable
internal data object TestStartRoute

@Serializable
internal data class TestLibraryRoute(val id: Long)

/** A start page of a later stage: three buttons, the middle one keeps the focus over Back. */
internal object TestStartFeature : TvFeature {
    override val rail = listOf(RailEntry("start", RailSection.Top, 10, Label.Res(R.string.nav_home), SlouchcatIcon.Home.vector, TestStartRoute))
    override val media = listOf(
        RailMedia { libraries -> libraries.map { RailEntry("lib-${it.id}", RailSection.Media, it.id.toInt(), Label.Plain(it.name), SlouchcatIcon.Film.vector, TestLibraryRoute(it.id)) } },
    )

    override fun NavGraphBuilder.register(app: TvScope) {
        composable<TestStartRoute> {
            val focus = rememberFocusMemory()
            Column(Modifier.tvPage()) {
                for (name in listOf("Eins", "Zwei", "Drei")) TvButton(name, onClick = {}, modifier = Modifier.focusKey(focus, name))
            }
            focus.StartFocus("Eins")
        }
        composable<TestLibraryRoute> { TvButton("Bibliothek", onClick = {}, modifier = Modifier.tvPage()) }
    }
}

/**
 * The shell over the real NavHost (K26 3.4, BAUKASTEN «Fokus-Engine»): the first page of every
 * session state, the rail (opens with the focus, Left in, Right back to where the focus was),
 * the profile at its foot, Back (pages, then the rail, then «nochmals Zurück», then leaving),
 * focus kept over Back, a lost session.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class TvNavHostTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val remote = Dpad(compose)
    private val context = ApplicationProvider.getApplicationContext<android.content.Context>()
    private var exited = 0
    private var now = 0L

    private fun s(id: Int, vararg args: Any) = context.getString(id, *args)

    private val noRedeem = object : Redeemer {
        override suspend fun redeem(server: HttpUrl, serverKey: String, voucher: String, tvKey: String, deviceName: String, systemVersion: String) = ApiResult.Offline
        override suspend fun discard(server: HttpUrl, serverKey: String, token: String): ApiResult<Unit> = ApiResult.Offline
    }

    private fun graph(signedIn: Boolean, profile: Boolean = true, libraries: List<LibraryNav> = emptyList()): TvGraph {
        val tokens = FakeTokens(if (signedIn) StoredCredentials("token", "device-1") else null)
        val accounts = FakeAccounts(if (signedIn) "owner@example.test" else null, if (signedIn && profile) PRIMARY else null)
        return TvGraph(
            context,
            features = listOf(SettingsFeature, TestStartFeature),
            serverStore = FakeServers(if (signedIn) HTTPS_SERVER else null),
            tokenStore = tokens,
            accountStore = accounts,
            relaySettings = MemoryRelaySettings(),
            serverProbe = FakeProbe(ProbeResult.Unreachable),
            auth = FakeAuth(),
            redeemer = noRedeem,
            librariesRepository = FakeLibraries(ApiResult.Ok(libraries)),
        )
    }

    private fun show(graph: TvGraph) {
        InstrumentationRegistry.getInstrumentation().setInTouchMode(false)
        compose.setContent { TvNavHost(graph, onExit = { exited++ }, clock = { now }) }
        compose.waitUntil(5_000) { graph.session.state.value != SessionState.Loading }
        remote.settle()
        remote.settle()
    }

    @Test
    fun withoutAServerTheSetupComesAndBackLeavesAfterTheSecondPress() {
        show(graph(signedIn = false))
        remote.assertFocused(s(R.string.tv_setup_phoneTitle))
        assertTrue(compose.onAllNodes(hasTestTag(ShellTags.RAIL)).fetchSemanticsNodes().isEmpty())
        // The way by hand and back to the choice: the focus stands where it was.
        remote.right()
        remote.ok()
        compose.onNodeWithText(s(R.string.tv_hand_serverLead)).assertExists()
        remote.back()
        remote.assertFocused(s(R.string.tv_setup_handTitle))
        remote.back()
        compose.onNodeWithText(s(R.string.tv_shell_exitHint)).assertExists()
        assertEquals(0, exited)
        now += 1_000
        remote.back()
        assertEquals(1, exited)
    }

    @Test
    fun theRailOpensWithTheFocusAndRightReturnsToThePage() {
        show(graph(signedIn = true, libraries = listOf(LibraryNav(7, "Filme", LibraryKind.VIDEO, emptyList(), 12))))
        remote.assertFocused("Eins")
        remote.down()
        remote.assertFocused("Zwei")
        remote.left()
        // The rail opens on the marked entry, with labels and the media of the profile.
        remote.assertFocused(s(R.string.nav_home))
        compose.onNodeWithText(s(R.string.tv_shell_media).uppercase()).assertExists()
        compose.onNode(hasContentDescription("Filme")).assertExists()
        remote.down()
        remote.assertFocused("Filme")
        remote.right()
        remote.assertFocused("Zwei")
        // A media entry opens its page; Back returns with the focus where it was.
        remote.left()
        remote.down()
        remote.ok()
        compose.onNodeWithText("Bibliothek").assertExists()
        remote.back()
        remote.assertFocused("Zwei")
    }

    @Test
    fun backOnTheFirstPageGoesToTheRailThenAsksThenLeaves() {
        show(graph(signedIn = true))
        remote.assertFocused("Eins")
        remote.back()
        remote.assertFocused(s(R.string.nav_home))
        remote.back()
        compose.onNodeWithText(s(R.string.tv_shell_exitHint)).assertExists()
        // Too late: the next Back asks again instead of leaving.
        now += 5_000
        remote.back()
        assertEquals(0, exited)
        now += 1_000
        remote.back()
        assertEquals(1, exited)
    }

    @Test
    fun theSettingsAndTheProfileOfTheRailAndBackFromThem() {
        show(graph(signedIn = true))
        remote.left()
        remote.down()
        remote.assertFocused(s(R.string.tv_settings_title))
        remote.ok()
        remote.assertFocused(s(R.string.tv_shell_switchProfile))
        // Back from a rail entry's page returns to the first page.
        remote.back()
        remote.assertFocused("Eins")
        // The profile at the foot of the rail: the choice without rail; Back returns.
        remote.left()
        remote.down(2)
        remote.assertFocused(PRIMARY.name)
        remote.ok()
        compose.onNodeWithText(s(R.string.profiles_picker_title)).assertExists()
        assertTrue(compose.onAllNodes(hasTestTag(ShellTags.RAIL)).fetchSemanticsNodes().isEmpty())
        remote.back()
        compose.onNode(hasTestTag(ShellTags.RAIL)).assertExists()
    }

    @Test
    fun withoutAProfileTheChoiceComesFirst() {
        show(graph(signedIn = true, profile = false))
        compose.onNodeWithText(s(R.string.profiles_picker_title)).assertExists()
        compose.onNodeWithText(s(R.string.tv_profiles_connected, "media.example.test", "owner@example.test")).assertExists()
    }

    @Test
    fun aLostSessionLeadsToTheSetup() {
        val graph = graph(signedIn = true)
        show(graph)
        graph.sessionEvents.report(SessionEvent.SignedOut, graph.session.epoch.value)
        // The image cache is cleared on the IO dispatcher; the main looper takes the end of it.
        compose.waitUntil(5_000) {
            ShadowLooper.idleMainLooper()
            graph.session.state.value == SessionState.NoServer
        }
        remote.settle()
        remote.assertFocused(s(R.string.tv_setup_phoneTitle))
        compose.onNodeWithText(s(R.string.tv_shell_signedOut)).assertExists()
        // A 401 of an earlier epoch changes nothing.
        graph.sessionEvents.report(SessionEvent.SignedOut, -1)
        remote.settle()
        remote.assertFocused(s(R.string.tv_setup_phoneTitle))
    }

    @Test
    fun anAppTooOldIsSaid() {
        val graph = graph(signedIn = true)
        show(graph)
        graph.sessionEvents.report(SessionEvent.AppTooOld)
        remote.settle()
        compose.onNodeWithText(s(R.string.app_update_body)).assertExists()
        remote.assertFocused(s(R.string.app_update_close))
        remote.ok()
        assertTrue(compose.onAllNodes(hasContentDescription(s(R.string.app_update_body))).fetchSemanticsNodes().isEmpty())
    }

    @Test
    fun theRegistryChecksItsEntries() {
        val registry = TvRegistry(listOf(SettingsFeature, TestStartFeature))
        assertEquals(listOf("start", "settings"), registry.rail.map { it.key })
        assertEquals("start", registry.home.key)
        assertEquals("settings", TvRegistry(listOf(SettingsFeature)).home.key)
        val twice = runCatching { TvRegistry(listOf(SettingsFeature, SettingsFeature)) }
        assertFalse(twice.isSuccess)
    }
}
