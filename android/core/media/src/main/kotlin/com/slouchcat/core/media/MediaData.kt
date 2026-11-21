package com.slouchcat.core.media

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.media.data.ApiCollectionsRepository
import com.slouchcat.core.media.data.ApiDetailRepository
import com.slouchcat.core.media.data.ApiHomeRepository
import com.slouchcat.core.media.data.ApiLibraryRepository
import com.slouchcat.core.media.data.ApiMyListsRepository
import com.slouchcat.core.media.data.ApiPersonRepository
import com.slouchcat.core.media.data.ApiProfileSettingsRepository
import com.slouchcat.core.media.data.ApiSearchRepository
import com.slouchcat.core.media.data.CollectionsRepository
import com.slouchcat.core.media.data.DetailRepository
import com.slouchcat.core.media.data.HomeRepository
import com.slouchcat.core.media.data.ItemRepository
import com.slouchcat.core.media.data.LibraryRepository
import com.slouchcat.core.media.data.MyListsRepository
import com.slouchcat.core.media.data.PersonRepository
import com.slouchcat.core.media.data.ProfileSettingsRepository
import com.slouchcat.core.media.data.SearchRepository
import okhttp3.Call
import java.util.WeakHashMap

/**
 * Everything a page of the phone or the TV app reads about titles, people, lists and profiles: the
 * repositories over the generated client and the counts the shell shows next to its entries. One
 * per app graph ([of]); the repositories are stateless, the counts live as long as the graph.
 *
 * A page never builds a repository itself; it takes the one of its graph
 * (`MediaData.of(graph) { MediaData.api(graph.api, graph.http) }`, in the apps behind an extension
 * such as `graph.media`). A test or the showcase of androidTest puts others in place with [install]
 * before the page is drawn (`MediaData.install(graph, MediaData.api(api, http).copy(home = FakeHome()))`).
 * Every answer passed the visibility layer of the active profile on the server; not visible is 404
 * (`ApiResult.NotFound`, rules 2 and 12).
 */
data class MediaData(
    val home: HomeRepository,
    val library: LibraryRepository,
    /** `getItem` and the watchlist; the start page and the lists change «Merken» through it. */
    val items: ItemRepository,
    /** The detail pages: [items] plus trailers, «gesehen» and «Empfehlen». */
    val detail: DetailRepository,
    val person: PersonRepository,
    val search: SearchRepository,
    val myLists: MyListsRepository,
    val collections: CollectionsRepository,
    val profiles: ProfileSettingsRepository,
    /** «Merkliste» in the media choice and «Mehr»: every page that changes the list adjusts it. */
    val watchlistCount: LiveCount = LiveCount(),
    /** «Sammlungen» in the media choice. */
    val collectionsCount: LiveCount = LiveCount(),
    /** «Profile verwalten» in «Mehr». */
    val profilesCount: LiveCount = LiveCount(),
) {
    /** Forgets the counts (another profile, signed out): the shell reads them again for the next one. */
    fun clearCounts() {
        watchlistCount.set(null)
        collectionsCount.set(null)
        profilesCount.set(null)
    }

    companion object {
        private val installed = WeakHashMap<Any, MediaData>()

        /** The repositories over the generated client of one app graph. */
        fun api(api: ApiCaller, http: Call.Factory): MediaData {
            val detail = ApiDetailRepository(api, http)
            return MediaData(
                home = ApiHomeRepository(api, http),
                library = ApiLibraryRepository(api, http),
                items = detail,
                detail = detail,
                person = ApiPersonRepository(api, http),
                search = ApiSearchRepository(api, http),
                myLists = ApiMyListsRepository(api, http),
                collections = ApiCollectionsRepository(api, http),
                profiles = ApiProfileSettingsRepository(api, http),
            )
        }

        /** What [graph] reads through: what [install] put in place, else [create] once. */
        fun of(graph: Any, create: () -> MediaData): MediaData = synchronized(installed) { installed.getOrPut(graph, create) }

        /** For tests: [graph] reads [data] instead of the server. */
        fun install(graph: Any, data: MediaData) = synchronized(installed) { installed[graph] = data }
    }
}
