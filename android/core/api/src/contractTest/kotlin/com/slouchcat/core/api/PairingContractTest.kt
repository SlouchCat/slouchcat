package com.slouchcat.core.api

import com.google.crypto.tink.subtle.X25519
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.generated.apis.AccountApi
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.infrastructure.ClientException
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.api.generated.models.PairingRedeemRequest
import com.slouchcat.core.api.generated.models.PairingVoucherRequest
import com.slouchcat.core.api.generated.models.SetupRequest
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import java.time.Duration
import java.time.OffsetDateTime

/**
 * «Fernseher verbinden» of the phone app (concept 27 section 10, stage 9) against a real server: the
 * voucher for the TV's key (`createPairingVoucher`), a key that is none (422), the redeem that makes
 * a TV under «Meine Geräte», and the cleanup: the TV signed out again, and a voucher whose phone
 * signed out redeems nothing (410).
 */
class PairingContractTest {
    private val base: HttpUrl = System.getProperty("slouchcat.contract.base").orEmpty().also {
        require(it.isNotEmpty()) { "run through `task android:contract`, which starts the server" }
    }.toHttpUrl()

    private val phone = ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0")
    private val tv = ClientIdentity(ClientIdentity.Kind.TV, "0.1.0")

    private fun signedIn(): OkHttpClient {
        val setup = SlouchcatApi.create(::SetupApi, base, SlouchcatHttp.client(phone))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = EMAIL, password = PASSWORD))
        val login = withinTheSignInLimit {
            SlouchcatApi.create(::AuthApi, base, SlouchcatHttp.client(phone)).appLogin(AppLoginRequest(EMAIL, PASSWORD, "Pairing phone", AppLoginRequest.Kind.PHONE))!!
        }
        val credentials = object : SessionCredentials {
            override val server = base
            override val bearerToken = login.token
            override val deviceId: String? = null
        }
        val http = SlouchcatHttp.client(phone, credentials)
        val auth = SlouchcatApi.create(::AuthApi, base, http)
        auth.selectProfile(auth.listMyProfiles()!!.single { it.isPrimary }.id)
        return http
    }

    private fun tvKey(): String = Base64Url.encode(X25519.publicFromPrivate(X25519.generatePrivateKey()))

    private fun redeem(voucher: String, key: String) = SlouchcatApi.create(::AuthApi, base, SlouchcatHttp.client(tv))
        .redeemPairingVoucher(PairingRedeemRequest(voucher, key, "Contract TV", "Android TV 14"))

    @Test
    fun aVoucherPairsATvThatSignsOutAgain() {
        val http = signedIn()
        val key = tvKey()
        val issued = SlouchcatApi.create(::AuthApi, base, http).createPairingVoucher(PairingVoucherRequest(key))!!
        assertEquals(32, Base64Url.decode(issued.voucher)!!.size)
        val left = Duration.between(OffsetDateTime.now(), issued.expiresAt)
        assertTrue("valid for $left", left > Duration.ofSeconds(60) && left <= Duration.ofSeconds(125))

        // The TV redeems: a device of the type tv under «Meine Geräte».
        val redeemed = redeem(issued.voucher, key)!!
        assertTrue(redeemed.sealedToken.isNotBlank() && redeemed.signature.isNotBlank())
        val account = SlouchcatApi.create(::AccountApi, base, http)
        val tvs = account.listMyDevices()!!.filter { it.type == MyDevice.Type.TV && it.name == "Contract TV" }
        assertEquals(1, tvs.size)

        // Cleanup: the phone signs the TV out; the voucher is used up.
        account.signOutMyDevice(tvs.single().id)
        assertFalse(account.listMyDevices()!!.any { it.id == tvs.single().id })
        expect(410) { redeem(issued.voucher, key) }
        SlouchcatApi.create(::AuthApi, base, http).logout()
    }

    @Test
    fun aKeyThatIsNoneIsRefused() {
        val http = signedIn()
        expect(422) { SlouchcatApi.create(::AuthApi, base, http).createPairingVoucher(PairingVoucherRequest("A".repeat(42) + "!")) }
        SlouchcatApi.create(::AuthApi, base, http).logout()
    }

    @Test
    fun aVoucherOfAPhoneThatSignedOutRedeemsNothing() {
        val http = signedIn()
        val key = tvKey()
        val issued = SlouchcatApi.create(::AuthApi, base, http).createPairingVoucher(PairingVoucherRequest(key))!!
        SlouchcatApi.create(::AuthApi, base, http).logout()
        expect(410) { redeem(issued.voucher, key) }
    }

    private fun expect(status: Int, call: () -> Any?) {
        try {
            call()
            fail("answered 2xx, want $status")
        } catch (e: ClientException) {
            assertEquals(status, e.statusCode)
        }
    }

    private companion object {
        const val EMAIL = "contract-owner@example.test"
        const val PASSWORD = "contract-password-0123"
    }
}
