package com.slouchcat.app.mobile.signin

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.common.KeyConfirmation
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.DeviceDescription
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerProbe
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.io.IOException
import java.security.GeneralSecurityException

/** A field the user has to fix before the app sends anything. */
enum class FieldError { EmailEmpty, EmailInvalid, PasswordEmpty }

/** Why the sign-in did not work. */
sealed interface SignInError {
    data object InvalidCredentials : SignInError

    data class TooManyAttempts(val seconds: Long) : SignInError

    /** No answer, or the server did not prove itself in this network (section 6). */
    data object Unreachable : SignInError

    data object AppTooOld : SignInError

    data class ServerTooOld(val serverVersion: String, val required: String) : SignInError

    /** The Keystore refused to store the token. */
    data object Storage : SignInError

    data object Failed : SignInError
}

data class SignInUiState(
    val server: SavedServer? = null,
    val email: String = "",
    /** Only in memory: never in the saved state, cleared after every attempt (rule 10). */
    val password: String = "",
    val showPassword: Boolean = false,
    val busy: Boolean = false,
    val emailError: FieldError? = null,
    val passwordError: FieldError? = null,
    val error: SignInError? = null,
    /** The user confirms a new key of the server by signing in (section 6). */
    val confirmKey: Boolean = false,
    /** How the device appears under «Meine Geräte». */
    val deviceName: String = "",
)

/**
 * «Anmelden» (mockup `/setup/login`, concept 27 4.2): `appLogin` with e-mail, password, the
 * device's name and kind and the device id it already holds; the token goes into the Keystore.
 * In [confirmKey] mode the new key of the server is checked for this sign-in only and stored once
 * the server accepted the password (section 6).
 */
class SignInViewModel(
    private val saved: SavedStateHandle,
    confirmKey: Boolean,
    private val session: SessionManager,
    private val auth: AuthRepository,
    private val probe: ServerProbe,
    private val servers: ServerStore,
    private val device: DeviceDescription,
    private val keys: KeyConfirmation,
) : ViewModel() {
    private val _state = MutableStateFlow(
        SignInUiState(email = saved[EMAIL] ?: "", confirmKey = confirmKey && keys.presentedKey != null, deviceName = device.name),
    )
    val state: StateFlow<SignInUiState> = _state.asStateFlow()

    private val _done = Channel<Unit>(Channel.BUFFERED)

    /** One event after a successful sign-in: the profile choice follows. */
    val done: Flow<Unit> = _done.receiveAsFlow()

    init {
        viewModelScope.launch {
            val server = servers.server.first() ?: return@launch
            _state.update { it.copy(server = server) }
            // While a new key waits for confirmation, nothing but the sign-in goes out.
            if (_state.value.confirmKey) return@launch
            // The version shown is the one the server reports now; a server that went back to an
            // older release says so before the user types the password.
            when (val result = probe.probe(server.address)) {
                is ProbeResult.Found -> {
                    if (result.info.version != server.version) servers.updateVersion(result.info.version)
                    _state.update { it.copy(server = server.copy(version = result.info.version)) }
                }
                is ProbeResult.TooOld -> _state.update { it.copy(error = SignInError.ServerTooOld(result.serverVersion, result.required)) }
                else -> Unit
            }
        }
    }

    fun onEmailChange(text: String) {
        if (_state.value.busy) return
        saved[EMAIL] = text
        _state.update { it.copy(email = text, emailError = null, error = it.error.takeIf { e -> e is SignInError.ServerTooOld }) }
    }

    fun onPasswordChange(text: String) {
        if (_state.value.busy) return
        _state.update { it.copy(password = text, passwordError = null, error = it.error.takeIf { e -> e is SignInError.ServerTooOld }) }
    }

    fun onTogglePassword() = _state.update { it.copy(showPassword = !it.showPassword) }

    fun onSubmit() {
        val now = _state.value
        if (now.busy || now.error is SignInError.ServerTooOld) return
        val email = now.email.trim()
        val emailError = when {
            email.isEmpty() -> FieldError.EmailEmpty
            !EMAIL_SHAPE.matches(email) -> FieldError.EmailInvalid
            else -> null
        }
        val passwordError = if (now.password.isEmpty()) FieldError.PasswordEmpty else null
        if (emailError != null || passwordError != null) {
            _state.update { it.copy(emailError = emailError, passwordError = passwordError, error = null) }
            return
        }
        val server = now.server ?: return
        val password = now.password
        _state.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            val key = keys.presentedKey.takeIf { now.confirmKey }
            if (key != null) keys.begin(key)
            val result = auth.appLogin(server.address, email, password, device, session.knownDeviceId)
            val error = when (result) {
                is ApiResult.Ok -> store(result.value, key != null)
                is ApiResult.Failed -> when {
                    result.code == ApiCaller.INVALID_CREDENTIALS -> SignInError.InvalidCredentials
                    result.status == 429 -> SignInError.TooManyAttempts(result.retryAfterSeconds ?: DEFAULT_PAUSE)
                    else -> SignInError.Failed
                }
                ApiResult.Offline -> SignInError.Unreachable
                ApiResult.AppTooOld -> SignInError.AppTooOld
                ApiResult.NotFound, is ApiResult.NotAllowed, ApiResult.SignedOut -> SignInError.Failed
            }
            if (error != null && key != null) keys.finish(confirmed = false)
            // The password leaves the state after every attempt, whatever the outcome.
            _state.update { it.copy(busy = false, password = "", showPassword = false, error = error) }
            if (error == null) _done.send(Unit)
        }
    }

    /** Stores the session; `null` when it worked. */
    private suspend fun store(login: com.slouchcat.core.api.generated.models.AppLogin, confirming: Boolean): SignInError? = try {
        if (confirming) keys.finish(confirmed = true)
        val profile = login.session.profile?.let { ActiveProfile(it.id, it.name, it.isPrimary, it.accent?.value, it.image) }
        session.signedIn(login.token, login.deviceId, login.session.account.email, profile)
        Log.i(TAG, "signed in", "kind" to device.kind.value)
        null
    } catch (e: GeneralSecurityException) {
        Log.e(TAG, "token not stored", error = e)
        SignInError.Storage
    } catch (e: IOException) {
        Log.e(TAG, "token not stored", error = e)
        SignInError.Storage
    }

    private companion object {
        const val TAG = "signin"
        const val EMAIL = "email"
        const val DEFAULT_PAUSE = 60L
        val EMAIL_SHAPE = Regex("""^[^\s@]+@[^\s@]+\.[^\s@]+$""")
    }
}
