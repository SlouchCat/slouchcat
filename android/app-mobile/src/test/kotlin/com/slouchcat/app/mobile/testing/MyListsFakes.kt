package com.slouchcat.app.mobile.testing

import com.slouchcat.app.mobile.media.Page
import com.slouchcat.app.mobile.mylists.MyListsRepository
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.WatchedEntry
import okhttp3.HttpUrl
import java.time.OffsetDateTime

/** «Merkliste» and «Gesehen» in memory: removing takes a title off, adding puts it first. */
class FakeMyLists(
    var watchlist: MutableList<ItemRef> = mutableListOf(),
    var watched: MutableList<WatchedEntry> = mutableListOf(),
) : MyListsRepository {
    /** When set, every answer is this failure. */
    var failure: ApiResult<Nothing>? = null

    /** When set, only the changes fail (the lists still load). */
    var changeFailure: ApiResult<Nothing>? = null
    val calls = mutableListOf<String>()

    override suspend fun watchlist(server: HttpUrl, offset: Int, limit: Int): ApiResult<Page<ItemRef>> {
        calls += "watchlist:$offset:$limit"
        failure?.let { return it }
        return ApiResult.Ok(Page(watchlist.drop(offset).take(limit), watchlist.size))
    }

    override suspend fun setWatchlist(server: HttpUrl, id: String, on: Boolean): ApiResult<Unit> {
        calls += "set:$id:$on"
        (changeFailure ?: failure)?.let { return it }
        if (on) watchlist.add(0, itemRef(id.takeLast(4).toInt(), inWatchlist = true)) else watchlist.removeAll { it.id == id }
        return ApiResult.Ok(Unit)
    }

    override suspend fun watched(server: HttpUrl, offset: Int, limit: Int): ApiResult<Page<WatchedEntry>> {
        calls += "watched:$offset"
        failure?.let { return it }
        return ApiResult.Ok(Page(watched.drop(offset).take(limit), watched.size))
    }

    override suspend fun setWatched(server: HttpUrl, id: String, watched: Boolean): ApiResult<Unit> {
        calls += "watched:$id:$watched"
        (changeFailure ?: failure)?.let { return it }
        if (!watched) this.watched.removeAll { it.item.id == id }
        return ApiResult.Ok(Unit)
    }
}

fun watchedEntry(n: Int, title: String = "Title $n", episodes: Int? = null, watchedCount: Int? = null) = WatchedEntry(
    item = itemRef(n, title, kind = if (episodes != null) ItemKind.SERIES else ItemKind.MOVIE, images = posters(ulid(n))).copy(watchedCount = watchedCount),
    lastWatchedAt = OffsetDateTime.parse("2026-11-09T20:00:00Z"),
    episodeCount = episodes,
)
