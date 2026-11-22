package com.slouchcat.app.mobile.more

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.common.KeyConfirmation
import com.slouchcat.app.mobile.common.SessionActions
import com.slouchcat.app.mobile.common.SessionHooks
import com.slouchcat.app.mobile.common.SignOutQuestion
import com.slouchcat.app.mobile.common.SignOutStep
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** The sheets of «Mehr». */
enum class MoreSheet { Server, SignOut, KeyCheck }

/** A question of a [SignOutStep] with the user's current answer. */
data class SignOutChoice(val step: SignOutStep, val question: SignOutQuestion, val answer: Boolean)

data class MoreUiState(
    val server: SavedServer? = null,
    val email: String = "",
    val profile: ActiveProfile? = null,
    val sheet: MoreSheet? = null,
    /** What the steps of later stages ask before signing out (stage 6: the downloads). */
    val choices: List<SignOutChoice> = emptyList(),
    val signingOut: Boolean = false,
    /** «Server wechseln»: after the sign-out the address comes next. */
    val changeServer: Boolean = false,
    /** The key the server's address presents instead of the remembered one (ADR 0040). */
    val newKey: String? = null,
    val rememberedKey: String? = null,
    val ownNetwork: Boolean = false,
)

/**
 * «Mehr» (mockup `/more`): the active profile, the warning while the connection is unencrypted
 * (concept 27 section 6), the rows of the features, the server, the sign-out (9.7) with the
 * questions of the [SessionHooks] steps, and the one place to confirm a new key of the server.
 */
class MoreViewModel(
    private val session: SessionManager,
    private val hooks: SessionHooks,
    private val actions: SessionActions,
    private val keys: KeyConfirmation,
) : ViewModel() {
    private val _state = MutableStateFlow(MoreUiState())
    val state: StateFlow<MoreUiState> = _state.asStateFlow()

    private val _confirmKey = Channel<Unit>(Channel.BUFFERED)

    /** One event after the user stated the own network: the sign-in with the new key follows. */
    val confirmKey: Flow<Unit> = _confirmKey.receiveAsFlow()

    init {
        viewModelScope.launch {
            session.state.collect { now ->
                if (now is SessionState.Ready) _state.update { it.copy(server = now.server, email = now.email, profile = now.profile) }
            }
        }
        viewModelScope.launch {
            keys.newKey.collect { key -> _state.update { it.copy(newKey = key, rememberedKey = keys.rememberedKey) } }
        }
    }

    fun onServer() = _state.update { it.copy(sheet = MoreSheet.Server) }

    fun onSignOutAsk(changeServer: Boolean = false) {
        if (_state.value.signingOut) return
        viewModelScope.launch {
            val choices = hooks.signOutSteps.mapNotNull { step -> step.question()?.let { SignOutChoice(step, it, it.default) } }
            _state.update { it.copy(sheet = MoreSheet.SignOut, choices = choices, changeServer = changeServer) }
        }
    }

    fun onChoice(index: Int, answer: Boolean) = _state.update { s ->
        s.copy(choices = s.choices.mapIndexed { i, c -> if (i == index) c.copy(answer = answer) else c })
    }

    fun onKeyCheck() = _state.update { if (it.newKey != null) it.copy(sheet = MoreSheet.KeyCheck, ownNetwork = false) else it }

    fun onOwnNetwork(own: Boolean) = _state.update { it.copy(ownNetwork = own) }

    fun onConfirmKey() {
        val now = _state.value
        if (!now.ownNetwork || now.newKey == null) return
        _state.update { it.copy(sheet = null, ownNetwork = false) }
        viewModelScope.launch { _confirmKey.send(Unit) }
    }

    fun onDismissSheet() {
        if (_state.value.signingOut) return
        _state.update { it.copy(sheet = null, ownNetwork = false) }
    }

    /** Signs out; the next page follows from the new session epoch («Abgemeldet» is shown there). */
    fun onSignOut() {
        val now = _state.value
        if (now.signingOut) return
        _state.update { it.copy(signingOut = true) }
        viewModelScope.launch {
            actions.signOut(now.choices.associate { it.step to it.answer }, toSetup = now.changeServer)
            _state.update { it.copy(signingOut = false, sheet = null) }
        }
    }
}
