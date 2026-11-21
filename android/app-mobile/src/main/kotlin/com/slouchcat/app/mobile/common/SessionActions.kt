package com.slouchcat.app.mobile.common

import androidx.annotation.StringRes
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.ServerIdentityGate
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.util.concurrent.CopyOnWriteArrayList

/** A switch in the sheet «Abmelden?» that a [SignOutStep] asks for (stage 6: delete the downloads). */
data class SignOutQuestion(@StringRes val title: Int, val subtitle: String? = null, val default: Boolean = true)

/**
 * Something that has to happen before the app signs out, while the token still works (concept
 * 27, 9.7: stage 6 deletes the downloads on the device and in the server). Register it in the
 * AppGraph with `graph.hooks.addSignOutStep(…)`; the sheet «Abmelden?» shows its question.
 */
interface SignOutStep {
    /** The switch to show, or `null` to ask nothing. */
    suspend fun question(): SignOutQuestion? = null

    /** [answer]: the state of the switch (`default` when nothing was asked). */
    suspend fun beforeSignOut(answer: Boolean)
}

/**
 * The hooks of a session's end, a profile change and a change of the account (concept 27, 8.4:
 * stop the playback, clear the image cache and every remembered list). Pages that keep lists
 * beyond their ViewModel register a cleaner; stage 6 registers its sign-out step; stage 8 renews
 * its push subscription after a new password.
 */
class SessionHooks {
    private val steps = CopyOnWriteArrayList<SignOutStep>()
    private val cleaners = CopyOnWriteArrayList<suspend () -> Unit>()
    private val accountChanged = CopyOnWriteArrayList<suspend () -> Unit>()

    fun addSignOutStep(step: SignOutStep) {
        steps += step
    }

    /** Runs at every profile change and every sign-out (also a lost session). */
    fun onProfileOrSessionEnd(cleaner: suspend () -> Unit) {
        cleaners += cleaner
    }

    /** Runs after the e-mail address or the password changed (the session stays). */
    fun onAccountChanged(hook: suspend () -> Unit) {
        accountChanged += hook
    }

    val signOutSteps: List<SignOutStep> get() = steps.toList()

    suspend fun clean() = runAll(cleaners, "cleaner")

    suspend fun accountChanged() = runAll(accountChanged, "account hook")

    private suspend fun runAll(hooks: List<suspend () -> Unit>, what: String) {
        for (hook in hooks) {
            runCatching { hook() }.onFailure { Log.w(TAG, "$what failed", error = it) }
        }
    }

    private companion object {
        const val TAG = "hooks"
    }
}

/** A short message the shell shows once, also across a new session epoch («Abgemeldet»). */
class ShellNotices {
    private val _pending = MutableStateFlow<Int?>(null)
    val pending: StateFlow<Int?> = _pending.asStateFlow()

    fun show(@StringRes text: Int) {
        _pending.value = text
    }

    fun consume() {
        _pending.value = null
    }
}

/** Where the next epoch without session starts: the sign-in, or the address after «Server wechseln». */
class EntryPlan {
    @Volatile private var setupNext = false

    fun requestSetupNext() {
        setupNext = true
    }

    fun takeSetupNext(): Boolean = setupNext.also { setupNext = false }
}

/**
 * The confirmation of a new key of the server (concept 27 section 6, ADR 0040, decision of the
 * build): only on the user's own way, never offered by itself.
 */
interface KeyConfirmation {
    /** The key the server presented while the remembered one did not prove itself; `null`: none. */
    val newKey: StateFlow<String?>

    /** The key remembered at the setup. */
    val rememberedKey: String?

    /** From now on until [finish]: prove against [key] and send neither token nor device id. */
    fun begin(key: String)

    /** After the sign-in: [confirmed] stores the key for good; otherwise the remembered key stays. */
    suspend fun finish(confirmed: Boolean)
}

class GateKeyConfirmation(
    private val gate: ServerIdentityGate,
    private val session: SessionManager,
) : KeyConfirmation {
    override val newKey: StateFlow<String?> get() = gate.newKey
    override val rememberedKey: String? get() = session.serverKey

    override fun begin(key: String) {
        session.credentialsSuppressed = true
        gate.candidateKey = key
    }

    override suspend fun finish(confirmed: Boolean) {
        val key = gate.candidateKey
        // The key counts at once in the session, before the store's flow (code review minor 11).
        if (confirmed && key != null) session.keyConfirmed(key)
        gate.candidateKey = null
        session.credentialsSuppressed = false
        gate.reset()
    }
}

/**
 * The session changes every page may cause: choose a profile, sign out, and the cleanup after a
 * lost session (401). One place, so the hooks always run.
 */
class SessionActions(
    private val session: SessionManager,
    private val auth: AuthRepository,
    private val hooks: SessionHooks,
    private val notices: ShellNotices = ShellNotices(),
    private val entry: EntryPlan = EntryPlan(),
) {
    /** The profile is active in the server: clear what the previous one left, then remember it. */
    suspend fun profileChosen(profile: ActiveProfile) {
        hooks.clean()
        session.profileChosen(profile)
    }

    /**
     * Signs out on this device: the steps first (they still have the token), then `logout`, then
     * the local session. Without an answer from the server the app still signs out here and says
     * that the server does not know (code review minor 14). [toSetup]: «Server wechseln» asks for
     * the address next.
     */
    suspend fun signOut(answers: Map<SignOutStep, Boolean> = emptyMap(), toSetup: Boolean = false) {
        for (step in hooks.signOutSteps) {
            val answer = answers[step] ?: step.question()?.default ?: true
            runCatching { step.beforeSignOut(answer) }.onFailure { Log.w(TAG, "sign-out step failed", error = it) }
        }
        val server = session.server
        val confirmed = if (server != null) {
            val result = auth.logout(server)
            if (result !is ApiResult.Ok && result !is ApiResult.SignedOut) Log.w(TAG, "logout not confirmed", "result" to result::class.simpleName)
            result is ApiResult.Ok || result is ApiResult.SignedOut
        } else {
            true
        }
        hooks.clean()
        if (toSetup) entry.requestSetupNext()
        notices.show(if (confirmed) R.string.app_signOut_done else R.string.app_signOut_doneOffline)
        session.signedOut()
    }

    /** The server ended the session (401 anywhere): forget it here as well. */
    suspend fun sessionLost() {
        hooks.clean()
        session.signedOut()
    }

    private companion object {
        const val TAG = "session"
    }
}
