package com.slouchcat.core.session

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runTest
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

private class Servers(initial: SavedServer?) : ServerStore {
    val flow = MutableStateFlow(initial)
    override val server: Flow<SavedServer?> = flow
    override suspend fun save(server: SavedServer) { flow.value = server }
    override suspend fun confirmKey(serverKey: String) { flow.value = flow.value?.copy(serverKey = serverKey) }
    override suspend fun updateVersion(version: String) { flow.value = flow.value?.copy(version = version) }
    override suspend fun clear() { flow.value = null }
}

private class Tokens(var stored: StoredCredentials? = null) : TokenStore {
    override suspend fun read() = stored
    override suspend fun write(credentials: StoredCredentials) { stored = credentials.takeIf { it.token != null || it.deviceId != null } }
}

private class Accounts(var email: String? = null, var profile: ActiveProfile? = null) : AccountStore {
    override suspend fun read() = email to profile
    override suspend fun write(email: String?, profile: ActiveProfile?) { this.email = email; this.profile = profile }
}

class SessionManagerTest {
    private val http = SavedServer("http://192.0.2.10:8096/".toHttpUrl(), "1.42.0", serverKey = "key")
    private val primary = ActiveProfile(1, "Primary", isPrimary = true, accent = "ember")

    private fun TestScope.manager(servers: Servers, tokens: Tokens = Tokens(), accounts: Accounts = Accounts()) =
        SessionManager(servers, tokens, accounts).also { it.start(backgroundScope) }

    @Test
    fun theStatesFollowSetupSignInAndProfile() = runTest(StandardTestDispatcher()) {
        val servers = Servers(null)
        val tokens = Tokens()
        val session = manager(servers, tokens)
        assertEquals(SessionState.NoServer, session.ready())

        servers.flow.value = http
        testScheduler.runCurrent()
        assertEquals(SessionState.SignedOut(http), session.state.value)
        assertNull(session.bearerToken)

        session.signedIn("token-1", "device-1", "a@example.test", null)
        assertEquals(SessionState.ChooseProfile(http, "a@example.test"), session.state.value)
        assertEquals(StoredCredentials("token-1", "device-1"), tokens.stored)
        assertEquals("token-1", session.bearerToken)
        // With a session the server knows the device; the header is not sent.
        assertNull(session.deviceId)

        session.profileChosen(primary)
        assertEquals(SessionState.Ready(http, "a@example.test", primary), session.state.value)
    }

    @Test
    fun signingOutKeepsServerAndDeviceId() = runTest(StandardTestDispatcher()) {
        val tokens = Tokens(StoredCredentials("token-1", "device-1"))
        val accounts = Accounts("a@example.test", primary)
        val session = manager(Servers(http), tokens, accounts)
        assertEquals(SessionState.Ready(http, "a@example.test", primary), session.ready())

        session.signedOut()
        assertEquals(SessionState.SignedOut(http), session.state.value)
        assertEquals(StoredCredentials(null, "device-1"), tokens.stored)
        assertNull(accounts.email)
        assertNull(session.bearerToken)
        assertEquals("device-1", session.deviceId)
        assertEquals("device-1", session.knownDeviceId)
    }

    @Test
    fun anEditedProfileInUseChangesWithoutANewEpoch() = runTest(StandardTestDispatcher()) {
        val accounts = Accounts("a@example.test", primary)
        val session = manager(Servers(http), Tokens(StoredCredentials("token-1", "device-1")), accounts)
        session.ready()
        val epoch = session.epoch.value
        val renamed = primary.copy(name = "Main", accent = "sky")
        session.profileUpdated(renamed)
        assertEquals(SessionState.Ready(http, "a@example.test", renamed), session.state.value)
        assertEquals(renamed, accounts.profile)
        assertEquals(epoch, session.epoch.value)
        // Another profile than the one in use changes nothing.
        session.profileUpdated(ActiveProfile(2, "Child", isPrimary = false))
        assertEquals(renamed, accounts.profile)
    }

    @Test
    fun aLostTokenForgetsAccountAndProfile() = runTest(StandardTestDispatcher()) {
        val accounts = Accounts("a@example.test", primary)
        val session = manager(Servers(http), Tokens(null), accounts)
        assertEquals(SessionState.SignedOut(http), session.ready())
        assertNull(accounts.profile)
    }

    @Test
    fun aServerOverHttpWithoutKeyNeedsTheSetup() = runTest(StandardTestDispatcher()) {
        val session = manager(Servers(http.copy(serverKey = null)), Tokens(StoredCredentials("t", "d")), Accounts("a@example.test", primary))
        assertEquals(SessionState.NoServer, session.ready())
        val https = SavedServer("https://media.example.test/".toHttpUrl(), "1.42.0")
        val secure = manager(Servers(https), Tokens(StoredCredentials("t", "d")), Accounts("a@example.test", primary))
        assertEquals(SessionState.Ready(https, "a@example.test", primary), secure.ready())
    }

    @Test
    fun suppressedCredentialsSendNoToken() = runTest(StandardTestDispatcher()) {
        val session = manager(Servers(http), Tokens(StoredCredentials("token-1", "device-1")), Accounts("a@example.test", primary))
        session.ready()
        session.credentialsSuppressed = true
        assertNull(session.bearerToken)
        // Nor the device id: the server with the unconfirmed key may be another device (L-1).
        assertNull(session.deviceId)
        assertNull(session.knownDeviceId)
        assertEquals("key", session.serverKey)
    }

    @Test
    fun anotherKeyUnderTheSameAddressIsAnotherServer() = runTest(StandardTestDispatcher()) {
        val servers = Servers(http)
        val tokens = Tokens(StoredCredentials("token-1", "device-1"))
        val session = manager(servers, tokens, Accounts("a@example.test", primary))
        var resets = 0
        session.onServerChanged = { resets++ }
        session.ready()
        // The setup ran again for the same address and got another key (security review H-2).
        servers.flow.value = http.copy(serverKey = "other-key")
        testScheduler.runCurrent()
        assertEquals(SessionState.SignedOut(http.copy(serverKey = "other-key")), session.state.value)
        assertNull("token and device id stayed with a new key", tokens.stored)
        assertNull(session.bearerToken)
        assertNull(session.knownDeviceId)
        assertEquals(1, resets)
    }

    @Test
    fun aConfirmedKeyCountsAtOnceAndKeepsTheSession() = runTest(StandardTestDispatcher()) {
        val servers = Servers(http)
        val session = manager(servers, Tokens(StoredCredentials("token-1", "device-1")), Accounts("a@example.test", primary))
        var resets = 0
        session.onServerChanged = { resets++ }
        session.ready()
        session.keyConfirmed("new-key")
        assertEquals("new-key", session.serverKey)
        testScheduler.runCurrent()
        assertEquals(0, resets)
        assertEquals("token-1", session.bearerToken)
    }

    @Test
    fun everyChangeOfUserCountsAnEpoch() = runTest(StandardTestDispatcher()) {
        val session = manager(Servers(http))
        session.ready()
        val start = session.epoch.value
        session.signedIn("t", "d", "a@example.test", null)
        session.profileChosen(primary)
        session.signedOut()
        assertEquals(start + 3, session.epoch.value)
    }

    @Test
    fun anotherServerEndsTheSession() = runTest(StandardTestDispatcher()) {
        val servers = Servers(http)
        val tokens = Tokens(StoredCredentials("token-1", "device-1"))
        val session = manager(servers, tokens, Accounts("a@example.test", primary))
        session.ready()
        val other = SavedServer("https://other.example.test/".toHttpUrl(), "1.42.0")
        servers.flow.value = other
        testScheduler.runCurrent()
        assertEquals(SessionState.SignedOut(other), session.state.value)
        // The device id of the old server goes as well (code review minor 10).
        assertNull(tokens.stored)
    }
}
