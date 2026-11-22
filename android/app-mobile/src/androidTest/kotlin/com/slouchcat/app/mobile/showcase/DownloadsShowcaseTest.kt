package com.slouchcat.app.mobile.showcase

import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.downloads.DownloadsRoute
import com.slouchcat.app.mobile.downloads.DownloadsSignOutStep
import com.slouchcat.app.mobile.playback.PlaybackSettingsFeature
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.features
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.offline.OfflineEngine
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.strings.R
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Stage 6 with made-up data (visual check against the mockup photos 32, 45, 46, 70, 72): the
 * download dialog of a film and of an episode, «Downloads» empty, with entries while loading,
 * without contact, the folds of auto-downloads and settings, the sign-out with its question about
 * the downloads. No server. Photos with `slouchcatShots` as `f…`.
 */
@RunWith(AndroidJUnit4::class)
class DownloadsShowcaseTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private val pages = features.map { feature ->
        if (feature is PlaybackSettingsFeature) PlaybackSettingsFeature(settings = { MemoryDeviceSettingsStore() }, repository = { PagesShowcaseData.playbackPrefs }) else feature
    }

    private val images = ShowcaseImages(BrowseShowcaseData.names + PagesShowcaseData.names + DownloadsShowcaseData.names).also { it.squareIds = BrowseShowcaseData.squareIds }

    private val server = DownloadsShowcaseData.Server()

    private val showcase = Showcase(
        compose, pages, BrowseShowcaseData.libraries, images, PagesShowcaseData.auth, PagesShowcaseData.devices,
        media = { graph -> PagesShowcaseData.data(MediaData.api(graph.api, graph.http)) },
        offline = DownloadsShowcaseData.data(context, server),
    )

    private val engine: OfflineEngine get() = showcase.graph.offline.engine

    @After
    fun close() = showcase.close()

    private fun open(target: String, vararg routes: Any) =
        InstrumentationRegistry.getInstrumentation().runOnMainSync { showcase.graph.links.request(AppLink(target, routes.toList())) }

    private fun click(text: String) =
        compose.onAllNodes((hasText(text) or hasContentDescription(text)) and hasClickAction()).onFirst().performScrollTo().performClick()

    @Test
    fun downloadsDialogOfflineAndSignOut() {
        showcase.start()
        showcase.graph.hooks.addSignOutStep(DownloadsSignOutStep(showcase.graph, context))
        showcase.waitFor(hasText(s(R.string.home_rows_continue)))

        // Empty.
        open("downloads", DownloadsRoute)
        showcase.waitFor(hasText(s(R.string.app_downloads_emptyTitle)))
        showcase.shot("f01-downloads-empty")

        // The dialog of a film, then of an episode (scope).
        open("start", ItemRoute(PagesShowcaseData.filmId))
        click(s(R.string.app_downloads_actionDownload))
        showcase.waitFor(hasText(s(R.string.app_downloads_loadsWifi)) or hasText(s(R.string.app_downloads_loadsAny)))
        showcase.shot("f02-dialog-film")
        showcase.back()
        open("start", ItemRoute(PagesShowcaseData.episodeId))
        click(s(R.string.app_downloads_actionDownload))
        showcase.waitFor(hasText(s(R.string.app_downloads_scopeSeason)))
        showcase.shot("f03-dialog-episode")
        showcase.back()

        // With entries: loading, waiting, done.
        server.downloads = DownloadsShowcaseData.states
        runBlocking {
            engine.sync()
            DownloadsShowcaseData.fill(engine, images)
        }
        open("downloads", DownloadsRoute)
        showcase.waitFor(hasText(s(R.string.app_downloads_sectionActive)))
        showcase.shot("f04-downloads")
        click(s(R.string.app_downloads_autoTitle))
        showcase.waitFor(hasText(s(R.string.offline_rules_enabled)))
        click(s(R.string.app_downloads_settingsTitle))
        showcase.waitFor(hasText(s(R.string.app_downloads_wifiOnly)))
        showcase.shot("f05-downloads-folds")

        // Without contact: the banner, only playable rows active.
        server.offline = true
        runBlocking { engine.sync() }
        open("start")
        open("downloads", DownloadsRoute)
        showcase.waitFor(hasText(s(R.string.app_downloads_offlineBanner)))
        showcase.shot("f06-downloads-offline")
        server.offline = false
        runBlocking { engine.sync() }

        // The sign-out asks about the downloads.
        open("more")
        showcase.waitFor(hasText(s(R.string.app_more_title)) and isHeading())
        click(s(R.string.nav_logout))
        showcase.waitFor(hasText(s(R.string.app_downloads_signOutQuestion)))
        showcase.shot("f07-sign-out")
    }
}
