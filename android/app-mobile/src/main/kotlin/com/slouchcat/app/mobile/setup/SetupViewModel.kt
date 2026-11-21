package com.slouchcat.app.mobile.setup

import androidx.lifecycle.SavedStateHandle
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
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.receiveAsFlow
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

    /** The http address leads to https; the field now holds [address]. */
    data class HttpsRequired(val address: String) : SetupError
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
    /** «Weiter» is storing the address; a second tap does nothing. */
    val saving: Boolean = false,
)

/**
 * The page «Serveradresse» (concept 27, 6; mockup /setup): read the address, warn about plain HTTP
 * outside the home network (decided on the resolved address), ask `getHealth`, store the address.
 */
class SetupViewModel(
    private val saved: SavedStateHandle,
    private val probe: ServerProbe,
    private val resolver: HostResolver,
    private val store: ServerStore,
) : ViewModel() {
    // What the user typed and an open warning survive the end of the process (SavedStateHandle);
    // a running check or a found server are asked again.
    private val _state = MutableStateFlow(
        SetupUiState(address = saved[ADDRESS] ?: "", insecureAddress = saved[INSECURE]),
    )
    val state: StateFlow<SetupUiState> = _state.asStateFlow()

    private val _done = Channel<Unit>(Channel.BUFFERED)

    /** One event when the address is stored: the page moves on. An event, not a state. */
    val done: Flow<Unit> = _done.receiveAsFlow()

    init {
        // Changing the address starts from the stored one (mockup: `U.addr = S.server`).
        if (saved.get<String>(ADDRESS) == null) {
            viewModelScope.launch {
                val stored = store.server.first() ?: return@launch
                update { if (it.address.isEmpty()) it.copy(address = ServerAddress.display(stored.address)) else it }
            }
        }
    }

    private fun update(change: (SetupUiState) -> SetupUiState) {
        _state.update(change)
        saved[ADDRESS] = _state.value.address
        saved[INSECURE] = _state.value.insecureAddress
    }

    fun onAddressChange(text: String) {
        if (_state.value.found != null || _state.value.busy) return
        update { it.copy(address = text, error = null) }
    }

    fun onConnect() {
        val now = _state.value
        if (now.busy || now.found != null) return
        when (val parsed = ServerAddress.parse(now.address)) {
            ParsedAddress.Empty -> update { it.copy(error = SetupError.Empty) }
            ParsedAddress.Invalid -> update { it.copy(error = SetupError.Invalid) }
            is ParsedAddress.Valid -> {
                update { it.copy(address = ServerAddress.display(parsed.url), error = null) }
                if (parsed.encrypted) check(parsed.url) else checkPlain(parsed.url)
            }
        }
    }

    /** Plain HTTP: no warning in the home network, a warning with confirmation outside it. */
    private fun checkPlain(url: HttpUrl) {
        update { it.copy(busy = true) }
        viewModelScope.launch {
            val home = try {
                resolver.resolve(url.host).let { all -> all.isNotEmpty() && all.all(HomeNetwork::isHome) }
            } catch (e: UnknownHostException) {
                Log.i(TAG, "host not resolved", error = e)
                update { it.copy(busy = false, error = SetupError.Unreachable) }
                return@launch
            }
            if (home) {
                runCheck(url)
            } else {
                update { it.copy(busy = false, insecureAddress = ServerAddress.display(url)) }
            }
        }
    }

    fun onConfirmInsecure() {
        val url = (ServerAddress.parse(_state.value.insecureAddress ?: return) as? ParsedAddress.Valid)?.url ?: return
        update { it.copy(insecureAddress = null) }
        check(url)
    }

    fun onDismissInsecure() {
        update { it.copy(insecureAddress = null) }
    }

    private fun check(url: HttpUrl) {
        update { it.copy(busy = true) }
        viewModelScope.launch { runCheck(url) }
    }

    private suspend fun runCheck(url: HttpUrl) {
        val error = when (val result = probe.probe(url)) {
            is ProbeResult.Found -> {
                update { it.copy(busy = false, found = result.info) }
                return
            }
            ProbeResult.Unreachable -> SetupError.Unreachable
            ProbeResult.NotSlouchcat -> SetupError.NotSlouchcat
            is ProbeResult.TooOld -> SetupError.TooOld(result.serverVersion, result.required)
            is ProbeResult.RequiresHttps -> {
                val https = ServerAddress.display(result.address)
                update { it.copy(busy = false, address = https, error = SetupError.HttpsRequired(https)) }
                return
            }
        }
        update { it.copy(busy = false, error = error) }
    }

    /** «Andere Adresse»: back to the field, the address stays to be edited. */
    fun onChangeAddress() {
        update { it.copy(found = null, error = null) }
    }

    /** «Weiter»: store the server and move on. */
    fun onNext() {
        val now = _state.value
        val info = now.found ?: return
        if (now.saving) return
        update { it.copy(saving = true) }
        viewModelScope.launch {
            store.save(SavedServer(info.address, info.version, info.userCertificate))
            Log.i(TAG, "server stored", "version" to info.version, "encrypted" to info.encrypted)
            _done.send(Unit)
        }
    }

    private companion object {
        const val TAG = "setup"
        const val ADDRESS = "address"
        const val INSECURE = "insecureAddress"
    }
}
