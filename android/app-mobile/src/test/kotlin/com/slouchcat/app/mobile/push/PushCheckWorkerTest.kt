package com.slouchcat.app.mobile.push

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.work.ListenableWorker
import androidx.work.WorkInfo
import androidx.work.WorkManager
import androidx.work.testing.TestListenableWorkerBuilder
import androidx.work.testing.WorkManagerTestInitHelper
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.SlouchcatApplication
import com.slouchcat.app.mobile.shell.features
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeAccounts
import com.slouchcat.core.browse.testing.FakeChannels
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import com.slouchcat.core.browse.testing.HTTP_SERVER
import com.slouchcat.core.browse.testing.PRIMARY
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.session.StoredCredentials
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** The check of the subscription in the background (code review M-4): what it calls, when it is planned. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class PushCheckWorkerTest {
    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val app = context as SlouchcatApplication
    private val platform = FakePlatform()

    @Before
    fun workManager() = WorkManagerTestInitHelper.initializeTestWorkManager(context)

    private fun graph() = AppGraph(
        context,
        features = features,
        serverStore = FakeServers(HTTP_SERVER),
        tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
        accountStore = FakeAccounts(EMAIL, PRIMARY),
        pushRegistrar = { g -> PushRegistrar(g.session, FakeChannels(), FakeRelay(), platform, MemoryPushStore()) { "Phone" } },
    )

    @Test
    fun theWorkerSyncsThePush() = runBlocking {
        val graph = graph()
        graph.session.state.first { it is SessionState.Ready }
        app.graph = graph
        val worker = TestListenableWorkerBuilder<PushCheckWorker>(context).build()
        assertEquals(ListenableWorker.Result.success(), worker.doWork())
        // The sync asked Google for the token: the subscription was checked and entered.
        assertEquals(1, platform.asked)
    }

    private fun planned(): List<WorkInfo> = WorkManager.getInstance(context).getWorkInfosForUniqueWork(PushCheckWorker.NAME).get()

    @Test
    fun onlyABuildWithFirebaseValuesPlansTheCheck() {
        PushStarter.start(app, graph(), configured = false)
        assertTrue(planned().isEmpty())
        PushStarter.start(app, graph(), configured = true)
        assertEquals(WorkInfo.State.ENQUEUED, planned().single().state)
    }
}
