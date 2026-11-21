package com.slouchcat.core.session

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.SystemApi
import com.slouchcat.core.api.generated.models.Health
import okhttp3.Call
import okhttp3.HttpUrl

/**
 * The server's own operations for a page. The pattern for every repository: an interface (the
 * seam a ViewModel test fakes) and one implementation that calls the generated client through
 * [ApiCaller], so every answer arrives as [ApiResult].
 */
interface ServerRepository {
    suspend fun health(server: HttpUrl): ApiResult<Health>
}

class ApiServerRepository(private val api: ApiCaller, private val http: Call.Factory) : ServerRepository {
    override suspend fun health(server: HttpUrl): ApiResult<Health> =
        api.call("server") { SlouchcatApi.create(::SystemApi, server, http).getHealth() }
}
