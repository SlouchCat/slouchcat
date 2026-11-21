package com.slouchcat.app.tv.common

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.pairing.PairingProtocol
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.map
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import java.util.concurrent.CopyOnWriteArrayList

/**
 * Hooks of a profile change and of the end of a session (concept 26, 5.7; like the phone app):
 * pages that keep something beyond their ViewModel register a cleaner; stage 3 stops the player.
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
        val server = session.server
        val confirmed = if (server == null) {
            true
        } else {
            val result = auth.logout(server)
            (result is ApiResult.Ok || result is ApiResult.SignedOut).also { if (!it) Log.w(TAG, "logout not confirmed", "result" to result::class.simpleName) }
        }
        hooks.clean()
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

/** The address of the relay (F-21): built in, changeable in the advanced settings; a seam for tests. */
interface RelaySettings {
    /** The relay in use; the built-in one until another is set. */
    val relay: Flow<HttpUrl>

    /** Sets another relay; `null` returns to the built-in one. */
    suspend fun set(address: HttpUrl?)

    companion object {
        val DEFAULT: HttpUrl = requireNotNull(PairingProtocol.DEFAULT_RELAY.toHttpUrlOrNull())
    }
}

private val Context.tvPreferences: DataStore<Preferences> by preferencesDataStore(name = "tv")

/** [RelaySettings] in a DataStore of the app. */
class DataStoreRelaySettings(context: Context) : RelaySettings {
    private val store = context.applicationContext.tvPreferences

    override val relay: Flow<HttpUrl> = store.data.map { prefs -> prefs[RELAY]?.toHttpUrlOrNull() ?: RelaySettings.DEFAULT }

    override suspend fun set(address: HttpUrl?) {
        store.edit { if (address == null) it.remove(RELAY) else it[RELAY] = address.toString() }
    }

    private companion object {
        val RELAY = stringPreferencesKey("relay")
    }
}

/** [RelaySettings] in memory (tests, previews). */
class MemoryRelaySettings(initial: HttpUrl = RelaySettings.DEFAULT) : RelaySettings {
    val flow = MutableStateFlow(initial)
    override val relay: Flow<HttpUrl> = flow

    override suspend fun set(address: HttpUrl?) {
        flow.value = address ?: RelaySettings.DEFAULT
    }
}
