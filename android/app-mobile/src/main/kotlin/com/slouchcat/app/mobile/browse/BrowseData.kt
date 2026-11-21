package com.slouchcat.app.mobile.browse

import com.slouchcat.app.mobile.AppGraph
import java.util.WeakHashMap

/**
 * The repositories of stage 4 for one [AppGraph]: over the generated client unless a test put
 * others in place ([install]; the showcase flow of androidTest shows the real pages with made-up
 * data). The graph stays untouched, so every area can keep its own seam in a file of its own.
 */
class BrowseData(val home: HomeRepository, val library: LibraryRepository, val items: ItemRepository) {
    companion object {
        private val installed = WeakHashMap<AppGraph, BrowseData>()

        /** What [graph] reads through; the server's unless [install] put something else in place. */
        fun of(graph: AppGraph): BrowseData = synchronized(installed) {
            installed.getOrPut(graph) {
                BrowseData(ApiHomeRepository(graph.api, graph.http), ApiLibraryRepository(graph.api, graph.http), ApiItemRepository(graph.api, graph.http))
            }
        }

        /** For tests: [graph] reads [data] instead of the server. */
        fun install(graph: AppGraph, data: BrowseData) = synchronized(installed) { installed[graph] = data }
    }
}

/** The repositories of stage 4 of this graph. */
val AppGraph.browse: BrowseData get() = BrowseData.of(this)
