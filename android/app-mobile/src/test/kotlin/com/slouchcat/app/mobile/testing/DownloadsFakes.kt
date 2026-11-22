package com.slouchcat.app.mobile.testing

import android.content.Context
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
import com.slouchcat.core.browse.testing.account
import com.slouchcat.core.browse.testing.episode
import com.slouchcat.core.browse.testing.profile
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.offline.FileFetcher
import com.slouchcat.core.offline.NetworkKind
import com.slouchcat.core.offline.OfflineEngine
import com.slouchcat.core.offline.OfflineFiles
import com.slouchcat.core.offline.OfflineServer
import com.slouchcat.core.offline.OfflineSession
import com.slouchcat.core.offline.OfflineStore
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import kotlinx.coroutines.CoroutineScope
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import java.io.File
import java.time.OffsetDateTime
import java.time.ZoneOffset

// The downloads of stage 6 in memory: an engine over an in-memory database and a server that
// answers from fields. Only made-up titles.

const val DL1 = "01HZY0000000000000000000A1"
const val DL2 = "01HZY0000000000000000000B2"
const val DL3 = "01HZY0000000000000000000C3"

fun offlineTime(ms: Long): OffsetDateTime = OffsetDateTime.ofInstant(java.time.Instant.ofEpochMilli(ms), ZoneOffset.UTC)

fun offlineItem(id: String, title: String, kind: ItemKind = ItemKind.MOVIE, parent: String? = null, episode: Int? = null) =
    ItemRef(id = id, kind = kind, title = title, inWatchlist = false, images = ItemImages(), parentId = parent, episodeNumber = episode, seasonNumber = episode?.let { 1 }, seriesTitle = parent?.let { "Sternfahrt" })

fun downloadState(id: String, item: ItemRef, value: DownloadStateValue = DownloadStateValue.READY, allowed: List<Long> = listOf(1), created: Long = 1_000) = DownloadState(
    id = id, state = value, quality = DownloadQuality.MEDIUM, auto = false, allowedProfileIds = allowed, estimatedBytes = 1_000,
    segmentsDone = 0, createdAt = offlineTime(created), readyAt = offlineTime(2_000), item = item, sizeBytes = 1_000,
)

fun offlineProfile(id: Long, name: String, hasPin: Boolean = false) = OfflineProfile(id = id, name = name, language = Language.DE, isPrimary = id == 1L, hasPin = hasPin)

val OFFLINE_RULES = DownloadRules(enabled = false, nextEpisodes = true, newPodcastEpisodes = false, wifiOnly = true, maxCount = 3, quality = DownloadQuality.MEDIUM)

fun options(vararg sizes: Pair<DownloadQuality, Long>) = DownloadOptions(
    qualities = sizes.map { (q, bytes) -> DownloadQualityOption(quality = q, width = 1280, height = if (q == DownloadQuality.HIGH) 1080 else 720, videoBitrate = 3000, audioBitrate = 128, estimatedBytes = bytes, copy = false) },
    audioTracks = emptyList(),
    subtitles = emptyList(),
    mediaFileId = "mf",
)

class FakeOfflineServer : OfflineServer {
    var syncAnswer: ApiResult<OfflineSync> = ApiResult.Ok(sync())
    val optionsFor = mutableMapOf<String, DownloadOptions>()
    val created = mutableListOf<Pair<String, DownloadRequest>>()
    var createAnswer: (String) -> ApiResult<DownloadList> = { id -> ApiResult.Ok(DownloadList(listOf(downloadState(DL3, offlineItem(id, "Neu"), DownloadStateValue.QUEUED)))) }
    val deletes = mutableListOf<String>()
    var rulesAnswer: ApiResult<DownloadRules> = ApiResult.Ok(OFFLINE_RULES)
    val savedRules = mutableListOf<DownloadRules>()
    var revoked: List<String> = emptyList()

    fun sync(downloads: List<DownloadState> = emptyList(), profiles: List<OfflineProfile> = listOf(offlineProfile(1, "Mara")), revoked: List<String> = emptyList()) =
        OfflineSync(serverTime = offlineTime(10_000), downloads = downloads, revoked = revoked, profiles = profiles, rules = OFFLINE_RULES)

    override suspend fun options(itemId: String, mediaFileId: String?): ApiResult<DownloadOptions> = optionsFor[itemId]?.let { ApiResult.Ok(it) } ?: ApiResult.NotFound

    override suspend fun create(itemId: String, request: DownloadRequest): ApiResult<DownloadList> {
        created += itemId to request
        return createAnswer(itemId)
    }

    override suspend fun delete(downloadId: String): ApiResult<Unit> = ApiResult.Ok(Unit).also { deletes += downloadId }

    override suspend fun manifest(downloadId: String): ApiResult<DownloadManifest> = ApiResult.NotFound

    override suspend fun sync(request: OfflineSyncRequest): ApiResult<OfflineSync> = syncAnswer

    override suspend fun rules(): ApiResult<DownloadRules> = rulesAnswer

    override suspend fun setRules(rules: DownloadRules): ApiResult<DownloadRules> = ApiResult.Ok(rules).also { savedRules += rules }

    override suspend fun revocations(ids: List<String>): ApiResult<RevocationList> = ApiResult.Ok(RevocationList(revoked))

    override suspend fun upload(batch: OfflineEventBatch): ApiResult<OfflineEventResult> =
        ApiResult.Ok(OfflineEventResult(accepted = batch.events.size, duplicates = 0, rejected = 0, results = emptyList()))

    override suspend fun accountId(): ApiResult<Long> = ApiResult.Ok(7)

    override fun absolute(path: String): HttpUrl? = "http://192.0.2.10:8096/".toHttpUrl().resolve(path)
}

/** An engine over an in-memory database for [account] with the active profile [profile]. */
fun testEngine(
    context: Context,
    root: File,
    server: OfflineServer,
    scope: CoroutineScope,
    session: OfflineSession,
    network: () -> NetworkKind = { NetworkKind.Wifi },
    clock: () -> Long = { 5_000 },
): OfflineEngine {
    val files = OfflineFiles(root)
    return OfflineEngine(
        store = OfflineStore.inMemory(context),
        files = files,
        server = server,
        fetcher = FileFetcher(OkHttpClient(), { Long.MAX_VALUE }),
        session = session,
        network = network,
        wifiOnly = { false },
        scope = scope,
        clock = clock,
        elapsed = clock,
        boot = { 1 },
    )
}

fun testOfflineData(engine: OfflineEngine) = OfflineData(engine, MemoryDeviceSettingsStore(), { NetworkKind.Wifi })

/** The server's key of the fakes and the account the fake server names. */
const val OFFLINE_SERVER = "server-key"
const val OFFLINE_ACCOUNT = "$OFFLINE_SERVER|#7"

fun offlineIdentity(profile: Long? = 1) = com.slouchcat.core.offline.OfflineIdentity(OFFLINE_SERVER, "viewer@example.test", profile)
