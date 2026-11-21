package com.slouchcat.core.session

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.AccountApi
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.models.AccountUpdate
import com.slouchcat.core.api.generated.models.AppLogin
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.api.generated.models.SelectProfileRequest
import com.slouchcat.core.api.generated.models.SessionInfo
import okhttp3.Call
import okhttp3.HttpUrl

/** What `appLogin` needs to know about this device (concept 27, 4.2). */
data class DeviceDescription(val name: String, val kind: AppLoginRequest.Kind)

/** Sign-in, session and profiles of the account (ADR 0040); the seam ViewModel tests fake. */
interface AuthRepository {
    suspend fun appLogin(server: HttpUrl, email: String, password: String, device: DeviceDescription, deviceId: String?): ApiResult<AppLogin>

    suspend fun session(server: HttpUrl): ApiResult<SessionInfo>

    suspend fun profiles(server: HttpUrl): ApiResult<List<ProfileSummary>>

    suspend fun selectProfile(server: HttpUrl, id: Long, pin: String?): ApiResult<SessionInfo>

    suspend fun logout(server: HttpUrl): ApiResult<Unit>

    suspend fun updateAccount(server: HttpUrl, currentPassword: String, email: String?, newPassword: String?): ApiResult<SessionInfo>
}

class ApiAuthRepository(private val api: ApiCaller, private val http: Call.Factory) : AuthRepository {
    private fun auth(server: HttpUrl) = SlouchcatApi.create(::AuthApi, server, http)

    override suspend fun appLogin(server: HttpUrl, email: String, password: String, device: DeviceDescription, deviceId: String?) =
        api.call("signin") { auth(server).appLogin(AppLoginRequest(email, password, device.name, device.kind, deviceId)) }

    override suspend fun session(server: HttpUrl) = api.call("session") { auth(server).getSession() }

    override suspend fun profiles(server: HttpUrl) = api.call("profiles") { auth(server).listMyProfiles() }

    override suspend fun selectProfile(server: HttpUrl, id: Long, pin: String?) =
        api.call("profiles") { auth(server).selectProfile(id, if (pin == null) null else SelectProfileRequest(pin)) }

    override suspend fun logout(server: HttpUrl) = api.call("signout") { auth(server).logout() }

    override suspend fun updateAccount(server: HttpUrl, currentPassword: String, email: String?, newPassword: String?) =
        api.call("account") {
            SlouchcatApi.create(::AccountApi, server, http).updateAccount(AccountUpdate(currentPassword = currentPassword, email = email, newPassword = newPassword))
        }
}

/** «Meine Geräte» (concept 27, 4.5). */
interface DevicesRepository {
    suspend fun devices(server: HttpUrl): ApiResult<List<MyDevice>>

    suspend fun signOut(server: HttpUrl, deviceId: String): ApiResult<Unit>
}

class ApiDevicesRepository(private val api: ApiCaller, private val http: Call.Factory) : DevicesRepository {
    private fun account(server: HttpUrl) = SlouchcatApi.create(::AccountApi, server, http)

    override suspend fun devices(server: HttpUrl) = api.call("devices") { account(server).listMyDevices() }

    override suspend fun signOut(server: HttpUrl, deviceId: String) = api.call("devices") { account(server).signOutMyDevice(deviceId) }
}

/** The libraries the active profile sees (`listVisibleLibraries`), for the media choice of the shell. */
interface LibrariesRepository {
    suspend fun libraries(server: HttpUrl): ApiResult<List<LibraryNav>>
}

class ApiLibrariesRepository(private val api: ApiCaller, private val http: Call.Factory) : LibrariesRepository {
    override suspend fun libraries(server: HttpUrl) = api.call("libraries") { SlouchcatApi.create(::ItemsApi, server, http).listVisibleLibraries() }
}
