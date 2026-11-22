package com.slouchcat.app.tv.shell

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.navigation.NavDestination
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraph
import androidx.navigation.NavHostController
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.common.MemoryRelaySettings
import com.slouchcat.app.tv.testing.FakeAccounts
import com.slouchcat.app.tv.testing.FakeAuth
import com.slouchcat.app.tv.testing.FakeLibraries
import com.slouchcat.app.tv.testing.FakeProbe
import com.slouchcat.app.tv.testing.FakeServers
import com.slouchcat.app.tv.testing.FakeTokens
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import androidx.compose.ui.unit.dp
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import kotlin.reflect.KClass

@Serializable
private data object ClashRoute

/** A feature as a parallel stage might write it; each test changes one thing. */
private class Probe(
    override val rail: List<RailEntry> = emptyList(),
    override val settings: List<SettingsEntry> = emptyList(),
    override val media: List<RailMedia> = emptyList(),
) : TvFeature {
    override fun NavGraphBuilder.register(app: TvScope) {
        composable<ClashRoute> {}
    }
}

/**
 * The register of the real app (review M-2): `features` from `Features.kt` as the app starts with
 * them. Two stages that take the same key or place clash here, in the CI, not at the app's start.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = TV)
class RegistryTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val context = ApplicationProvider.getApplicationContext<android.content.Context>()

    @Test
    fun theRealFeaturesMakeARegisterWithoutClashes() {
        val registry = TvRegistry(features)
        assertTrue(registry.rail.isNotEmpty())
        // Every key once, every place once per section, the shell's own settings rows untouched.
        assertEquals(registry.rail.size, registry.rail.map { it.key }.toSet().size)
        assertEquals(registry.rail.size, registry.rail.map { it.section to it.order }.toSet().size)
        assertTrue(registry.settings.none { it.key in BUILT_IN_SETTINGS.keys || it.order in BUILT_IN_SETTINGS.values })
        // Media entries of every feature, for a few libraries, also pass their checks.
        val libraries = listOf(LibraryNav(1, "Filme", LibraryKind.VIDEO, emptyList(), 1), LibraryNav(2, "Serien", LibraryKind.VIDEO, emptyList(), 1))
        registry.mediaEntries(libraries)
    }

    @Test
    fun theRailOfStage2bIsAsInTheMockup() {
        val registry = TvRegistry(features)
        // Start, Suche on top, Einstellungen at the bottom; «Start» is home.
        assertEquals(listOf("start", "search", "settings"), registry.rail.map { it.key })
        assertEquals("start", registry.home.key)
        // «Medien»: the video libraries in the server's order (audio waits for stage 4); the lists
        // are no entries of the rail (code review M-3) but «Meine Listen» of the menu.
        val libraries = listOf(
            LibraryNav(1, "Filme", LibraryKind.VIDEO, emptyList(), 1),
            LibraryNav(2, "Hörbücher", LibraryKind.AUDIOBOOKS, emptyList(), 1),
            LibraryNav(3, "Serien", LibraryKind.VIDEO, emptyList(), 1),
        )
        assertEquals(listOf("library-1", "library-3"), registry.mediaEntries(libraries).map { it.key })
        assertEquals(listOf("watchlist", "watched", "collections"), registry.lists.map { it.key })
        // «Profile verwalten» in the settings (order 20) and from the profile choice.
        assertEquals(listOf("manage" to 20), registry.settings.map { it.key to it.order })
        assertTrue(registry.manageProfiles != null)
    }

    @Test
    fun theRailHoldsAsManyLibrariesAsFitOn1080p() {
        // 540 dp − 2 × safe-y − brand − profile, minus Start, Suche, Wünsche, Einstellungen and two
        // gaps, in steps of a media entry, minus the radio and «Alle Bibliotheken …».
        assertEquals(5, railLibraryCapacity(540.dp))
        assertTrue(railLibraryCapacity(300.dp) >= 1)
    }

    @Test
    fun everyEntryOfTheRealFeaturesHasAPage() {
        val graph = TvGraph(
            context,
            features = features,
            serverStore = FakeServers(null),
            tokenStore = FakeTokens(null),
            accountStore = FakeAccounts(null, null),
            relaySettings = MemoryRelaySettings(),
            serverProbe = FakeProbe(ProbeResult.Unreachable),
            auth = FakeAuth(),
            librariesRepository = FakeLibraries(ApiResult.Ok(emptyList())),
        )
        var nav: NavHostController? = null
        compose.setContent {
            val controller = rememberNavController()
            nav = controller
            val app = TvScope(graph, TvNavigator(controller, graph.registry))
            NavHost(controller, startDestination = SetupRoute) {
                composable<SetupRoute> {}
                for (feature in graph.registry.features) with(feature) { register(app) }
            }
        }
        compose.waitForIdle()
        val destinations = nav!!.graph.all()
        val routes = graph.registry.rail.map { it.route } + graph.registry.settings.map { it.route } + listOfNotNull(graph.registry.manageProfiles)
        val missing = routes.filter { route -> destinations.none { it.hasRoute(route::class) } }
        assertEquals("entries without a page", emptyList<Any>(), missing)
        val fullScreen: Set<KClass<*>> = graph.registry.fullScreen
        assertTrue(fullScreen.all { route -> destinations.any { it.hasRoute(route) } })
    }

    @Test
    fun aClashOfTwoStagesIsFound() {
        val start = RailEntry("start", RailSection.Top, 10, Label.Res(R.string.nav_home), SlouchcatIcon.Home.vector, ClashRoute)
        fun fails(vararg features: TvFeature) = runCatching { TvRegistry(features.toList()) }.isFailure
        assertFalse(fails(Probe(rail = listOf(start))))
        // Same key, same place, same route.
        assertTrue(fails(Probe(rail = listOf(start)), Probe(rail = listOf(start.copy(order = 20)))))
        assertTrue(fails(Probe(rail = listOf(start)), Probe(rail = listOf(start.copy(key = "search")))))
        // A settings row on a key or place of the shell's own rows.
        val row = SettingsEntry("playback", 30, R.string.tv_settings_title, SlouchcatIcon.Settings.vector, ClashRoute)
        assertFalse(fails(Probe(rail = listOf(start), settings = listOf(row))))
        assertTrue(fails(Probe(rail = listOf(start), settings = listOf(row.copy(key = "server")))))
        assertTrue(fails(Probe(rail = listOf(start), settings = listOf(row.copy(order = SettingsOrder.ABOUT)))))
        assertTrue(fails(Probe(rail = listOf(start), settings = listOf(row)), Probe(settings = listOf(row.copy(key = "radio")))))
        // A media entry must come from RailMedia and know its own page.
        assertTrue(fails(Probe(rail = listOf(start, start.copy(key = "lib", section = RailSection.Media)))))
        val media = TvRegistry(listOf(Probe(rail = listOf(start), media = listOf(RailMedia { list -> list.map { start.copy(key = "lib-${it.id}", section = RailSection.Media) } }))))
        assertTrue(runCatching { media.mediaEntries(listOf(LibraryNav(1, "Filme", LibraryKind.VIDEO, emptyList(), 1))) }.isFailure)
    }

    private fun NavGraph.all(): List<NavDestination> = flatMap { if (it is NavGraph) it.all() else listOf(it) }
}
