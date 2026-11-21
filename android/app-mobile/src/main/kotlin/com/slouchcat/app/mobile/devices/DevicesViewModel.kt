package com.slouchcat.app.mobile.devices

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.toLoadable
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.session.DevicesRepository
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

/** Why signing a device out did not work. */
enum class DeviceSignOutError { PrimaryOnly, Unreachable, Failed }

data class DevicesUiState(
    val devices: Loadable<List<MyDevice>> = Loadable.Loading,
    /** Signing out needs the primary profile (or none); the others read only (concept 27, 4.5). */
    val canSignOut: Boolean = false,
    /** The device the sheet «… abmelden?» asks about. */
    val asking: MyDevice? = null,
    val signingOut: Boolean = false,
    val error: DeviceSignOutError? = null,
)

/**
 * «Meine Geräte» (mockup `/devices`, concept 27 4.5): `listMyDevices`, and `signOutMyDevice`
 * for another device after a question. The own device is signed out with «Abmelden» on «Mehr».
 */
class DevicesViewModel(
    private val session: SessionManager,
    private val repository: DevicesRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(DevicesUiState(canSignOut = (session.state.value as? SessionState.Ready)?.profile?.isPrimary == true))
    val state: StateFlow<DevicesUiState> = _state.asStateFlow()

    private val _signedOut = Channel<String>(Channel.BUFFERED)

    /** The name of a device that was signed out, once, for the message. */
    val signedOut: Flow<String> = _signedOut.receiveAsFlow()

    private var loading: Job? = null

    init {
        load()
    }

    fun retry() = load()

    private fun load(quiet: Boolean = false) {
        val server = session.server ?: return
        loading?.cancel()
        if (!quiet) _state.update { it.copy(devices = Loadable.Loading) }
        loading = viewModelScope.launch {
            val result = repository.devices(server)
            _state.update { it.copy(devices = result.toLoadable { list -> list.isEmpty() }) }
        }
    }

    fun onAsk(device: MyDevice) {
        if (!_state.value.canSignOut || device.current) return
        _state.update { it.copy(asking = device, error = null) }
    }

    fun onDismiss() {
        if (_state.value.signingOut) return
        _state.update { it.copy(asking = null, error = null) }
    }

    fun onConfirm(unnamed: String) {
        val now = _state.value
        val device = now.asking ?: return
        if (now.signingOut) return
        val server = session.server ?: return
        _state.update { it.copy(signingOut = true, error = null) }
        viewModelScope.launch {
            val error = when (val result = repository.signOut(server, device.id)) {
                is ApiResult.Ok, ApiResult.NotFound -> null
                is ApiResult.NotAllowed -> if (result.code == PRIMARY_REQUIRED) DeviceSignOutError.PrimaryOnly else DeviceSignOutError.Failed
                ApiResult.Offline -> DeviceSignOutError.Unreachable
                else -> DeviceSignOutError.Failed
            }
            _state.update { it.copy(signingOut = false, asking = if (error == null) null else it.asking, error = error) }
            if (error == null) {
                _signedOut.send(device.name?.takeIf { it.isNotBlank() } ?: unnamed)
                load(quiet = true)
            }
        }
    }

    private companion object {
        const val PRIMARY_REQUIRED = "primary_profile_required"
    }
}
