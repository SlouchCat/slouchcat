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
    private val server: () -> HttpUrl,
) : PlaybackServer {
    private fun items() = SlouchcatApi.create(::ItemsApi, server(), http)

    override suspend fun start(itemId: String, request: PlaybackRequest) = api.call("player") { items().startPlayback(itemId, request) }

    override suspend fun progress(itemId: String, event: ProgressEvent) = api.call("player") { items().reportProgress(itemId, event) }

    override suspend fun state(sessionId: String) = api.call("player") { items().getPlaybackSessionState(sessionId) }

    override suspend fun stop(sessionId: String) = api.call("player") { items().stopPlayback(sessionId) }

    override suspend fun trickplay(itemId: String, mediaFileId: String) = api.call("player") { items().getTrickplay(itemId, mediaFileId) }

    override suspend fun episodes(seasonId: String) = api.call("player") { items().getItem(seasonId) }.map { it.episodes.orEmpty() }

    override fun absolute(path: String): String? = ServerPaths.absolute(server(), path)
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
    private val server: () -> HttpUrl,
) : PlaybackPrefsRepository {
    private fun profiles() = SlouchcatApi.create(::ProfilesApi, server(), http)

    override suspend fun prefs() = api.call("playback-prefs") { profiles().getMyPlaybackPrefs() }

    override suspend fun save(prefs: PlaybackPrefs) = api.call("playback-prefs") { profiles().putMyPlaybackPrefs(prefs) }

    override suspend fun resetSeries() = api.call("playback-prefs") { profiles().resetMySeriesTracks() }
}
