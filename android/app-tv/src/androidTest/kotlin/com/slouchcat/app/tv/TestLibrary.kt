package com.slouchcat.app.tv

import com.slouchcat.core.api.SlouchcatApi
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import okhttp3.Cookie
import okhttp3.CookieJar
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody

/**
 * The administration as a browser uses it, only to add the library of the test media
 * (scripts/android-test-media.sh) and wait for its scan (as the phone app's DetailFlowTest does):
 * the TV may not call `admin` operations itself, so the test does it beside the app.
 */
internal class TestLibrary(private val base: HttpUrl, private val email: String, private val password: String, private val pin: String) {
    /**
     * The web session of the account, shared by every test of one run (one process): each sign-in
     * and profile choice counts against the server's limit of 10 attempts a minute per address, which
     * the TV's own sign-in and the phone of the script (apicheck) need.
     */
    private val cookies: MutableList<Cookie> = synchronized(SESSIONS) { SESSIONS.getOrPut("$base|$email") { mutableListOf() } }
    private val http = OkHttpClient.Builder()
        .cookieJar(object : CookieJar {
            override fun saveFromResponse(url: HttpUrl, cookies: List<Cookie>) {
                synchronized(this@TestLibrary.cookies) {
                    this@TestLibrary.cookies.removeAll { old -> cookies.any { it.name == old.name } }
                    this@TestLibrary.cookies += cookies
                }
            }

            override fun loadForRequest(url: HttpUrl) = synchronized(cookies) { cookies.toList() }
        })
        .build()

    /** Signs in with the primary profile, unless the shared web session still has one. */
    private fun signIn() {
        if (cookies.isNotEmpty() && runCatching { call("GET", "admin/libraries") }.isSuccess) return
        call("POST", "auth/login", """{"email":${Json.encodeToString(email)},"password":${Json.encodeToString(password)}}""")
        val primary = primary()
        val withPin = primary["hasPin"]?.jsonPrimitive?.boolean == true
        call("POST", "auth/profiles/${primary["id"]!!.jsonPrimitive.long}/select", if (withPin) """{"pin":"$pin"}""" else null)
    }

    private fun primary(): JsonObject = call("GET", "auth/profiles")!!.jsonArray.map { it.jsonObject }.single { it["isPrimary"]!!.jsonPrimitive.boolean }

    /** The primary profile with [pin], as a household has it (through the shared session, no sign-in of its own). */
    fun primaryWithPin() {
        signIn()
        call("PUT", "profiles/${primary()["id"]!!.jsonPrimitive.long}/pin", """{"pin":"$pin"}""")
    }

    private fun call(method: String, path: String, body: String? = null): JsonElement? {
        val request = Request.Builder()
            .url(base.newBuilder().addPathSegments(SlouchcatApi.BASE_PATH).addPathSegments(path).build())
            .header("X-Requested-With", "Slouchcat")
            .method(method, if (method == "GET") null else (body ?: "").toRequestBody("application/json".toMediaType()))
            .build()
        http.newCall(request).execute().use { response ->
            val text = response.body.string()
            check(response.isSuccessful) { "$method $path: ${response.code} $text" }
            return text.takeIf { it.isNotBlank() }?.let(Json::parseToJsonElement)
        }
    }

    private fun library(name: String): JsonObject? =
        call("GET", "admin/libraries")!!.jsonArray.map { it.jsonObject }.firstOrNull { it["name"]?.jsonPrimitive?.content == name }

    /** The library [name] over the films and series of [media], scanned. */
    fun ensure(media: String, name: String) {
        signIn()
        if (library(name) == null) {
            val movies = Json.encodeToString("$media/Movies")
            val tv = Json.encodeToString("$media/TV")
            call(
                "POST",
                "admin/libraries",
                """{"name":${Json.encodeToString(name)},"kind":"video","roots":[{"path":$movies,"content":"movie","providers":["tmdb"]},{"path":$tv,"content":"series","providers":["tmdb"]}]}""",
            )
        }
        val deadline = System.nanoTime() + SCAN_TIMEOUT_NS
        while (true) {
            val state = (library(name)!!["scan"] as JsonObject)["state"]!!.jsonPrimitive.content
            check(state != "failed" && state != "aborted") { "the scan of the test library $state" }
            if (state == "done") return
            check(System.nanoTime() < deadline) { "the scan of the test library did not finish" }
            Thread.sleep(POLL_MS)
        }
    }

    private companion object {
        /** The cookies of the web session per server and account (see [cookies]). */
        val SESSIONS = mutableMapOf<String, MutableList<Cookie>>()

        const val SCAN_TIMEOUT_NS = 120_000_000_000L
        const val POLL_MS = 250L
    }
}
