package com.slouchcat.app.mobile.push

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.push.RelayResult
import com.slouchcat.core.api.push.WebPush
import com.slouchcat.core.browse.testing.FakeChannels
import com.slouchcat.core.browse.testing.HTTP_SERVER
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.settle
import com.slouchcat.core.media.DevicePushState
import com.slouchcat.core.media.PushFailure
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/** The memory of a device: wanted, and the record. */
class MemoryPushStore(var on: Boolean = true, var record: PushRecord? = null) : PushStore {
    override suspend fun wanted() = on
    override suspend fun setWanted(on: Boolean) { this.on = on }
    override suspend fun read() = record
    override suspend fun write(record: PushRecord?) { this.record = record }
}

class FakePlatform(var tokenValue: String? = "fcm-token-1", override val available: Boolean = true) : PushPlatform {
    override suspend fun token() = tokenValue
}

/** A relay that hands out an address per token and VAPID key, as `relay/cmd/push` does. */
class FakeRelay(var answer: RelayResult? = null) : RelayClient {
    val calls = mutableListOf<Pair<String, String>>()
    override fun register(fcmToken: String, vapidKey: String): RelayResult {
        calls += fcmToken to vapidKey
        return answer ?: RelayResult.Registered("https://push.example.test/v1/push/1.sealed-${calls.size}")
    }
}

/** Registration, token change, VAPID change, switching off, failures (concept 27, 7.3; ADR 0044). */
@OptIn(ExperimentalCoroutinesApi::class)
class PushRegistrarTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun TestScope.registrar(channels: FakeChannels, relay: FakeRelay, platform: FakePlatform, store: MemoryPushStore): PushRegistrar {
        val s = session(backgroundScope)
        settle()
        return PushRegistrar(s, channels, relay, platform, store) { "Pixel 9 · Android 16" }
    }

    @Test
    fun registersAtTheRelayAndEntersTheAddressWithItsOwnKey() = runTest(dispatcher) {
        val channels = FakeChannels()
        val relay = FakeRelay()
        val store = MemoryPushStore()
        val push = registrar(channels, relay, FakePlatform(), store)
        push.sync()
        assertEquals(DevicePushState.On, push.state.value)
        assertEquals(listOf("fcm-token-1" to "BPUBLICVAPIDKEY"), relay.calls)
        val request = channels.subscriptions.single()
        assertEquals("https://push.example.test/v1/push/1.sealed-1", request.endpoint.toString())
        val record = store.record!!
        assertEquals(WebPush.p256dh(record.keys), request.propertyKeys.p256dh)
        assertEquals(WebPush.auth(record.keys), request.propertyKeys.auth)
        assertEquals("Pixel 9 · Android 16", request.deviceName)
        assertEquals(HTTP_SERVER.address.toString(), record.server)
        // A second start with nothing changed asks the server only whether the subscription stands.
        push.sync()
        assertEquals(1, relay.calls.size)
        assertEquals(1, channels.subscriptions.size)
    }

    @Test
    fun aNewGoogleTokenOrVapidKeyEntersTheSubscriptionAgain() = runTest(dispatcher) {
        val channels = FakeChannels()
        val relay = FakeRelay()
        val platform = FakePlatform()
        val store = MemoryPushStore()
        val push = registrar(channels, relay, platform, store)
        push.sync()
        val keys = store.record!!.keys
        platform.tokenValue = "fcm-token-2"
        push.sync()
        assertEquals("fcm-token-2", relay.calls.last().first)
        assertEquals(2, channels.subscriptions.size)
        // The keys stay: only the address changed.
        assertTrue(keys === store.record!!.keys)
        channels.vapid = ApiResult.Ok("BNEWVAPID")
        push.sync()
        assertEquals("BNEWVAPID", relay.calls.last().second)
        // The server lost the subscription (new VAPID key, «Meine Geräte»): entered again.
        channels.replacePush(emptyList())
        push.sync()
        assertEquals(4, channels.subscriptions.size)
    }

    @Test
    fun switchingOffRemovesTheSubscription() = runTest(dispatcher) {
        val channels = FakeChannels()
        val store = MemoryPushStore()
        val push = registrar(channels, FakeRelay(), FakePlatform(), store)
        push.sync()
        val id = store.record!!.subscriptionId
        push.disable()
        assertEquals(DevicePushState.Off, push.state.value)
        assertTrue("unsubscribe:$id" in channels.calls)
        assertNull(store.record)
        assertFalse(store.on)
        push.enable()
        assertEquals(DevicePushState.On, push.state.value)
    }

    @Test
    fun failuresKeepNothingHalfDone() = runTest(dispatcher) {
        val channels = FakeChannels()
        val store = MemoryPushStore()
        val relay = FakeRelay(RelayResult.Failed(503))
        val push = registrar(channels, relay, FakePlatform(), store)
        push.sync()
        assertEquals(DevicePushState.Failed(PushFailure.Relay), push.state.value)
        assertTrue(channels.subscriptions.isEmpty())
        assertNull(store.record)
        relay.answer = null
        registrar(channels, relay, FakePlatform(tokenValue = null), store).also { it.sync() }.let {
            assertEquals(DevicePushState.Failed(PushFailure.Google), it.state.value)
        }
        channels.vapid = ApiResult.NotFound
        push.sync()
        assertEquals(DevicePushState.ServerOff, push.state.value)
        channels.vapid = ApiResult.Ok("B")
        channels.changeFailure = ApiResult.Offline
        push.sync()
        assertEquals(DevicePushState.Failed(PushFailure.Offline), push.state.value)
    }

    @Test
    fun aBuildWithoutPushDoesNothing() = runTest(dispatcher) {
        val channels = FakeChannels()
        val push = registrar(channels, FakeRelay(), FakePlatform(available = false), MemoryPushStore())
        push.sync()
        push.enable()
        assertEquals(DevicePushState.NotInBuild, push.state.value)
        assertTrue(channels.calls.isEmpty())
    }

    @Test
    fun signingOutForgetsTheRecord() = runTest(dispatcher) {
        val store = MemoryPushStore()
        val push = registrar(FakeChannels(), FakeRelay(), FakePlatform(), store)
        push.sync()
        push.forget()
        assertNull(store.record)
        assertTrue(store.on)
    }
}
