package com.slouchcat.app.mobile.profilesettings

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.ProfilesApi
import com.slouchcat.core.api.generated.models.LibrarySummary
import com.slouchcat.core.api.generated.models.OfflinePinRequest
import com.slouchcat.core.api.generated.models.PinRequest
import com.slouchcat.core.api.generated.models.Profile
import com.slouchcat.core.api.generated.models.ProfileRequest
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.api.generated.models.ProfileUpdate
import com.slouchcat.core.api.generated.models.SessionInfo
import okhttp3.Call
import okhttp3.HttpUrl

/**
 * «Profile verwalten» (K-2): the profiles of the own account with their settings. Every change
 * needs the account's primary profile active; the server answers 403 `primary_profile_required`
 * otherwise and decides every limit itself (the account's ceiling, the libraries it may give).
 */
interface ProfileSettingsRepository {
    /** `listProfiles`: every setting; only with the primary profile active. */
    suspend fun profiles(server: HttpUrl): ApiResult<List<Profile>>

    /** `listMyProfiles`: names and PIN marks, for the page another profile only reads. */
    suspend fun summaries(server: HttpUrl): ApiResult<List<ProfileSummary>>

    /** `listAccountLibraries`: the libraries the account may give a profile. */
    suspend fun libraries(server: HttpUrl): ApiResult<List<LibrarySummary>>

    /** `getSession`: the rating country's steps and the account's ceiling. */
    suspend fun session(server: HttpUrl): ApiResult<SessionInfo>

    suspend fun create(server: HttpUrl, request: ProfileRequest): ApiResult<Profile>

    suspend fun update(server: HttpUrl, id: Long, update: ProfileUpdate): ApiResult<Profile>

    suspend fun delete(server: HttpUrl, id: Long): ApiResult<Unit>

    suspend fun setPin(server: HttpUrl, id: Long, pin: String): ApiResult<Unit>

    suspend fun removePin(server: HttpUrl, id: Long): ApiResult<Unit>

    suspend fun setOfflinePin(server: HttpUrl, id: Long, pin: String): ApiResult<Unit>

    suspend fun removeOfflinePin(server: HttpUrl, id: Long): ApiResult<Unit>
}

class ApiProfileSettingsRepository(private val api: ApiCaller, private val http: Call.Factory) : ProfileSettingsRepository {
    private fun client(server: HttpUrl) = SlouchcatApi.create(::ProfilesApi, server, http)

    override suspend fun profiles(server: HttpUrl) = api.call("profiles") { client(server).listProfiles() }

    override suspend fun summaries(server: HttpUrl) = api.call("profiles") { SlouchcatApi.create(::AuthApi, server, http).listMyProfiles() }

    override suspend fun libraries(server: HttpUrl) = api.call("profiles") { client(server).listAccountLibraries() }

    override suspend fun session(server: HttpUrl) = api.call("session") { SlouchcatApi.create(::AuthApi, server, http).getSession() }

    override suspend fun create(server: HttpUrl, request: ProfileRequest) = api.call("profiles") { client(server).createProfile(request) }

    override suspend fun update(server: HttpUrl, id: Long, update: ProfileUpdate) = api.call("profiles") { client(server).updateProfile(id, update) }

    override suspend fun delete(server: HttpUrl, id: Long) = api.call("profiles") { client(server).deleteProfile(id) }

    override suspend fun setPin(server: HttpUrl, id: Long, pin: String) = api.call("profiles") { client(server).setProfilePin(id, PinRequest(pin)) }

    override suspend fun removePin(server: HttpUrl, id: Long) = api.call("profiles") { client(server).removeProfilePin(id) }

    override suspend fun setOfflinePin(server: HttpUrl, id: Long, pin: String) =
        api.call("profiles") { client(server).setOfflinePin(id, OfflinePinRequest(pin)) }

    override suspend fun removeOfflinePin(server: HttpUrl, id: Long) = api.call("profiles") { client(server).removeOfflinePin(id) }
}
