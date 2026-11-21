package com.slouchcat.core.offline

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.OfflineApi
import com.slouchcat.core.api.generated.models.DownloadList
import com.slouchcat.core.api.generated.models.DownloadManifest
import com.slouchcat.core.api.generated.models.DownloadOptions
import com.slouchcat.core.api.generated.models.DownloadRequest
import com.slouchcat.core.api.generated.models.DownloadRules
import com.slouchcat.core.api.generated.models.OfflineEventBatch
import com.slouchcat.core.api.generated.models.OfflineEventResult
import com.slouchcat.core.api.generated.models.OfflineSync
import com.slouchcat.core.api.generated.models.OfflineSyncRequest
import com.slouchcat.core.api.generated.models.RevocationCheck
import com.slouchcat.core.api.generated.models.RevocationList
import com.slouchcat.core.player.ServerPaths
import okhttp3.Call
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/**
 * The offline operations of the contract (concept 27 2.4) the app uses as they are; the seam of the
 * tests. Every call answers an [ApiResult]; 401 and 426 reach the app's session events as everywhere.
 */
interface OfflineServer {
    suspend fun options(itemId: String, mediaFileId: String? = null): ApiResult<DownloadOptions>

    suspend fun create(itemId: String, request: DownloadRequest): ApiResult<DownloadList>

    suspend fun delete(downloadId: String): ApiResult<Unit>

    suspend fun manifest(downloadId: String): ApiResult<DownloadManifest>

    suspend fun sync(request: OfflineSyncRequest): ApiResult<OfflineSync>

    suspend fun rules(): ApiResult<DownloadRules>

    suspend fun setRules(rules: DownloadRules): ApiResult<DownloadRules>

    /** `checkOfflineRevocations`: public, the device names itself in `X-Slouchcat-Device` without a session. */
    suspend fun revocations(ids: List<String>): ApiResult<RevocationList>

    suspend fun upload(batch: OfflineEventBatch): ApiResult<OfflineEventResult>

    /** A path of the server (`fileUrl`, a subtitle, a sheet, a poster) under its address; `null` for another host. */
    fun absolute(path: String): HttpUrl?
}

class ApiOfflineServer(
    private val api: ApiCaller,
    private val http: Call.Factory,
    /** The server of the device (stays after a sign-out: the revocation list needs it). */
    private val server: () -> HttpUrl?,
) : OfflineServer {
    private suspend fun <T : Any> offline(block: (OfflineApi) -> T?): ApiResult<T> {
        val base = server() ?: return ApiResult.SignedOut
        return api.call("offline") { block(SlouchcatApi.create(::OfflineApi, base, http)) }
    }

    override suspend fun options(itemId: String, mediaFileId: String?) = offline { it.getDownloadOptions(itemId, mediaFileId) }

    override suspend fun create(itemId: String, request: DownloadRequest) = offline { it.createDownloads(itemId, request) }

    override suspend fun delete(downloadId: String) = offline { it.deleteDownload(downloadId) }

    override suspend fun manifest(downloadId: String) = offline { it.getDownloadManifest(downloadId) }

    override suspend fun sync(request: OfflineSyncRequest) = offline { it.syncOffline(request) }

    override suspend fun rules() = offline { it.getDownloadRules() }

    override suspend fun setRules(rules: DownloadRules) = offline { it.setDownloadRules(rules) }

    override suspend fun revocations(ids: List<String>) = offline { it.checkOfflineRevocations(RevocationCheck(ids.take(MAX_HELD))) }

    override suspend fun upload(batch: OfflineEventBatch) = offline { it.uploadOfflineEvents(batch) }

    override fun absolute(path: String): HttpUrl? = server()?.let { ServerPaths.absolute(it, path) }?.toHttpUrlOrNull()
}
