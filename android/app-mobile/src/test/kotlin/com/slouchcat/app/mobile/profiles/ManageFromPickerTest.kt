package com.slouchcat.app.mobile.profiles

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.material3.Text
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.MORE_TARGET
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.app.mobile.profilesettings.ProfileSettingsFeature
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.TopDestination
import com.slouchcat.app.mobile.testing.FakeAuth
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeAccounts
import com.slouchcat.core.browse.testing.FakeProfileSettings
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import com.slouchcat.core.browse.testing.HTTPS_SERVER
import com.slouchcat.core.browse.testing.profile
import com.slouchcat.core.browse.testing.summary
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@Serializable internal data object PickerStartRoute

@Serializable internal data object PickerMoreRoute

/** «Start» and «Mehr» that are only texts. */
internal object PickerTargetsFeature : Feature {
    override val destinations = listOf(
        TopDestination("start", 10, PickerStartRoute, R.string.nav_home, SlouchcatIcon.Home),
        TopDestination(MORE_TARGET, 50, PickerMoreRoute, R.string.app_more_title, SlouchcatIcon.Dots),
    )

    override fun NavGraphBuilder.register(app: AppScope) {
        composable<PickerStartRoute> { Text("start page") }
        composable<PickerMoreRoute> { Text("more page") }
    }
}

/**
 * «Profile verwalten» in the first profile choice (code review M-3 of part a): no bar without a
 * profile; the button chooses the primary profile first (the server decides, with its PIN where it
 * has one), then «Profile verwalten» opens in the new session, under «Mehr».
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36], qualifiers = "w412dp-h915dp")
class ManageFromPickerTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val auth = FakeAuth().apply { profiles = ApiResult.Ok(listOf(summary(1, "Primary", primary = true), summary(2, "Child", hasPin = true))) }

    private val graph = AppGraph(
        context,
        features = listOf(PickerTargetsFeature, ProfileSettingsFeature { FakeProfileSettings(profile(1, "Primary", primary = true), profile(2, "Child", rating = 12)) }),
        serverStore = FakeServers(HTTPS_SERVER),
        tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
        accountStore = FakeAccounts(EMAIL, null),
        authRepository = auth,
    )

    private fun waitFor(matcher: SemanticsMatcher) = compose.waitUntil(5_000) { compose.onAllNodes(matcher).fetchSemanticsNodes().isNotEmpty() }

    private fun none(matcher: SemanticsMatcher) = compose.onAllNodes(matcher).fetchSemanticsNodes().isEmpty()

    @Test
    fun manageFromTheFirstChoiceChoosesThePrimaryProfileFirst() {
        compose.setContent { SlouchcatNavHost(graph) }
        waitFor(hasText(context.getString(R.string.profiles_picker_title)))
        // No profile, no targets: nothing leads into the app past the choice.
        assertTrue(none(hasContentDescription(context.getString(R.string.nav_home), substring = true)))
        compose.onNode(hasText(context.getString(R.string.app_profiles_manage_title)) and hasClickAction()).performScrollTo().performClick()
        waitFor(hasText(context.getString(R.string.app_profiles_manage_title)) and isHeading())
        waitFor(hasText("Child"))
        assertEquals(listOf("profiles", "select:1:-"), auth.calls)
        // A profile now: the bar is there, «Mehr» marked.
        assertTrue(!none(hasContentDescription(context.getString(R.string.nav_home), substring = true)))
    }

    @Test
    fun aPrimaryProfileWithAPinAsksForItFirst() {
        auth.profiles = ApiResult.Ok(listOf(summary(1, "Primary", hasPin = true, primary = true), summary(2, "Child")))
        compose.setContent { SlouchcatNavHost(graph) }
        waitFor(hasText(context.getString(R.string.profiles_picker_title)))
        compose.onNode(hasText(context.getString(R.string.app_profiles_manage_title)) and hasClickAction()).performScrollTo().performClick()
        waitFor(hasText(context.getString(R.string.profiles_pin_hint)))
        assertEquals(listOf("profiles"), auth.calls)
    }
}
