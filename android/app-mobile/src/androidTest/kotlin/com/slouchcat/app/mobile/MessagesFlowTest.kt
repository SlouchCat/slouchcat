package com.slouchcat.app.mobile

import android.Manifest
import android.app.NotificationManager
import android.graphics.Bitmap
import android.os.Build
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.isToggleable
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.mobile.browse.NotificationsRoute
import com.slouchcat.app.mobile.home.HomeRoute
import com.slouchcat.app.mobile.more.MoreRoute
import com.slouchcat.app.mobile.notifications.NotificationSettingsRoute
import com.slouchcat.app.mobile.push.PushData
import com.slouchcat.app.mobile.push.PushPlatform
import com.slouchcat.app.mobile.push.PushReceiver
import com.slouchcat.app.mobile.push.PushRegistrar
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.apis.NotifyApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.apis.WishesApi
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Channel
import com.slouchcat.core.api.generated.models.ChannelSettingUpdate
import com.slouchcat.core.api.generated.models.Notification
import com.slouchcat.core.api.generated.models.SetupRequest
import com.slouchcat.core.api.push.RelayResult
import com.slouchcat.core.browse.notifications.badge
import com.slouchcat.core.media.DevicePushState
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import kotlinx.serialization.json.longOrNull
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/**
 * Stage 8 in the emulator (scripts/android-helpers.sh starts the Seerr and TMDB mocks of web/e2e
 * eleven and twelve below the server's port, the push relay 83 and `fcmfake` 84 above it):
 * a wish from the search's «Noch nicht in der Bibliothek?», its state, «Zurückziehen»; a second
 * wish Seerr declines lands in the inbox, the bell shows it, opening it marks it read and leads to
 * «Meine Wünsche»; the account's push channel on, this device's subscription through the local
 * push relay (a Google token of `fcmfake`: there is no real FCM without a Firebase project), seen
 * at the server, a test message through relay and `fcmfake` fed to the app's receiver, shown as a
 * notification; off again. Everything it changes it puts back. With `slouchcatShots` it
 * photographs each page (prefix `w`).
 */
@RunWith(AndroidJUnit4::class)
class MessagesFlowTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val args = InstrumentationRegistry.getArguments()
    private val server: HttpUrl = (args.getString("slouchcatServer") ?: "http://10.0.2.2:8597").toHttpUrl()
    private val email = args.getString("slouchcatEmail") ?: "e2e-owner@example.test"
    private val password = args.getString("slouchcatPassword") ?: "e2e-password-0123"
    private val pin = args.getString("slouchcatPin") ?: "246810"
    private val shots = args.getString("slouchcatShots") != null
    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, BuildConfig.VERSION_NAME)

    // The helpers of scripts/android-helpers.sh: their ports come as arguments, the defaults as there.
    private fun port(name: String, default: Int) = args.getString(name)?.toIntOrNull() ?: default
    private val seerrPort = port("slouchcatSeerrPort", server.port - 11)
    private val seerrForServer = "http://127.0.0.1:$seerrPort"
    private val seerrFromHere = server.newBuilder().port(seerrPort).encodedPath("/").build()
    private val relayFromHere = server.newBuilder().port(port("slouchcatPushPort", server.port + 83)).encodedPath("/").build()
    private val fcmFromHere = server.newBuilder().port(port("slouchcatFcmPort", server.port + 84)).encodedPath("/").build()

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = 20_000) = compose.waitForNode(matcher, timeout)

    private fun waitForText(text: String, timeout: Long = 20_000) = waitFor(hasText(text, substring = true), timeout)

    private fun shown(matcher: SemanticsMatcher) = compose.isShown(matcher, unmerged = true)

    private fun button(text: String) = compose.onAllNodes((hasText(text) or hasContentDescription(text)) and hasClickAction()).onLast()

    private fun tap(matcher: SemanticsMatcher) = compose.onAllNodes(matcher and hasClickAction()).onLast().performSemanticsAction(SemanticsActions.OnClick)

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        InstrumentationRegistry.getInstrumentation().waitForIdleSync()
        compose.mainClock.advanceTimeBy(FRAME_MS)
        compose.waitForIdle()
        val bitmap = InstrumentationRegistry.getInstrumentation().uiAutomation.takeScreenshot() ?: return
        val dir = File(context.getExternalFilesDir(null), "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private val graph: AppGraph get() = (context.applicationContext as SlouchcatApplication).graph

    private fun open(target: String, vararg routes: Any) = InstrumentationRegistry.getInstrumentation().runOnMainSync { graph.links.request(AppLink(target, routes.toList())) }

    private fun ready(): Boolean = (graph.session.state.value as? SessionState.Ready)?.profile != null

    // --- The administration's part (web session of DetailFlowTest.Admin) -------------------------

    private val admin by lazy { DetailFlowTest.Admin(server, email, password, pin).also { it.signIn() } }

    private fun call(method: String, path: String, body: String? = null, http: OkHttpClient = admin.http, base: HttpUrl = server, prefix: Boolean = true): JsonElement? {
        val url = base.newBuilder().apply { if (prefix) addPathSegments(SlouchcatApi.BASE_PATH) }
            .addPathSegments(path.substringBefore('?'))
            .encodedQuery(path.substringAfter('?', "").ifEmpty { null })
            .build()
        val request = Request.Builder().url(url).header("X-Requested-With", "Slouchcat")
            .method(method, if (method == "GET") null else (body ?: "").toRequestBody("application/json".toMediaType())).build()
        http.newCall(request).execute().use { response ->
            val text = response.body.string()
            check(response.isSuccessful) { "$method $path: ${response.code} $text" }
            return text.takeIf { it.isNotBlank() }?.let(Json::parseToJsonElement)
        }
    }

    private var account: Long? = null

    /** TMDB key, Seerr connected, wishes on, the owner linked to Seerr's user 3 (the web e2e's `openWishes`). */
    private fun openWishes() {
        call("PUT", "admin/settings", """{"keys":{"tmdb":"$TMDB_KEY"}}""")
        call("PUT", "admin/seerr", """{"url":"$seerrForServer","apiKey":"$SEERR_KEY","enabled":true}""")
        call("POST", "admin/seerr/test", "{}")
        call("PUT", "admin/wishes/settings", """{"enabled":true}""")
        val id = call("GET", "auth/session")!!.jsonObject["account"]!!.jsonObject["id"]!!.jsonPrimitive.long
        account = id
        call("POST", "admin/wishes/mappings/users/refresh", "{}")
        compose.waitUntil(20_000) { (call("GET", "admin/wishes/mappings/users") as JsonArray).any { it.jsonObject["id"]!!.jsonPrimitive.long == SEERR_USER } }
        call("PUT", "admin/wishes/mappings/accounts/$id", """{"seerrUserId":$SEERR_USER}""")
    }

    private fun closeWishes() {
        val id = account ?: return
        runCatching {
            val wishes = SlouchcatApi.create(::WishesApi, server, graph.http)
            wishes.listWishes(WishesApi.StateListWishes.OPEN, 0, 100)!!.items.forEach { runCatching { wishes.withdrawWish(it.id) } }
        }
        runCatching { declineOpen() }
        runCatching { call("PUT", "admin/wishes/mappings/accounts/$id", """{"seerrUserId":null}""") }
        runCatching { call("PUT", "admin/wishes/settings", """{"enabled":false}""") }
        runCatching { call("PUT", "admin/seerr", """{"clear":true,"enabled":false}""") }
        runCatching { call("PUT", "admin/settings", """{"clear":["tmdb"]}""") }
        runCatching { call("POST", "__reset", "{}", OkHttpClient(), seerrFromHere, prefix = false) }
    }

    /** Seerr declines every open request; the sync of the server closes the wishes as declined. */
    private fun declineOpen() {
        val open = call("GET", "admin/wishes?state=open&limit=100")
        val items = (open?.jsonObject?.get("items") as? JsonArray).orEmpty().map { it.jsonObject }
        for (wish in items) {
            val ids = listOfNotNull(wish["seerrRequestId"]?.jsonPrimitive?.longOrNull) +
                (wish["seasons"] as? JsonArray).orEmpty().mapNotNull { it.jsonObject["seerrRequestId"]?.jsonPrimitive?.longOrNull }
            ids.forEach { call("POST", "__control", """{"request":{"id":$it,"status":3}}""", OkHttpClient(), seerrFromHere, prefix = false) }
        }
        if (items.isNotEmpty()) call("POST", "admin/seerr/sync", "{}")
    }

    // --- The app ----------------------------------------------------------------------------------

    private fun signIn() {
        compose.waitUntil(20_000) { ready() || shown(hasText(s(R.string.app_setup_title))) || shown(hasText(s(R.string.login_title)) and isHeading()) }
        if (ready()) return
        if (shown(hasText(s(R.string.app_setup_title)))) {
            compose.onNode(hasSetTextAction() and hasText(s(R.string.app_setup_addressLabel))).performTextInput(server.toString().trimEnd('/'))
            button(s(R.string.app_setup_connect)).performScrollTo().performClick()
            waitForText(s(R.string.app_setup_found))
            button(s(R.string.app_setup_next)).performScrollTo().performClick()
            waitFor(hasText(s(R.string.login_title)) and isHeading())
        }
        compose.onNode(hasSetTextAction() and hasText(s(R.string.login_email))).performTextInput(email)
        compose.onNode(hasSetTextAction() and hasText(s(R.string.login_password))).performTextInput(password)
        button(s(R.string.login_submit)).performScrollTo().performClick()
        compose.waitUntil(20_000) { ready() || shown(hasContentDescription(s(R.string.profiles_picker_primary), substring = true) and hasClickAction()) }
        if (!ready()) {
            compose.onAllNodes(hasContentDescription(s(R.string.profiles_picker_primary), substring = true) and hasClickAction()).onFirst().performClick()
            compose.waitUntil(20_000) { ready() || shown(hasText(s(R.string.profiles_pin_hint))) }
            if (!ready()) {
                pin.forEach { button(it.toString()).performClick() }
                compose.onAllNodes(hasContentDescription(s(R.string.profiles_pin_submit))).onLast().performClick()
            }
        }
        compose.waitUntil(20_000) { ready() }
    }

    /** A wish from the search: the title outside the library ([query]), its page, «Anfordern». */
    private fun wishFromSearch(query: String, titles: List<String>): String {
        // A fresh search page (the target keeps the page opened last).
        open("search", com.slouchcat.app.mobile.browse.SearchRoute())
        waitFor(hasContentDescription(s(R.string.search_label)))
        compose.onNode(hasSetTextAction()).performTextInput(query)
        waitForText(s(R.string.app_wishes_missingTitle))
        val title = compose.waitUntilFound(titles)
        shot("w01-search-missing")
        tap(hasText(title, substring = true))
        waitFor(hasText(title) and isHeading())
        shot("w02-wish-request")
        button(s(R.string.app_wishes_request)).performScrollTo().performClick()
        waitFor(hasText(s(R.string.app_wishes_requestNote)))
        shot("w03-wish-sheet")
        button(s(R.string.app_wishes_request)).performClick()
        waitFor(hasText(s(R.string.wishes_withdraw_action)))
        return title
    }

    private fun androidx.compose.ui.test.junit4.ComposeTestRule.waitUntilFound(candidates: List<String>): String {
        var found: String? = null
        // The waits look at the unmerged tree (the title's text node); the tap then goes to the merged row.
        waitForNode(candidates.map { hasText(it, substring = true) }.reduce { a, b -> a or b }, 20_000)
        found = candidates.first { shown(hasText(it, substring = true)) }
        return found!!
    }

    private var pushBefore: Boolean? = null
    private var registrar: PushRegistrar? = null

    @After
    fun putBack() = runBlocking {
        runCatching { registrar?.disable() }
        if (graph.session.state.value is SessionState.Ready) {
            pushBefore?.let { before ->
                runCatching {
                    val notify = SlouchcatApi.create(::NotifyApi, server, graph.http)
                    val push = notify.getMyChannels()!!.channels.single { it.channel == Channel.PUSH }
                    notify.updateMyChannel(Channel.PUSH, ChannelSettingUpdate(before, push.events))
                }
            }
            closeWishes()
            graph.sessionActions.signOut()
        }
        graph.serverStore.clear()
    }

    @Test
    fun wishesInboxAndPush() {
        val setup = SlouchcatApi.create(::SetupApi, server, SlouchcatHttp.client(identity))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = email, password = password))
        openWishes()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            InstrumentationRegistry.getInstrumentation().uiAutomation.grantRuntimePermission(context.packageName, Manifest.permission.POST_NOTIFICATIONS)
        }
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        signIn()

        // A wish from the search, its state on its page and in «Meine Wünsche», withdrawn.
        val first = wishFromSearch("Glas", listOf("Glasvogel", "Glass Bird"))
        shot("w04-wish-state")
        tap(hasText(s(R.string.app_wishes_toMine)))
        waitFor(hasText(first, substring = true))
        shot("w05-wishes-mine")
        tap(hasContentDescription(s(R.string.wishes_row_withdrawLabel, first)))
        waitForText(s(R.string.wishes_withdraw_title))
        button(s(R.string.wishes_withdraw_confirm)).performClick()
        waitForText(s(R.string.wishes_state_withdrawn))
        tap(hasText(s(R.string.wishes_area_discover)))
        waitForText(s(R.string.app_wishes_discoverIntro))
        shot("w06-wishes-discover")

        // A second wish Seerr declines: the inbox gets it, the bell shows it, opening reads it.
        val second = wishFromSearch("Kupfer", listOf("Kupfermond", "Copper Moon"))
        declineOpen()
        // The sync answers 202 and runs as a job: wait until the server has written the entry
        // (the profile's inbox through the API) before the app reads it. Any unread entry is not
        // enough: the podcast's new episode is one, too.
        val address = graph.session.server!!
        compose.waitUntil(30_000) {
            val page = runBlocking { graph.media.notifications.page(address, 0, 50) } as? ApiResult.Ok
            page?.value?.notifications.orEmpty().any { it.kind == Notification.Kind.WISH_DECLINED }
        }
        runBlocking { graph.media.unreadCount.refresh { graph.media.notifications.badge(address) } }
        open("start", HomeRoute)
        open("start", HomeRoute, NotificationsRoute)
        waitFor(hasText(s(R.string.notifications_title)) and isHeading())
        waitFor(hasContentDescription(s(R.string.notifications_wish_declined), substring = true) and hasClickAction())
        shot("w07-notifications")
        tap(hasContentDescription(s(R.string.notifications_wish_declined), substring = true))
        waitFor(hasText(second, substring = true))
        open("start", HomeRoute, NotificationsRoute)
        waitFor(hasContentDescription(s(R.string.notifications_entryLabel, second, s(R.string.notifications_wish_declined), ""), substring = true))

        // The account's push channel on, this device subscribed through the local relay, a test
        // message through relay and fcmfake to the app's receiver, the notification; off again.
        val notify = SlouchcatApi.create(::NotifyApi, server, graph.http)
        pushBefore = notify.getMyChannels()!!.channels.single { it.channel == Channel.PUSH }.enabled
        open("more", MoreRoute, NotificationSettingsRoute)
        waitForText(s(R.string.app_notifications_accountChannels).uppercase())
        if (pushBefore == false) {
            tap(hasText(s(R.string.app_notifications_pushChannel)))
            compose.onAllNodes(hasText(s(R.string.settings_notifications_enabled)) and isToggleable()).onFirst().performClick()
            button(s(R.string.common_save)).performClick()
            waitForText(s(R.string.settings_notifications_saved))
        }
        val token = "e2e-" + System.nanoTime()
        val push = PushRegistrar(
            graph.session,
            graph.media.channels,
            { fcmToken, vapid -> register(fcmToken, vapid) },
            object : PushPlatform {
                override val available = true

                override suspend fun token() = token
            },
            PushData.store(context),
        ) { graph.device.name }.also { registrar = it }
        runBlocking { push.enable() }
        assertEquals(DevicePushState.On, push.state.value)
        val subscriptions = notify.getMyChannels()!!.channels.single { it.channel == Channel.PUSH }.push!!.subscriptions
        assertTrue("the server lists the device", subscriptions.any { it.thisDevice })
        notify.testMyChannel(Channel.PUSH)
        val wp = compose.waitForMessage(token)
        val id = runBlocking { PushReceiver.receive(context, graph, wp) }
        assertTrue("the notification shows", id != null)
        val manager = context.getSystemService(NotificationManager::class.java)
        assertTrue(manager.activeNotifications.any { it.id == id })
        shot("w08-settings-push-on")
        // A tap opens the app at the message's path (the test message links to the start).
        manager.activeNotifications.single { it.id == id }.notification.contentIntent.send()
        waitFor(hasContentDescription(s(R.string.nav_notifications), substring = true))
        runBlocking { push.disable() }
        assertTrue(notify.getMyChannels()!!.channels.single { it.channel == Channel.PUSH }.push!!.subscriptions.none { it.thisDevice })
        scenario.close()
    }

    /** The relay of the run, asked from the emulator; it answers with its own (host-side) address. */
    private fun register(token: String, vapid: String): RelayResult {
        val answer = call("POST", "v1/register", """{"fcmToken":"$token","vapidKey":"$vapid"}""", OkHttpClient(), relayFromHere, prefix = false)
        return RelayResult.Registered(answer!!.jsonObject["endpoint"]!!.jsonPrimitive.content)
    }

    private fun androidx.compose.ui.test.junit4.ComposeTestRule.waitForMessage(token: String): String {
        var wp: String? = null
        waitUntil(30_000) {
            val received = call("GET", "messages?token=$token", http = OkHttpClient(), base = fcmFromHere, prefix = false)
            wp = (received as? JsonArray)?.firstOrNull()?.jsonObject?.get("data")?.jsonObject?.get("wp")?.jsonPrimitive?.content
            wp != null
        }
        return wp!!
    }

    private companion object {
        const val SEERR_KEY = "e2e-seerr-key-4f2a9c71d3b8"
        const val TMDB_KEY = "e2e-tmdb-key-8c41d0a7f2b9"
        const val SEERR_USER = 3L
        const val FRAME_MS = 500L
    }
}
