package com.slouchcat.app.tv.setup

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.tv.common.takeServer
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.AppLogin
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.DeviceDescription
import com.slouchcat.core.session.HomeNetwork
import com.slouchcat.core.session.HostResolver
import com.slouchcat.core.session.ParsedAddress
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerAddress
import com.slouchcat.core.session.ServerInfo
import com.slouchcat.core.session.ServerProbe
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.io.IOException
import java.net.UnknownHostException
import java.security.GeneralSecurityException

/** The three steps of the mockup (`HD_STEPS`). */
enum class HandStep { Server, Email, Password }

/** What the check of the address found. */
sealed interface AddressCheck {
    data object Busy : AddressCheck

    /** A Slouchcat server; [needsConsent]: plain HTTP outside the home network (E-18 warning). */
    data class Found(val info: ServerInfo, val needsConsent: Boolean) : AddressCheck

    data object Empty : AddressCheck

    data object Invalid : AddressCheck

    data object Unreachable : AddressCheck

    data object NotSlouchcat : AddressCheck

    data class TooOld(val serverVersion: String, val required: String) : AddressCheck

    /** The http address leads to https; the field now holds [address]. */
    data class HttpsRequired(val address: String) : AddressCheck
}

/** Why the sign-in did not work. */
sealed interface HandError {
    data object EmailInvalid : HandError

    data object InvalidCredentials : HandError

    data class TooManyAttempts(val seconds: Long) : HandError

    data object Unreachable : HandError

    data object ServerUnconfirmed : HandError

    data object AppTooOld : HandError

    data object Storage : HandError

    data object Failed : HandError
}

data class HandUiState(
    val step: HandStep = HandStep.Server,
    val address: String = "",
    val check: AddressCheck? = null,
    /** «Ich verstehe das und will trotzdem verbinden». */
    val httpConsent: Boolean = false,
    val email: String = "",
    /** Only in memory: never in the saved state, cleared after every attempt (rule 10). */
    val password: String = "",
    val showPassword: Boolean = false,
    val busy: Boolean = false,
    val error: HandError? = null,
) {
    val found: ServerInfo? get() = (check as? AddressCheck.Found)?.info

    /** «Weiter» of the first step. */
    val serverReady: Boolean get() = (check as? AddressCheck.Found)?.let { !it.needsConsent || httpConsent } == true

    val emailReady: Boolean get() = EMAIL_SHAPE.matches(email.trim())

    /** Never the password (security review L-2 of the phone app). */
    override fun toString(): String = "HandUiState(step=$step, address=$address, check=$check, busy=$busy, error=$error, password=[redacted])"

    companion object {
        val EMAIL_SHAPE = Regex("""^[^\s@]+@[^\s@]+\.[^\s@]+$""")
    }
}

/**
 * «Von Hand einrichten» (concept 26 4.4, mockup `/setup/hand`, photo 04): the address, checked with
 * `getHealth` (warning for HTTP outside the home network, as on the phone), e-mail and password,
 * then `appLogin` with the kind `tv`. The server is stored right before the sign-in; the profile
 * choice follows from the new session epoch.
 */
class HandSetupViewModel(
    private val saved: SavedStateHandle,
    private val probe: ServerProbe,
    private val resolver: HostResolver,
    private val servers: ServerStore,
    private val session: SessionManager,
    private val auth: AuthRepository,
    private val device: DeviceDescription,
    /** After the sign-in, with the whole server address (the message «Verbunden mit …», K-9). */
    private val onSignedIn: (host: String) -> Unit = {},
) : ViewModel() {
    private val _state = MutableStateFlow(
        HandUiState(
            step = saved.get<String>(STEP)?.let { runCatching { HandStep.valueOf(it) }.getOrNull() } ?: HandStep.Server,
            address = saved[ADDRESS] ?: "",
            email = saved[EMAIL] ?: "",
        ),
    )
    val state: StateFlow<HandUiState> = _state.asStateFlow()

    private var checking: Job? = null

    init {
        // A found server is asked again after the process ended; the steps after it need it.
        val now = _state.value
        if (now.address.isNotEmpty()) checkAddress(now.address, keepStep = true)
    }

    private fun update(change: (HandUiState) -> HandUiState) {
        _state.update(change)
        val s = _state.value
        saved[STEP] = s.step.name
        saved[ADDRESS] = s.address
        saved[EMAIL] = s.email
    }

    /** The address from the input; checked at once. */
    fun onAddress(text: String) {
        if (_state.value.busy) return
        checkAddress(text, keepStep = false)
    }

    private fun checkAddress(text: String, keepStep: Boolean) {
        checking?.cancel()
        val parsed = ServerAddress.parse(text)
        when (parsed) {
            ParsedAddress.Empty -> return update { it.copy(address = "", check = AddressCheck.Empty, httpConsent = false, step = HandStep.Server) }
            ParsedAddress.Invalid -> return update { it.copy(address = text.trim(), check = AddressCheck.Invalid, httpConsent = false, step = HandStep.Server) }
            is ParsedAddress.Valid -> Unit
        }
        val url = parsed.url
        update { it.copy(address = ServerAddress.display(url), check = AddressCheck.Busy, httpConsent = false, step = if (keepStep) it.step else HandStep.Server) }
        checking = viewModelScope.launch {
            val home = if (url.isHttps) {
                true
            } else {
                try {
                    resolver.resolve(url.host).let { all -> all.isNotEmpty() && all.all(HomeNetwork::isHome) }
                } catch (e: UnknownHostException) {
                    Log.i(TAG, "host not resolved", error = e)
                    return@launch update { it.copy(check = AddressCheck.Unreachable, step = HandStep.Server) }
                }
            }
            val check = when (val result = probe.probe(url)) {
                is ProbeResult.Found -> AddressCheck.Found(result.info, needsConsent = !result.info.encrypted && !home)
                ProbeResult.Unreachable -> AddressCheck.Unreachable
                ProbeResult.NotSlouchcat -> AddressCheck.NotSlouchcat
                is ProbeResult.TooOld -> AddressCheck.TooOld(result.serverVersion, result.required)
                is ProbeResult.RequiresHttps -> {
                    val https = ServerAddress.display(result.address)
                    return@launch update { it.copy(address = https, check = AddressCheck.HttpsRequired(https), step = HandStep.Server) }
                }
            }
            update { if (check is AddressCheck.Found) it.copy(check = check) else it.copy(check = check, step = HandStep.Server) }
        }
    }

    fun onConsent(consent: Boolean) = update { it.copy(httpConsent = consent) }

    /** «Weiter»: the next step when this one is complete. */
    fun onNext() {
        val now = _state.value
        if (now.busy) return
        when (now.step) {
            HandStep.Server -> if (now.serverReady) update { it.copy(step = HandStep.Email, error = null) }
            HandStep.Email -> if (now.emailReady) update { it.copy(step = HandStep.Password, error = null) } else update { it.copy(error = HandError.EmailInvalid) }
            HandStep.Password -> onSignIn()
        }
    }

    /** Back on the page: one step back; `false` on the first step (the page closes). */
    fun onBack(): Boolean {
        val now = _state.value
        if (now.busy) return true
        val previous = when (now.step) {
            HandStep.Server -> return false
            HandStep.Email -> HandStep.Server
            HandStep.Password -> HandStep.Email
        }
        update { it.copy(step = previous, error = null, password = "", showPassword = false) }
        return true
    }

    fun onEmail(text: String) = update { it.copy(email = text.trim(), error = null) }

    fun onPassword(text: String) {
        if (_state.value.busy) return
        _state.update { it.copy(password = text, error = null) }
    }

    fun onTogglePassword() = _state.update { it.copy(showPassword = !it.showPassword) }

    fun onSignIn() {
        val now = _state.value
        val info = now.found ?: return
        if (now.busy || now.password.isEmpty() || !now.serverReady) return
        if (!now.emailReady) return update { it.copy(error = HandError.EmailInvalid) }
        val password = now.password
        val email = now.email.trim()
        update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            val error = try {
                signIn(info, email, password)
            } finally {
                // The password leaves the state after every attempt, whatever the outcome.
                _state.update { it.copy(password = "", showPassword = false) }
            }
            update { it.copy(busy = false, error = error) }
        }
    }

    private suspend fun signIn(info: ServerInfo, email: String, password: String): HandError? {
        val server = SavedServer(info.address, info.version, info.userCertificate, info.serverKey)
        if (server.needsSetup) return HandError.Failed
        // The session takes the server first (it forgets the session of any other one).
        if (!session.takeServer(servers, server)) return HandError.Failed
        return when (val result = auth.appLogin(server.address, email, password, device, session.knownDeviceId)) {
            is ApiResult.Ok -> store(result.value).also { if (it == null) onSignedIn(ServerAddress.display(server.address)) }
            is ApiResult.Failed -> when {
                result.code == ApiCaller.INVALID_CREDENTIALS -> HandError.InvalidCredentials
                result.code == ApiCaller.SERVER_UNCONFIRMED -> HandError.ServerUnconfirmed
                result.status == HTTP_TOO_MANY -> HandError.TooManyAttempts(result.retryAfterSeconds ?: DEFAULT_PAUSE)
                else -> HandError.Failed
            }
            ApiResult.Offline -> HandError.Unreachable
            ApiResult.AppTooOld -> HandError.AppTooOld
            ApiResult.NotFound, is ApiResult.NotAllowed, ApiResult.SignedOut -> HandError.Failed
        }
    }

    private suspend fun store(login: AppLogin): HandError? = try {
        val profile = login.session.profile?.let { ActiveProfile(it.id, it.name, it.isPrimary, it.accent?.value, it.image) }
        session.signedIn(login.token, login.deviceId, login.session.account.email, profile)
        Log.i(TAG, "signed in by hand")
        null
    } catch (e: GeneralSecurityException) {
        Log.e(TAG, "token not stored", error = e)
        HandError.Storage
    } catch (e: IOException) {
        Log.e(TAG, "token not stored", error = e)
        HandError.Storage
    }

    private companion object {
        const val TAG = "hand"
        const val STEP = "step"
        const val ADDRESS = "address"
        const val EMAIL = "email"
        const val HTTP_TOO_MANY = 429
        const val DEFAULT_PAUSE = 60L
    }
}
