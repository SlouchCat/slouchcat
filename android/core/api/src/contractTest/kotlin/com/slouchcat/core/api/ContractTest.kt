package com.slouchcat.core.api

import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.crypto.ServerProof
import com.slouchcat.core.api.generated.apis.AccountApi
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.apis.SystemApi
import com.slouchcat.core.api.generated.infrastructure.ClientException
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.Health
import com.slouchcat.core.api.generated.models.ServerProofRequest
import com.slouchcat.core.api.generated.models.SessionKind
import com.slouchcat.core.api.generated.models.SetupRequest
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

/**
 * The generated client against a real server (task android:contract, concept 27 12 «Vertrag»): the
 * task starts the server of this repository on its own port with empty data and passes its address
 * as `slouchcat.contract.base`. The app's way through it: prove the server, `appLogin` with a
 * Bearer token, choose a profile, read a list, «Meine Geräte», sign out (then 401).
 */
class ContractTest {
    private val base: HttpUrl = System.getProperty("slouchcat.contract.base").orEmpty().also {
        require(it.isNotEmpty()) { "run through `task android:contract`, which starts the server" }
    }.toHttpUrl()

    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0")

    /** The client of the app: the gate of «Server weist sich aus» and the token of the session. */
    private class Session(val base: HttpUrl) : SessionCredentials, TrustedServer {
        override val server: HttpUrl = base

        @Volatile override var serverKey: String? = null

        @Volatile override var bearerToken: String? = null

        override val deviceId: String? = null
    }

    private fun client(session: Session): OkHttpClient {
        val gate = ServerIdentityGate(session, { 1L }, ApiServerProver(SlouchcatHttp.client(identity)), SessionEvents())
        return SlouchcatHttp.client(identity, session, gate = gate)
    }

    private fun ensureSetUp() {
        val setup = SlouchcatApi.create(::SetupApi, base, SlouchcatHttp.client(identity))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = EMAIL, password = PASSWORD))
    }

    @Test
    fun healthNamesContractAndKey() {
        val health = SlouchcatApi.create(::SystemApi, base, SlouchcatHttp.client(identity)).getHealth()!!
        assertEquals(Health.Status.OK, health.status)
        assertTrue(health.version.isNotBlank())
        assertTrue(health.contractVersion.isNotBlank())
        assertEquals(32, Base64Url.decode(health.serverKey)!!.size)
    }

    @Test
    fun theServerProvesItsKey() {
        val http = SlouchcatHttp.client(identity)
        val key = SlouchcatApi.create(::SystemApi, base, http).getHealth()!!.serverKey
        val nonce = ServerProof.nonce()
        val proof = SlouchcatApi.create(::SystemApi, base, http).proveServer(ServerProofRequest(Base64Url.encode(nonce)))!!
        assertEquals(key, proof.serverKey)
        assertTrue(ServerProof.valid(key, nonce, proof.signature))
        // Another nonce: the old signature does not fit.
        assertTrue(!ServerProof.valid(key, ServerProof.nonce(), proof.signature))
    }

    @Test
    fun signInChooseAProfileReadSeeTheDeviceAndSignOut() {
        ensureSetUp()
        val session = Session(base)
        val http = client(session)
        // As the setup does: the key comes from getHealth over a client without gate and credentials.
        session.serverKey = SlouchcatApi.create(::SystemApi, base, SlouchcatHttp.client(identity)).getHealth()!!.serverKey

        val login = SlouchcatApi.create(::AuthApi, base, http)
            .appLogin(AppLoginRequest(EMAIL, PASSWORD, DEVICE_NAME, AppLoginRequest.Kind.PHONE))!!
        assertEquals(EMAIL, login.session.account.email)
        // An account with a single profile starts with it (ADR 0040, as at `login`).
        login.session.profile?.let { assertTrue(it.isPrimary) }
        assertTrue(login.deviceId.matches(Regex("^[0-9a-f]{32}$")))
        session.bearerToken = login.token

        val auth = SlouchcatApi.create(::AuthApi, base, http)
        val primary = auth.listMyProfiles()!!.single { it.isPrimary }
        val selected = auth.selectProfile(primary.id)!!
        assertEquals(primary.id, selected.profile!!.id)

        val items = SlouchcatApi.create(::ItemsApi, base, http)
        assertTrue("a fresh server has no libraries", items.listVisibleLibraries()!!.isEmpty())
        assertNotNull(items.getHome()!!.rows)

        val devices = SlouchcatApi.create(::AccountApi, base, http).listMyDevices()!!
        val mine = devices.single { it.current }
        assertEquals(login.deviceId, mine.id)
        assertEquals(DEVICE_NAME, mine.name)
        assertEquals(SessionKind.APP, mine.sessionKind)

        auth.logout()
        try {
            auth.getSession()
            fail("the token still works after logout")
        } catch (e: ClientException) {
            assertEquals(401, e.statusCode)
        }
    }

    @Test
    fun aWrongPasswordIsInvalidCredentials() {
        ensureSetUp()
        try {
            SlouchcatApi.create(::AuthApi, base, SlouchcatHttp.client(identity))
                .appLogin(AppLoginRequest(EMAIL, "wrong-password-0123", DEVICE_NAME, AppLoginRequest.Kind.PHONE))
            fail("a wrong password signed in")
        } catch (e: ClientException) {
            assertEquals(401, e.statusCode)
        }
    }

    private companion object {
        const val EMAIL = "contract-owner@example.test"
        const val PASSWORD = "contract-password-0123"
        const val DEVICE_NAME = "Contract phone · Android 16"
    }
}
