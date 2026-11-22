package com.slouchcat.core.media.data

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.RadioApi
import com.slouchcat.core.api.generated.models.RadioFacets
import com.slouchcat.core.api.generated.models.RadioNow
import com.slouchcat.core.api.generated.models.RadioPreferences
import com.slouchcat.core.api.generated.models.RadioSort
import com.slouchcat.core.api.generated.models.RadioStation
import com.slouchcat.core.api.map
import com.slouchcat.core.media.Page
import okhttp3.Call
import okhttp3.HttpUrl

/**
 * The filter of the station list (web `filters.ts`, the part the app offers): a search, a country,
 * the profile's favourites, the order. Every value goes to the server unchanged; the server answers
 * only the stations of the offer the administration chose.
 */
data class RadioQuery(
    val text: String? = null,
    val country: String? = null,
    val favorites: Boolean = false,
    val sort: RadioSort = RadioSort.POPULAR,
)

/**
 * The catalogue of the web radio for the active profile (concept 16, concept 19 part B; web
 * `features/radio/queries.ts`): stations in pages, the countries with their counts, the profile's
 * favourites («Meine Sender») and its default country, and what a station plays now (`getRadioNow`,
 * read by the server from the stream; the client never parses the stream itself). Listening
 * (`startRadioListen`, heartbeat, end) belongs to the player (`:core:player`, `RadioServer`).
 */
interface RadioRepository {
    /** `listRadioStations`: 503 `radio_catalog_loading` before the first sync of the catalogue. */
    suspend fun stations(server: HttpUrl, query: RadioQuery, offset: Int, limit: Int): ApiResult<Page<RadioStation>>

    /** `getRadioFacets` of [query] (the countries for the chips). */
    suspend fun facets(server: HttpUrl, query: RadioQuery): ApiResult<RadioFacets>

    /** `listRadioFavorites` in the profile's order. */
    suspend fun favorites(server: HttpUrl): ApiResult<List<RadioStation>>

    /** `addRadioFavorite` or `removeRadioFavorite`. */
    suspend fun setFavorite(server: HttpUrl, stationId: Long, on: Boolean): ApiResult<Unit>

    /** `getRadioPreferences`: `effectiveCountry` is empty when the profile's country has no station in the offer. */
    suspend fun preferences(server: HttpUrl): ApiResult<RadioPreferences>

    /** `getRadioNow`: the titles of the last minutes, newest last. */
    suspend fun now(server: HttpUrl, stationId: Long): ApiResult<RadioNow>
}

class ApiRadioRepository(private val api: ApiCaller, private val http: Call.Factory) : RadioRepository {
    private fun radio(server: HttpUrl) = SlouchcatApi.create(::RadioApi, server, http)

    override suspend fun stations(server: HttpUrl, query: RadioQuery, offset: Int, limit: Int) = api.call("radio-stations") {
        radio(server).listRadioStations(
            q = query.text?.takeIf { it.isNotBlank() },
            country = query.country?.let(::listOf),
            favorites = if (query.favorites) true else null,
            sort = query.sort,
            offset = offset,
            limit = limit.coerceAtMost(MAX_LIMIT),
        )
    }.map { Page(it.items, it.total) }

    override suspend fun facets(server: HttpUrl, query: RadioQuery) = api.call("radio-facets") {
        radio(server).getRadioFacets(
            q = query.text?.takeIf { it.isNotBlank() },
            favorites = if (query.favorites) true else null,
        )
    }

    override suspend fun favorites(server: HttpUrl) = api.call("radio-favorites") { radio(server).listRadioFavorites() }

    override suspend fun setFavorite(server: HttpUrl, stationId: Long, on: Boolean) = api.call("radio-favorite") {
        if (on) radio(server).addRadioFavorite(stationId) else radio(server).removeRadioFavorite(stationId)
    }

    override suspend fun preferences(server: HttpUrl) = api.call("radio-preferences") { radio(server).getRadioPreferences() }

    override suspend fun now(server: HttpUrl, stationId: Long) = api.call("radio-now") { radio(server).getRadioNow(stationId) }

    private companion object {
        /** The server's upper bound of a page of stations. */
        const val MAX_LIMIT = 120
    }
}
