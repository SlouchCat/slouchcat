package com.slouchcat.core.api

import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.apis.SystemApi
import com.slouchcat.core.api.generated.models.Health
import com.slouchcat.core.api.generated.models.LoginRequest
import com.slouchcat.core.api.generated.models.SetupRequest
import okhttp3.Cookie
import okhttp3.CookieJar
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * The generated client against a real server (task android:contract, concept 27 12 «Vertrag»): the
 * task starts the server of this repository on its own port with empty data and passes its address
 * as `slouchcat.contract.base`. Signs in with the browser cookie for now; stage 3 part 2 moves this
 * to the Bearer of an app session.
 */
class ContractTest {
    private val base: HttpUrl = System.getProperty("slouchcat.contract.base").orEmpty().also {
        require(it.isNotEmpty()) { "run through `task android:contract`, which starts the server" }
    }.toHttpUrl()

    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0")

    /** A cookie jar of its own: the transition until the app signs in with a Bearer token. */
    private fun client(): OkHttpClient {
        val cookies = mutableMapOf<String, Cookie>()
        val jar = object : CookieJar {
            override fun saveFromResponse(url: HttpUrl, cookies0: List<Cookie>) = cookies0.forEach { cookies[it.name] = it }
            override fun loadForRequest(url: HttpUrl) = cookies.values.toList()
        }
        return SlouchcatHttp.client(identity, builder = OkHttpClient.Builder().cookieJar(jar))
    }

    @Test
    fun healthAnswersWithoutSession() {
        val health = SlouchcatApi.create(::SystemApi, base, client()).getHealth()
        assertNotNull(health)
        assertEquals(Health.Status.OK, health!!.status)
        assertTrue(health.version.isNotBlank())
    }

    @Test
    fun signInAndReadAsTheApp() {
        val http = client()
        val setup = SlouchcatApi.create(::SetupApi, base, http)
        if (setup.getSetupStatus()!!.setupRequired) {
            setup.runSetup(SetupRequest(email = EMAIL, password = PASSWORD))
        }
        // A second client signs in from scratch, as an app does after the setup in the browser.
        val app = client()
        val appAuth = SlouchcatApi.create(::AuthApi, base, app)
        val session = appAuth.login(LoginRequest(email = EMAIL, password = PASSWORD))!!
        assertEquals(EMAIL, session.account.email)

        val profiles = appAuth.listMyProfiles()!!
        val primary = profiles.single { it.isPrimary }
        val selected = appAuth.selectProfile(primary.id)!!
        assertEquals(primary.id, selected.profile!!.id)

        val items = SlouchcatApi.create(::ItemsApi, base, app)
        val libraries = items.listVisibleLibraries()!!
        assertTrue("a fresh server has no libraries", libraries.isEmpty())
        val home = items.getHome()!!
        assertNotNull(home.rows)
    }

    private companion object {
        const val EMAIL = "contract-owner@example.test"
        const val PASSWORD = "contract-password-0123"
    }
}
