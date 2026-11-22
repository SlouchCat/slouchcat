package com.slouchcat.core.offline

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.DownloadList
import com.slouchcat.core.api.generated.models.DownloadManifest
import com.slouchcat.core.api.generated.models.DownloadOptions
import com.slouchcat.core.api.generated.models.DownloadQuality
import com.slouchcat.core.api.generated.models.DownloadRequest
import com.slouchcat.core.api.generated.models.DownloadRules
import com.slouchcat.core.api.generated.models.DownloadSegment
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
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import java.time.OffsetDateTime
import java.time.ZoneOffset

// Made-up downloads, profiles and a server in memory for the tests of :core:offline.

const val SERVER_KEY = "server-key"
const val ACCOUNT = "$SERVER_KEY|#7"

/** The session of the tests: the server's key, the e-mail address, the active profile. */
fun identity(profile: Long? = 1) = OfflineIdentity(SERVER_KEY, "mara@example.test", profile)
const val D1 = "01HZY0000000000000000000A1"
const val D2 = "01HZY0000000000000000000B2"
const val D3 = "01HZY0000000000000000000C3"

fun at(ms: Long): OffsetDateTime = OffsetDateTime.ofInstant(java.time.Instant.ofEpochMilli(ms), ZoneOffset.UTC)

fun item(id: String = "i1", title: String = "Morgenrot", kind: ItemKind = ItemKind.MOVIE, parent: String? = null, episode: Int? = null) =
    ItemRef(id = id, kind = kind, title = title, inWatchlist = false, images = ItemImages(), parentId = parent, episodeNumber = episode)

fun state(
    id: String = D1,
    value: DownloadStateValue = DownloadStateValue.READY,
    allowed: List<Long> = listOf(1),
    item: ItemRef? = item(),
    created: Long = 1_000,
    readyAt: Long? = 2_000,
    auto: Boolean = false,
) = DownloadState(
    id = id,
    state = value,
    quality = DownloadQuality.MEDIUM,
    auto = auto,
    allowedProfileIds = allowed,
    estimatedBytes = 1000,
    segmentsDone = 0,
    createdAt = at(created),
    readyAt = readyAt?.let(::at),
    item = item,
    sizeBytes = 1000,
)

fun profile(id: Long = 1, name: String = "Mara", hasPin: Boolean = false, salt: String? = null) =
    OfflineProfile(id = id, name = name, language = Language.DE, isPrimary = id == 1L, hasPin = hasPin, pinSalt = salt)

val RULES = DownloadRules(enabled = false, nextEpisodes = true, newPodcastEpisodes = false, wifiOnly = true, maxCount = 3, quality = DownloadQuality.MEDIUM)

fun answer(
    downloads: List<DownloadState> = emptyList(),
    revoked: List<String> = emptyList(),
    profiles: List<OfflineProfile> = listOf(profile()),
    serverTime: Long = 10_000,
) = OfflineSync(serverTime = at(serverTime), downloads = downloads, revoked = revoked, profiles = profiles, rules = RULES)

/** A manifest of [size] bytes: init 10 bytes, segments of 100 bytes after it. */
fun manifest(id: String = D1, size: Long = 1010, subtitles: Boolean = false): DownloadManifest {
    val segments = (0 until ((size - 10) / 100).toInt()).map { DownloadSegment(startMs = it * 2000L, durationMs = 2000, offset = 10 + it * 100L, length = 100) }
    return DownloadManifest(
        id = id,
        itemId = "i1",
        sizeBytes = size,
        durationMs = segments.size * 2000L,
        mimeType = "video/mp4",
        codecs = "avc1.64001f,mp4a.40.2",
        width = 1280,
        height = 720,
        initLength = 10,
        segments = segments,
        fileUrl = "/api/v1/downloads/$id/file?dt=token",
        subtitles = if (subtitles) {
            listOf(
                com.slouchcat.core.api.generated.models.DownloadSubtitle(
                    track = com.slouchcat.core.api.generated.models.Track(
                        id = 7, index = 2, kind = com.slouchcat.core.api.generated.models.Track.Kind.SUBTITLE, codec = "subrip",
                        default = false, forced = false, source = com.slouchcat.core.api.generated.models.Track.Source.EMBEDDED, text = true, language = "de",
                    ),
                    url = "/api/v1/downloads/$id/subtitles/7.vtt?dt=token",
                ),
            )
        } else {
            emptyList()
        },
        chapters = emptyList(),
    )
}

/** The offline operations answered from memory; records what the engine sent. */
class FakeOfflineServer(var base: HttpUrl = "http://192.0.2.10:8096/".toHttpUrl()) : OfflineServer {
    var syncAnswer: ApiResult<OfflineSync> = ApiResult.Ok(answer())
    var revoked: ApiResult<RevocationList> = ApiResult.Ok(RevocationList(emptyList()))
    var deleteAnswer: ApiResult<Unit> = ApiResult.Ok(Unit)
    var created: ApiResult<DownloadList> = ApiResult.Ok(DownloadList(listOf(state(value = DownloadStateValue.QUEUED))))
    val manifests = mutableMapOf<String, DownloadManifest>()
    val syncs = mutableListOf<OfflineSyncRequest>()
    val revocationChecks = mutableListOf<List<String>>()
    val deletes = mutableListOf<String>()
    val uploads = mutableListOf<OfflineEventBatch>()

    override suspend fun options(itemId: String, mediaFileId: String?): ApiResult<DownloadOptions> = ApiResult.NotFound

    override suspend fun create(itemId: String, request: DownloadRequest) = created

    override suspend fun delete(downloadId: String): ApiResult<Unit> = deleteAnswer.also { deletes += downloadId }

    var manifestCalls = 0

    override suspend fun manifest(downloadId: String): ApiResult<DownloadManifest> = manifests[downloadId].also { manifestCalls++ }?.let { ApiResult.Ok(it) } ?: ApiResult.NotFound

    override suspend fun sync(request: OfflineSyncRequest): ApiResult<OfflineSync> = syncAnswer.also { syncs += request }

    override suspend fun rules(): ApiResult<DownloadRules> = ApiResult.Ok(RULES)

    override suspend fun setRules(rules: DownloadRules): ApiResult<DownloadRules> = ApiResult.Ok(rules)

    override suspend fun revocations(ids: List<String>): ApiResult<RevocationList> = revoked.also { revocationChecks += ids }

    override suspend fun upload(batch: OfflineEventBatch): ApiResult<OfflineEventResult> {
        uploads += batch
        return ApiResult.Ok(OfflineEventResult(accepted = batch.events.size, duplicates = 0, rejected = 0, results = emptyList()))
    }

    var accountAnswer: ApiResult<Long> = ApiResult.Ok(7)

    override suspend fun accountId(): ApiResult<Long> = accountAnswer

    override fun absolute(path: String): HttpUrl? = base.resolve(path)
}
