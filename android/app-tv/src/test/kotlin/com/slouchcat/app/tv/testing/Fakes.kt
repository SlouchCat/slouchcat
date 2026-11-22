package com.slouchcat.app.tv.testing

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.AppLogin
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.api.generated.models.SessionInfo
import com.slouchcat.core.api.generated.models.SessionKind
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.sessionInfo
import com.slouchcat.core.browse.testing.summary
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.DeviceDescription
import com.slouchcat.core.session.DevicesRepository
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.ServerProbe
import okhttp3.HttpUrl
import java.time.OffsetDateTime

val TV_DEVICE = DeviceDescription("Shield · Android TV 11", com.slouchcat.core.api.generated.models.AppLoginRequest.Kind.TV)

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

class FakeProbe(var result: ProbeResult) : ServerProbe {
    override suspend fun probe(address: HttpUrl) = result
}
