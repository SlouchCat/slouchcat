package com.slouchcat.app.mobile.showcase

import android.content.Context
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.downloads.OfflineData
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.DownloadList
import com.slouchcat.core.api.generated.models.DownloadManifest
import com.slouchcat.core.api.generated.models.DownloadOptions
import com.slouchcat.core.api.generated.models.DownloadQuality
import com.slouchcat.core.api.generated.models.DownloadQualityOption
import com.slouchcat.core.api.generated.models.DownloadRequest
import com.slouchcat.core.api.generated.models.DownloadRules
import com.slouchcat.core.api.generated.models.DownloadState
import com.slouchcat.core.api.generated.models.DownloadStateValue
import com.slouchcat.core.api.generated.models.ItemImages
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.Language
import com.slouchcat.core.api.generated.models.OfflineEventBatch
import com.slouchcat.core.api.generated.models.OfflineEventResult
import com.slouchcat.core.api.generated.models.OfflineProfile
import com.slouchcat.core.api.generated.models.OfflineSync
import com.slouchcat.core.api.generated.models.OfflineSyncRequest
import com.slouchcat.core.api.generated.models.RevocationList
import com.slouchcat.core.api.generated.models.SeriesRef
import com.slouchcat.core.offline.FileFetcher
import com.slouchcat.core.offline.NetworkKind
import com.slouchcat.core.offline.OfflineIdentity
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import com.slouchcat.core.offline.OfflineEngine
import com.slouchcat.core.offline.OfflineFiles
import com.slouchcat.core.offline.OfflineServer
import com.slouchcat.core.offline.OfflineStore
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import java.io.File
import java.time.OffsetDateTime

/**
 * Made-up downloads of the showcase (stage 6): a film on the device, an episode coming in, two
 * waiting, a paused one; an offline server switch; options of three qualities for the dialog.
 * No server, no files of real titles.
 */
object DownloadsShowcaseData {
    private const val GB = 1_000_000_000L
    private val ids = listOf("01HZY0000000000000000000A1", "01HZY0000000000000000000B2", "01HZY0000000000000000000C3", "01HZY0000000000000000000D4", "01HZY0000000000000000000E5")
    private val series = SeriesRef(id = "show-1", title = "Sternfahrt", images = ItemImages())

    private fun ref(id: String, title: String, kind: ItemKind = ItemKind.MOVIE, episode: Int? = null) = ItemRef(
        id = id, kind = kind, title = title, inWatchlist = false, images = ItemImages(), year = 2025,
        parentId = if (episode != null) "season-1" else null, seasonNumber = episode?.let { 1 }, episodeNumber = episode,
        seriesTitle = if (episode != null) series.title else null, seriesRef = if (episode != null) series else null,
    )

    private fun state(i: Int, item: ItemRef, value: DownloadStateValue, size: Long, done: Int = 0, error: String? = null) = DownloadState(
        id = ids[i], state = value, quality = if (i == 0) DownloadQuality.HIGH else DownloadQuality.MEDIUM, auto = i == 3, allowedProfileIds = listOf(SHOWCASE_PROFILE.id),
        estimatedBytes = size, segmentsDone = done, segmentsTotal = 100, createdAt = OffsetDateTime.now().minusHours(5L - i), sizeBytes = size,
        readyAt = if (value == DownloadStateValue.READY) OffsetDateTime.now() else null, item = item, error = error,
    )

    val states = listOf(
        state(0, ref("dl-film", "Nachtzug nach Lumen"), DownloadStateValue.READY, (4.2 * GB).toLong()),
        state(1, ref("dl-e2", "Treibgut", ItemKind.EPISODE, 2), DownloadStateValue.READY, (2.0 * GB).toLong()),
        state(2, ref("dl-e3", "Die Karte", ItemKind.EPISODE, 3), DownloadStateValue.PREPARING, (1.9 * GB).toLong(), done = 35),
        state(3, ref("dl-film2", "Fünf Brücken"), DownloadStateValue.QUEUED, (0.47 * GB).toLong(), error = "budget_full"),
        state(4, ref("dl-film3", "Stimmen im Schnee"), DownloadStateValue.READY, (1.1 * GB).toLong()),
    )

    /** The server of the showcase; [offline] makes every call fail as without network. */
    class Server : OfflineServer {
        @Volatile var offline = false
        @Volatile var downloads: List<DownloadState> = emptyList()

        private fun profiles() = listOf(
            OfflineProfile(id = SHOWCASE_PROFILE.id, name = SHOWCASE_PROFILE.name, language = Language.DE, isPrimary = true, hasPin = false),
            OfflineProfile(id = 2, name = "Kim", language = Language.DE, isPrimary = false, hasPin = true, pinSalt = "00112233445566778899aabbccddeeff"),
        )

        override suspend fun options(itemId: String, mediaFileId: String?): ApiResult<DownloadOptions> = if (offline) ApiResult.Offline else ApiResult.Ok(
            DownloadOptions(
                qualities = listOf(
                    DownloadQualityOption(DownloadQuality.HIGH, 1920, 1080, 6000, 128, (4.2 * GB).toLong(), copy = false),
                    DownloadQualityOption(DownloadQuality.MEDIUM, 1280, 720, 3000, 128, (1.8 * GB).toLong(), copy = false),
                    DownloadQualityOption(DownloadQuality.LOW, 854, 480, 1200, 96, 696_000_000, copy = false),
                ),
                audioTracks = emptyList(),
                subtitles = emptyList(),
                mediaFileId = "mf-$itemId",
                episodeCount = 4,
            ),
        )

        override suspend fun create(itemId: String, request: DownloadRequest): ApiResult<DownloadList> = ApiResult.Offline

        override suspend fun delete(downloadId: String): ApiResult<Unit> = ApiResult.Offline

        override suspend fun manifest(downloadId: String): ApiResult<DownloadManifest> = ApiResult.Offline

        override suspend fun sync(request: OfflineSyncRequest): ApiResult<OfflineSync> = if (offline) ApiResult.Offline else ApiResult.Ok(
            OfflineSync(serverTime = OffsetDateTime.now(), downloads = downloads, revoked = emptyList(), profiles = profiles(), rules = RULES),
        )

        override suspend fun rules(): ApiResult<DownloadRules> = if (offline) ApiResult.Offline else ApiResult.Ok(RULES)

        override suspend fun setRules(rules: DownloadRules): ApiResult<DownloadRules> = ApiResult.Ok(rules)

        override suspend fun revocations(ids: List<String>): ApiResult<RevocationList> = ApiResult.Ok(RevocationList(emptyList()))

        override suspend fun upload(batch: OfflineEventBatch): ApiResult<OfflineEventResult> = ApiResult.Ok(OfflineEventResult(0, 0, 0, emptyList()))

        override suspend fun accountId(): ApiResult<Long> = ApiResult.Ok(1)

        override fun absolute(path: String): HttpUrl? = "http://192.0.2.10:8096/".toHttpUrl().resolve(path)
    }

    val RULES = DownloadRules(enabled = true, nextEpisodes = true, newPodcastEpisodes = true, wifiOnly = true, maxCount = 3, quality = DownloadQuality.MEDIUM)

    /** The offline data of the showcase for [graph]: an engine over a database in memory and [server]. */
    fun data(context: Context, server: Server): (AppGraph) -> OfflineData = { graph ->
        val root = File(context.cacheDir, "showcase-offline").apply { deleteRecursively() }
        val engine = OfflineEngine(
            store = OfflineStore.inMemory(context),
            files = OfflineFiles(root),
            server = server,
            fetcher = FileFetcher(OkHttpClient(), { Long.MAX_VALUE }),
            session = { OfflineIdentity("showcase-key", "mara@example.test", SHOWCASE_PROFILE.id) },
            network = { if (server.offline) NetworkKind.None else NetworkKind.Wifi },
            wifiOnly = { true },
            scope = graph.scope,
        )
        OfflineData(engine, MemoryDeviceSettingsStore(), { if (server.offline) NetworkKind.None else NetworkKind.Wifi })
    }

    /**
     * After a sync: the film complete, the first episode at 40 %, the last one paused at 25 %. The
     * files have the sizes the list names (the storage adds up), the posters are drawn like every
     * other picture of the showcases.
     */
    suspend fun fill(engine: OfflineEngine, images: ShowcaseImages) {
        engine.updateDownload(ids[0]) { it?.copy(complete = true, bytesWritten = it.sizeBytes ?: 0, posterStored = true) }
        engine.updateDownload(ids[1]) { it?.copy(bytesWritten = (0.8 * GB).toLong(), posterStored = true) }
        engine.updateDownload(ids[4]) { it?.copy(paused = true, bytesWritten = (0.27 * GB).toLong(), posterStored = true) }
        engine.updateDownload(ids[2]) { it?.copy(posterStored = true) }
        engine.updateDownload(ids[3]) { it?.copy(posterStored = true) }
        withContext(Dispatchers.IO) {
            for (d in engine.allDownloads()) {
                engine.files.folder(d.id).mkdirs()
                // Sparse: the length counts, the disk stays empty.
                java.io.RandomAccessFile(engine.files.media(d.id), "rw").use { it.setLength(d.bytesWritten) }
                val item = d.item ?: continue
                val kind = if (item.episodeNumber != null) "backdrop" else "poster"
                engine.files.poster(d.id).outputStream().use { images.draw("/api/v1/items/${item.id}/images/$kind/w400").compress(android.graphics.Bitmap.CompressFormat.PNG, 100, it) }
            }
        }
    }

    /** The titles of the showcase's downloads for the drawn pictures. */
    val names: Map<String, String> = states.mapNotNull { it.item }.associate { it.id to it.title } + ("show-1" to "Sternfahrt")
}
