package com.slouchcat.app.mobile.common

import androidx.annotation.StringRes
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.ServerIdentityGate
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.SessionManager
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
 * The hooks of a session's end and of a profile change (concept 27, 8.4: stop the playback,
 * clear the image cache and every remembered list). Pages that keep lists beyond their
 * ViewModel register a cleaner; stage 6 registers its sign-out step.
 */
class SessionHooks {
    private val steps = CopyOnWriteArrayList<SignOutStep>()
    private val cleaners = CopyOnWriteArrayList<suspend () -> Unit>()

    fun addSignOutStep(step: SignOutStep) {
        steps += step
    }

    /** Runs at every profile change and every sign-out (also a lost session). */
    fun onProfileOrSessionEnd(cleaner: suspend () -> Unit) {
        cleaners += cleaner
    }

    val signOutSteps: List<SignOutStep> get() = steps.toList()

    suspend fun clean() {
        for (cleaner in cleaners) {
            runCatching { cleaner() }.onFailure { Log.w(TAG, "cleaner failed", error = it) }
        }
    }

    private companion object {
        const val TAG = "hooks"
    }
}

/** What the gate of «Server weist sich aus» offers the sign-in for a new key (concept 27 section 6). */
interface KeyConfirmation {
    /** The new key the server presented, `null` when there is none to confirm. */
    val presentedKey: String?

    /** From now on until [finish]: prove against the new key and send no token. */
    fun begin(key: String)

    /** After the sign-in: [confirmed] stores the key for good; otherwise the remembered key stays. */
    suspend fun finish(confirmed: Boolean)
}

class GateKeyConfirmation(
    private val gate: ServerIdentityGate,
    private val session: SessionManager,
    private val servers: ServerStore,
) : KeyConfirmation {
    override val presentedKey: String? get() = gate.presentedKey

    override fun begin(key: String) {
        session.credentialsSuppressed = true
        gate.candidateKey = key
    }

    override suspend fun finish(confirmed: Boolean) {
        val key = gate.candidateKey
        if (confirmed && key != null) servers.confirmKey(key)
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
) {
    /** The profile is active in the server: remember it and clear what the previous one left. */
    suspend fun profileChosen(profile: ActiveProfile) {
        hooks.clean()
        session.profileChosen(profile)
    }

    /**
     * Signs out on this device: the steps first (they still have the token), then `logout`, then
     * the local session. Without an answer from the server the app still signs out here; the
     * session then stays in «Meine Geräte» until it is signed out there.
     */
    suspend fun signOut(answers: Map<SignOutStep, Boolean> = emptyMap()) {
        for (step in hooks.signOutSteps) {
            val answer = answers[step] ?: step.question()?.default ?: true
            runCatching { step.beforeSignOut(answer) }.onFailure { Log.w(TAG, "sign-out step failed", error = it) }
        }
        val server = session.server
        if (server != null) {
            val result = auth.logout(server)
            if (result !is ApiResult.Ok && result !is ApiResult.SignedOut) Log.w(TAG, "logout not confirmed", "result" to result::class.simpleName)
        }
        hooks.clean()
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
