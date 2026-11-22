package com.slouchcat.core.browse.testing

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.PersonRef
import com.slouchcat.core.api.generated.models.SearchResult
import com.slouchcat.core.media.data.SearchQuery
import com.slouchcat.core.media.data.SearchRepository
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CompletableDeferred
import okhttp3.HttpUrl

/** A search over [titles] (by text in the title, by [genreOf] for a genre), with people and collections. */
class FakeSearch(
    var titles: List<ItemRef> = emptyList(),
    var people: List<PersonRef> = emptyList(),
    var collections: List<CollectionRef> = emptyList(),
) : SearchRepository {
    var genres: ApiResult<List<GenreRef>> = ApiResult.Ok(listOf(GenreRef("Drama", 3), GenreRef("Comedy", 2)))

    /** When set, the answer fails with it. */
    var failure: ApiResult<SearchResult>? = null
    var genreOf: (ItemRef) -> String? = { null }

    /** Every request: text, genre, offset. */
    val asked = mutableListOf<Triple<String?, String?, Int>>()

    /** Requests that were cancelled while they waited for [gate]. */
    val cancelled = mutableListOf<String?>()

    /** When set, a request waits for it (an answer still under way). */
    var gate: CompletableDeferred<Unit>? = null

    override suspend fun search(server: HttpUrl, query: SearchQuery, offset: Int, limit: Int): ApiResult<SearchResult> {
        asked += Triple(query.text, query.genre, offset)
        try {
            gate?.await()
        } catch (e: CancellationException) {
            cancelled += query.text
            throw e
        }
        failure?.let { return it }
        val text = query.text?.lowercase()
        val found = titles.filter { (text == null || text in it.title.lowercase()) && (query.genre == null || genreOf(it) == query.genre) }
        val page = found.drop(offset).take(limit)
        val first = offset == 0
        return ApiResult.Ok(
            SearchResult(
                items = page,
                total = found.size,
                offset = offset,
                limit = limit,
                people = if (first && text != null) people.filter { text in it.name.lowercase() } else emptyList(),
                collections = if (first && text != null) collections.filter { text in it.name.lowercase() } else emptyList(),
            ),
        )
    }

    override suspend fun genres(server: HttpUrl) = genres
}

fun personRef(id: Long, name: String) = PersonRef(id, name, image = image("people/$id/image/w185", 185))

fun collectionRef(id: String, name: String, count: Int, cover: ItemRef? = null, seen: Int? = null) =
    CollectionRef(id = id, name = name, count = count, cover = cover, seenCount = seen, started = seen?.let { it > 0 })
