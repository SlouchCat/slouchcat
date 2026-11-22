package com.slouchcat.app.mobile.downloads

import android.content.Context
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.DL1
import com.slouchcat.app.mobile.testing.DL2
import com.slouchcat.app.mobile.testing.FakeOfflineServer
import com.slouchcat.app.mobile.testing.downloadState
import com.slouchcat.app.mobile.testing.offlineItem
import com.slouchcat.app.mobile.testing.offlineProfile
import com.slouchcat.app.mobile.testing.options
import com.slouchcat.app.mobile.testing.testEngine
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.DownloadQuality
import com.slouchcat.core.api.generated.models.DownloadStateValue
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemImages
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeAccounts
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import com.slouchcat.core.browse.testing.HTTP_SERVER
import com.slouchcat.core.browse.testing.PRIMARY
import com.slouchcat.core.media.data.ItemRepository
import com.slouchcat.core.offline.OfflineAccount
import com.slouchcat.core.offline.OfflineEngine
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.StoredCredentials
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.runBlocking
import okhttp3.HttpUrl
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config

/** «Downloads» and the download dialog over a real engine (database in memory) and a server in memory. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class DownloadsViewModelTest {
    @get:Rule val temp = TemporaryFolder()
    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val server = FakeOfflineServer()
    private val account = OfflineData.accountKey(HTTP_SERVER.address.toString(), EMAIL)
    private var profile = 1L
    private val engine: OfflineEngine by lazy { testEngine(context, temp.newFolder("offline"), server, scope, { OfflineAccount(account, profile) }) }
    private val session = SessionManager(FakeServers(HTTP_SERVER), FakeTokens(StoredCredentials("token", "device-1")), FakeAccounts(EMAIL, PRIMARY))

    @After
    fun tearDown() = scope.cancel()

    /** Lets the main looper run while the IO work of the engine goes on, until [done] holds (at most 10 s). */
    private fun until(done: () -> Boolean) {
        val end = System.nanoTime() + 10_000_000_000L
        while (System.nanoTime() < end) {
            shadowOf(Looper.getMainLooper()).idle()
            if (done()) return
            Thread.onSpinWait()
        }
        assertTrue("condition not reached", done())
    }

    private fun model(): DownloadsViewModel {
        session.start(CoroutineScope(Dispatchers.Main))
        until { session.state.value !is com.slouchcat.core.session.SessionState.Loading }
        return DownloadsViewModel(engine, session, MemoryDeviceSettingsStore(), MutableStateFlow(null))
    }

    @Test
    fun rowsFollowTheServerAndOnlyTheProfilesAllowed() {
        server.syncAnswer = ApiResult.Ok(
            server.sync(
                downloads = listOf(
                    downloadState(DL1, offlineItem("i1", "Morgenrot"), DownloadStateValue.QUEUED),
                    downloadState(DL2, offlineItem("i2", "Lichtjahre"), allowed = listOf(2)),
                ),
            ),
        )
        val vm = model()
        until { vm.state.value.rows.isNotEmpty() }
        assertEquals(listOf("Morgenrot"), vm.state.value.rows.map { it.title })
        assertEquals(Section.Waiting, vm.state.value.rows.single().section)
        until { vm.state.value.profileName == "Mara" }
    }

    @Test
    fun deleteAsksThenDeletesOnTheServerAndTheDevice() {
        server.syncAnswer = ApiResult.Ok(server.sync(downloads = listOf(downloadState(DL1, offlineItem("i1", "Morgenrot"), DownloadStateValue.QUEUED))))
        val vm = model()
        until { vm.state.value.rows.isNotEmpty() }
        vm.onDeleteAsk(vm.state.value.rows)
        until { vm.state.value.sheet == DownloadsSheet.Delete }
        vm.onDeleteConfirm()
        until { vm.state.value.rows.isEmpty() }
        assertEquals(listOf(DL1), server.deletes)
    }

    @Test
    fun pauseKeepsTheDownloadAndResumeClearsIt() {
        server.syncAnswer = ApiResult.Ok(server.sync(downloads = listOf(downloadState(DL1, offlineItem("i1", "Morgenrot"), DownloadStateValue.QUEUED))))
        val vm = model()
        until { vm.state.value.rows.isNotEmpty() }
        vm.onPause(DL1)
        until { vm.state.value.rows.single().state is RowState.Paused }
        vm.onResume(DL1)
        until { vm.state.value.rows.single().state == RowState.Queued }
    }

    @Test
    fun theRuleLoadsAndSaves() {
        val vm = model()
        vm.onToggleAuto()
        until { vm.state.value.rule != null }
        vm.onRule { it.copy(enabled = true, maxCount = 40) }
        until { vm.state.value.rule?.rules?.enabled == true }
        assertEquals(DownloadsViewModel.MAX_COUNT, vm.state.value.rule?.rules?.maxCount)
        vm.onSaveRule()
        until { server.savedRules.isNotEmpty() }
        assertEquals(true, server.savedRules.single().enabled)
    }

    @Test
    fun anotherProfileOfflineNeedsItsPin() {
        server.syncAnswer = ApiResult.Ok(
            server.sync(
                downloads = listOf(downloadState(DL1, offlineItem("i1", "Morgenrot"), allowed = listOf(1, 2))),
                profiles = listOf(offlineProfile(1, "Mara"), offlineProfile(2, "Kim", hasPin = true)),
            ),
        )
        val vm = model()
        until { vm.state.value.offlineProfiles.size == 2 }
        vm.onPickProfile(vm.state.value.offlineProfiles.first { it.id == 2L })
        until { vm.state.value.pin != null }
        // No verifier on this device: the PIN has to be entered online once.
        assertTrue(vm.state.value.pin!!.unavailable)
        assertTrue(vm.state.value.pin.toString().contains("digits=0"))
    }

    private class Items(private val details: Map<String, ItemDetail>) : ItemRepository {
        override suspend fun item(server: HttpUrl, id: String) = details[id]?.let { ApiResult.Ok(it) } ?: ApiResult.NotFound
        override suspend fun setWatchlist(server: HttpUrl, id: String, on: Boolean) = ApiResult.Ok(Unit)
    }

    private fun detail(id: String, kind: ItemKind, title: String, parent: String? = null, episode: Int? = null, episodes: List<com.slouchcat.core.api.generated.models.ItemRef>? = null) = ItemDetail(
        id = id, kind = kind, title = title, inWatchlist = false, images = ItemImages(), recommendedByMe = false, canRecommend = false,
        collections = emptyList(), genres = emptyList(), mediaFiles = emptyList(), parentId = parent, episodeNumber = episode, seasonNumber = 1, episodes = episodes,
    )

    @Test
    fun theDialogOffersEpisodeNextThreeAndSeasonWithSizes() {
        val eps = (1..5).map { offlineItem("e$it", "Folge $it", ItemKind.EPISODE, parent = "s1", episode = it) }
        val season = detail("s1", ItemKind.SEASON, "Staffel 1", episodes = eps)
        listOf("e2", "e3", "e4").forEach { server.optionsFor[it] = options(DownloadQuality.HIGH to 100, DownloadQuality.MEDIUM to 50) }
        server.optionsFor["s1"] = options(DownloadQuality.HIGH to 500, DownloadQuality.MEDIUM to 250).copy(episodeCount = 5)
        val vm = DownloadDialogViewModel(engine, Items(mapOf("s1" to season)), MemoryDeviceSettingsStore(), { HTTP_SERVER.address }, "e2")
        vm.open(detail("e2", ItemKind.EPISODE, "Folge 2", parent = "s1", episode = 2))
        until { !vm.state.value.loading }
        val state = vm.state.value
        assertEquals(listOf(DownloadScope.One, DownloadScope.Next, DownloadScope.Season), state.scopes)
        assertEquals(listOf("e2", "e3", "e4"), state.next.map { it.id })
        assertEquals(100L, state.size)
        vm.onScope(DownloadScope.Next)
        until { vm.state.value.scope == DownloadScope.Next }
        assertEquals(300L, vm.state.value.size)
        vm.onQuality(DownloadQuality.MEDIUM)
        vm.onStart("Sternfahrt")
        until { server.created.size == 3 }
        assertEquals(listOf("e2", "e3", "e4"), server.created.map { it.first })
        assertEquals(DownloadQuality.MEDIUM, server.created.first().second.quality)
        runBlocking { until { runBlocking { engine.allDownloads() }.isNotEmpty() } }
    }

    @Test
    fun tooLittleSpaceBlocksTheStart() {
        server.optionsFor["m1"] = options(DownloadQuality.HIGH to Long.MAX_VALUE / 2)
        val vm = DownloadDialogViewModel(engine, Items(emptyMap()), MemoryDeviceSettingsStore(), { HTTP_SERVER.address }, "m1")
        vm.open(detail("m1", ItemKind.MOVIE, "Morgenrot"))
        until { !vm.state.value.loading }
        assertTrue(vm.state.value.tooLittle)
        vm.onStart("Morgenrot")
        until { true }
        assertTrue(server.created.isEmpty())
    }
}
