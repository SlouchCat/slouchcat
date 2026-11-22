package com.slouchcat.core.pairing

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.models.PairingRedeemRequest
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.delay
import okhttp3.HttpUrl
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import java.time.Duration
import java.time.Instant

/** What the server answered to `redeemPairingVoucher`. */
data class Redeemed(val sealedToken: String, val signature: String) {
    override fun toString(): String = "Redeemed(sealedToken=[redacted], signature=[redacted])"
}

/** The calls of the TV to the user's server during a pairing; the seam tests fake. */
interface Redeemer {
    /** `redeemPairingVoucher` at [server]; [serverKey] from the packet («Server weist sich aus»). */
    suspend fun redeem(server: HttpUrl, serverKey: String, voucher: String, tvKey: String, deviceName: String, systemVersion: String): ApiResult<Redeemed>

    /** «Nicht mein Konto»: `logout` with the token that was never stored, so the device session ends at once. */
    suspend fun discard(server: HttpUrl, serverKey: String, token: String): ApiResult<Unit>
}

/**
 * [Redeemer] over the generated client. [clientFor] builds the client for the server of the packet:
 * the shared client of the app without the session's credentials and with «Server weist sich aus»
 * against the packet's key, so nothing reaches a server over HTTP that does not prove that key.
 */
class ApiRedeemer(
    private val api: ApiCaller,
    private val clientFor: (server: HttpUrl, serverKey: String) -> OkHttpClient,
) : Redeemer {
    override suspend fun redeem(server: HttpUrl, serverKey: String, voucher: String, tvKey: String, deviceName: String, systemVersion: String) =
        api.call("pairing") {
            val answer = SlouchcatApi.create(::AuthApi, server, clientFor(server, serverKey))
                .redeemPairingVoucher(PairingRedeemRequest(voucher, tvKey, deviceName, systemVersion))
            // A 2xx without a body is `Failed(empty)` (ApiCaller).
            answer?.let { Redeemed(it.sealedToken, it.signature) }
        }

    override suspend fun discard(server: HttpUrl, serverKey: String, token: String): ApiResult<Unit> = api.call("pairing") {
        // The bearer only towards the server itself, per hop (like SlouchcatHttp.Credentials).
        val bearer = Interceptor { chain ->
            val request = chain.request()
            if (SlouchcatHttp.sameOrigin(server, request.url)) chain.proceed(request.newBuilder().header("Authorization", "Bearer $token").build()) else chain.proceed(request)
        }
        val client = clientFor(server, serverKey).newBuilder().addNetworkInterceptor(bearer).build()
        SlouchcatApi.create(::AuthApi, server, client).logout()
    }
}

/** The steps of the TV's pairing; [TvPairing] does them, the tests of the TV page fake them. */
interface PairingFlow {
    suspend fun start(): TvPairing.Started

    /** Until a packet, [until], or a silent relay; see [TvPairing.awaitPacket]. */
    suspend fun awaitPacket(meeting: TvPairing.Meeting, until: Instant = meeting.expiresAt): TvPairing.Waited

    suspend fun redeem(meeting: TvPairing.Meeting, packet: PairingPacket, server: HttpUrl, deviceName: String, systemVersion: String): TvPairing.Redeem

    /** «Nicht mein Konto»: ends the device session of [token] at once; `true` when the server confirmed. */
    suspend fun discard(server: HttpUrl, serverKey: String, token: String): Boolean

    suspend fun close(meeting: TvPairing.Meeting)
}

/**
 * The TV's side of one pairing (concept 26, 4.3): a one-time key and pickup secret, the meeting point
 * at the relay, the QR code, the polling every two seconds, the packet, the redeem at the server and
 * the sealed token. Keys and secret live in [Meeting] only, in memory, for this one pairing.
 */
class TvPairing(
    private val relay: RelayClient,
    private val redeemer: Redeemer,
    private val pollInterval: Duration = PairingProtocol.POLL_INTERVAL,
    private val now: () -> Instant = Instant::now,
) : PairingFlow {
    /** One open meeting point with its secrets; [destroy] overwrites them. */
    class Meeting(
        val code: PairingCode,
        val expiresAt: Instant,
        internal val keys: TvKeyPair,
        internal val secret: PickupSecret,
    ) {
        fun destroy() {
            keys.destroy()
            secret.destroy()
        }

        override fun toString(): String = "Meeting(expiresAt=$expiresAt)"
    }

    sealed interface Started {
        data class Ok(val meeting: Meeting) : Started

        /** The relay answers nothing (no internet, wrong address of the relay, relay down). */
        data object RelayUnreachable : Started

        /** Too many open meeting points; another try in a moment may work. */
        data object RelayBusy : Started

        data object Failed : Started
    }

    sealed interface Waited {
        data class Packet(val packet: PairingPacket) : Waited

        /** The five minutes ran out, or the meeting point is gone at the relay. */
        data object Expired : Waited

        /** No answer of the relay several times in a row. */
        data object RelayUnreachable : Waited

        /** Something lay there that this TV cannot open (not for its key, broken, refused). */
        data object Invalid : Waited
    }

    sealed interface Redeem {
        data class Ok(val answer: RedeemAnswer) : Redeem

        /** 410: voucher unknown, expired, used or bound to another key; the TV shows a new code. */
        data object VoucherInvalid : Redeem

        /** No answer of the server, or it did not prove the packet's key over HTTP. */
        data object ServerUnreachable : Redeem

        /** The answer is not signed by the packet's key or cannot be opened: thrown away. */
        data object Untrusted : Redeem

        data object AppTooOld : Redeem

        data class TooManyAttempts(val seconds: Long?) : Redeem

        data object Failed : Redeem
    }

    override suspend fun start(): Started {
        val keys = TvKeyPair.generate()
        val secret = PickupSecret.generate()
        return when (val opened = relay.open(secret)) {
            is RelayClient.Open.Ok -> Started.Ok(Meeting(PairingCode(relay.base, opened.id, keys.publicText), opened.expiresAt, keys, secret))
            else -> {
                keys.destroy()
                secret.destroy()
                when (opened) {
                    RelayClient.Open.Unreachable -> Started.RelayUnreachable
                    RelayClient.Open.Busy -> Started.RelayBusy
                    else -> Started.Failed
                }
            }
        }
    }

    /**
     * Asks the relay every [pollInterval] until a packet arrives, [until] (the expiry, or earlier to
     * renew the code in time) has passed, or the relay stays silent.
     */
    override suspend fun awaitPacket(meeting: Meeting, until: Instant): Waited {
        var silent = 0
        while (now().isBefore(until)) {
            when (val poll = relay.poll(meeting.code.id, meeting.secret)) {
                is RelayClient.Poll.Packet -> {
                    val packet = PairingPacket.open(meeting.keys, meeting.code.id, poll.sealed)
                    if (packet == null) Log.w(TAG, "packet not for this TV")
                    return if (packet == null) Waited.Invalid else Waited.Packet(packet)
                }
                RelayClient.Poll.Waiting -> silent = 0
                RelayClient.Poll.Gone -> return Waited.Expired
                RelayClient.Poll.Refused -> return Waited.Invalid
                RelayClient.Poll.Unreachable, is RelayClient.Poll.Failed -> if (++silent >= SILENT_POLLS) return Waited.RelayUnreachable
            }
            delay(pollInterval.toMillis())
        }
        // Also when [until] lies before the expiry: the caller renews the code before it runs out.
        return Waited.Expired
    }

    /**
     * Redeems the packet's voucher at [server] (the packet's address, read): the signature over the
     * sealed answer is checked with the packet's key first, then the answer is opened (ADR 0040).
     */
    override suspend fun redeem(meeting: Meeting, packet: PairingPacket, server: HttpUrl, deviceName: String, systemVersion: String): Redeem {
        val key = packet.serverKey?.takeIf { PairingProtocol.isKey(it) } ?: return Redeem.Untrusted
        val result = redeemer.redeem(server, key, packet.voucher, meeting.keys.publicText, deviceName, systemVersion)
        return when (result) {
            is ApiResult.Ok -> {
                val sealed = Base64Url.decode(result.value.sealedToken)
                when {
                    sealed == null || !RedeemSignature.valid(key, sealed, result.value.signature) -> Redeem.Untrusted.also { Log.w(TAG, "redeem answer not signed by the packet's key") }
                    else -> RedeemAnswer.open(meeting.keys, sealed)?.let { Redeem.Ok(it) } ?: Redeem.Untrusted
                }
            }
            is ApiResult.Failed -> when {
                result.status == HTTP_GONE -> Redeem.VoucherInvalid
                result.status == HTTP_TOO_MANY -> Redeem.TooManyAttempts(result.retryAfterSeconds)
                result.code == ApiCaller.SERVER_UNCONFIRMED -> Redeem.ServerUnreachable
                else -> Redeem.Failed
            }
            ApiResult.Offline -> Redeem.ServerUnreachable
            ApiResult.AppTooOld -> Redeem.AppTooOld
            ApiResult.NotFound, is ApiResult.NotAllowed, ApiResult.SignedOut -> Redeem.Failed
        }
    }

    override suspend fun discard(server: HttpUrl, serverKey: String, token: String): Boolean = redeemer.discard(server, serverKey, token) is ApiResult.Ok

    /** Gives the meeting point up at the relay (best effort) and destroys its secrets. */
    override suspend fun close(meeting: Meeting) {
        relay.cancel(meeting.code.id, meeting.secret)
        meeting.destroy()
    }

    private companion object {
        const val TAG = "pairing"
        const val SILENT_POLLS = 3
        const val HTTP_GONE = 410
        const val HTTP_TOO_MANY = 429
    }
}
