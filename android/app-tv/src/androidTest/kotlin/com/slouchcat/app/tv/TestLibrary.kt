package com.slouchcat.app.tv

import com.slouchcat.core.api.SlouchcatApi
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import kotlinx.serialization.json.longOrNull
import okhttp3.Cookie
import okhttp3.CookieJar
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody

/**
 * The administration as a browser uses it, only to add the library of the test media
 * (scripts/android-test-media.sh) and wait for its scan (as the phone app's DetailFlowTest does),
 * and for stage 5 to connect the Seerr mock, let it decline a wish and publish an announcement:
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

    /** A call of the server's API in the web session; [at] another service (the Seerr mock) with [client]. */
    private fun call(method: String, path: String, body: String? = null, client: OkHttpClient = http, at: HttpUrl? = null): JsonElement? {
        val url = at?.newBuilder()?.addPathSegments(path)?.build()
            ?: base.newBuilder().addPathSegments(SlouchcatApi.BASE_PATH).addPathSegments(path.substringBefore('?'))
                .encodedQuery(path.substringAfter('?', "").ifEmpty { null }).build()
        val request = Request.Builder()
            .url(url)
            .header("X-Requested-With", "Slouchcat")
            .method(method, if (method == "GET") null else (body ?: "").toRequestBody("application/json".toMediaType()))
            .build()
        client.newCall(request).execute().use { response ->
            val text = response.body.string()
            check(response.isSuccessful) { "$method $path: ${response.code} $text" }
            return text.takeIf { it.isNotBlank() }?.let(Json::parseToJsonElement)
        }
    }

    private fun library(name: String): JsonObject? =
        call("GET", "admin/libraries")!!.jsonArray.map { it.jsonObject }.firstOrNull { it["name"]?.jsonPrimitive?.content == name }

    /** The library [name] over the films and series of [media], scanned. */
    fun ensure(media: String, name: String) = ensure(media to name)

    /** Each library (folder to name), scanned, in the one web session of the run ([signIn]). */
    fun ensure(vararg libraries: Pair<String, String>) {
        signIn()
        for ((media, name) in libraries) scan(media, name)
    }

    private fun scan(media: String, name: String) {
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

    // --- Stage 5: wishes through the Seerr mock of web/e2e, an announcement -----------------------

    private var account: Long? = null

    /** The client for the Seerr mock (no session of the server), one for the run (code review L-11). */
    private val plain = OkHttpClient()

    /**
     * TMDB key, Seerr connected ([seerr] as the server reaches it), wishes on, the account linked to
     * Seerr's user 3 (the phone app's MessagesFlowTest, the web e2e's `openWishes`).
     */
    fun wishesOn(seerr: String) {
        signIn()
        call("PUT", "admin/settings", """{"keys":{"tmdb":"$TMDB_KEY"}}""")
        call("PUT", "admin/seerr", """{"url":"$seerr","apiKey":"$SEERR_KEY","enabled":true}""")
        call("POST", "admin/seerr/test", "{}")
        call("PUT", "admin/wishes/settings", """{"enabled":true}""")
        val id = call("GET", "auth/session")!!.jsonObject["account"]!!.jsonObject["id"]!!.jsonPrimitive.long
        account = id
        call("POST", "admin/wishes/mappings/users/refresh", "{}")
        val deadline = System.nanoTime() + SCAN_TIMEOUT_NS
        while (call("GET", "admin/wishes/mappings/users")!!.jsonArray.none { it.jsonObject["id"]!!.jsonPrimitive.long == SEERR_USER }) {
            check(System.nanoTime() < deadline) { "Seerr's users did not come" }
            Thread.sleep(POLL_MS)
        }
        call("PUT", "admin/wishes/mappings/accounts/$id", """{"seerrUserId":$SEERR_USER}""")
    }

    /** Seerr ([seerrHere] as the device reaches it) declines every open request; the server's sync closes the wishes. */
    fun declineOpen(seerrHere: HttpUrl) {
        signIn()
        val items = (call("GET", "admin/wishes?state=open&limit=100")?.jsonObject?.get("items") as? JsonArray).orEmpty().map { it.jsonObject }
        for (wish in items) {
            val ids = listOfNotNull(wish["seerrRequestId"]?.jsonPrimitive?.longOrNull) +
                (wish["seasons"] as? JsonArray).orEmpty().mapNotNull { it.jsonObject["seerrRequestId"]?.jsonPrimitive?.longOrNull }
            ids.forEach { call("POST", "__control", """{"request":{"id":$it,"status":3}}""", plain, seerrHere) }
        }
        if (items.isNotEmpty()) call("POST", "admin/seerr/sync", "{}")
    }

    /** An announcement of the main user, published at once; its id. */
    fun announce(title: String, body: String): Long {
        signIn()
        val made = call("POST", "admin/announcements", """{"title":${Json.encodeToString(title)},"body":${Json.encodeToString(body)},"publish":true}""")
        return made!!.jsonObject["id"]!!.jsonPrimitive.long
    }

    /** Everything [wishesOn] and [announce] changed, back; each part on its own (also after a failure). */
    fun wishesOff(seerrHere: HttpUrl, announcement: Long?) {
        runCatching { signIn() }
        runCatching { declineOpen(seerrHere) }
        announcement?.let { id -> runCatching { call("DELETE", "admin/announcements/$id") } }
        account?.let { id -> runCatching { call("PUT", "admin/wishes/mappings/accounts/$id", """{"seerrUserId":null}""") } }
        runCatching { call("PUT", "admin/wishes/settings", """{"enabled":false}""") }
        runCatching { call("PUT", "admin/seerr", """{"clear":true,"enabled":false}""") }
        runCatching { call("PUT", "admin/settings", """{"clear":["tmdb"]}""") }
        runCatching { call("POST", "__reset", "{}", plain, seerrHere) }
    }

    /**
     * Stage 4: an audio library [name] of [kind] (`audiobooks`, `podcasts`) over [folder] with
     * [content] (`audiobook`, `podcast`), local metadata only, scanned.
     */
    fun ensureAudio(name: String, kind: String, folder: String, content: String) {
        signIn()
        if (library(name) == null) {
            call(
                "POST",
                "admin/libraries",
                """{"name":${Json.encodeToString(name)},"kind":"$kind","roots":[{"path":${Json.encodeToString(folder)},"content":"$content","providers":["local"]}]}""",
            )
        }
        waitForScan(name)
    }

    /** Stage 4 with `RADIO=1`: the test stations (first catalog sync) and an offer of every country, as the phone's AudioFlowTest. */
    fun ensureRadio() {
        signIn()
        if (call("GET", "admin/radio/catalog")!!.jsonObject["stationsTotal"]!!.jsonPrimitive.int == 0) {
            call("POST", "admin/radio/catalog/sync")
            val deadline = System.nanoTime() + SCAN_TIMEOUT_NS
            while (call("GET", "admin/radio/catalog")!!.jsonObject["stationsTotal"]!!.jsonPrimitive.int == 0) {
                check(System.nanoTime() < deadline) { "the radio catalog did not arrive" }
                Thread.sleep(POLL_MS)
            }
        }
        val offer = call("GET", "admin/radio/offer")!!.jsonObject
        val steps = offer["countries"]!!.jsonArray.map { it.jsonObject }.filter { it["total"]!!.jsonPrimitive.int > 0 }.flatMap {
            val code = it["code"]!!.jsonPrimitive.content
            listOf("""{"op":"chooseCountry","country":"$code"}""", """{"op":"markAll","country":"$code"}""")
        }
        call("POST", "admin/radio/offer/apply", """{"version":${offer["version"]},"steps":[${steps.joinToString(",")}]}""")
    }

    private fun waitForScan(name: String) {
        val deadline = System.nanoTime() + SCAN_TIMEOUT_NS
        while (true) {
            val state = (library(name)!!["scan"] as JsonObject)["state"]!!.jsonPrimitive.content
            check(state != "failed" && state != "aborted") { "the scan of $name $state" }
            if (state == "done") return
            check(System.nanoTime() < deadline) { "the scan of $name did not finish" }
            Thread.sleep(POLL_MS)
        }
    }

    private companion object {
        const val SEERR_KEY = "e2e-seerr-key-4f2a9c71d3b8"
        const val TMDB_KEY = "e2e-tmdb-key-8c41d0a7f2b9"
        const val SEERR_USER = 3L

        /** The cookies of the web session per server and account (see [cookies]). */
        val SESSIONS = mutableMapOf<String, MutableList<Cookie>>()

        const val SCAN_TIMEOUT_NS = 120_000_000_000L
        const val POLL_MS = 250L
    }
}
