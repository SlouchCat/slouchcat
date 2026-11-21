package com.slouchcat.app.mobile.account

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.common.SessionHooks
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.session.AuthRepository
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

enum class AccountSheet { Email, Password }

/** What is wrong with a field of the two sheets. */
enum class AccountFieldError { EmailEmpty, EmailInvalid, SameEmail, EmailTaken, CurrentEmpty, WrongPassword, TooShort, SamePassword, Mismatch }

/** What went wrong beyond a field. */
sealed interface AccountError {
    data class TooManyAttempts(val seconds: Long) : AccountError

    data object Unreachable : AccountError

    data object Failed : AccountError
}

/** The fields of the open sheet; the passwords live only here, never in a saved state. */
data class AccountForm(
    val newEmail: String = "",
    val current: String = "",
    val newPassword: String = "",
    val repeat: String = "",
    val errors: Map<String, AccountFieldError> = emptyMap(),
    val error: AccountError? = null,
    val busy: Boolean = false,
) {
    /** Never a password (security review L-2). */
    override fun toString(): String = "AccountForm(errors=$errors, error=$error, busy=$busy, passwords=[redacted])"
}

/** The message after a change. */
enum class AccountMessage { EmailChanged, PasswordChanged }

data class AccountUiState(
    val email: String = "",
    val server: SavedServer? = null,
    val sheet: AccountSheet? = null,
    val form: AccountForm = AccountForm(),
)

/**
 * «Konto» (mockup `/account`, concept 27 3.1): change the e-mail address or the password with
 * `updateAccount` and the current password. After a new password this session stays and every
 * other one ends (the sheet says so before).
 */
class AccountViewModel(
    private val session: SessionManager,
    private val auth: AuthRepository,
    private val hooks: SessionHooks = SessionHooks(),
) : ViewModel() {
    private val _state = MutableStateFlow(AccountUiState())
    val state: StateFlow<AccountUiState> = _state.asStateFlow()

    private val _messages = Channel<AccountMessage>(Channel.BUFFERED)
    val messages: Flow<AccountMessage> = _messages.receiveAsFlow()

    init {
        viewModelScope.launch {
            session.state.collect { now ->
                if (now is SessionState.Ready) _state.update { it.copy(email = now.email, server = now.server) }
            }
        }
    }

    fun onEmailAsk() = _state.update { it.copy(sheet = AccountSheet.Email, form = AccountForm()) }

    fun onPasswordAsk() = _state.update { it.copy(sheet = AccountSheet.Password, form = AccountForm()) }

    fun onDismiss() {
        if (_state.value.form.busy) return
        _state.update { it.copy(sheet = null, form = AccountForm()) }
    }

    fun onField(field: String, value: String) {
        if (_state.value.form.busy) return
        _state.update { s ->
            val form = when (field) {
                NEW_EMAIL -> s.form.copy(newEmail = value)
                CURRENT -> s.form.copy(current = value)
                NEW_PASSWORD -> s.form.copy(newPassword = value)
                REPEAT -> s.form.copy(repeat = value)
                else -> s.form
            }
            s.copy(form = form.copy(errors = form.errors - field, error = null))
        }
    }

    fun onSave() {
        val now = _state.value
        val form = now.form
        if (form.busy) return
        val sheet = now.sheet ?: return
        val errors = buildMap {
            if (form.current.isEmpty()) put(CURRENT, AccountFieldError.CurrentEmpty)
            if (sheet == AccountSheet.Email) {
                val email = form.newEmail.trim()
                when {
                    email.isEmpty() -> put(NEW_EMAIL, AccountFieldError.EmailEmpty)
                    !EMAIL_SHAPE.matches(email) -> put(NEW_EMAIL, AccountFieldError.EmailInvalid)
                    email.equals(now.email, ignoreCase = true) -> put(NEW_EMAIL, AccountFieldError.SameEmail)
                }
            } else {
                when {
                    form.newPassword.length < MIN_PASSWORD -> put(NEW_PASSWORD, AccountFieldError.TooShort)
                    form.newPassword == form.current -> put(NEW_PASSWORD, AccountFieldError.SamePassword)
                    form.repeat != form.newPassword -> put(REPEAT, AccountFieldError.Mismatch)
                }
            }
        }
        if (errors.isNotEmpty()) {
            _state.update { it.copy(form = form.copy(errors = errors)) }
            return
        }
        val server = session.server ?: return
        _state.update { it.copy(form = form.copy(busy = true, error = null)) }
        viewModelScope.launch {
            val email = form.newEmail.trim().takeIf { sheet == AccountSheet.Email }
            val password = form.newPassword.takeIf { sheet == AccountSheet.Password }
            when (val result = auth.updateAccount(server, form.current, email, password)) {
                is ApiResult.Ok -> {
                    session.accountChanged(result.value.account.email)
                    // Stage 8 renews its push subscription here (code review minor 20).
                    hooks.accountChanged()
                    _state.update { it.copy(sheet = null, form = AccountForm()) }
                    _messages.send(if (sheet == AccountSheet.Email) AccountMessage.EmailChanged else AccountMessage.PasswordChanged)
                }
                is ApiResult.Failed -> fail(
                    when {
                        result.code == ApiCaller.INVALID_CREDENTIALS -> mapOf(CURRENT to AccountFieldError.WrongPassword)
                        result.code == EMAIL_TAKEN -> mapOf(NEW_EMAIL to AccountFieldError.EmailTaken)
                        else -> emptyMap()
                    },
                    if (result.status == 429) AccountError.TooManyAttempts(result.retryAfterSeconds ?: DEFAULT_PAUSE) else null,
                    result.code,
                )
                ApiResult.Offline -> fail(emptyMap(), AccountError.Unreachable, null)
                else -> fail(emptyMap(), AccountError.Failed, null)
            }
        }
    }

    private fun fail(errors: Map<String, AccountFieldError>, error: AccountError?, code: String?) {
        val shown = error ?: if (errors.isEmpty()) AccountError.Failed else null
        // The current password is typed again after a wrong one; the new ones stay.
        _state.update { s ->
            s.copy(
                form = s.form.copy(
                    busy = false,
                    errors = errors,
                    error = shown,
                    current = if (code == ApiCaller.INVALID_CREDENTIALS) "" else s.form.current,
                ),
            )
        }
    }

    companion object {
        const val NEW_EMAIL = "newEmail"
        const val CURRENT = "current"
        const val NEW_PASSWORD = "newPassword"
        const val REPEAT = "repeat"

        /** `AccountUpdate.newPassword` minLength in the contract. */
        const val MIN_PASSWORD = 10
        private const val EMAIL_TAKEN = "email_taken"
        private const val DEFAULT_PAUSE = 60L
        private val EMAIL_SHAPE = Regex("""^[^\s@]+@[^\s@]+\.[^\s@]+$""")
    }
}
