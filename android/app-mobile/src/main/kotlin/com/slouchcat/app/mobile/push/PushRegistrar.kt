package com.slouchcat.app.mobile.push

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Channel
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
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/** Google's push in this build (`play` with Firebase values); the bundle `foss` has none. */
interface PushPlatform {
    val available: Boolean

    /** The current Google token; `null` when it cannot be read. Only asked while push is wanted. */
    suspend fun token(): String?

    /** Push is off on this device: Google's library fetches and keeps nothing more (code review m-6). */
    suspend fun stop() = Unit
}

/** The push relay as the registrar uses it (tests bring a fake). */
fun interface RelayClient {
    fun register(fcmToken: String, vapidKey: String): RelayResult
}

/**
 * What the device keeps about its subscription: the server it belongs to, the keys (the private
 * one only on the device, encrypted, never in a backup), and what it registered with.
 * [pendingDelete]: switched off while the server could not confirm the removal; the id is removed
 * at the next sync, the keys are gone already (code review M-3). [toString] names no secret (rule 10).
 */
data class PushRecord(
    val server: String,
    val keys: PushKeys?,
    val fcmToken: String? = null,
    val vapidKey: String? = null,
    val endpoint: String? = null,
    val subscriptionId: Long? = null,
    val pendingDelete: Long? = null,
) {
    override fun toString(): String = "PushRecord(subscriptionId=$subscriptionId, pendingDelete=$pendingDelete)"
}

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
 * and enters the sealed address with a fresh P-256 key and auth secret of the device at the server
 * (`createPushSubscription`; a new one replaces the device's older one). [sync] checks at every
 * start, on a new Google token, after the account changed and in the background about twice a day
 * ([PushCheckWorker]) whether the subscription still stands (a new VAPID key deletes every
 * subscription on the server) and enters it again with new keys if not. Switching off removes it at
 * the server; without an answer the removal waits for the next sync and the page says so. The app
 * enters its subscription with any profile; whether the account gets push decides the primary
 * profile with the channel (the page says so).
 */
class PushRegistrar(
    private val session: SessionManager,
    private val channels: ChannelsRepository,
    private val relay: RelayClient,
    private val platform: PushPlatform,
    private val store: PushStore,
    // RelayClient.register blocks (OkHttp execute); callers run on the main dispatcher.
    private val io: CoroutineDispatcher = Dispatchers.IO,
    private val deviceName: () -> String,
) : DevicePush {
    private val _state = MutableStateFlow<DevicePushState>(if (platform.available) DevicePushState.Off else DevicePushState.NotInBuild)
    override val state: StateFlow<DevicePushState> = _state.asStateFlow()
    private val mutex = Mutex()

    // The wish and the sync under one lock: a «switch off» cannot be lost to a sync in flight (code review m-2).
    override suspend fun enable() = locked {
        if (!platform.available) return@locked
        store.setWanted(true)
        syncLocked()
    }

    override suspend fun disable() = locked {
        if (!platform.available) return@locked
        store.setWanted(false)
        syncLocked()
    }

    override suspend fun refresh() = sync()

    /** Brings the subscription in line with the wish of the device and the session. */
    suspend fun sync() = locked { syncLocked() }

    /** One step at a time; a failure of the store or the address never brings the app down (code review m-1). */
    private suspend fun locked(block: suspend () -> Unit) = mutex.withLock {
        try {
            block()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            // The class only: a message may carry an address (rule 10, code review m-10).
            Log.w(TAG, "push failed", "kind" to e.javaClass.simpleName)
            _state.value = DevicePushState.Failed(PushFailure.Server)
        }
    }

    private suspend fun syncLocked() {
        if (!platform.available) {
            _state.value = DevicePushState.NotInBuild
            return
        }
        val server = session.server
        val signedIn = session.state.value is SessionState.Ready || session.state.value is SessionState.ChooseProfile
        var record = store.read()
        val ours = server != null && signedIn && record?.server == server.toString()

        // A removal the server did not confirm yet: try again first.
        val pending = record?.pendingDelete
        if (pending != null && ours) {
            if (removed(channels.unsubscribe(server!!, pending))) {
                record = record!!.copy(pendingDelete = null)
                store.write(record.takeIf { it.keys != null })
            }
        }

        if (!store.wanted()) {
            val id = record?.subscriptionId
            when {
                id == null -> if (record?.pendingDelete == null) store.write(null)
                ours && removed(channels.unsubscribe(server!!, id)) -> store.write(null)
                // No answer: the keys go now (nothing can be read any more), the removal later.
                else -> store.write(PushRecord(record!!.server, keys = null, pendingDelete = id))
            }
            platform.stop()
            _state.value = if (store.read()?.pendingDelete != null) DevicePushState.OffPending else DevicePushState.Off
            return
        }
        if (server == null || !signedIn) {
            _state.value = DevicePushState.Off
            return
        }
        _state.value = DevicePushState.Busy
        _state.value = subscribe(server, record?.takeIf { it.server == server.toString() && it.keys != null })
    }

    private fun removed(result: ApiResult<Unit>) = result is ApiResult.Ok || result == ApiResult.NotFound

    private suspend fun subscribe(server: HttpUrl, known: PushRecord?): DevicePushState {
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
        val endpoint = when (val result = withContext(io) { relay.register(token, vapid) }) {
            is RelayResult.Registered -> result.endpoint
            is RelayResult.Failed -> {
                Log.w(TAG, "relay refused", "status" to result.status)
                return DevicePushState.Failed(PushFailure.Relay)
            }
        }
        val uri = endpoint.toHttpUrlOrNull()?.toUri() ?: return DevicePushState.Failed(PushFailure.Relay)
        // Every new subscription gets a new key pair and auth secret: a new VAPID key, a new
        // password, a lost subscription leave nothing of the old keys usable (security review L-3).
        val keys = WebPush.generate()
        val request = PushSubscriptionRequest(uri, PushSubscriptionRequestKeys(WebPush.p256dh(keys), WebPush.auth(keys)), deviceName().take(DEVICE_NAME_MAX))
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
    private suspend fun stillThere(server: HttpUrl, id: Long): Boolean {
        val all = channels.channels(server) as? ApiResult.Ok ?: return true
        val push = all.value.channels.firstOrNull { it.channel == Channel.PUSH }?.push ?: return false
        return push.subscriptions.any { it.id == id && it.thisDevice }
    }

    /** Signed out: the server deleted the device's subscription with the session; the device forgets it too. */
    suspend fun forget() = locked {
        store.write(null)
        _state.value = if (platform.available) DevicePushState.Off else DevicePushState.NotInBuild
    }

    /** The keys of the subscription of the current server, for an arriving message (security review I-7). */
    suspend fun keysFor(): PushKeys? = mutex.withLock {
        runCatching { store.read() }.getOrNull()?.takeIf { it.server == session.server?.toString() }?.keys
    }

    private companion object {
        const val TAG = "push"
        const val DEVICE_NAME_MAX = 80
    }
}
