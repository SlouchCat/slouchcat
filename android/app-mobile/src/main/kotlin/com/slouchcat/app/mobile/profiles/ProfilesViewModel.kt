package com.slouchcat.app.mobile.profiles

import android.os.SystemClock
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.common.SessionActions
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.api.generated.models.SessionInfo
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.toLoadable
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** What the PIN pad says under the dots. */
enum class PinMessage { Prompt, Invalid, Locked, Again, Unreachable, Failed }

data class PinState(
    val profile: ProfileSummary,
    /** The digits typed so far; 4 to 8 (contract `SelectProfileRequest.pin`). */
    val digits: String = "",
    val checking: Boolean = false,
    val message: PinMessage = PinMessage.Prompt,
    /** Seconds the server still locks the profile (`pin_locked`), counting down. */
    val lockedSeconds: Long = 0,
) {
    val locked: Boolean get() = lockedSeconds > 0
    val canSubmit: Boolean get() = digits.length >= MIN_DIGITS && !checking && !locked

    /** Never the digits (security review L-2). */
    override fun toString(): String = "PinState(profile=${profile.id}, digits=${digits.length}, message=$message, locked=$lockedSeconds)"

    companion object {
        const val MIN_DIGITS = 4
        const val MAX_DIGITS = 8
    }
}

/** Choosing a profile without PIN failed. */
enum class ProfilesError { Unreachable, Failed }

data class ProfilesUiState(
    val profiles: Loadable<List<ProfileSummary>> = Loadable.Loading,
    /** The profile the session ran as last (the ring in the mockup). */
    val current: Long? = null,
    val email: String = "",
    val host: String = "",
    val pin: PinState? = null,
    /** A profile without PIN is being activated. */
    val selecting: Long? = null,
    val error: ProfilesError? = null,
    val signingOut: Boolean = false,
)

/**
 * «Wer schaut?» (mockup `/profiles`): `listMyProfiles`, `selectProfile` with the PIN where the
 * profile has one. Wrong PINs and the lock come from the server (`pin_invalid`, `pin_locked` with
 * its pause); the page counts the pause down. A change of profile clears what the previous one
 * left (concept 27, 8.4) through [SessionActions].
 */
class ProfilesViewModel(
    private val session: SessionManager,
    private val auth: AuthRepository,
    private val actions: SessionActions,
    /** Time since boot in ms; the lock runs on it, also while the device sleeps (code review minor 15). */
    private val clock: () -> Long = SystemClock::elapsedRealtime,
    /** Opens «Profile verwalten» once the primary profile is active (an in-app jump); `null` without the page. */
    private val manage: (() -> Unit)? = null,
) : ViewModel() {
    private val _state = MutableStateFlow(ProfilesUiState())
    val state: StateFlow<ProfilesUiState> = _state.asStateFlow()

    private var loading: Job? = null
    private var countdown: Job? = null

    /** «Profile verwalten» was tapped: after the choice of the primary profile the page opens. */
    private var thenManage = false

    /** Until when the server locks a profile (per profile; closing the pad does not forget it). */
    private val lockedUntil = mutableMapOf<Long, Long>()

    init {
        when (val now = session.state.value) {
            is SessionState.ChooseProfile -> _state.update { it.copy(email = now.email, host = now.server.address.host) }
            is SessionState.Ready -> _state.update { it.copy(email = now.email, host = now.server.address.host, current = now.profile.id) }
            else -> Unit
        }
        load()
    }

    fun retry() = load()

    private fun load() {
        val server = session.server ?: return
        loading?.cancel()
        _state.update { it.copy(profiles = Loadable.Loading) }
        loading = viewModelScope.launch {
            val result = auth.profiles(server)
            _state.update { it.copy(profiles = result.toLoadable { list -> list.isEmpty() }) }
        }
    }

    /**
     * «Profile verwalten»: only the primary profile may (`primary_profile_required`). Active already:
     * the page opens. Otherwise the picker chooses the primary profile first, with its PIN, and the
     * page opens in the new session (rule 12: the server checks the PIN and the right).
     */
    fun onManage() {
        val open = manage ?: return
        val now = session.state.value
        if (now is SessionState.Ready && now.profile.isPrimary) return open()
        val primary = (_state.value.profiles as? Loadable.Ready)?.value?.firstOrNull { it.isPrimary } ?: return
        thenManage = true
        onPick(primary, manage = true)
    }

    fun onPick(profile: ProfileSummary) = onPick(profile, manage = false)

    private fun onPick(profile: ProfileSummary, manage: Boolean) {
        val now = _state.value
        if (now.selecting != null || now.pin != null || now.signingOut) return
        thenManage = manage
        if (profile.hasPin) {
            _state.update { it.copy(pin = PinState(profile), error = null) }
            resumeLock(profile.id)
            return
        }
        _state.update { it.copy(selecting = profile.id, error = null) }
        viewModelScope.launch {
            val server = session.server ?: return@launch _state.update { it.copy(selecting = null, error = ProfilesError.Failed) }
            when (val result = auth.selectProfile(server, profile.id, null)) {
                is ApiResult.Ok -> chosen(profile, result.value)
                is ApiResult.NotAllowed -> _state.update {
                    // The profile got a PIN meanwhile.
                    if (result.code == PIN_REQUIRED) it.copy(selecting = null, pin = PinState(profile)) else it.copy(selecting = null, error = ProfilesError.Failed)
                }
                ApiResult.Offline -> _state.update { it.copy(selecting = null, error = ProfilesError.Unreachable) }
                else -> _state.update { it.copy(selecting = null, error = ProfilesError.Failed) }
            }
        }
    }

    fun onDigit(digit: Char) {
        val pin = _state.value.pin ?: return
        if (!digit.isDigit() || pin.checking || pin.locked || pin.digits.length >= PinState.MAX_DIGITS) return
        _state.update { it.copy(pin = pin.copy(digits = pin.digits + digit, message = if (pin.message == PinMessage.Again) PinMessage.Prompt else pin.message)) }
    }

    fun onDelete() {
        val pin = _state.value.pin ?: return
        if (pin.checking || pin.locked || pin.digits.isEmpty()) return
        _state.update { it.copy(pin = pin.copy(digits = pin.digits.dropLast(1))) }
    }

    fun onCancelPin() {
        if (_state.value.pin?.checking == true) return
        thenManage = false
        countdown?.cancel()
        _state.update { it.copy(pin = null) }
    }

    fun onSubmitPin() {
        val pin = _state.value.pin ?: return
        if (!pin.canSubmit) return
        _state.update { it.copy(pin = pin.copy(checking = true)) }
        viewModelScope.launch {
            val server = session.server ?: return@launch updatePin { it.copy(checking = false, message = PinMessage.Failed) }
            val result = auth.selectProfile(server, pin.profile.id, pin.digits)
            when {
                result is ApiResult.Ok -> chosen(pin.profile, result.value)
                result is ApiResult.NotAllowed && result.code == PIN_INVALID -> updatePin { it.copy(checking = false, digits = "", message = PinMessage.Invalid) }
                result is ApiResult.Failed && result.status == 429 -> lock(result.retryAfterSeconds ?: DEFAULT_LOCK)
                result is ApiResult.Offline -> updatePin { it.copy(checking = false, message = PinMessage.Unreachable) }
                else -> updatePin { it.copy(checking = false, digits = "", message = PinMessage.Failed) }
            }
        }
    }

    private fun updatePin(change: (PinState) -> PinState) = _state.update { s -> s.copy(pin = s.pin?.let(change)) }

    /** The server locks the profile for [seconds]; the pad counts down from the clock and then invites again. */
    private fun lock(seconds: Long) {
        val pin = _state.value.pin ?: return
        lockedUntil[pin.profile.id] = clock() + seconds.coerceAtLeast(1) * SECOND
        updatePin { it.copy(checking = false, digits = "") }
        resumeLock(pin.profile.id)
    }

    /** Shows the rest of a lock of [profile] that is still running, counting down to the clock. */
    private fun resumeLock(profile: Long) {
        countdown?.cancel()
        fun left(): Long = ((lockedUntil[profile] ?: 0) - clock() + SECOND - 1).coerceAtLeast(0) / SECOND
        if (left() == 0L) return
        countdown = viewModelScope.launch {
            while (true) {
                val seconds = left()
                updatePin { pin ->
                    if (pin.profile.id != profile) pin else pin.copy(lockedSeconds = seconds, message = if (seconds == 0L) PinMessage.Again else PinMessage.Locked)
                }
                if (seconds == 0L) {
                    lockedUntil.remove(profile)
                    break
                }
                // The rest comes from the clock at each step, so a late tick does not add up.
                delay(SECOND)
            }
        }
    }

    private suspend fun chosen(summary: ProfileSummary, info: SessionInfo) {
        val active = info.profile
        val profile = if (active != null) {
            ActiveProfile(active.id, active.name, active.isPrimary, active.accent?.value, active.image)
        } else {
            ActiveProfile(summary.id, summary.name, summary.isPrimary, summary.accent?.value, summary.image)
        }
        countdown?.cancel()
        _state.update { it.copy(selecting = null, pin = null, current = profile.id) }
        // «Profile verwalten» waits as a jump for the new epoch.
        if (thenManage && profile.isPrimary) manage?.invoke()
        thenManage = false
        // A new session epoch: SlouchcatNavHost opens the first page.
        actions.profileChosen(profile)
    }

    /** «Abmelden» on the picker (a way out when no profile can be opened). */
    fun onSignOut() {
        if (_state.value.signingOut) return
        _state.update { it.copy(signingOut = true) }
        viewModelScope.launch { actions.signOut() }
    }

    private companion object {
        const val PIN_REQUIRED = "pin_required"
        const val PIN_INVALID = "pin_invalid"
        const val DEFAULT_LOCK = 30L
        const val SECOND = 1_000L
    }
}
