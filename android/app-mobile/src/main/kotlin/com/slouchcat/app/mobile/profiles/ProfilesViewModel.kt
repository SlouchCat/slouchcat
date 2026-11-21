package com.slouchcat.app.mobile.profiles

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.SessionActions
import com.slouchcat.app.mobile.common.toLoadable
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.api.generated.models.SessionInfo
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
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
) : ViewModel() {
    private val _state = MutableStateFlow(ProfilesUiState())
    val state: StateFlow<ProfilesUiState> = _state.asStateFlow()

    private val _done = Channel<Unit>(Channel.BUFFERED)

    /** One event when a profile is active: the app opens its first page. */
    val done: Flow<Unit> = _done.receiveAsFlow()

    private val _signedOut = Channel<Unit>(Channel.BUFFERED)

    /** One event after «Abmelden» on this page. */
    val signedOut: Flow<Unit> = _signedOut.receiveAsFlow()

    private var loading: Job? = null
    private var countdown: Job? = null

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

    fun onPick(profile: ProfileSummary) {
        val now = _state.value
        if (now.selecting != null || now.pin != null || now.signingOut) return
        if (profile.hasPin) {
            _state.update { it.copy(pin = PinState(profile), error = null) }
            return
        }
        _state.update { it.copy(selecting = profile.id, error = null) }
        viewModelScope.launch {
            val server = session.server ?: return@launch
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
        countdown?.cancel()
        _state.update { it.copy(pin = null) }
    }

    fun onSubmitPin() {
        val pin = _state.value.pin ?: return
        if (!pin.canSubmit) return
        _state.update { it.copy(pin = pin.copy(checking = true)) }
        viewModelScope.launch {
            val server = session.server ?: return@launch
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

    /** The server locks the profile for [seconds]; the pad counts down and then invites again. */
    private fun lock(seconds: Long) {
        updatePin { it.copy(checking = false, digits = "", message = PinMessage.Locked, lockedSeconds = seconds.coerceAtLeast(1)) }
        countdown?.cancel()
        countdown = viewModelScope.launch {
            while ((_state.value.pin?.lockedSeconds ?: 0) > 0) {
                delay(SECOND)
                updatePin { pin ->
                    val left = (pin.lockedSeconds - 1).coerceAtLeast(0)
                    pin.copy(lockedSeconds = left, message = if (left == 0L) PinMessage.Again else PinMessage.Locked)
                }
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
        actions.profileChosen(profile)
        countdown?.cancel()
        _state.update { it.copy(selecting = null, pin = null, current = profile.id) }
        _done.send(Unit)
    }

    /** «Abmelden» on the picker (a way out when no profile can be opened). */
    fun onSignOut() {
        if (_state.value.signingOut) return
        _state.update { it.copy(signingOut = true) }
        viewModelScope.launch {
            actions.signOut()
            _signedOut.send(Unit)
        }
    }

    private companion object {
        const val PIN_REQUIRED = "pin_required"
        const val PIN_INVALID = "pin_invalid"
        const val DEFAULT_LOCK = 30L
        const val SECOND = 1_000L
    }
}
