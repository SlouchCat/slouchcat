package com.slouchcat.app.mobile.downloads

import android.content.Context
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.more.MoreFeature
import com.slouchcat.app.mobile.testing.DL1
import com.slouchcat.app.mobile.testing.FakeOfflineServer
import com.slouchcat.app.mobile.testing.downloadState
import com.slouchcat.app.mobile.testing.offlineIdentity
import com.slouchcat.app.mobile.testing.offlineItem
import com.slouchcat.app.mobile.testing.testEngine
import com.slouchcat.app.mobile.testing.testOfflineData
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeAccounts
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import com.slouchcat.core.browse.testing.HTTPS_SERVER
import com.slouchcat.core.browse.testing.PRIMARY
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config

/**
 * When the app opens the offline library by itself (code review M-3, ADR 0043 point 8): only at the
 * start of an epoch and only after two failed attempts; never later in the middle of what the user
 * does; when the server answers again the user stays and the shell says so.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class OfflineFollowTest {
    @get:Rule val temp = TemporaryFolder()
    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val server = FakeOfflineServer()

    private val graph by lazy {
        AppGraph(
            context,
            features = listOf(MoreFeature, DownloadsFeature),
            serverStore = FakeServers(HTTPS_SERVER),
            tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
            accountStore = FakeAccounts(EMAIL, PRIMARY),
            offlineData = { testOfflineData(testEngine(context, temp.newFolder("offline"), server, scope, { offlineIdentity(PRIMARY.id) })) },
        )
    }

    @After
    fun tearDown() = scope.cancel()

    private fun ready() {
        val end = System.nanoTime() + 5_000_000_000L
        while (graph.session.state.value !is SessionState.Ready && System.nanoTime() < end) shadowOf(Looper.getMainLooper()).idle()
        assertTrue(graph.session.state.value is SessionState.Ready)
    }

    /** A complete download of the profile on the device, synced once while the server answered. */
    private suspend fun aFilmOnTheDevice() {
        server.syncAnswer = ApiResult.Ok(server.sync(downloads = listOf(downloadState(DL1, offlineItem("i1", "Morgenrot")))))
        graph.offline.engine.sync()
        graph.offline.engine.updateDownload(DL1) { it?.copy(complete = true) }
    }

    @Test
    fun theStartWithoutServerOpensTheDownloadsOnce() = runBlocking {
        ready()
        aFilmOnTheDevice()
        server.syncAnswer = ApiResult.Offline
        // Mid-epoch failures (the ticker) never jump.
        graph.offline.syncAndFetch(graph, atStart = false)
        graph.offline.syncAndFetch(graph, atStart = false)
        assertNull(graph.links.pending.value)
        // A new epoch: two failures at its start, then «Downloads».
        graph.session.profileChosen(PRIMARY)
        graph.offline.engine.sync()
        graph.offline.syncAndFetch(graph, atStart = true)
        assertEquals(DownloadsFeature.TARGET, graph.links.pending.value?.target)
        graph.links.consume()
        // Back online: no jump back, a notice instead.
        server.syncAnswer = ApiResult.Ok(server.sync())
        graph.offline.syncAndFetch(graph, atStart = false)
        assertNull(graph.links.pending.value)
        assertEquals(R.string.app_downloads_backOnline, graph.notices.pending.value)
    }

    @Test
    fun oneFailedAttemptAtTheStartIsNotYetOffline() = runBlocking {
        ready()
        aFilmOnTheDevice()
        // One failed attempt (a timeout, a restart of the server), then the server answers at the start.
        server.syncAnswer = ApiResult.Offline
        graph.offline.engine.sync()
        server.syncAnswer = ApiResult.Ok(server.sync())
        graph.offline.syncAndFetch(graph, atStart = true)
        assertNull(graph.links.pending.value)
    }
}
