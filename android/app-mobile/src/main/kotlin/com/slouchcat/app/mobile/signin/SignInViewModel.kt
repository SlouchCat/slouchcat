package com.slouchcat.app.mobile.signin

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.common.KeyConfirmation
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.AppLogin
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.DeviceDescription
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerProbe
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
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

    /** Another key under the server's address: nothing was sent (ADR 0040). */
    data object ServerUnconfirmed : SignInError

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
    /** The user chose to confirm the new key [newKey] by signing in (section 6). */
    val confirmKey: Boolean = false,
    /** The key the server's address presented instead of the remembered one. */
    val newKey: String? = null,
    val rememberedKey: String? = null,
    /** The sheet with both fingerprints is open; [ownNetwork] is its switch. */
    val keyCheck: Boolean = false,
    val ownNetwork: Boolean = false,
    /** How the device appears under «Meine Geräte». */
    val deviceName: String = "",
) {
    /** Never the password (security review L-2). */
    override fun toString(): String =
        "SignInUiState(server=${server?.address}, busy=$busy, error=$error, confirmKey=$confirmKey, password=[redacted])"
}

/**
 * «Anmelden» (mockup `/setup/login`, concept 27 4.2): `appLogin` with e-mail, password, the
 * device's name and kind and the device id it already holds; the token goes into the Keystore. The
 * next page follows from the new session epoch.
 *
 * A new key of the server (ADR 0040, decision of the build): nothing is sent to it until the user
 * opened the check, saw both fingerprints and stated «Ich bin in meinem eigenen Netz» (here or under
 * «Mehr › Server», then [confirmKey]). Only then the sign-in goes to the new key, without token and
 * device id; the key counts once the server accepted the password.
 */
class SignInViewModel(
    private val saved: SavedStateHandle,
    confirmKey: Boolean,
    private val session: SessionManager,
    private val auth: AuthRepository,
    private val probe: ServerProbe,
    private val device: DeviceDescription,
    private val keys: KeyConfirmation,
) : ViewModel() {
    private val _state = MutableStateFlow(
        SignInUiState(
            email = saved[EMAIL] ?: "",
            confirmKey = confirmKey && keys.newKey.value != null,
            rememberedKey = keys.rememberedKey,
            deviceName = device.name,
        ),
    )
    val state: StateFlow<SignInUiState> = _state.asStateFlow()

    init {
        viewModelScope.launch {
            keys.newKey.collect { key ->
                _state.update { it.copy(newKey = key, rememberedKey = keys.rememberedKey, confirmKey = it.confirmKey && key != null) }
            }
        }
        viewModelScope.launch {
            val server = session.savedServer ?: return@launch
            _state.update { it.copy(server = server) }
            if (_state.value.confirmKey) return@launch
            // The version shown is the one the server reports now. Only shown, not stored: the
            // answer comes before any proof and may come from another device (security review L-4).
            when (val result = probe.probe(server.address)) {
                is ProbeResult.Found -> _state.update { it.copy(server = server.copy(version = result.info.version)) }
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

    /** Opens the check of the new key; nothing happens without one. */
    fun onCheckKey() = _state.update { if (it.newKey != null) it.copy(keyCheck = true, ownNetwork = false) else it }

    fun onOwnNetwork(own: Boolean) = _state.update { it.copy(ownNetwork = own) }

    /** The user stated the own network: the next sign-in goes to the new key. */
    fun onConfirmKey() = _state.update {
        if (it.ownNetwork && it.newKey != null) it.copy(confirmKey = true, keyCheck = false, error = null) else it
    }

    fun onDismissKey() = _state.update { it.copy(keyCheck = false, ownNetwork = false) }

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
        val key = keys.newKey.value.takeIf { now.confirmKey }
        if (now.confirmKey && key == null) return
        _state.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            var confirmed = false
            if (key != null) keys.begin(key)
            try {
                // While confirming, the device id stays here (security review L-1).
                val result = auth.appLogin(server.address, email, password, device, session.knownDeviceId)
                val error = when (result) {
                    is ApiResult.Ok -> store(result.value, key).also { confirmed = key != null && it == null }
                    is ApiResult.Failed -> when {
                        result.code == ApiCaller.INVALID_CREDENTIALS -> SignInError.InvalidCredentials
                        result.code == ApiCaller.SERVER_UNCONFIRMED -> SignInError.ServerUnconfirmed
                        result.status == 429 -> SignInError.TooManyAttempts(result.retryAfterSeconds ?: DEFAULT_PAUSE)
                        else -> SignInError.Failed
                    }
                    ApiResult.Offline -> SignInError.Unreachable
                    ApiResult.AppTooOld -> SignInError.AppTooOld
                    ApiResult.NotFound, is ApiResult.NotAllowed, ApiResult.SignedOut -> SignInError.Failed
                }
                // The password leaves the state after every attempt, whatever the outcome.
                _state.update { it.copy(busy = false, password = "", showPassword = false, error = error) }
            } finally {
                // Also when the page goes away meanwhile (code review M-1): the credentials come back.
                if (key != null && !confirmed) withContext(NonCancellable) { keys.finish(confirmed = false) }
            }
        }
    }

    /** Stores the session; `null` when it worked. */
    private suspend fun store(login: AppLogin, key: String?): SignInError? = try {
        if (key != null) keys.finish(confirmed = true)
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
