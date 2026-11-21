package com.slouchcat.core.media.data

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.models.Person
import okhttp3.Call
import okhttp3.HttpUrl

/** A person with the titles the profile sees. */
interface PersonRepository {
    /** `getPerson`. */
    suspend fun person(server: HttpUrl, id: Long): ApiResult<Person>
}

class ApiPersonRepository(private val api: ApiCaller, private val http: Call.Factory) : PersonRepository {
    override suspend fun person(server: HttpUrl, id: Long) = api.call("person") { SlouchcatApi.create(::ItemsApi, server, http).getPerson(id) }
}
