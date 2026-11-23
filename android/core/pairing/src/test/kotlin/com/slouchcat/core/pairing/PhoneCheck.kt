package com.slouchcat.core.pairing

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SessionCredentials
import com.slouchcat.core.api.SessionEvents
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.SystemApi
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.SelectProfileRequest
import kotlinx.coroutines.runBlocking
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import kotlin.system.exitProcess

/**
 * The phone's side of a pairing as a JVM program (task android:e2e:pair, concept 27 stage 9): the
 * same Kotlin the phone app runs ([ScannedCode], [PhonePairing], [ApiPairingServer]) against the TV
 * app in the emulator, in place of `apicheck -pair-phone`. Not a test: `./gradlew
 * :core:pairing:phoneCheck --args="…"` (scripts/android-tv-e2e.sh with PHONE=kotlin).
 *
 * Arguments as `name=value`: `server` (as this program reaches it), `packetServer` (as the TV
 * reaches it), `relay` (as this program reaches it), `codeRelay` (the relay the phone app would have
 * set, the one the code names), `code`, `email`, `password`, `pin` (password and PIN better as `SLOUCHCAT_PAIR_PASSWORD`, `SLOUCHCAT_PAIR_PIN`). Prints status lines only, never
 * the code, the voucher or a token.
 */
fun main(args: Array<String>) {
    val a = args.associate { it.substringBefore('=') to it.substringAfter('=') }
    fun url(name: String): HttpUrl = requireNotNull(a[name]) { "missing $name=" }.toHttpUrl()
    val outcome = runBlocking { phone(url("server"), url("packetServer"), url("relay"), url("codeRelay"), a.getValue("code"), a.getValue("email"), secret(a, "password"), a["pin"] ?: System.getenv("SLOUCHCAT_PAIR_PIN").orEmpty()) }
    println("phonecheck: $outcome")
    exitProcess(if (outcome == "the TV is listed under «Meine Geräte»") 0 else 1)
}

/** Password from the arguments or, so it is not on the command line, from `SLOUCHCAT_PAIR_PASSWORD`. */
private fun secret(a: Map<String, String>, name: String): String =
    a[name] ?: requireNotNull(System.getenv("SLOUCHCAT_PAIR_PASSWORD")) { "missing $name= or SLOUCHCAT_PAIR_PASSWORD" }

private suspend fun phone(server: HttpUrl, packetServer: HttpUrl, relay: HttpUrl, codeRelay: HttpUrl, text: String, email: String, password: String, pin: String): String {
    val code = when (val read = ScannedCode.read(text, codeRelay)) {
        is ScannedCode.Read.Ok -> read.code
        else -> return "the code was refused: $read"
    }
    val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0")
    val plain = SlouchcatHttp.client(identity)
    val health = SlouchcatApi.create(::SystemApi, server, plain).getHealth()!!
    val login = SlouchcatApi.create(::AuthApi, server, plain).appLogin(AppLoginRequest(email, password, "phonecheck", AppLoginRequest.Kind.PHONE))!!
    val credentials = object : SessionCredentials {
        override val server = server
        override val bearerToken = login.token
        override val deviceId: String? = null
    }
    val http = SlouchcatHttp.client(identity, credentials)
    val auth = SlouchcatApi.create(::AuthApi, server, http)
    val primary = auth.listMyProfiles()!!.single { it.isPrimary }
    auth.selectProfile(primary.id, if (primary.hasPin) SelectProfileRequest(pin) else null)
    println("phonecheck: signed in, primary profile chosen")
    val pairing = PhonePairing(RelayClient(OkHttpClient.Builder().followRedirects(false).build(), relay), ApiPairingServer(ApiCaller(SessionEvents()), http, server))
    try {
        val voucher = when (val issue = pairing.issue(code)) {
            is PhonePairing.Issue.Ok -> issue.voucher
            else -> return "no voucher: $issue"
        }
        println("phonecheck: voucher issued")
        val delivered = pairing.deliver(code, voucher, packetServer, health.serverKey)
        if (delivered != PhonePairing.Deliver.Delivered) return "the relay refused the packet: $delivered"
        println("phonecheck: packet laid down for the TV")
        return when (val linked = pairing.awaitTv(voucher)) {
            is PhonePairing.Linked.Ok -> "the TV is listed under «Meine Geräte»"
            PhonePairing.Linked.TimedOut -> "no TV within two minutes"
            PhonePairing.Linked.Several -> "more than one new TV under «Meine Geräte»"
            PhonePairing.Linked.Ended -> "the session may not ask «Meine Geräte»"
        }
    } finally {
        runCatching { auth.logout() }
    }
}
