package com.slouchcat.core.player

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.ProfilesApi
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.PlaybackInfo
import com.slouchcat.core.api.generated.models.PlaybackPrefs
import com.slouchcat.core.api.generated.models.PlaybackRequest
import com.slouchcat.core.api.generated.models.PlaybackSessionState
import com.slouchcat.core.api.generated.models.Progress
import com.slouchcat.core.api.generated.models.ProgressEvent
import com.slouchcat.core.api.generated.models.Trickplay
import com.slouchcat.core.api.map
import okhttp3.Call
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/**
 * The operations of a playback (the seam of the tests: a fake server). Every call answers an
 * [ApiResult]; 401 and 426 reach the app's session events through [ApiCaller] as everywhere.
 */
interface PlaybackServer {
    /** `startPlayback`: the server decides direct, remux or transcode (rule 12). */
    suspend fun start(itemId: String, request: PlaybackRequest): ApiResult<PlaybackInfo>

    /** `reportProgress`. */
    suspend fun progress(itemId: String, event: ProgressEvent): ApiResult<Progress>

    /** `getPlaybackSessionState`: whether the server stopped the playback. */
    suspend fun state(sessionId: String): ApiResult<PlaybackSessionState>

    /** `stopPlayback`: revokes the stream token. */
    suspend fun stop(sessionId: String): ApiResult<Unit>

    /** `getTrickplay`: asked again while the set is `queued` or `running`. */
    suspend fun trickplay(itemId: String, mediaFileId: String): ApiResult<Trickplay>

    /** The episodes of a season (`getItem` of the season) for the episode list. */
    suspend fun episodes(seasonId: String): ApiResult<List<ItemRef>>

    /** A path of the server (`streamUrl`, `subtitleUrl`, a trickplay sheet) as an absolute address, `null` for another host. */
    fun absolute(path: String): String?
}

/** [PlaybackServer] over the generated client and the shared HTTP client of the app. */
class ApiPlaybackServer(
    private val api: ApiCaller,
    private val http: Call.Factory,
    /** The server of the session; `null` once signed out (a last stop after the sign-out ends quietly). */
    private val server: () -> HttpUrl?,
) : PlaybackServer {
    private suspend fun <T : Any> items(block: (ItemsApi) -> T?): ApiResult<T> {
        val base = server() ?: return ApiResult.SignedOut
        return api.call("player") { block(SlouchcatApi.create(::ItemsApi, base, http)) }
    }

    override suspend fun start(itemId: String, request: PlaybackRequest) = items { it.startPlayback(itemId, request) }

    override suspend fun progress(itemId: String, event: ProgressEvent) = items { it.reportProgress(itemId, event) }

    override suspend fun state(sessionId: String) = items { it.getPlaybackSessionState(sessionId) }

    override suspend fun stop(sessionId: String) = items { it.stopPlayback(sessionId) }

    override suspend fun trickplay(itemId: String, mediaFileId: String) = items { it.getTrickplay(itemId, mediaFileId) }

    override suspend fun episodes(seasonId: String) = items { it.getItem(seasonId) }.map { it.episodes.orEmpty() }

    override fun absolute(path: String): String? = server()?.let { ServerPaths.absolute(it, path) }
}

/** Paths of the server under its address, a sub path included (as `ServerImages` of the app). */
object ServerPaths {
    private const val API_PREFIX = "/api/v1/"

    /**
     * [path] under [server]; an absolute address only of the server's own origin (scheme, host,
     * port), so a token in it never travels to another host or unencrypted when the server is not.
     * Only `http` and `https` (OkHttp parses no other scheme).
     */
    fun absolute(server: HttpUrl, path: String): String? {
        if (path.isBlank()) return null
        val base = SlouchcatApi.basePath(server)
        val url = when {
            path.contains("://") -> path.toHttpUrlOrNull()?.takeIf { it.scheme == server.scheme && it.host == server.host && it.port == server.port }
            path.startsWith("//") -> null
            path.startsWith(API_PREFIX) -> (base + "/" + path.removePrefix(API_PREFIX)).toHttpUrlOrNull()
            else -> (base + "/" + path.trimStart('/')).toHttpUrlOrNull()
        }
        return url?.takeIf { it.scheme == "http" || it.scheme == "https" }?.toString()
    }
}

/** The language and subtitle preferences of the active profile (`getMyPlaybackPrefs` and the rest). */
interface PlaybackPrefsRepository {
    suspend fun prefs(): ApiResult<PlaybackPrefs>

    suspend fun save(prefs: PlaybackPrefs): ApiResult<PlaybackPrefs>

    /** `resetMySeriesTracks`: forget the languages chosen per series. */
    suspend fun resetSeries(): ApiResult<Unit>
}

class ApiPlaybackPrefsRepository(
    private val api: ApiCaller,
    private val http: Call.Factory,
    private val server: () -> HttpUrl?,
) : PlaybackPrefsRepository {
    private suspend fun <T : Any> profiles(block: (ProfilesApi) -> T?): ApiResult<T> {
        val base = server() ?: return ApiResult.SignedOut
        return api.call("playback-prefs") { block(SlouchcatApi.create(::ProfilesApi, base, http)) }
    }

    override suspend fun prefs() = profiles { it.getMyPlaybackPrefs() }

    override suspend fun save(prefs: PlaybackPrefs) = profiles { it.putMyPlaybackPrefs(prefs) }

    override suspend fun resetSeries() = profiles { it.resetMySeriesTracks() }
}
