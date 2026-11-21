package com.slouchcat.app.mobile.more

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
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
enum class MoreSheet { Server, SignOut }

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
)

/**
 * «Mehr» (mockup `/more`): the active profile, the warning while the connection is unencrypted
 * (concept 27 section 6), the rows of the features, the server, and the sign-out (9.7) with the
 * questions of the [SessionHooks] steps.
 */
class MoreViewModel(
    private val session: SessionManager,
    private val hooks: SessionHooks,
    private val actions: SessionActions,
) : ViewModel() {
    private val _state = MutableStateFlow(MoreUiState())
    val state: StateFlow<MoreUiState> = _state.asStateFlow()

    private val _signedOut = Channel<Unit>(Channel.BUFFERED)

    /** One event after the sign-out: the app shows the sign-in. */
    val signedOut: Flow<Unit> = _signedOut.receiveAsFlow()

    init {
        viewModelScope.launch {
            session.state.collect { now ->
                if (now is SessionState.Ready) _state.update { it.copy(server = now.server, email = now.email, profile = now.profile) }
            }
        }
    }

    fun onServer() = _state.update { it.copy(sheet = MoreSheet.Server) }

    fun onSignOutAsk() {
        if (_state.value.signingOut) return
        viewModelScope.launch {
            val choices = hooks.signOutSteps.mapNotNull { step -> step.question()?.let { SignOutChoice(step, it, it.default) } }
            _state.update { it.copy(sheet = MoreSheet.SignOut, choices = choices) }
        }
    }

    fun onChoice(index: Int, answer: Boolean) = _state.update { s ->
        s.copy(choices = s.choices.mapIndexed { i, c -> if (i == index) c.copy(answer = answer) else c })
    }

    fun onDismissSheet() {
        if (_state.value.signingOut) return
        _state.update { it.copy(sheet = null) }
    }

    fun onSignOut() {
        val now = _state.value
        if (now.signingOut) return
        _state.update { it.copy(signingOut = true) }
        viewModelScope.launch {
            actions.signOut(now.choices.associate { it.step to it.answer })
            _state.update { it.copy(signingOut = false, sheet = null) }
            _signedOut.send(Unit)
        }
    }
}
