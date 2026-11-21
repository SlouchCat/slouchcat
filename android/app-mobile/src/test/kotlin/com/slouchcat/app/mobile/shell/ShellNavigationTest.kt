package com.slouchcat.app.mobile.shell

import androidx.activity.ComponentActivity
import androidx.compose.material3.Text
import androidx.compose.runtime.getValue
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.navigation.NavGraphBuilder
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@Serializable internal data object HomeRoute

@Serializable internal data object DetailRoute

@Serializable internal data object OtherRoute

@Serializable internal data object PlayerRoute

/** Two features as later stages bring them: a target with a page below it, a second target, a full-screen page. */
internal object HomeFeature : Feature {
    override val destinations = listOf(TopDestination("home", 10, HomeRoute, R.string.nav_home, SlouchcatIcon.Home))
    override val fullScreen = setOf(PlayerRoute::class)
    override fun NavGraphBuilder.register(app: AppScope) {
        composable<HomeRoute> { Text("home page") }
        composable<DetailRoute> { Text("detail page") }
        composable<PlayerRoute> { Text("player page") }
    }
}

internal object OtherFeature : Feature {
    override val destinations = listOf(TopDestination("other", 50, OtherRoute, R.string.app_more_title, SlouchcatIcon.Dots))
    override fun NavGraphBuilder.register(app: AppScope) {
        composable<OtherRoute> { Text("other page") }
    }
}

/**
 * The navigation of the shell (rule 8, «Zurück führt hinaus»): the first target is the bottom of
 * the stack, a page counts to the target it was opened from, back from a target leaves the app.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36], qualifiers = "w412dp-h915dp")
class ShellNavigationTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val registry = ShellRegistry(listOf(OtherFeature, HomeFeature))
    private lateinit var nav: NavHostController
    private lateinit var navigator: Navigator
    private var frame: ShellFrame? = null

    private fun host(start: Any = HomeRoute) = compose.setContent {
        nav = rememberNavController()
        navigator = Navigator(nav, registry)
        val current by nav.currentBackStackEntryAsState()
        // As SlouchcatNavHost does: the last target page shown counts for the pages below it.
        var last by remember { mutableStateOf<String?>(null) }
        val here = registry.targetOf(current)?.key
        if (here != null) last = here
        frame = registry.frame(current, last, emptyList())
        NavHost(nav, startDestination = start) {
            composable<ProfilesRoute> { Text("profiles page") }
            for (feature in registry.features) with(feature) { register(AppScope(appGraph(), navigator)) }
        }
    }

    private fun back() = compose.activityRule.scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }

    private fun finishing(): Boolean {
        var result = false
        compose.activityRule.scenario.onActivity { result = it.isFinishing }
        return result
    }

    @Test
    fun theTargetsSortByOrderAndTheFirstIsHome() {
        assertEquals(listOf("home", "other"), registry.destinations.map { it.key })
        assertEquals(HomeRoute, registry.home.route)
        assertTrue(registry.showsTargets)
    }

    @Test
    fun aPageCountsToItsTargetAndBackLeavesFromTheFirst() {
        host()
        compose.runOnUiThread { navigator.navigate(DetailRoute) }
        compose.waitForIdle()
        compose.onNodeWithText("detail page").assertExists()
        assertEquals("home", frame?.selectedTarget)
        assertFalse(frame!!.fullScreen)
        back()
        compose.waitForIdle()
        compose.onNodeWithText("home page").assertExists()
        back()
        assertTrue(finishing())
    }

    @Test
    fun anotherTargetGoesBackToTheFirst() {
        host()
        compose.runOnUiThread { navigator.openTop(registry.destinations.last()) }
        compose.waitForIdle()
        compose.onNodeWithText("other page").assertExists()
        assertEquals("other", frame?.selectedTarget)
        back()
        compose.waitForIdle()
        compose.onNodeWithText("home page").assertExists()
        assertFalse(finishing())
    }

    @Test
    fun fullScreenAndEntryPagesHaveNoFrame() {
        host()
        compose.runOnUiThread { navigator.navigate(PlayerRoute) }
        compose.waitForIdle()
        assertTrue(frame!!.fullScreen)
        compose.runOnUiThread { navigator.toProfiles() }
        compose.waitForIdle()
        assertTrue(frame!!.fullScreen)
    }

    @Test
    fun theProfileChoiceAsFirstPageIsLeftWithBack() {
        host(start = ProfilesRoute)
        compose.onNodeWithText("profiles page").assertExists()
        back()
        assertTrue(finishing())
    }

    @Test
    fun afterTheProfileChoiceTheFirstTargetStandsAlone() {
        host(start = ProfilesRoute)
        compose.runOnUiThread { navigator.toHome() }
        compose.waitForIdle()
        compose.onNodeWithText("home page").assertExists()
        back()
        assertTrue(finishing())
    }
}

/** The graph of the test application (Robolectric starts SlouchcatApplication). */
private fun appGraph(): com.slouchcat.app.mobile.AppGraph =
    androidx.test.core.app.ApplicationProvider.getApplicationContext<com.slouchcat.app.mobile.SlouchcatApplication>().graph
