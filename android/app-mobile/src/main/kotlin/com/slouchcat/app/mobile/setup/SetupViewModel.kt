package com.slouchcat.app.mobile.setup

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.session.HomeNetwork
import com.slouchcat.core.session.HostResolver
import com.slouchcat.core.session.ParsedAddress
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerAddress
import com.slouchcat.core.session.ServerInfo
import com.slouchcat.core.session.ServerProbe
import com.slouchcat.core.session.ServerStore
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import okhttp3.HttpUrl
import java.net.UnknownHostException

/** Why the address does not lead to a usable server. */
sealed interface SetupError {
    data object Empty : SetupError

    data object Invalid : SetupError

    data object Unreachable : SetupError

    data object NotSlouchcat : SetupError

    data class TooOld(val serverVersion: String, val required: String) : SetupError
}

data class SetupUiState(
    /** What the field shows; after «Verbinden» the address as the app read it. */
    val address: String = "",
    /** The check is running. */
    val busy: Boolean = false,
    val error: SetupError? = null,
    /** The warning about plain HTTP outside the home network waits for an answer. */
    val insecureAddress: String? = null,
    /** The server answered; the page shows it and «Weiter». */
    val found: ServerInfo? = null,
    /** The address is stored; the app moves on. */
    val saved: Boolean = false,
)

/**
 * The page «Serveradresse» (concept 27, 6; mockup /setup): read the address, warn about plain HTTP
 * outside the home network (decided on the resolved address), ask `getHealth`, store the address.
 */
class SetupViewModel(
    private val probe: ServerProbe,
    private val resolver: HostResolver,
    private val store: ServerStore,
) : ViewModel() {
    private val _state = MutableStateFlow(SetupUiState())
    val state: StateFlow<SetupUiState> = _state.asStateFlow()

    private var pending: HttpUrl? = null

    init {
        // Changing the address starts from the stored one (mockup: `U.addr = S.server`).
        viewModelScope.launch {
            val saved = store.server.first() ?: return@launch
            _state.update { if (it.address.isEmpty()) it.copy(address = ServerAddress.display(saved.address)) else it }
        }
    }

    fun onAddressChange(text: String) {
        if (_state.value.found != null || _state.value.busy) return
        _state.update { it.copy(address = text, error = null) }
    }

    fun onConnect() {
        val now = _state.value
        if (now.busy || now.found != null) return
        when (val parsed = ServerAddress.parse(now.address)) {
            ParsedAddress.Empty -> _state.update { it.copy(error = SetupError.Empty) }
            ParsedAddress.Invalid -> _state.update { it.copy(error = SetupError.Invalid) }
            is ParsedAddress.Valid -> {
                _state.update { it.copy(address = ServerAddress.display(parsed.url), error = null) }
                if (parsed.encrypted) check(parsed.url) else checkPlain(parsed.url)
            }
        }
    }

    /** Plain HTTP: no warning in the home network, a warning with confirmation outside it. */
    private fun checkPlain(url: HttpUrl) {
        _state.update { it.copy(busy = true) }
        viewModelScope.launch {
            val home = try {
                resolver.resolve(url.host).let { all -> all.isNotEmpty() && all.all(HomeNetwork::isHome) }
            } catch (e: UnknownHostException) {
                Log.i(TAG, "host not resolved", error = e)
                _state.update { it.copy(busy = false, error = SetupError.Unreachable) }
                return@launch
            }
            if (home) {
                runCheck(url)
            } else {
                pending = url
                _state.update { it.copy(busy = false, insecureAddress = ServerAddress.display(url)) }
            }
        }
    }

    fun onConfirmInsecure() {
        val url = pending ?: return
        pending = null
        _state.update { it.copy(insecureAddress = null) }
        check(url)
    }

    fun onDismissInsecure() {
        pending = null
        _state.update { it.copy(insecureAddress = null) }
    }

    private fun check(url: HttpUrl) {
        _state.update { it.copy(busy = true) }
        viewModelScope.launch { runCheck(url) }
    }

    private suspend fun runCheck(url: HttpUrl) {
        val error = when (val result = probe.probe(url)) {
            is ProbeResult.Found -> {
                _state.update { it.copy(busy = false, found = result.info) }
                return
            }
            ProbeResult.Unreachable -> SetupError.Unreachable
            ProbeResult.NotSlouchcat -> SetupError.NotSlouchcat
            is ProbeResult.TooOld -> SetupError.TooOld(result.serverVersion, result.required)
        }
        _state.update { it.copy(busy = false, error = error) }
    }

    /** «Andere Adresse»: back to the field, the address stays to be edited. */
    fun onChangeAddress() {
        _state.update { it.copy(found = null, error = null) }
    }

    /** «Weiter»: store the server and move on. */
    fun onNext() {
        val info = _state.value.found ?: return
        viewModelScope.launch {
            store.save(SavedServer(info.address, info.version))
            Log.i(TAG, "server stored", "version" to info.version, "encrypted" to info.encrypted)
            _state.update { it.copy(saved = true) }
        }
    }

    private companion object {
        const val TAG = "setup"
    }
}
