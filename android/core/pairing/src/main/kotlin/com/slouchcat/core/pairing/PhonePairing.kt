package com.slouchcat.core.pairing

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.generated.apis.AccountApi
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.api.generated.models.PairingVoucherRequest
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.delay
import okhttp3.Call
import okhttp3.HttpUrl
import java.security.GeneralSecurityException
import java.time.Duration
import java.time.Instant

/**
 * What the phone makes of a scanned text (concept 27 section 10 step 2, K-5): only a QR code of the
 * TV app of version 1 that names exactly the relay in use. Nothing of the text is ever opened as an
 * address: the phone talks to its own relay and takes from the code only the meeting point and the
 * TV's key.
 */
object ScannedCode {
    /** Longer than any code of version 1 (a relay address of a few dozen characters and 75 more). */
    const val MAX_LENGTH = 512

    sealed interface Read {
        data class Ok(val code: PairingCode) : Read {
            override fun toString(): String = "Ok"
        }

        /** Not a pairing code: another QR code, text, a link to somewhere else, something broken. */
        data object NotACode : Read

        /** A code of the TV app of another version than this app reads. */
        data object OtherVersion : Read

        /** A code of version 1 for another relay than the one in use; [relay] is its host, as text only. */
        data class OtherRelay(val relay: String) : Read
    }

    fun read(text: String, relay: HttpUrl): Read {
        val trimmed = text.trim()
        if (trimmed.isEmpty() || trimmed.length > MAX_LENGTH || trimmed.any { it.isISOControl() }) return Read.NotACode
        val code = PairingCode.parse(trimmed)
        if (code == null) {
            // The same form with another version: the TV app is newer than this one.
            val version = Regex("""#v=([0-9]{1,3})&""").find(trimmed)?.groupValues?.get(1)
            val shaped = trimmed.substringBefore('#').trimEnd('/').endsWith("/" + PairingProtocol.HELP_PATH)
            return if (shaped && version != null && version != PairingProtocol.VERSION.toString()) Read.OtherVersion else Read.NotACode
        }
        return if (sameRelay(code.relay, relay)) Read.Ok(code) else Read.OtherRelay(code.relay.host)
    }

    /** Scheme, host, port and path: a code over `http` never matches a relay over `https`. */
    private fun sameRelay(a: HttpUrl, b: HttpUrl): Boolean =
        a.scheme == b.scheme && a.host.equals(b.host, ignoreCase = true) && a.port == b.port &&
            a.encodedPath.trimEnd('/') == b.encodedPath.trimEnd('/')
}

/** A voucher of the own server (`createPairingVoucher`); a secret until it is sealed for the TV. */
class IssuedVoucher(internal val voucher: String, val expiresAt: Instant, val knownDevices: Set<String>) {
    /** Never the voucher (rule 10, K-8). */
    override fun toString(): String = "IssuedVoucher(expiresAt=$expiresAt, voucher=[redacted])"
}

/** The calls of the phone to its own server during a pairing; the seam tests fake. */
interface PairingServer {
    /** `createPairingVoucher` for the TV's key from the QR code. */
    suspend fun voucher(tvKey: String): ApiResult<Pair<String, Instant>>

    /** `listMyDevices`: the account's devices («Meine Geräte»). */
    suspend fun devices(): ApiResult<List<MyDevice>>
}

/** [PairingServer] over the generated client and the app's client with the session's credentials. */
class ApiPairingServer(private val api: ApiCaller, private val http: Call.Factory, private val server: HttpUrl) : PairingServer {
    override suspend fun voucher(tvKey: String): ApiResult<Pair<String, Instant>> = api.call("pairing") {
        SlouchcatApi.create(::AuthApi, server, http).createPairingVoucher(PairingVoucherRequest(tvKey))?.let { it.voucher to it.expiresAt.toInstant() }
    }

    override suspend fun devices(): ApiResult<List<MyDevice>> = api.call("pairing") { SlouchcatApi.create(::AccountApi, server, http).listMyDevices() }
}

/** The steps of the phone's pairing; [PhonePairing] does them, the tests of the page fake them. */
interface PhonePairingFlow {
    suspend fun issue(code: PairingCode): PhonePairing.Issue

    suspend fun deliver(code: PairingCode, voucher: IssuedVoucher, serverAddress: HttpUrl, serverKey: String): PhonePairing.Deliver

    suspend fun awaitTv(voucher: IssuedVoucher): PhonePairing.Linked
}

/**
 * The phone's side of one pairing (concept 27 section 10, 7.1, 7.2): the voucher of the own server
 * for the TV's key, the packet sealed for that key and laid down at the relay, then the wait until
 * the TV appears under «Meine Geräte». The steps are separate so the page can show each one.
 */
class PhonePairing(
    private val relay: RelayClient,
    private val server: PairingServer,
    private val pollInterval: Duration = PairingProtocol.POLL_INTERVAL,
    /** How long the phone waits for the TV after the packet (section 10 step 6: 2 minutes). */
    private val waitForTv: Duration = WAIT_FOR_TV,
    private val now: () -> Instant = Instant::now,
) : PhonePairingFlow {
    sealed interface Issue {
        data class Ok(val voucher: IssuedVoucher) : Issue

        /** 403 `primary_profile_required`: only the primary profile (or none) pairs a TV (F-23). */
        data object PrimaryOnly : Issue

        /** 403 otherwise: this session may not (a browser or a device session; never the app). */
        data class NotAllowed(val code: String?) : Issue

        /** 429 `too_many_attempts`: [seconds] the server names. */
        data class TooManyAttempts(val seconds: Long?) : Issue

        /** No answer of the own server. */
        data object Offline : Issue

        /** Another key under the server's address (ADR 0040): nothing secret goes there. */
        data object ServerUnconfirmed : Issue

        /** 401 or 426: the shell reacts; the page only stops. */
        data object SessionEnded : Issue

        data object Failed : Issue
    }

    enum class Deliver {
        Delivered,

        /** 409 or 410: somebody was quicker (section 10 step 5, «untergeschobenes Paket»). */
        Occupied,

        /** 404: the meeting point is unknown or ran out. */
        Expired,

        /** No answer of the relay (7.7). */
        RelayUnreachable,

        Failed,
    }

    sealed interface Linked {
        /** The TV redeemed: [device] is its row in «Meine Geräte». */
        data class Ok(val device: MyDevice) : Linked

        /** No new TV within [waitForTv]. */
        data object TimedOut : Linked
    }

    /**
     * Step 4: what «Meine Geräte» lists before (to tell the new TV apart), then the voucher for the
     * TV's key from [code].
     */
    override suspend fun issue(code: PairingCode): Issue {
        val before = when (val devices = server.devices()) {
            is ApiResult.Ok -> devices.value.mapTo(mutableSetOf()) { it.id }
            else -> return issueFailure(devices)
        }
        return when (val voucher = server.voucher(code.tvKey)) {
            is ApiResult.Ok -> Issue.Ok(IssuedVoucher(voucher.value.first, voucher.value.second, before))
            else -> issueFailure(voucher)
        }
    }

    private fun issueFailure(result: ApiResult<*>): Issue = when (result) {
        is ApiResult.Ok -> Issue.Failed
        is ApiResult.NotAllowed -> if (result.code == PRIMARY_REQUIRED) Issue.PrimaryOnly else Issue.NotAllowed(result.code)
        is ApiResult.Failed -> when {
            result.status == HTTP_TOO_MANY -> Issue.TooManyAttempts(result.retryAfterSeconds)
            result.code == ApiCaller.SERVER_UNCONFIRMED -> Issue.ServerUnconfirmed
            else -> Issue.Failed
        }
        ApiResult.Offline -> Issue.Offline
        ApiResult.SignedOut, ApiResult.AppTooOld -> Issue.SessionEnded
        ApiResult.NotFound -> Issue.Failed
    }

    /**
     * Step 5: seals server address, voucher and the server's key for the TV's key and the meeting
     * point of [code] and lays the packet down at the relay. [serverAddress] is the address under
     * which this phone reaches its server; the TV redeems there.
     */
    override suspend fun deliver(code: PairingCode, voucher: IssuedVoucher, serverAddress: HttpUrl, serverKey: String): Deliver {
        val key = Base64Url.decode(code.tvKey) ?: return Deliver.Failed
        val packet = PairingPacket(server = serverAddress.toString().trimEnd('/'), voucher = voucher.voucher, serverKey = serverKey)
        val sealed = try {
            packet.seal(key, code.id)
        } catch (e: GeneralSecurityException) {
            Log.w(TAG, "the TV's key seals nothing", error = e)
            return Deliver.Failed
        }
        if (sealed.size > PairingProtocol.MAX_PACKET_BYTES) return Deliver.Failed
        return when (relay.put(code.id, sealed)) {
            RelayClient.Put.Delivered -> Deliver.Delivered
            RelayClient.Put.Occupied, RelayClient.Put.Consumed -> Deliver.Occupied
            RelayClient.Put.Unknown -> Deliver.Expired
            RelayClient.Put.Unreachable -> Deliver.RelayUnreachable
            RelayClient.Put.Failed -> Deliver.Failed
        }
    }

    /**
     * Step 6: asks «Meine Geräte» every [pollInterval] until a TV appears that was not there before
     * [voucher] was issued, at most [waitForTv]. A list that does not load counts as «not yet».
     */
    override suspend fun awaitTv(voucher: IssuedVoucher): Linked {
        val until = now().plus(waitForTv)
        while (now().isBefore(until)) {
            val devices = server.devices()
            if (devices is ApiResult.Ok) {
                devices.value.firstOrNull { it.id !in voucher.knownDevices && it.type == MyDevice.Type.TV }?.let { return Linked.Ok(it) }
            }
            delay(pollInterval.toMillis())
        }
        return Linked.TimedOut
    }

    companion object {
        val WAIT_FOR_TV: Duration = Duration.ofMinutes(2)
        private const val TAG = "pairing"
        private const val PRIMARY_REQUIRED = "primary_profile_required"
        private const val HTTP_TOO_MANY = 429
    }
}
