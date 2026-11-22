package com.slouchcat.app.mobile.downloads

import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isSelectable
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.FakeOfflineServer
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.offlineIdentity
import com.slouchcat.app.mobile.testing.options
import com.slouchcat.app.mobile.testing.testEngine
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.DownloadQuality
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemImages
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.browse.testing.HTTP_SERVER
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.data.ItemRepository
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.strings.R
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import okhttp3.HttpUrl
import org.junit.After
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/** The download dialog of a detail page (rule 8, code review m-9): two widths, 200 % font, every control labelled, the choice, Cancel closes. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class DownloadDialogScreenTest {
    @get:Rule val compose = createComposeRule()

    @get:Rule val temp = TemporaryFolder()

    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val server = FakeOfflineServer().apply {
        optionsFor["m1"] = options(DownloadQuality.HIGH to 4_200_000_000, DownloadQuality.MEDIUM to 1_800_000_000, DownloadQuality.LOW to 696_000_000)
    }

    @After
    fun tearDown() = scope.cancel()

    private val film = ItemDetail(
        id = "m1", kind = ItemKind.MOVIE, title = "Morgenrot", inWatchlist = false, images = ItemImages(), recommendedByMe = false,
        canRecommend = false, collections = emptyList(), genres = emptyList(), mediaFiles = emptyList(), year = 2026,
    )

    private val items = object : ItemRepository {
        override suspend fun item(server: HttpUrl, id: String) = ApiResult.NotFound
        override suspend fun setWatchlist(server: HttpUrl, id: String, on: Boolean) = ApiResult.Ok(Unit)
    }

    private fun dialog() {
        val engine = testEngine(context, temp.newFolder("offline"), server, scope, { offlineIdentity(1) })
        val vm = DownloadDialogViewModel(engine, items, MemoryDeviceSettingsStore(), { HTTP_SERVER.address }, "m1")
        vm.open(film)
        compose.setContent {
            val state by vm.state.collectAsStateWithLifecycle()
            SlouchcatTheme { DownloadDialog(state, film.title, "2026", vm) }
        }
        compose.waitUntil(10_000) { compose.onAllNodes(hasText(s(R.string.offline_quality_low), substring = true) and isSelectable()).fetchSemanticsNodes().isNotEmpty() }
        compose.onNode(hasText(s(R.string.app_downloads_dialogTitle)) and androidx.compose.ui.test.isHeading()).assertIsDisplayed()
        compose.onNode(hasText(s(R.string.offline_quality_high), substring = true) and isSelectable()).assertIsSelected()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNode(hasText(s(R.string.offline_quality_low), substring = true) and isSelectable()).performScrollTo().performClick()
        assertTrue(vm.state.value.quality == DownloadQuality.LOW)
        compose.onNode(hasText(s(R.string.common_cancel)) and hasClickAction()).performScrollTo().performClick()
        assertFalse(vm.state.value.open)
    }

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun dialogPhone() = dialog()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun dialogTablet() = dialog()

    @Test @Config(qualifiers = "w412dp-h915dp", fontScale = 2f)
    fun dialogPhoneLargeFont() = dialog()
}
