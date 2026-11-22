package com.slouchcat.app.tv.common

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.SessionManager
import java.util.concurrent.CopyOnWriteArrayList

/**
 * Hooks of a profile change and of the end of a session (concept 26, 5.7; like the phone app):
 * pages that keep something beyond their ViewModel register a cleaner; stage 3 stops the player.
 * Without UI and without Android: it equals the phone app's `SessionHooks` and can move to
 * `:core:session` with it (review N-15).
 */
class TvSessionHooks {
    private val cleaners = CopyOnWriteArrayList<suspend () -> Unit>()

    fun onProfileOrSessionEnd(cleaner: suspend () -> Unit) {
        cleaners += cleaner
    }

    suspend fun clean() {
        for (cleaner in cleaners) runCatching { cleaner() }.onFailure { Log.w(TAG, "cleaner failed", error = it) }
    }

    private companion object {
        const val TAG = "hooks"
    }
}

/**
 * The session changes of the TV: choose a profile, sign this TV out (it forgets server and account
 * and shows the setup again, mockup `ACT.mdLogoutOk`), and the cleanup after a lost session.
 * Each change is a new session epoch: the navigation starts afresh by itself.
 */
class TvSessionActions(
    private val session: SessionManager,
    private val servers: ServerStore,
    private val auth: AuthRepository,
    private val hooks: TvSessionHooks,
) {
    suspend fun profileChosen(profile: ActiveProfile) {
        hooks.clean()
        session.profileChosen(profile)
    }

    /**
     * «Diesen Fernseher abmelden»: `logout` (ends the device session in the server), then the TV
     * forgets token, account and server. `true` when the server confirmed; without an answer the
     * TV still forgets everything (the session then stays under «Meine Geräte» until signed out there).
     */
    suspend fun signOut(): Boolean {
        // First the cleaners: a playback still running ends at the server with the token it began with.
        hooks.clean()
        val server = session.server
        val confirmed = if (server == null) {
            true
        } else {
            val result = auth.logout(server)
            (result is ApiResult.Ok || result is ApiResult.SignedOut).also { if (!it) Log.w(TAG, "logout not confirmed", "result" to result::class.simpleName) }
        }
        // The server first: the epoch of signedOut() then opens the setup, with nothing left.
        servers.clear()
        session.signedOut()
        return confirmed
    }

    /** The server ended the session (401 anywhere): the TV forgets it as well and is set up again. */
    suspend fun sessionLost() {
        hooks.clean()
        servers.clear()
        session.signedOut()
    }

    private companion object {
        const val TAG = "session"
    }
}
