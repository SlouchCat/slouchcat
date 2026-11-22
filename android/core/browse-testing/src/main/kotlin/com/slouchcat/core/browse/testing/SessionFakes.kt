package com.slouchcat.core.browse.testing

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.api.generated.models.Account
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.api.generated.models.SessionInfo
import com.slouchcat.core.session.AccountStore
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.LibrariesRepository
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.session.TokenStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.test.TestScope
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl

/* Test doubles of the seams of core/session, shared by the ViewModel and screen tests of both apps. */

val HTTP_SERVER = SavedServer("http://192.0.2.10:8096/".toHttpUrl(), "1.42.0", serverKey = "server-key")
val HTTPS_SERVER = SavedServer("https://media.example.test/".toHttpUrl(), "1.42.0")
const val EMAIL = "viewer@example.test"
val PRIMARY = ActiveProfile(1, "Primary", isPrimary = true, accent = "ember")
val CHILD = ActiveProfile(2, "Child", isPrimary = false, accent = "sky")

class FakeServers(initial: SavedServer?) : ServerStore {
    val flow = MutableStateFlow(initial)
    var confirmed: String? = null
    override val server: Flow<SavedServer?> = flow
    override suspend fun save(server: SavedServer) {
        flow.value = server
    }
    override suspend fun confirmKey(serverKey: String) {
        confirmed = serverKey
        flow.value = flow.value?.copy(serverKey = serverKey)
    }
    override suspend fun updateVersion(version: String) {
        flow.value = flow.value?.copy(version = version)
    }
    override suspend fun clear() {
        flow.value = null
    }
}

class FakeTokens(var stored: StoredCredentials? = null) : TokenStore {
    override suspend fun read() = stored
    override suspend fun write(credentials: StoredCredentials) {
        stored = credentials.takeIf { it.token != null || it.deviceId != null }
    }
}

class FakeAccounts(var email: String? = null, var profile: ActiveProfile? = null) : AccountStore {
    override suspend fun read() = email to profile
    override suspend fun write(email: String?, profile: ActiveProfile?) {
        this.email = email
        this.profile = profile
    }
}

class FakeLibraries(var answer: ApiResult<List<LibraryNav>> = ApiResult.Ok(emptyList())) : LibrariesRepository {
    var calls = 0
    override suspend fun libraries(server: HttpUrl) = answer.also { calls++ }
}

/** A started [SessionManager] over fakes; signed in with [profile] when [token] is set. */
fun session(
    scope: CoroutineScope,
    server: SavedServer? = HTTP_SERVER,
    token: String? = "token",
    profile: ActiveProfile? = PRIMARY,
    tokens: FakeTokens = FakeTokens(token?.let { StoredCredentials(it, "device-1") }),
    servers: FakeServers = FakeServers(server),
): SessionManager = SessionManager(servers, tokens, FakeAccounts(if (token != null) EMAIL else null, if (token != null) profile else null)).also { it.start(scope) }

fun account(email: String = EMAIL) = Account(1, email, Account.Role.USER, isOwner = false, maxRating = 18, allLibraries = true, libraryIds = emptyList())

fun sessionInfo(email: String = EMAIL) = SessionInfo(account(email))

fun summary(id: Long, name: String, hasPin: Boolean = false, primary: Boolean = false, accent: Accent? = null) =
    ProfileSummary(id, name, hasPin, primary, accent = accent)

/**
 * Runs everything that is due, also the work in `backgroundScope` (the session follows its stores
 * there): `advanceUntilIdle` alone leaves background work waiting.
 */
@OptIn(kotlinx.coroutines.ExperimentalCoroutinesApi::class)
fun TestScope.settle() {
    repeat(3) {
        testScheduler.advanceUntilIdle()
        testScheduler.runCurrent()
    }
}
