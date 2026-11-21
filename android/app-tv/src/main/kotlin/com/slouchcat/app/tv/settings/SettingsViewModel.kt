package com.slouchcat.app.tv.settings

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.tv.common.RelaySettings
import com.slouchcat.app.tv.common.TvSessionActions
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import okhttp3.HttpUrl

data class SettingsUiState(
    val server: SavedServer? = null,
    val email: String = "",
    val profileName: String = "",
    val profileAccent: String? = null,
    /** How this TV appears under «Meine Geräte». */
    val tvName: String = "",
    val appVersion: String = "",
    val relay: HttpUrl = RelaySettings.DEFAULT,
    /** The server's address answers with another key (ADR 0040): «Server nicht bestätigt». */
    val unconfirmed: Boolean = false,
    val signingOut: Boolean = false,
)

/**
 * «Einstellungen» of the TV (mockup `/settings`, photos 36, 62 to 64): server and account, signing
 * this TV out, the version, the address of the relay. A sign-out starts a new session epoch: the
 * navigation opens the setup by itself.
 */
class SettingsViewModel(
    private val session: SessionManager,
    private val actions: TvSessionActions,
    private val relaySettings: RelaySettings,
    newKey: Flow<String?>,
    tvName: String,
    appVersion: String,
    /** Says the outcome of the sign-out (a message across the new epoch). */
    private val onSignedOut: (confirmed: Boolean) -> Unit,
    /** The sign-out ends this page with the epoch: it runs in the app's scope to the end. */
    private val appScope: CoroutineScope,
) : ViewModel() {
    private val _state = MutableStateFlow(SettingsUiState(tvName = tvName, appVersion = appVersion))
    val state: StateFlow<SettingsUiState> = _state.asStateFlow()

    init {
        viewModelScope.launch {
            session.state.collect { s ->
                when (s) {
                    is SessionState.Ready -> _state.update { it.copy(server = s.server, email = s.email, profileName = s.profile.name, profileAccent = s.profile.accent) }
                    is SessionState.ChooseProfile -> _state.update { it.copy(server = s.server, email = s.email) }
                    else -> Unit
                }
            }
        }
        viewModelScope.launch { relaySettings.relay.collect { relay -> _state.update { it.copy(relay = relay) } } }
        viewModelScope.launch { newKey.collect { key -> _state.update { it.copy(unconfirmed = key != null) } } }
    }

    /** «Diesen Fernseher abmelden» after the question. */
    fun onSignOut() {
        if (_state.value.signingOut) return
        _state.update { it.copy(signingOut = true) }
        appScope.launch {
            val confirmed = actions.signOut()
            onSignedOut(confirmed)
        }
    }

    /** A new address of the relay; `null` returns to the built-in one. */
    fun onRelay(address: HttpUrl?) {
        viewModelScope.launch { relaySettings.set(address) }
    }
}
