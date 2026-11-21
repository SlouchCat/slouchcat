package com.slouchcat.app.mobile.connected

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.toLoadable
import com.slouchcat.core.api.map
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerRepository
import com.slouchcat.core.session.ServerStore
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import okhttp3.HttpUrl

/** What the page shows of the stored server: its address and the version it reports now. */
data class ConnectedServer(val address: HttpUrl, val version: String, val userCertificate: Boolean = false) {
    val encrypted: Boolean get() = address.isHttps
}

/** The pattern of a page's state: one [Loadable] per thing the page loads. */
data class ConnectedUiState(val server: Loadable<ConnectedServer> = Loadable.Loading)

/**
 * «Verbunden mit …»: reads the stored server, then asks it for its version (the pattern every
 * page follows: load in [load], show [Loadable], [retry] after a failure).
 */
class ConnectedViewModel(
    private val store: ServerStore,
    private val repository: ServerRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(ConnectedUiState())
    val state: StateFlow<ConnectedUiState> = _state.asStateFlow()
    private var loading: Job? = null

    init {
        load()
    }

    fun retry() = load()

    private fun load() {
        loading?.cancel()
        _state.update { it.copy(server = Loadable.Loading) }
        loading = viewModelScope.launch {
            val saved: SavedServer = store.server.first() ?: run {
                _state.update { it.copy(server = Loadable.Empty) }
                return@launch
            }
            val result = repository.health(saved.address).map { ConnectedServer(saved.address, it.version, saved.userCertificate) }
            _state.update { it.copy(server = result.toLoadable()) }
        }
    }
}
