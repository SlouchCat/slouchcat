package com.slouchcat.app.mobile.search

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.DiscoverApi
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.SearchRequest
import com.slouchcat.core.api.generated.models.SearchResult
import okhttp3.Call
import okhttp3.HttpUrl

/** What the search asks: a text of at least two characters, a genre, or both (`SearchRequest` without paging). */
data class SearchQuery(val text: String?, val genre: String?)

/**
 * The search of the active profile (`search`, `listGenres`). Every answer passed the visibility
 * layer on the server (rules 2 and 12); the app only shows it.
 */
interface SearchRepository {
    /** One page of titles; people and collections come with every page (the first ten). */
    suspend fun search(server: HttpUrl, query: SearchQuery, offset: Int, limit: Int): ApiResult<SearchResult>

    /** The genres of the visible titles, most frequent first (at most 20). */
    suspend fun genres(server: HttpUrl): ApiResult<List<GenreRef>>
}

class ApiSearchRepository(private val api: ApiCaller, private val http: Call.Factory) : SearchRepository {
    private fun discover(server: HttpUrl) = SlouchcatApi.create(::DiscoverApi, server, http)

    override suspend fun search(server: HttpUrl, query: SearchQuery, offset: Int, limit: Int) = api.call("search") {
        discover(server).search(SearchRequest(query = query.text, genre = query.genre, offset = offset, limit = limit))
    }

    override suspend fun genres(server: HttpUrl) = api.call("genres") { discover(server).listGenres() }
}
