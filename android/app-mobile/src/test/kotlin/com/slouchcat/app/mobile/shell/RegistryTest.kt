package com.slouchcat.app.mobile.shell

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.navigation.NavDestination
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraph
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.CollectionRoute
import com.slouchcat.app.mobile.browse.CollectionsRoute
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.LibraryRoute
import com.slouchcat.app.mobile.browse.ListRoute
import com.slouchcat.app.mobile.browse.PersonRoute
import com.slouchcat.app.mobile.browse.PlayerRoute
import com.slouchcat.app.mobile.browse.ProfilesSettingsRoute
import com.slouchcat.app.mobile.browse.SearchRoute
import com.slouchcat.app.mobile.browse.WatchedRoute
import com.slouchcat.app.mobile.browse.WatchlistRoute
import com.slouchcat.app.mobile.downloads.DownloadsRoute
import com.slouchcat.app.mobile.downloads.OfflinePlayerRoute
import com.slouchcat.app.mobile.home.HomeRoute
import com.slouchcat.app.mobile.more.AccountRoute
import com.slouchcat.app.mobile.more.DevicesRoute
import com.slouchcat.app.mobile.more.MoreRoute
import com.slouchcat.app.mobile.playback.PlaybackSettingsRoute
import com.slouchcat.app.mobile.tvlink.RelayRoute
import com.slouchcat.app.mobile.tvlink.TvLinkRoute
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.browse.testing.FakeAccounts
import com.slouchcat.core.browse.testing.FakeLibraries
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.io.File
import kotlin.reflect.KClass

/**
 * The register of the real phone app: `features` from `Features.kt` as the app starts with them.
 * Two stages that take the same key, place or route clash here, in the CI, not at the app's start;
 * every page of stages 3 to 6 has exactly one destination, every target, row of «Mehr» and media
 * entry leads to a registered page, and every text the app uses comes from a namespace of
 * `android/strings-namespaces.txt`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class RegistryTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val libraries = listOf(
        LibraryNav(1, "Filme", LibraryKind.VIDEO, emptyList(), 10),
        LibraryNav(2, "Hörbücher", LibraryKind.AUDIOBOOKS, emptyList(), 3),
    )

    /** The pages stages 3 to 5 built: each exactly once in the NavHost. */
    private val pages: List<KClass<*>> = listOf(
        HomeRoute::class, LibraryRoute::class, ListRoute::class, ItemRoute::class, PersonRoute::class, SearchRoute::class,
        WatchlistRoute::class, WatchedRoute::class, CollectionsRoute::class, CollectionRoute::class, ProfilesSettingsRoute::class,
        PlayerRoute::class, PlaybackSettingsRoute::class, MoreRoute::class, DevicesRoute::class, AccountRoute::class,
        DownloadsRoute::class, OfflinePlayerRoute::class,
        com.slouchcat.app.mobile.browse.ListenRoute::class, com.slouchcat.app.mobile.browse.AudioSeriesRoute::class,
        com.slouchcat.app.mobile.browse.AudiobooksRoute::class, com.slouchcat.app.mobile.browse.PodcastsRoute::class,
        com.slouchcat.app.mobile.browse.RadioRoute::class,
        TvLinkRoute::class, RelayRoute::class,
    )

    @Test
    fun theRealFeaturesMakeARegisterWithoutClashes() {
        // The checks of ShellRegistry.init: keys, places and routes of the targets, rows of «Mehr».
        val registry = ShellRegistry(features)
        assertEquals(listOf("start", "search", "downloads", "more"), registry.destinations.map { it.key })
        assertEquals(registry.more.size, registry.more.map { it.route::class }.toSet().size)
        // Media entries of every feature, for a few libraries, also pass their checks.
        val entries = registry.mediaEntries(libraries)
        assertEquals(entries.size, entries.map { it.section to it.order }.toSet().size)
        // Stage 7: the small player in the slot of the shell, Webradio in the media choice and «Mehr».
        assertTrue(registry.accessory != null)
        assertTrue(entries.any { it.key == "radio" })
        assertEquals(
            listOf("watchlist", "watched", "collections", "audiobooks", "podcasts", "radio"),
            registry.more.filter { it.section == MoreSection.Library }.map { it.key },
        )
    }

    @Test
    fun everyPageOnceAndEveryEntryLeadsToAPage() {
        val graph = AppGraph(
            ApplicationProvider.getApplicationContext<Context>(),
            features = features,
            serverStore = FakeServers(null),
            tokenStore = FakeTokens(null),
            accountStore = FakeAccounts(null, null),
            librariesRepository = FakeLibraries(ApiResult.Ok(libraries)),
        )
        var nav: NavHostController? = null
        compose.setContent {
            val controller = rememberNavController()
            nav = controller
            val app = AppScope(graph, Navigator(controller, graph.registry))
            NavHost(controller, startDestination = SetupRoute) {
                composable<SetupRoute> {}
                for (feature in graph.registry.features) with(feature) { register(app) }
            }
        }
        compose.waitForIdle()
        val destinations = nav!!.graph.all()
        val counts = pages.associateWith { page -> destinations.count { it.hasRoute(page) } }
        assertEquals("pages not exactly once", emptyMap<KClass<*>, Int>(), counts.filterValues { it != 1 })
        val routes = graph.registry.destinations.map { it.route } + graph.registry.more.map { it.route } +
            graph.registry.mediaEntries(libraries).map { it.route }
        val missing = routes.filter { route -> destinations.none { it.hasRoute(route::class) } }
        assertEquals("entries without a page", emptyList<Any>(), missing)
        assertTrue(graph.registry.fullScreen.all { route -> destinations.any { it.hasRoute(route) } })
    }

    @Test
    fun everyTextOfTheAppComesFromAListedNamespace() {
        // The working directory of a unit test is the module (android/app-mobile).
        val listed = File("../strings-namespaces.txt").readLines().map { it.trim() }.filter { it.isNotEmpty() && !it.startsWith("#") }
        val prefixes = listed.map { it.replace('.', '_') }
        val used = File("src/main/kotlin").walkTopDown().filter { it.extension == "kt" }
            .flatMap { file -> Regex("""\bR\.(?:string|plurals)\.(\w+)""").findAll(file.readText()).map { it.groupValues[1] } }
            .toSortedSet()
        assertTrue("no texts found: wrong directory?", used.isNotEmpty())
        val outside = used.filter { name -> prefixes.none { name == it || name.startsWith(it + "_") } }
        assertEquals("texts outside android/strings-namespaces.txt", emptyList<String>(), outside)
    }

    private fun NavGraph.all(): List<NavDestination> = flatMap { if (it is NavGraph) it.all() else listOf(it) }
}
