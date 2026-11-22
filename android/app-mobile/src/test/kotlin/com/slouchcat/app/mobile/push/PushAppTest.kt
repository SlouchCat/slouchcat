package com.slouchcat.app.mobile.push

import android.Manifest
import android.app.Application
import android.app.NotificationManager
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import androidx.test.core.app.ActivityScenario
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.MainActivity
import com.slouchcat.app.mobile.SlouchcatApplication
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.NotificationsRoute
import com.slouchcat.app.mobile.home.HomeRoute
import com.slouchcat.app.mobile.more.DevicesRoute
import com.slouchcat.app.mobile.more.MoreRoute
import com.slouchcat.app.mobile.notifications.NotificationSettingsRoute
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.features
import com.slouchcat.app.mobile.wishes.WishTitleRoute
import com.slouchcat.app.mobile.wishes.WishesFeature
import com.slouchcat.app.mobile.wishes.WishesRoute
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.push.PushKeys
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeAccounts
import com.slouchcat.core.browse.testing.FakeChannels
import com.slouchcat.core.browse.testing.FakeNotifications
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import com.slouchcat.core.browse.testing.HTTP_SERVER
import com.slouchcat.core.browse.testing.PRIMARY
import com.slouchcat.core.browse.testing.notification
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.session.StoredCredentials
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import java.io.File
import java.util.Base64

/**
 * The way of a push message in the app: received, decrypted, shown, the bell kept right; the tap
 * through the activity nothing outside the app can start; the paths a tap opens.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class PushAppTest {
    private val context = ApplicationProvider.getApplicationContext<Context>()

    /** The 2700 vector of testdata/crypto/webpush.json: a message of the relay with its keys. */
    private val vector = run {
        val dir = System.getProperty("slouchcat.vectors") ?: File("../../testdata/crypto").absolutePath
        val case = Json.parseToJsonElement(File(dir, "webpush.json").readText()).jsonObject["cases"]!!.jsonArray
            .map { it.jsonObject }.first { it["input"]!!.jsonObject["recordSize"]!!.jsonPrimitive.int == 2700 }
        val input = case["input"]!!.jsonObject
        fun bytes(key: String) = Base64Url.decode(input[key]!!.jsonPrimitive.content)!!
        PushKeys(bytes("uaPrivateKey"), bytes("uaPublicKey"), bytes("authSecret")) to Base64.getEncoder().encodeToString(bytes("body"))
    }

    @Before
    fun clean() = RecentMessages.reset().also { File(context.noBackupFilesDir, "push-seen.txt").delete() }

    @After
    fun forget() = RecentMessages.reset()

    private fun graph(inbox: FakeNotifications): AppGraph {
        val store = MemoryPushStore(record = PushRecord(HTTP_SERVER.address.toString(), vector.first))
        return AppGraph(
            context,
            features = features,
            serverStore = FakeServers(HTTP_SERVER),
            tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
            accountStore = FakeAccounts(EMAIL, PRIMARY),
            mediaData = { g -> MediaData.api(g.api, g.http).copy(notifications = inbox, channels = FakeChannels()) },
            pushRegistrar = { g -> PushRegistrar(g.session, FakeChannels(), FakeRelay(), FakePlatform(), store) { "Phone" } },
        )
    }

    @Test
    fun aMessageIsShownOnceAndTheBellKeepsItsNumber() = runBlocking {
        shadowOf(context as Application).grantPermissions(Manifest.permission.POST_NOTIFICATIONS)
        val inbox = FakeNotifications(entries = mutableListOf(notification(1), notification(2), notification(3, read = true)))
        val graph = graph(inbox)
        graph.session.state.first { it is SessionState.Ready }
        graph.media.unreadCount.set(2)
        val id = PushReceiver.receive(context, graph, vector.second)
        assertNotNull(id)
        // The bell reads its number again instead of losing it (code review M-2).
        assertEquals(2, graph.media.unreadCount.count.value)
        val manager = context.getSystemService(NotificationManager::class.java)
        val shown = shadowOf(manager).allNotifications.single()
        assertEquals("Anna: Neue Folge", shown.extras.getString("android.title"))
        // On the lock screen only that there is something (security review L-5).
        assertEquals(android.app.Notification.VISIBILITY_PRIVATE, shown.visibility)
        assertNotNull(shown.publicVersion)
        assertFalse(shown.publicVersion.extras.getString("android.text").orEmpty().contains("Folge"))
        // The tap goes to the activity of the app only it can start.
        assertEquals(PushTapActivity::class.java.name, shadowOf(shown.contentIntent).savedIntent.component!!.className)
        // The same body again (a replay): not shown a second time (security review L-4).
        assertNull(PushReceiver.receive(context, graph, vector.second))
        assertEquals(1, shadowOf(manager).allNotifications.size)
    }

    @Test
    fun theTapActivityIsNotExported() {
        val info = context.packageManager.getActivityInfo(ComponentName(context, PushTapActivity::class.java), 0)
        assertFalse(info.exported)
        val found = context.packageManager.queryIntentActivities(Intent(Intent.ACTION_VIEW).setPackage(context.packageName), 0)
        assertFalse(found.any { it.activityInfo.name == PushTapActivity::class.java.name })
    }

    @Test
    fun aForeignIntentWithThePathOpensNothing() {
        val intent = Intent(context, MainActivity::class.java).putExtra(PushNotifier.EXTRA_PATH, "/settings/devices")
        ActivityScenario.launch<MainActivity>(intent).use {
            val graph = (context as SlouchcatApplication).graph
            assertNull(graph.links.pending.value)
        }
    }

    @Test
    fun theTappedPathOpensItsPage() {
        val graph = graph(FakeNotifications())
        fun opened(path: String): AppLink? {
            graph.links.consume()
            PushLinks.open(context, graph, path)
            return graph.links.pending.value
        }
        assertEquals(AppLink("start", listOf(ItemRoute("42"))), opened("/items/42"))
        assertEquals(AppLink(WishesFeature.TARGET, listOf(WishesRoute(tab = WishesFeature.MINE))), opened("/wishes?focus=3"))
        assertEquals(AppLink(WishesFeature.TARGET, listOf(WishesRoute(), WishTitleRoute("movie", 603))), opened("/titles/movie/603"))
        assertEquals(AppLink("more", listOf(MoreRoute, DevicesRoute)), opened("/settings/devices"))
        assertEquals(AppLink("start", listOf(HomeRoute, NotificationsRoute)), opened("/notifications"))
        assertEquals(AppLink("more", listOf(MoreRoute, NotificationSettingsRoute)), opened("/settings/notifications"))
        assertEquals(AppLink("start", listOf(HomeRoute)), opened("/"))
        assertEquals(AppLink("start", listOf(HomeRoute)), opened("/unknown/page"))
        // The administration opens in the browser, not in the app.
        graph.links.consume()
        PushLinks.open(context, graph, "/admin/wishes")
        assertNull(graph.links.pending.value)
    }

    @Test
    fun theChannelOfTheSystemFollowsThePath() {
        assertEquals(PushChannel.Content, PushChannel.of("/items/1"))
        assertEquals(PushChannel.Wishes, PushChannel.of("/wishes"))
        assertEquals(PushChannel.Wishes, PushChannel.of("/titles/movie/603"))
        assertEquals(PushChannel.Account, PushChannel.of("/settings/devices"))
        assertEquals(PushChannel.General, PushChannel.of("/"))
        assertEquals(PushChannel.General, PushChannel.of(null))
    }
}

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class FilePushStoreTest {
    private val dir = ApplicationProvider.getApplicationContext<Context>().cacheDir

    private class Box(var broken: Boolean = false) : com.slouchcat.core.session.SecretBox {
        override fun seal(plain: ByteArray) = plain.reversedArray()
        override fun open(sealed: ByteArray): ByteArray = if (broken) throw java.security.GeneralSecurityException("gone") else sealed.reversedArray()
        override fun forget() = Unit
    }

    @Test
    fun theStoreKeepsItsWishAndStartsOverWhenTheKeyIsGone() = runBlocking {
        val file = File(dir, "push-test.bin").also { it.delete() }
        val box = Box()
        val store = FilePushStore(file, box)
        store.setWanted(false)
        val keys = com.slouchcat.core.api.push.WebPush.generate()
        store.write(PushRecord("https://media.example.test/", keys, "t", "v", "e", 3))
        assertEquals(3L, store.read()!!.subscriptionId)
        store.write(null)
        assertFalse(store.wanted())
        assertNull(store.read())
        store.write(PushRecord("https://media.example.test/", null, pendingDelete = 9))
        assertEquals(9L, store.read()!!.pendingDelete)
        box.broken = true
        assertNull(store.read())
        file.writeText("garbage")
        box.broken = false
        assertNull(store.read())
    }
}
