package com.slouchcat.app.mobile.testing

import com.slouchcat.app.mobile.common.KeyConfirmation
import com.slouchcat.app.mobile.common.SessionActions
import com.slouchcat.app.mobile.common.SessionHooks
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.api.generated.models.Account
import com.slouchcat.core.api.generated.models.AppLogin
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.api.generated.models.SessionInfo
import com.slouchcat.core.api.generated.models.SessionKind
import com.slouchcat.core.session.AccountStore
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.DeviceDescription
import com.slouchcat.core.session.DevicesRepository
import com.slouchcat.core.session.LibrariesRepository
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerProbe
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.session.TokenStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import java.time.OffsetDateTime

/** Test doubles of the seams of core/session, shared by the ViewModel and screen tests. */
val HTTP_SERVER = SavedServer("http://192.0.2.10:8096/".toHttpUrl(), "1.42.0", serverKey = "server-key")
val HTTPS_SERVER = SavedServer("https://media.example.test/".toHttpUrl(), "1.42.0")
const val EMAIL = "viewer@example.test"
val PRIMARY = ActiveProfile(1, "Primary", isPrimary = true, accent = "ember")
val CHILD = ActiveProfile(2, "Child", isPrimary = false, accent = "sky")
val PHONE = DeviceDescription("Pixel 9 · Android 16", com.slouchcat.core.api.generated.models.AppLoginRequest.Kind.PHONE)

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

fun device(
    id: String,
    name: String?,
    current: Boolean = false,
    type: MyDevice.Type = MyDevice.Type.PHONE,
    kind: SessionKind? = SessionKind.APP,
    downloads: Int = 0,
) = MyDevice(
    id = id,
    lastSeenAt = OffsetDateTime.parse("2026-11-21T10:15:00Z"),
    downloads = downloads,
    current = current,
    name = name,
    type = type,
    sessionKind = kind,
    signedInAt = kind?.let { OffsetDateTime.parse("2026-11-20T08:00:00Z") },
)

/** [AuthRepository] with answers set per test and a record of the calls. */
class FakeAuth : AuthRepository {
    val calls = mutableListOf<String>()
    var login: ApiResult<AppLogin> = ApiResult.Ok(AppLogin("new-token", "device-2", sessionInfo()))
    var profiles: ApiResult<List<ProfileSummary>> = ApiResult.Ok(listOf(summary(1, "Primary", primary = true), summary(2, "Child", hasPin = true)))
    var select: (Long, String?) -> ApiResult<SessionInfo> = { _, _ -> ApiResult.Ok(sessionInfo()) }
    var logout: ApiResult<Unit> = ApiResult.Ok(Unit)
    var update: ApiResult<SessionInfo> = ApiResult.Ok(sessionInfo())
    var lastPassword: String? = null
    var lastDeviceId: String? = null

    override suspend fun appLogin(server: HttpUrl, email: String, password: String, device: DeviceDescription, deviceId: String?): ApiResult<AppLogin> {
        calls += "login:$email:${device.kind.value}"
        lastPassword = password
        lastDeviceId = deviceId
        return login
    }

    override suspend fun session(server: HttpUrl): ApiResult<SessionInfo> = ApiResult.Ok(sessionInfo())

    override suspend fun profiles(server: HttpUrl) = profiles.also { calls += "profiles" }

    override suspend fun selectProfile(server: HttpUrl, id: Long, pin: String?) = select(id, pin).also { calls += "select:$id:${pin ?: "-"}" }

    override suspend fun logout(server: HttpUrl) = logout.also { calls += "logout" }

    override suspend fun updateAccount(server: HttpUrl, currentPassword: String, email: String?, newPassword: String?): ApiResult<SessionInfo> {
        calls += "update:${email ?: "-"}:${if (newPassword == null) "-" else "new"}"
        return update
    }
}

class FakeDevices(var list: ApiResult<List<MyDevice>> = ApiResult.Ok(emptyList())) : DevicesRepository {
    val signedOut = mutableListOf<String>()
    var signOut: ApiResult<Unit> = ApiResult.Ok(Unit)
    override suspend fun devices(server: HttpUrl) = list
    override suspend fun signOut(server: HttpUrl, deviceId: String) = signOut.also { if (it is ApiResult.Ok) signedOut += deviceId }
}

class FakeLibraries(var answer: ApiResult<List<LibraryNav>> = ApiResult.Ok(emptyList())) : LibrariesRepository {
    var calls = 0
    override suspend fun libraries(server: HttpUrl) = answer.also { calls++ }
}

class FakeProbe(var result: ProbeResult) : ServerProbe {
    override suspend fun probe(address: HttpUrl) = result
}

class FakeKeys(override var presentedKey: String? = null) : KeyConfirmation {
    val log = mutableListOf<String>()
    override fun begin(key: String) {
        log += "begin:$key"
    }
    override suspend fun finish(confirmed: Boolean) {
        log += "finish:$confirmed"
    }
}

fun actions(session: SessionManager, auth: AuthRepository, hooks: SessionHooks = SessionHooks()) = SessionActions(session, auth, hooks)

/**
 * Runs everything that is due, also the work in `backgroundScope` (the session follows its stores
 * there): `advanceUntilIdle` alone leaves background work waiting.
 */
@OptIn(kotlinx.coroutines.ExperimentalCoroutinesApi::class)
fun kotlinx.coroutines.test.TestScope.settle() {
    repeat(3) {
        testScheduler.advanceUntilIdle()
        testScheduler.runCurrent()
    }
}
