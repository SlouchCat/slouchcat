package com.slouchcat.core.api

import com.slouchcat.core.api.generated.infrastructure.Serializer
import kotlinx.serialization.json.Json
import okhttp3.Call
import okhttp3.HttpUrl

/**
 * Entry to the generated client (package `com.slouchcat.core.api.generated`). Use it instead of the
 * generated constructors: touching this object configures the JSON of the client first, and the
 * base address of every operation comes from the server address.
 *
 * ```
 * val system = SlouchcatApi.create(::SystemApi, server, http)
 * val health = system.getHealth()
 * ```
 */
object SlouchcatApi {
    /** The path of the API below the server address (`servers` in api/openapi.yaml). */
    const val BASE_PATH = "api/v1"

    init {
        // Absent values stay absent instead of going out as `null` (stage 0, point 3). Unknown
        // fields are ignored by the generated configuration itself.
        Serializer.kotlinxSerializationJsonConfiguration = { explicitNulls = false }
    }

    /** The JSON of the generated client, configured. */
    val json: Json get() = Serializer.kotlinxSerializationJson

    /** `https://media.example.test/sub` -> `https://media.example.test/sub/api/v1`. */
    fun basePath(server: HttpUrl): String =
        server.newBuilder().addPathSegments(BASE_PATH).build().toString().trimEnd('/')

    /** One generated API class (`SystemApi`, `AuthApi` …) for [server] over the shared [client]. */
    fun <T> create(factory: (String, Call.Factory) -> T, server: HttpUrl, client: Call.Factory): T =
        factory(basePath(server), client)
}
