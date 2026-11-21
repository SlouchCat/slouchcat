package com.slouchcat.app.mobile.testing

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Collection
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.media.data.CollectionsRepository
import kotlinx.coroutines.CompletableDeferred
import okhttp3.HttpUrl

/** Collections in memory; [parts] per collection id. */
class FakeCollections(
    var list: ApiResult<List<CollectionRef>> = ApiResult.Ok(emptyList()),
    val parts: MutableMap<String, MutableList<ItemRef>> = mutableMapOf(),
) : CollectionsRepository {
    var seen: MutableMap<String, Int> = mutableMapOf()
    var failure: ApiResult<Nothing>? = null
    var changeFailure: ApiResult<Nothing>? = null
    val calls = mutableListOf<String>()

    /** Holds every watchlist change until a test completes it. */
    var watchlistGate: CompletableDeferred<Unit>? = null

    override suspend fun collections(server: HttpUrl): ApiResult<List<CollectionRef>> {
        calls += "list"
        return failure ?: list
    }

    override suspend fun collection(server: HttpUrl, id: String, offset: Int, limit: Int): ApiResult<Collection> {
        calls += "get:$id:$offset"
        failure?.let { return it }
        val all = parts[id] ?: return ApiResult.NotFound
        val page = all.drop(offset).take(limit)
        val name = (list as? ApiResult.Ok)?.value?.firstOrNull { it.id == id }?.name ?: "Collection $id"
        val next = all.firstOrNull()
        return ApiResult.Ok(
            Collection(
                id = id, name = name, items = page, total = all.size, offset = offset, limit = limit,
                seenCount = seen[id] ?: 0, started = (seen[id] ?: 0) > 0, next = next, nextPart = next?.let { 1 },
            ),
        )
    }

    override suspend fun setWatched(server: HttpUrl, id: String, watched: Boolean): ApiResult<Unit> {
        calls += "watched:$id:$watched"
        (changeFailure ?: failure)?.let { return it }
        seen[id] = if (watched) parts[id]?.size ?: 0 else 0
        return ApiResult.Ok(Unit)
    }

    override suspend fun setWatchlist(server: HttpUrl, itemId: String, on: Boolean): ApiResult<Unit> {
        calls += "watchlist:$itemId:$on"
        watchlistGate?.await()
        (changeFailure ?: failure)?.let { return it }
        parts.values.forEach { list -> list.replaceAll { if (it.id == itemId) it.copy(inWatchlist = on) else it } }
        return ApiResult.Ok(Unit)
    }
}
