package com.slouchcat.app.tv.profilesettings

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.api.generated.models.Language
import com.slouchcat.core.api.generated.models.LibrarySummary
import com.slouchcat.core.api.generated.models.PlaybackLimits
import com.slouchcat.core.api.generated.models.Profile
import com.slouchcat.core.api.generated.models.ProfileRequest
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.api.generated.models.ProfileUpdate
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.data.ProfileSettingsRepository
import com.slouchcat.core.media.toLoadable
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import okhttp3.HttpUrl

/*
 * «Profile verwalten» of the TV app: the ViewModel of the phone app (`app-mobile/.../profilesettings`)
 * without the offline PIN, which a TV may not set (E-8, Anhang B: `setOfflinePin` is not for `tv`).
 */

/** The PIN of a profile (K-2); the offline PIN of AN-19 is not for the TV (E-8). */
enum class PinKind(val min: Int, val max: Int) {
    Pin(4, 8),
}

/** What saving does with a PIN: leave it, set the new digits, or take it off. */
sealed interface PinChange {
    data object Keep : PinChange

    class Set(val digits: String) : PinChange {
        /** Never the digits (security review L-2). */
        override fun toString() = "Set(${digits.length})"
    }

    data object Remove : PinChange
}

/** Why the PIN form refuses its digits. */
enum class PinError { Format, Mismatch }

/** An age the profile may get: its label from the rating country, off above the account's ceiling. */
data class RatingChoice(val age: Int, val label: String?, val enabled: Boolean)

/** The open sheet «Profil bearbeiten» or «Neues Profil». */
data class ProfileEditor(
    /** `null`: a new profile. */
    val id: Long? = null,
    val isPrimary: Boolean = false,
    val name: String = "",
    val accent: Accent = Accent.EMBER,
    val rating: Int = 18,
    val language: Language = Language.DE,
    val allLibraries: Boolean = true,
    val libraryIds: Set<Long> = emptySet(),
    /** Minutes per day as typed; empty: no limit. */
    val watchLimit: String = "",
    /** The other limits of the profile, sent back unchanged (`limits` is one field). */
    val limits: PlaybackLimits = PlaybackLimits(),
    val hasPin: Boolean = false,
    val pin: PinChange = PinChange.Keep,
    /** The PIN form open in the sheet, with its two fields. */
    val pinForm: PinKind? = null,
    val first: String = "",
    val second: String = "",
    val pinError: PinError? = null,
    val nameMissing: Boolean = false,
    val watchLimitInvalid: Boolean = false,
    val languagesOpen: Boolean = false,
    /** «Profil löschen» pressed: the sheet shows the warning with «Behalten» and «Löschen». */
    val confirmDelete: Boolean = false,
    val busy: Boolean = false,
    /** The `code` of a refused request (`validation_failed`, `primary_profile_required` …), or [FAILED]. */
    val error: String? = null,
) {
    val isNew: Boolean get() = id == null

    /** Whether the PIN counts as there after saving. */
    fun pinOn(): Boolean = when (pin) {
        PinChange.Keep -> hasPin
        is PinChange.Set -> true
        PinChange.Remove -> false
    }

    /** Never the digits of the PIN fields (security review L-2). */
    override fun toString(): String = "ProfileEditor(id=$id, pin=$pin, form=$pinForm, busy=$busy, error=$error)"

    companion object {
        const val FAILED = "failed"
    }
}

data class ProfileSettingsUiState(
    /** The active profile is the account's primary one: only it changes profiles (else read-only). */
    val canManage: Boolean = true,
    val profiles: Loadable<List<Profile>> = Loadable.Loading,
    /** Read-only: names and PIN marks (`listMyProfiles`). */
    val summaries: Loadable<List<ProfileSummary>> = Loadable.Loading,
    val libraries: List<LibrarySummary> = emptyList(),
    val ratings: List<RatingChoice> = FALLBACK_AGES.map { RatingChoice(it, null, true) },
    val editor: ProfileEditor? = null,
) {
    /**
     * E-9, F-19 (web `advisePin`): the account has a profile with an age limit and its primary
     * profile has no PIN. A recommendation, no requirement.
     */
    val advisePin: Boolean
        get() {
            val list = (profiles as? Loadable.Ready)?.value ?: return false
            val primary = list.firstOrNull { it.isPrimary } ?: return false
            return !primary.hasPin && list.any { !it.isPrimary && it.maxRating < NO_AGE_LIMIT }
        }

    fun ratingLabel(age: Int): String? = ratings.firstOrNull { it.age == age }?.label

    companion object {
        /** The highest rating: a profile below it has an age limit. */
        const val NO_AGE_LIMIT = 18

        /** The steps without a rating system (an older server), as in the web app. */
        val FALLBACK_AGES = listOf(0, 6, 12, 16, 18)
    }
}

/** What the page tells once (a toast). */
sealed interface ProfileMessage {
    data class Added(val name: String) : ProfileMessage

    data object Saved : ProfileMessage

    data class Deleted(val name: String) : ProfileMessage

    /** The profile is saved, a PIN change was refused. */
    data object Partly : ProfileMessage
}

/**
 * «Profile verwalten» (mockup `/settings/profiles`, web `ProfileSettingsPage`): the profiles of
 * the account; with the primary profile active each opens «Profil bearbeiten» (name, colour,
 * rating, PIN, delete; language, libraries and watch time stay as they are) and «Profil
 * hinzufügen» creates one. Another profile only reads the list (`primary_profile_required`). PINs
 * change on «Speichern», as in the mockup. [onActiveChanged]: the profile in use was saved (its
 * name and colour in the session, its libraries in the media choice).
 */
class ProfileSettingsViewModel(
    private val session: SessionManager,
    private val repository: ProfileSettingsRepository,
    /** «Profile verwalten» in «Mehr»: the number of profiles, set again after every load. */
    private val profilesCount: LiveCount? = null,
    private val onActiveChanged: suspend (before: Profile?, saved: Profile, partly: Boolean) -> Unit = { _, _, _ -> },
) : ViewModel() {
    // A mirror of the server's rule, not a decision: the server answers `primary_profile_required`
    // for any other profile and stays the authority (rule 12, code review L-11).
    private val _state = MutableStateFlow(ProfileSettingsUiState(canManage = activeIsPrimary()))
    val state: StateFlow<ProfileSettingsUiState> = _state.asStateFlow()

    private val _messages = Channel<ProfileMessage>(Channel.BUFFERED)
    val messages: Flow<ProfileMessage> = _messages.receiveAsFlow()

    private var loading: Job? = null

    init {
        load()
    }

    private fun activeIsPrimary(): Boolean = (session.state.value as? SessionState.Ready)?.profile?.isPrimary == true

    private fun active(): ActiveProfile? = (session.state.value as? SessionState.Ready)?.profile

    fun retry() = load()

    private fun load(quiet: Boolean = false) {
        val server = session.server ?: return
        loading?.cancel()
        loading = viewModelScope.launch {
            if (!_state.value.canManage) return@launch readOnly(server)
            if (!quiet) _state.update { it.copy(profiles = Loadable.Loading) }
            when (val result = repository.profiles(server)) {
                is ApiResult.Ok -> {
                    _state.update { it.copy(profiles = Loadable.Ready(result.value)) }
                    profilesCount?.set(result.value.size)
                    if (_state.value.libraries.isEmpty() || !quiet) loadChoices(server)
                }
                is ApiResult.NotAllowed -> {
                    // The server says this profile may not (rule 12: it decides, the page follows).
                    _state.update { it.copy(canManage = false) }
                    readOnly(server)
                }
                else -> if (!quiet || _state.value.profiles !is Loadable.Ready) _state.update { it.copy(profiles = result.toLoadable()) }
            }
        }
    }

    private suspend fun readOnly(server: HttpUrl) {
        _state.update { it.copy(summaries = Loadable.Loading, editor = null) }
        val result = repository.summaries(server)
        _state.update { it.copy(summaries = result.toLoadable { list -> list.isEmpty() }) }
    }

    /** The libraries and the rating steps the sheet offers. */
    private suspend fun loadChoices(server: HttpUrl) {
        (repository.libraries(server) as? ApiResult.Ok)?.let { libs -> _state.update { it.copy(libraries = libs.value) } }
        (repository.session(server) as? ApiResult.Ok)?.value?.let { info ->
            val ceiling = info.account.maxRating
            val steps = info.ratingSystem?.steps?.map { it.age to it.label } ?: ProfileSettingsUiState.FALLBACK_AGES.map { it to null }
            val ages = (steps.map { it.first } + ceiling).distinct().sorted()
            // Steps above the account's ceiling are only shown as not available; the server refuses them
            // itself (`validation_failed`, rule 12, code review L-11).
            _state.update { s -> s.copy(ratings = ages.map { age -> RatingChoice(age, steps.firstOrNull { it.first == age }?.second, age <= ceiling) }) }
        }
    }

    // ---- the sheet ------------------------------------------------------------------------

    fun onEdit(profile: Profile, pinForm: PinKind? = null) {
        if (!_state.value.canManage) return
        _state.update {
            it.copy(
                editor = ProfileEditor(
                    id = profile.id,
                    isPrimary = profile.isPrimary,
                    name = profile.name,
                    accent = profile.accent ?: Accent.EMBER,
                    rating = profile.maxRating,
                    language = profile.language,
                    allLibraries = profile.allLibraries,
                    libraryIds = profile.libraryIds.toSet(),
                    watchLimit = profile.limits.watchLimitMinutes?.toString().orEmpty(),
                    limits = profile.limits,
                    hasPin = profile.hasPin,
                    pinForm = pinForm,
                ),
            )
        }
    }

    /** «PIN setzen» of the recommendation: the primary profile's sheet with the PIN form open (mockup `suPinRec`). */
    fun onAdvice() {
        val primary = (_state.value.profiles as? Loadable.Ready)?.value?.firstOrNull { it.isPrimary } ?: return
        onEdit(primary, PinKind.Pin)
    }

    fun onAdd() {
        if (!_state.value.canManage) return
        val list = (_state.value.profiles as? Loadable.Ready)?.value.orEmpty()
        // The first colour no other profile has (web `firstFreeAccent`; none is ember); the account's ceiling as rating.
        val accent = ACCENTS.firstOrNull { a -> list.none { (it.accent ?: Accent.EMBER) == a } } ?: ACCENTS.first()
        val ceiling = _state.value.ratings.filter { it.enabled }.maxOfOrNull { it.age } ?: ProfileSettingsUiState.NO_AGE_LIMIT
        val language = list.firstOrNull { it.isPrimary }?.language ?: Language.DE
        _state.update { it.copy(editor = ProfileEditor(accent = accent, rating = ceiling, language = language)) }
    }

    fun onDismiss() = _state.update { if (it.editor?.busy == true) it else it.copy(editor = null) }

    private fun edit(change: (ProfileEditor) -> ProfileEditor) = _state.update { s -> s.editor?.let { s.copy(editor = change(it)) } ?: s }

    fun onName(name: String) = edit { it.copy(name = name.take(MAX_NAME), nameMissing = false) }

    fun onAccent(accent: Accent) = edit { it.copy(accent = accent) }

    fun onRating(age: Int) = edit { it.copy(rating = age) }

    fun onPinForm(kind: PinKind?) = edit { it.copy(pinForm = kind, first = "", second = "", pinError = null) }

    fun onPinFirst(digits: String) = edit { it.copy(first = digits.filter(Char::isDigit).take(it.pinForm?.max ?: 0), pinError = null) }

    fun onPinSecond(digits: String) = edit { it.copy(second = digits.filter(Char::isDigit).take(it.pinForm?.max ?: 0), pinError = null) }

    /** «Übernehmen»: the new PIN counts for «Speichern»; `false` when the form refuses it. */
    fun onPinApply(): Boolean {
        val editor = _state.value.editor ?: return false
        val kind = editor.pinForm ?: return true
        val error = when {
            editor.first.length !in kind.min..kind.max -> PinError.Format
            editor.first != editor.second -> PinError.Mismatch
            else -> null
        }
        if (error != null) {
            edit { it.copy(pinError = error) }
            return false
        }
        val change = PinChange.Set(editor.first)
        edit {
            it.copy(pin = change,pinForm = null, first = "", second = "", pinError = null)
        }
        return true
    }

    /** «Entfernen»: the PIN goes on «Speichern» (a PIN only typed here is simply dropped). */
    fun onPinRemove() = edit { it.copy(pin = if (it.hasPin) PinChange.Remove else PinChange.Keep) }

    fun onDeleteAsk(ask: Boolean) = edit { it.copy(confirmDelete = ask && !it.isPrimary && !it.isNew) }

    fun onDelete() {
        val editor = _state.value.editor ?: return
        val id = editor.id ?: return
        val server = session.server ?: return
        if (editor.isPrimary || editor.busy) return
        edit { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            when (val result = repository.delete(server, id)) {
                is ApiResult.Ok -> {
                    _state.update { it.copy(editor = null) }
                    _messages.send(ProfileMessage.Deleted(editor.name))
                    load(quiet = true)
                }
                else -> edit { it.copy(busy = false, confirmDelete = false, error = code(result)) }
            }
        }
    }

    fun onSave() {
        val editor = _state.value.editor ?: return
        if (editor.busy) return
        val server = session.server ?: return
        // An open PIN form counts as «Übernehmen» first (mockup `suSave`); an empty one as «Abbrechen» (code review L-10).
        if (editor.pinForm != null && editor.first.isEmpty() && editor.second.isEmpty()) onPinForm(null)
        if (_state.value.editor?.pinForm != null && !onPinApply()) return
        val now = _state.value.editor ?: return
        val name = now.name.trim()
        val minutes = now.watchLimit.takeIf { it.isNotEmpty() }?.toIntOrNull()
        val limitInvalid = now.watchLimit.isNotEmpty() && (minutes == null || minutes !in 1..MAX_WATCH_MINUTES)
        if (name.isEmpty() || limitInvalid) {
            edit { it.copy(nameMissing = name.isEmpty(), watchLimitInvalid = limitInvalid) }
            return
        }
        edit { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            val limits = now.limits.copy(watchLimitMinutes = minutes)
            val libraries = if (now.allLibraries) emptySet() else now.libraryIds
            val saved = if (now.id == null) {
                repository.create(
                    server,
                    ProfileRequest(
                        name = name,
                        maxRating = now.rating,
                        language = now.language,
                        accent = now.accent,
                        allLibraries = now.allLibraries,
                        libraryIds = libraries,
                        pin = (now.pin as? PinChange.Set)?.digits,
                        limits = limits,
                    ),
                )
            } else {
                repository.update(
                    server,
                    now.id,
                    ProfileUpdate(
                        name = name,
                        maxRating = now.rating,
                        language = now.language,
                        accent = now.accent,
                        allLibraries = now.allLibraries,
                        libraryIds = libraries,
                        limits = limits,
                    ),
                )
            }
            if (saved !is ApiResult.Ok) {
                edit { it.copy(busy = false, error = code(saved)) }
                return@launch
            }
            val profile = saved.value
            val pins = buildList {
                // A new profile got its PIN with createProfile.
                if (now.id != null) {
                    when (val change = now.pin) {
                        is PinChange.Set -> add(repository.setPin(server, profile.id, change.digits))
                        PinChange.Remove -> add(repository.removePin(server, profile.id))
                        PinChange.Keep -> Unit
                    }
                }
            }
            _state.update { it.copy(editor = null) }
            val before = (_state.value.profiles as? Loadable.Ready)?.value?.firstOrNull { it.id == profile.id }
            val partly = pins.any { it !is ApiResult.Ok }
            // With a new epoch the page goes: the result travels with the change (code review m-2 of part a).
            if (profile.id == active()?.id) onActiveChanged(before, profile, partly)
            _messages.send(
                when {
                    partly -> ProfileMessage.Partly
                    now.id == null -> ProfileMessage.Added(profile.name)
                    else -> ProfileMessage.Saved
                },
            )
            load(quiet = true)
        }
    }

    private fun code(result: ApiResult<*>): String = when (result) {
        is ApiResult.Failed -> result.code ?: ProfileEditor.FAILED
        is ApiResult.NotAllowed -> result.code ?: ProfileEditor.FAILED
        ApiResult.NotFound -> NOT_FOUND
        ApiResult.Offline -> OFFLINE
        else -> ProfileEditor.FAILED
    }

    companion object {
        /** The longest name the server takes. */
        const val MAX_NAME = 60

        /** The longest watch time per day (`WatchMinutes`). */
        const val MAX_WATCH_MINUTES = 1440

        const val NOT_FOUND = "not_found"
        const val OFFLINE = "network"

        /** The accents of the contract, in the order of the web app's picker. */
        val ACCENTS = Accent.entries.filter { it != Accent.UNKNOWN_DEFAULT_OPEN_API }
    }
}
