package com.slouchcat.core.pairing

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.generated.apis.AccountApi
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.api.generated.models.PairingVoucherRequest
import com.slouchcat.core.api.generated.models.SessionKind
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
            // The same form with another version (in any order of the fields): a newer TV app.
            val fields = trimmed.substringAfter('#', "").split('&').associate { it.substringBefore('=') to it.substringAfter('=', "") }
            val version = fields["v"]?.takeIf { v -> v.length in 1..3 && v.all { it.isDigit() } }
            val shaped = trimmed.contains('#') && trimmed.substringBefore('#').trimEnd('/').endsWith("/" + PairingProtocol.HELP_PATH)
            return if (shaped && version != null && version != PairingProtocol.VERSION.toString()) Read.OtherVersion else Read.NotACode
        }
        return if (sameRelay(code.relay, relay)) Read.Ok(code) else Read.OtherRelay(code.relay.host)
    }

    /** Scheme, host, port and path: a code over `http` never matches a relay over `https`. */
    private fun sameRelay(a: HttpUrl, b: HttpUrl): Boolean =
        a.scheme == b.scheme && a.host.equals(b.host, ignoreCase = true) && a.port == b.port &&
            a.encodedPath.trimEnd('/') == b.encodedPath.trimEnd('/')
}

/**
 * A voucher of the own server (`createPairingVoucher`); a secret until it is sealed for the TV. A
 * voucher that is never laid down (the page was left, the relay refused) lapses at the server after
 * two minutes; the contract has no withdrawal.
 */
class IssuedVoucher(internal val voucher: String, val expiresAt: Instant, val knownDevices: Set<String>) {
    /** When the server issued it: the contract gives two minutes from the issue (`PairingVoucher.expiresAt`). */
    val issuedAt: Instant get() = expiresAt.minus(VOUCHER_LIFE)

    /** Never the voucher (rule 10, K-8). */
    override fun toString(): String = "IssuedVoucher(expiresAt=$expiresAt, voucher=[redacted])"

    companion object {
        val VOUCHER_LIFE: Duration = Duration.ofMinutes(2)
    }
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

    /** Waits at most [within] (the whole two minutes by default, a short look after 409/410). */
    suspend fun awaitTv(voucher: IssuedVoucher, within: Duration = PhonePairing.WAIT_FOR_TV): PhonePairing.Linked
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

    /**
     * What «Meine Geräte» shows after the packet. The contract gives the phone no id of the device
     * that redeemed (concept 27 section 10 step 6): the TV is the one device session of the type `tv`
     * that started after the voucher and was not listed before. Two such devices at once (another
     * phone of the account paired a TV in the same minute) are [Several], never a guess.
     */
    sealed interface Linked {
        /** The TV redeemed: [device] is its row in «Meine Geräte». */
        data class Ok(val device: MyDevice) : Linked

        /** More than one new TV: the page sends the user to «Meine Geräte». */
        data object Several : Linked

        /** No new TV within the time. */
        data object TimedOut : Linked

        /** 401, 403 or 426: the session may not ask any longer. */
        data object Ended : Linked
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
     * [voucher] was issued (see [Linked]), at most [within]. A list that does not load counts as «not
     * yet»; 401, 403 and 426 end the wait.
     */
    override suspend fun awaitTv(voucher: IssuedVoucher, within: Duration): Linked {
        val until = now().plus(within)
        val since = voucher.issuedAt.minus(CLOCK_SLACK)
        while (now().isBefore(until)) {
            when (val devices = server.devices()) {
                is ApiResult.Ok -> {
                    val fresh = devices.value.filter { d ->
                        d.id !in voucher.knownDevices && d.type == MyDevice.Type.TV && d.sessionKind == SessionKind.DEVICE &&
                            d.signedInAt?.toInstant()?.isBefore(since) == false
                    }
                    if (fresh.size == 1) return Linked.Ok(fresh.single())
                    if (fresh.size > 1) return Linked.Several
                }
                ApiResult.SignedOut, ApiResult.AppTooOld, is ApiResult.NotAllowed -> return Linked.Ended
                else -> Unit
            }
            delay(pollInterval.toMillis())
        }
        return Linked.TimedOut
    }

    companion object {
        val WAIT_FOR_TV: Duration = Duration.ofMinutes(2)

        /** How long the phone looks for its TV after 409/410 or no answer of the relay (security review L-1). */
        val LOOK_FOR_TV: Duration = Duration.ofSeconds(12)

        /** Server time of the session against server time of the voucher: the same clock, a margin for rounding. */
        private val CLOCK_SLACK: Duration = Duration.ofSeconds(5)
        private const val TAG = "pairing"
        private const val PRIMARY_REQUIRED = "primary_profile_required"
        private const val HTTP_TOO_MANY = 429
    }
}
