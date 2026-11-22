package com.slouchcat.app.mobile.push

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.PushSubscriptionRequest
import com.slouchcat.core.api.generated.models.PushSubscriptionRequestKeys
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.api.push.PushKeys
import com.slouchcat.core.api.push.RelayResult
import com.slouchcat.core.api.push.WebPush
import com.slouchcat.core.media.DevicePush
import com.slouchcat.core.media.DevicePushState
import com.slouchcat.core.media.PushFailure
import com.slouchcat.core.media.data.ChannelsRepository
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import java.net.URI

/** Google's push in this build (`play` with Firebase values); the bundle `foss` has none. */
interface PushPlatform {
    val available: Boolean

    /** The current Google token; `null` when it cannot be read. */
    suspend fun token(): String?
}

/** The push relay as the registrar uses it (tests bring a fake). */
fun interface RelayClient {
    fun register(fcmToken: String, vapidKey: String): RelayResult
}

/**
 * What the device keeps about its subscription: the server it belongs to, the keys (the private
 * one only on the device, encrypted, never in a backup), and what it registered with.
 */
data class PushRecord(
    val server: String,
    val keys: PushKeys,
    val fcmToken: String? = null,
    val vapidKey: String? = null,
    val endpoint: String? = null,
    val subscriptionId: Long? = null,
)

/** The device's push settings and record. */
interface PushStore {
    /** «Push-Meldungen» on this device (on until the person turns it off). */
    suspend fun wanted(): Boolean

    suspend fun setWanted(on: Boolean)

    suspend fun read(): PushRecord?

    suspend fun write(record: PushRecord?)
}

/**
 * The push subscription of this device (concept 27, 7.3; ADR 0044): reads the server's VAPID key
 * (`getPushPublicKey`), Google's token, registers both at the push relay (`POST /v1/register`)
 * and enters the sealed address with the device's own P-256 key and auth secret at the server
 * (`createPushSubscription`; a new one replaces the device's older one). [sync] checks at every
 * start, on a new Google token and when the account changed whether the subscription still stands
 * (a new VAPID key deletes every subscription on the server) and enters it again if not. The app
 * enters its subscription with any profile; whether the account gets push decides the primary
 * profile with the channel (the page says so).
 */
class PushRegistrar(
    private val session: SessionManager,
    private val channels: ChannelsRepository,
    private val relay: RelayClient,
    private val platform: PushPlatform,
    private val store: PushStore,
    private val deviceName: () -> String,
) : DevicePush {
    private val _state = MutableStateFlow<DevicePushState>(if (platform.available) DevicePushState.Off else DevicePushState.NotInBuild)
    override val state: StateFlow<DevicePushState> = _state.asStateFlow()
    private val mutex = Mutex()

    override suspend fun enable() {
        if (!platform.available) return
        store.setWanted(true)
        sync()
    }

    override suspend fun disable() {
        if (!platform.available) return
        store.setWanted(false)
        sync()
    }

    override suspend fun refresh() = sync()

    /** Brings the subscription in line with the wish of the device and the session. */
    suspend fun sync() = mutex.withLock {
        if (!platform.available) {
            _state.value = DevicePushState.NotInBuild
            return@withLock
        }
        val server = session.server
        val signedIn = session.state.value is SessionState.Ready || session.state.value is SessionState.ChooseProfile
        val record = store.read()
        if (!store.wanted()) {
            if (server != null && signedIn && record?.server == server.toString()) record.subscriptionId?.let { channels.unsubscribe(server, it) }
            if (record != null) store.write(null)
            _state.value = DevicePushState.Off
            return@withLock
        }
        if (server == null || !signedIn) {
            _state.value = DevicePushState.Off
            return@withLock
        }
        _state.value = DevicePushState.Busy
        _state.value = subscribe(server, record?.takeIf { it.server == server.toString() })
    }

    private suspend fun subscribe(server: okhttp3.HttpUrl, known: PushRecord?): DevicePushState {
        val vapid = when (val result = channels.vapidKey(server)) {
            is ApiResult.Ok -> result.value
            ApiResult.NotFound -> return DevicePushState.ServerOff
            ApiResult.Offline -> return DevicePushState.Failed(PushFailure.Offline)
            else -> return DevicePushState.Failed(PushFailure.Server)
        }
        val token = platform.token() ?: return DevicePushState.Failed(PushFailure.Google)
        if (known != null && known.fcmToken == token && known.vapidKey == vapid && known.subscriptionId != null && stillThere(server, known.subscriptionId)) {
            return DevicePushState.On
        }
        val endpoint = when (val result = relay.register(token, vapid)) {
            is RelayResult.Registered -> result.endpoint
            is RelayResult.Failed -> {
                Log.w(TAG, "relay refused", "status" to result.status)
                return DevicePushState.Failed(PushFailure.Relay)
            }
        }
        val keys = known?.keys ?: WebPush.generate()
        val request = PushSubscriptionRequest(URI(endpoint), PushSubscriptionRequestKeys(WebPush.p256dh(keys), WebPush.auth(keys)), deviceName().take(80))
        return when (val result = channels.subscribe(server, request)) {
            is ApiResult.Ok -> {
                store.write(PushRecord(server.toString(), keys, token, vapid, endpoint, result.value.id))
                DevicePushState.On
            }
            ApiResult.NotFound -> DevicePushState.ServerOff
            ApiResult.Offline -> DevicePushState.Failed(PushFailure.Offline)
            else -> DevicePushState.Failed(PushFailure.Server)
        }
    }

    /** Whether the server still lists this device's subscription (`getMyChannels`, push). */
    private suspend fun stillThere(server: okhttp3.HttpUrl, id: Long): Boolean {
        val all = channels.channels(server) as? ApiResult.Ok ?: return true
        val push = all.value.channels.firstOrNull { it.channel == com.slouchcat.core.api.generated.models.Channel.PUSH }?.push ?: return false
        return push.subscriptions.any { it.id == id && it.thisDevice }
    }

    /** Signed out: the server deleted the device's subscription with the session; the device forgets it too. */
    suspend fun forget() = mutex.withLock {
        store.write(null)
        _state.value = if (platform.available) DevicePushState.Off else DevicePushState.NotInBuild
    }

    /** The keys of the subscription of [server], for an arriving message. */
    suspend fun keysFor(): PushKeys? = store.read()?.keys

    private companion object {
        const val TAG = "push"
    }
}
