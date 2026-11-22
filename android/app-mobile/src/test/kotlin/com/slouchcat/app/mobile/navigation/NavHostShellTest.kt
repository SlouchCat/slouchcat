package com.slouchcat.app.mobile.navigation

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.foundation.layout.Column
import androidx.compose.material3.Text
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.Label
import com.slouchcat.app.mobile.shell.MediaEntries
import com.slouchcat.app.mobile.shell.MediaEntry
import com.slouchcat.app.mobile.shell.MediaSection
import com.slouchcat.app.mobile.shell.ShellTopBar
import com.slouchcat.app.mobile.shell.TopDestination
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.browse.testing.CHILD
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeAccounts
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import com.slouchcat.core.browse.testing.HTTPS_SERVER
import com.slouchcat.core.browse.testing.PRIMARY
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.session.LibrariesRepository
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.Serializable
import okhttp3.HttpUrl
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@Serializable internal data object StartRoute

@Serializable internal data object StartDetailRoute

@Serializable internal data object OtherRoute

@Serializable internal data object OtherDetailRoute

@Serializable internal data object PlayerRoute

@Serializable internal data object FilmsRoute

/** A page with a counter kept by `rememberSaveable`: it shows whether the page's state survived. */
@androidx.compose.runtime.Composable
private fun CounterPage(name: String, onNext: (() -> Unit)? = null, onPlay: (() -> Unit)? = null) {
    var count by rememberSaveable { mutableIntStateOf(0) }
    Column {
        ShellTopBar(name)
        Text("$name count $count")
        androidx.compose.material3.Button(onClick = { count++ }) { Text("$name plus") }
        if (onNext != null) androidx.compose.material3.Button(onClick = onNext) { Text("$name next") }
        if (onPlay != null) androidx.compose.material3.Button(onClick = onPlay) { Text("$name play") }
    }
}

/** Two targets, pages below them, a full-screen page and a media entry, as stages 4 to 8 bring them. */
internal object ShellTestFeature : Feature {
    override val destinations = listOf(
        TopDestination("start", 10, StartRoute, R.string.nav_home, SlouchcatIcon.Home),
        TopDestination("other", 50, OtherRoute, R.string.app_more_title, SlouchcatIcon.Dots),
    )
    override val fullScreen = setOf(PlayerRoute::class)

    /** Films only once the libraries are there: the media choice goes from empty to full. */
    override val media = listOf(
        MediaEntries { libraries ->
            libraries.map { MediaEntry("films", MediaSection.Media, 10, Label.Plain(it.name), SlouchcatIcon.Film, FilmsRoute) }
        },
    )

    override fun NavGraphBuilder.register(app: AppScope) {
        composable<StartRoute> { CounterPage("start", onNext = { app.navigator.navigate(StartDetailRoute) }, onPlay = { app.navigator.navigate(PlayerRoute) }) }
        composable<StartDetailRoute> { CounterPage("startdetail") }
        composable<OtherRoute> { CounterPage("other", onNext = { app.navigator.navigate(OtherDetailRoute) }) }
        composable<OtherDetailRoute> { CounterPage("otherdetail") }
        composable<PlayerRoute> { CounterPage("player") }
        composable<FilmsRoute> { CounterPage("films") }
    }
}

/**
 * The shell as the app runs it (code review M-9): the real [SlouchcatNavHost] with test features,
 * in both widths. Profile change empties every target's pages (B-1); a page keeps its state over a
 * full-screen page and the arrival of the media choice (B-2); Back closes the menu (M-6); a media
 * entry opens under «Start» (M-7).
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class NavHostShellTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val librariesArrive = CompletableDeferred<Unit>()
    private val libraries = object : LibrariesRepository {
        override suspend fun libraries(server: HttpUrl): ApiResult<List<LibraryNav>> {
            librariesArrive.await()
            return ApiResult.Ok(listOf(LibraryNav(1, "Films", LibraryKind.VIDEO, emptyList(), 12)))
        }
    }

    private val graph = AppGraph(
        ApplicationProvider.getApplicationContext<Context>(),
        features = listOf(ShellTestFeature),
        serverStore = FakeServers(HTTPS_SERVER),
        tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
        accountStore = FakeAccounts(EMAIL, PRIMARY),
        librariesRepository = libraries,
    )

    private fun start() {
        compose.setContent { SlouchcatNavHost(graph) }
        waitForText("start count 0")
    }

    private fun waitForText(text: String) = compose.waitUntil(5_000) {
        compose.onAllNodes(androidx.compose.ui.test.hasText(text)).fetchSemanticsNodes().isNotEmpty()
    }

    /** The media entry «Films» lies on the screen: in the open menu or in the side column (the closed menu keeps it off screen). */
    private fun filmsOnScreen() = compose.onAllNodes(androidx.compose.ui.test.hasText("Films") and hasClickAction())
        .fetchSemanticsNodes().any { it.boundsInRoot.right > 0f }

    private fun tap(text: String) = compose.onNodeWithText(text).performClick()

    /** A target of the bar (named by its description) or of the side column (by its text). */
    private fun target(label: String) =
        compose.onNode((hasContentDescription(label) or androidx.compose.ui.test.hasText(label)) and hasClickAction()).performClick()

    private fun back() = compose.activityRule.scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }

    private fun finishing(): Boolean {
        var result = false
        compose.activityRule.scenario.onActivity { result = it.isFinishing }
        return result
    }

    private fun aPageKeepsItsStateOverAFullScreenPageAndTheMediaChoice() {
        start()
        repeat(3) { tap("start plus") }
        waitForText("start count 3")
        tap("start play")
        waitForText("player count 0")
        back()
        waitForText("start count 3")
        // The media choice arrives: the frame changes, the page stays.
        librariesArrive.complete(Unit)
        compose.waitUntil(5_000) {
            compose.onAllNodes(hasContentDescription(s(R.string.app_shell_menuOpen)) or androidx.compose.ui.test.hasText("Films")).fetchSemanticsNodes().isNotEmpty()
        }
        compose.onNodeWithText("start count 3").assertExists()
    }

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun stateSurvivesFramePhone() = aPageKeepsItsStateOverAFullScreenPageAndTheMediaChoice()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun stateSurvivesFrameTablet() = aPageKeepsItsStateOverAFullScreenPageAndTheMediaChoice()

    private fun aTargetKeepsItsPagesUntilTheProfileChanges() {
        start()
        target(s(R.string.app_more_title))
        waitForText("other count 0")
        tap("other next")
        tap("otherdetail plus")
        waitForText("otherdetail count 1")
        target(s(R.string.nav_home))
        waitForText("start count 0")
        // Back to «other»: its page below is restored with its state.
        target(s(R.string.app_more_title))
        waitForText("otherdetail count 1")
        // Another profile: every target starts afresh (code review B-1).
        runBlocking { graph.sessionActions.profileChosen(CHILD) }
        waitForText("start count 0")
        target(s(R.string.app_more_title))
        waitForText("other count 0")
        compose.onNodeWithText("otherdetail count 1").assertDoesNotExist()
    }

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun profileChangeEmptiesTargetsPhone() = aTargetKeepsItsPagesUntilTheProfileChanges()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun profileChangeEmptiesTargetsTablet() = aTargetKeepsItsPagesUntilTheProfileChanges()

    private fun aMediaEntryOpensUnderStart() {
        librariesArrive.complete(Unit)
        start()
        target(s(R.string.app_more_title))
        waitForText("other count 0")
        compose.waitUntil(5_000) {
            compose.onAllNodes(hasContentDescription(s(R.string.app_shell_menuOpen)) or androidx.compose.ui.test.hasText("Films")).fetchSemanticsNodes().isNotEmpty()
        }
        val menu = compose.onAllNodes(hasContentDescription(s(R.string.app_shell_menuOpen))).fetchSemanticsNodes()
        if (menu.isNotEmpty()) compose.onNodeWithContentDescription(s(R.string.app_shell_menuOpen)).performClick()
        compose.waitUntil(5_000) { filmsOnScreen() }
        compose.onNode(androidx.compose.ui.test.hasText("Films") and hasClickAction()).performClick()
        waitForText("films count 0")
        back()
        // Back leads to «Start», not to «other» (code review M-7).
        waitForText("start count 0")
    }

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun mediaEntryUnderStartPhone() = aMediaEntryOpensUnderStart()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun mediaEntryUnderStartTablet() = aMediaEntryOpensUnderStart()

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun backClosesTheMenu() {
        librariesArrive.complete(Unit)
        start()
        compose.waitUntil(5_000) { compose.onAllNodes(hasContentDescription(s(R.string.app_shell_menuOpen))).fetchSemanticsNodes().isNotEmpty() }
        compose.onNodeWithContentDescription(s(R.string.app_shell_menuOpen)).performClick()
        compose.waitUntil(5_000) { filmsOnScreen() }
        back()
        compose.waitUntil(5_000) { !filmsOnScreen() }
        assertFalse("Back left the app instead of closing the menu", finishing())
        compose.onNodeWithText("start count 0").assertExists()
        back()
        assertTrue(finishing())
    }

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun theTargetOfAPageBelowStaysMarked() {
        start()
        tap("start next")
        waitForText("startdetail count 0")
        compose.onNode(hasContentDescription(s(R.string.nav_home)) and hasClickAction(), useUnmergedTree = true)
            .assert(SemanticsMatcher.expectValue(SemanticsProperties.Selected, true))
    }

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun aNoticeStaysOnScreen() {
        start()
        graph.notices.show(R.string.app_signOut_done)
        waitForText(s(R.string.app_signOut_done))
        // Consuming the notice must not close the snackbar again.
        compose.mainClock.advanceTimeBy(1_000)
        compose.onNodeWithText(s(R.string.app_signOut_done)).assertExists()
    }

    private fun s(id: Int): String = ApplicationProvider.getApplicationContext<Context>().getString(id)
}
