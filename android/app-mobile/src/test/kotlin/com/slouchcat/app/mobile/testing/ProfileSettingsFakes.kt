package com.slouchcat.app.mobile.testing

import com.slouchcat.app.mobile.profilesettings.ProfileSettingsRepository
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.api.generated.models.EffectiveLimits
import com.slouchcat.core.api.generated.models.Language
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibrarySummary
import com.slouchcat.core.api.generated.models.PlaybackLimits
import com.slouchcat.core.api.generated.models.Profile
import com.slouchcat.core.api.generated.models.ProfileRequest
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.api.generated.models.ProfileUpdate
import com.slouchcat.core.api.generated.models.RatingStep
import com.slouchcat.core.api.generated.models.RatingSystem
import com.slouchcat.core.api.generated.models.SessionInfo
import com.slouchcat.core.api.generated.models.SubtitleMode
import okhttp3.HttpUrl

fun profile(
    id: Long,
    name: String,
    primary: Boolean = false,
    rating: Int = 18,
    hasPin: Boolean = false,
    hasOfflinePin: Boolean = false,
    accent: Accent? = Accent.EMBER,
    watchLimit: Int? = null,
    allLibraries: Boolean = true,
    libraryIds: List<Long> = emptyList(),
) = Profile(
    id = id,
    name = name,
    maxRating = rating,
    language = Language.DE,
    allLibraries = allLibraries,
    libraryIds = libraryIds,
    hasPin = hasPin,
    hasOfflinePin = hasOfflinePin,
    isPrimary = primary,
    limits = PlaybackLimits(maxStreams = 2, watchLimitMinutes = watchLimit),
    effectiveLimits = EffectiveLimits(),
    audioLanguages = setOf("de"),
    subtitleMode = SubtitleMode.FORCED,
    subtitleLanguages = setOf("de"),
    accent = accent,
)

/** The FSK steps with their labels, as a server in Germany sends them. */
val FSK = RatingSystem("DE", listOf(0, 6, 12, 16, 18).map { RatingStep(it, "FSK $it") }, name = "FSK")

/** The profiles of an account in memory, with every change recorded (PINs only by their length). */
class FakeProfileSettings(
    vararg initial: Profile,
) : ProfileSettingsRepository {
    val profiles = initial.toMutableList()
    var libraries: List<LibrarySummary> = listOf(LibrarySummary(1, "Filme", LibraryKind.VIDEO), LibrarySummary(2, "Serien", LibraryKind.VIDEO))
    var ceiling = 18
    var system: RatingSystem? = FSK

    /** The answer of `listProfiles` instead of the list (403 `primary_profile_required`). */
    var listAnswer: ApiResult<List<Profile>>? = null
    var saveFailure: ApiResult<Nothing>? = null
    var pinFailure: ApiResult<Nothing>? = null
    val calls = mutableListOf<String>()
    val created = mutableListOf<ProfileRequest>()
    val updated = mutableListOf<Pair<Long, ProfileUpdate>>()
    private var nextId = 100L

    override suspend fun profiles(server: HttpUrl): ApiResult<List<Profile>> {
        calls += "list"
        return listAnswer ?: ApiResult.Ok(profiles.toList())
    }

    override suspend fun summaries(server: HttpUrl): ApiResult<List<ProfileSummary>> {
        calls += "summaries"
        return ApiResult.Ok(profiles.map { ProfileSummary(it.id, it.name, it.hasPin, it.isPrimary, accent = it.accent) })
    }

    override suspend fun libraries(server: HttpUrl) = ApiResult.Ok(libraries)

    override suspend fun session(server: HttpUrl) =
        ApiResult.Ok(SessionInfo(account().copy(maxRating = ceiling), ratingSystem = system))

    override suspend fun create(server: HttpUrl, request: ProfileRequest): ApiResult<Profile> {
        calls += "create:${request.name}:pin=${request.pin?.length ?: 0}"
        saveFailure?.let { return it }
        created += request.copy(pin = request.pin?.let { "*".repeat(it.length) })
        val made = profile(
            nextId++, request.name, rating = request.maxRating ?: 18, hasPin = request.pin != null, accent = request.accent,
            watchLimit = request.limits?.watchLimitMinutes,
        )
        profiles += made
        return ApiResult.Ok(made)
    }

    override suspend fun update(server: HttpUrl, id: Long, update: ProfileUpdate): ApiResult<Profile> {
        calls += "update:$id"
        saveFailure?.let { return it }
        updated += id to update
        val index = profiles.indexOfFirst { it.id == id }.takeIf { it >= 0 } ?: return ApiResult.NotFound
        val now = profiles[index]
        val next = now.copy(
            name = update.name ?: now.name,
            maxRating = update.maxRating ?: now.maxRating,
            accent = update.accent ?: now.accent,
            language = update.language ?: now.language,
            allLibraries = update.allLibraries ?: now.allLibraries,
            libraryIds = update.libraryIds?.toList() ?: now.libraryIds,
            limits = update.limits ?: now.limits,
        )
        profiles[index] = next
        return ApiResult.Ok(next)
    }

    override suspend fun delete(server: HttpUrl, id: Long): ApiResult<Unit> {
        calls += "delete:$id"
        profiles.removeAll { it.id == id }
        return ApiResult.Ok(Unit)
    }

    private fun change(id: Long, call: String, transform: (Profile) -> Profile): ApiResult<Unit> {
        calls += call
        pinFailure?.let { return it }
        val index = profiles.indexOfFirst { it.id == id }.takeIf { it >= 0 } ?: return ApiResult.NotFound
        profiles[index] = transform(profiles[index])
        return ApiResult.Ok(Unit)
    }

    override suspend fun setPin(server: HttpUrl, id: Long, pin: String) = change(id, "pin:$id:${pin.length}") { it.copy(hasPin = true) }

    override suspend fun removePin(server: HttpUrl, id: Long) = change(id, "unpin:$id") { it.copy(hasPin = false) }

    override suspend fun setOfflinePin(server: HttpUrl, id: Long, pin: String) = change(id, "offline:$id:${pin.length}") { it.copy(hasOfflinePin = true) }

    override suspend fun removeOfflinePin(server: HttpUrl, id: Long) = change(id, "unoffline:$id") { it.copy(hasOfflinePin = false) }
}
