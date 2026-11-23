package com.slouchcat.core.media.data

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.models.AudioSeries
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.map
import com.slouchcat.core.media.Page
import okhttp3.Call
import okhttp3.HttpUrl

/**
 * What the pages of audiobooks and podcasts read beyond [DetailRepository] (concept 27 3.1, web
 * `SeriesPage` and `PodcastEpisodes`): the series of an audiobook with its volumes and the episodes
 * of a podcast in pages. Chapters come with `getItem` (`ItemDetail.chapters`). The server answers
 * only what the active profile sees; a series with fewer than two visible volumes is 404.
 */
interface AudioRepository {
    /** `getAudioSeries`: the volumes in their order, `next` to listen to. */
    suspend fun series(server: HttpUrl, libraryId: Long, seriesId: String): ApiResult<AudioSeries>

    /** `listPodcastEpisodes`: newest first, [limit] at most 200. */
    suspend fun episodes(server: HttpUrl, podcastId: String, offset: Int, limit: Int): ApiResult<Page<ItemRef>>
}

class ApiAudioRepository(private val api: ApiCaller, private val http: Call.Factory) : AudioRepository {
    override suspend fun series(server: HttpUrl, libraryId: Long, seriesId: String) = api.call("audio-series") {
        SlouchcatApi.create(::ItemsApi, server, http).getAudioSeries(libraryId, seriesId)
    }

    override suspend fun episodes(server: HttpUrl, podcastId: String, offset: Int, limit: Int) = api.call("podcast-episodes") {
        SlouchcatApi.create(::ItemsApi, server, http).listPodcastEpisodes(podcastId, offset, limit)
    }.map { Page(it.episodes, it.total) }
}
