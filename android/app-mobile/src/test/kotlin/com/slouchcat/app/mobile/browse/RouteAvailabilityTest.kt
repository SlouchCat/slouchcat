package com.slouchcat.app.mobile.browse

import androidx.compose.material3.Text
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.Navigator
import com.slouchcat.app.mobile.shell.ShellRegistry
import com.slouchcat.app.mobile.shell.TopDestination
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@Serializable
internal data object AvailabilityHome

/** A link to a route of stage 4 is a control only once a feature registered the route's page. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36], qualifiers = "w412dp-h915dp")
class RouteAvailabilityTest {
    @get:Rule
    val compose = createComposeRule()

    private val home = object : Feature {
        override val destinations = listOf(TopDestination("start", 10, AvailabilityHome, R.string.nav_home, SlouchcatIcon.Home))
        override fun NavGraphBuilder.register(app: AppScope) = Unit
    }

    @Test
    fun onlyRegisteredRoutesOpen() {
        val registry = ShellRegistry(listOf(home))
        lateinit var navigator: Navigator
        compose.setContent {
            val nav = rememberNavController()
            navigator = remember(nav) { Navigator(nav, registry, mutableStateOf(null)) }
            NavHost(nav, startDestination = AvailabilityHome) {
                composable<AvailabilityHome> { Text("home") }
                composable<ItemRoute> { Text("item") }
            }
        }
        compose.onNodeWithText("home").assertExists()
        compose.runOnIdle {
            assertTrue(navigator.canOpen(ItemRoute::class))
            assertFalse(navigator.canOpen(PersonRoute::class))
            assertFalse(navigator.canOpen(PlayerRoute::class))
            assertNull(navigator.opener(PersonRoute(1)))
            val open = navigator.opener(ItemRoute("01J00000000000000000000001"))
            assertNotNull(open)
            open!!.invoke()
        }
        compose.onNodeWithText("item").assertExists()
    }
}
