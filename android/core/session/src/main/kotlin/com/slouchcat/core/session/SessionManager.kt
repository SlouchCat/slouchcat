package com.slouchcat.core.session

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.slouchcat.core.api.SessionCredentials
import com.slouchcat.core.api.TrustedServer
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import okhttp3.HttpUrl

/** The profile the session runs as, as the app remembers it between starts. */
data class ActiveProfile(
    val id: Long,
    val name: String,
    val isPrimary: Boolean,
    /** `Accent` of the contract (`ember` …); `null` for the default. */
    val accent: String? = null,
    /** `Profile.image`, relative to the server. */
    val image: String? = null,
)

/** Where the app is between the start and a chosen profile (concept 27, 3.1). */
sealed interface SessionState {
    data object Loading : SessionState

    /** No server yet, or one over HTTP without a remembered key: the setup. */
    data object NoServer : SessionState

    data class SignedOut(val server: SavedServer) : SessionState

    data class ChooseProfile(val server: SavedServer, val email: String) : SessionState

    data class Ready(val server: SavedServer, val email: String, val profile: ActiveProfile) : SessionState
}

/** Account and profile of the session, not secret; a seam for tests. */
interface AccountStore {
    suspend fun read(): Pair<String?, ActiveProfile?>

    suspend fun write(email: String?, profile: ActiveProfile?)
}

private val Context.sessionPreferences: DataStore<Preferences> by preferencesDataStore(name = "session")

/** [AccountStore] in a DataStore that the backup leaves out (data extraction rules name only «server»). */
class DataStoreAccountStore(context: Context) : AccountStore {
    private val store = context.applicationContext.sessionPreferences

    override suspend fun read(): Pair<String?, ActiveProfile?> {
        val prefs = store.data.first()
        val id = prefs[PROFILE_ID]
        val name = prefs[PROFILE_NAME]
        val profile = if (id != null && name != null) {
            ActiveProfile(id, name, prefs[PROFILE_PRIMARY] ?: false, prefs[PROFILE_ACCENT], prefs[PROFILE_IMAGE])
        } else {
            null
        }
        return prefs[EMAIL] to profile
    }

    override suspend fun write(email: String?, profile: ActiveProfile?) {
        store.edit { prefs ->
            prefs.clear()
            if (email != null) prefs[EMAIL] = email
            if (profile != null) {
                prefs[PROFILE_ID] = profile.id
                prefs[PROFILE_NAME] = profile.name
                prefs[PROFILE_PRIMARY] = profile.isPrimary
                profile.accent?.let { prefs[PROFILE_ACCENT] = it }
                profile.image?.let { prefs[PROFILE_IMAGE] = it }
            }
        }
    }

    private companion object {
        val EMAIL = stringPreferencesKey("email")
        val PROFILE_ID = longPreferencesKey("profileId")
        val PROFILE_NAME = stringPreferencesKey("profileName")
        val PROFILE_PRIMARY = booleanPreferencesKey("profilePrimary")
        val PROFILE_ACCENT = stringPreferencesKey("profileAccent")
        val PROFILE_IMAGE = stringPreferencesKey("profileImage")
    }
}

/**
 * The session of the app (concept 27, 4.2): the server, the token and device id from the
 * [TokenStore], account and profile from the [AccountStore]. The shared HTTP client reads its
 * credentials here ([SessionCredentials]) and the gate of «Server weist sich aus» the remembered
 * key ([TrustedServer]). Pages read [state]; only the sign-in, the profile choice and the sign-out
 * change it.
 *
 * The token goes only to the own server and only while [credentialsSuppressed] is off (it is on
 * while the user confirms a new key of the server, concept 27 section 6). The device id goes in
 * `X-Slouchcat-Device` only without a token: with a session the server knows the device anyway.
 */
class SessionManager(
    private val servers: ServerStore,
    private val tokens: TokenStore,
    private val accounts: AccountStore,
) : SessionCredentials, TrustedServer {
    private val _state = MutableStateFlow<SessionState>(SessionState.Loading)
    val state: StateFlow<SessionState> = _state.asStateFlow()

    private val mutex = Mutex()

    @Volatile private var saved: SavedServer? = null

    @Volatile private var token: String? = null

    @Volatile private var device: String? = null

    @Volatile private var email: String? = null

    @Volatile private var profile: ActiveProfile? = null

    @Volatile
    var credentialsSuppressed: Boolean = false

    /** Called (under the lock) when the server changes: another address or another key (security review H-2). */
    @Volatile
    var onServerChanged: () -> Unit = {}

    private val _epoch = MutableStateFlow(0L)

    /**
     * Counts every change of who uses the app: sign-in, profile choice, sign-out. The navigation
     * starts afresh with each (code review B-1); a 401 of an older epoch changes nothing (minor 8).
     */
    val epoch: StateFlow<Long> = _epoch.asStateFlow()

    /** The server as stored, with its version and key. */
    val savedServer: SavedServer? get() = saved

    override val server: HttpUrl? get() = saved?.address
    override val serverKey: String? get() = saved?.serverKey
    override val bearerToken: String? get() = if (credentialsSuppressed) null else token

    /** Only without a session; never while a new key is being confirmed (security review L-1). */
    override val deviceId: String? get() = if (token == null && !credentialsSuppressed) device else null

    /** The device id the server handed out, sent along with the next sign-in (not while confirming a key). */
    val knownDeviceId: String? get() = if (credentialsSuppressed) null else device

    /** Reads everything once and then follows the server address (the setup page writes it). */
    fun start(scope: CoroutineScope) {
        scope.launch {
            mutex.withLock {
                val stored = tokens.read()
                token = stored?.token
                device = stored?.deviceId
                val (mail, chosen) = accounts.read()
                email = mail
                profile = chosen
                if (token == null && (mail != null || chosen != null)) {
                    // The token is gone (key lost): account and profile go with it.
                    accounts.write(null, null)
                    email = null
                    profile = null
                }
            }
            servers.server.collect { server ->
                mutex.withLock {
                    val old = saved
                    // Another address or another key: another server (H-2). Its session, its device id
                    // and every proof of the old one are worthless there (code review minor 10).
                    val changed = old != null && server != null && (old.address != server.address || old.serverKey != server.serverKey)
                    saved = server
                    if (changed) {
                        clearLocked()
                        device = null
                        tokens.write(StoredCredentials(null, null))
                        onServerChanged()
                        Log.i(TAG, "server changed, session cleared")
                    }
                    publish(newEpoch = changed)
                }
            }
        }
    }

    /** Waits until [start] has read the stores. */
    suspend fun ready(): SessionState = state.first { it != SessionState.Loading }

    /**
     * The user confirmed a new key by signing in (section 6): the key counts at once, before the
     * store's flow brings it back (code review minor 11). Not a change of server.
     */
    suspend fun keyConfirmed(key: String) = mutex.withLock {
        saved = saved?.copy(serverKey = key)
        servers.confirmKey(key)
    }

    suspend fun signedIn(newToken: String, newDeviceId: String, accountEmail: String, chosen: ActiveProfile?) = mutex.withLock {
        tokens.write(StoredCredentials(newToken, newDeviceId))
        token = newToken
        device = newDeviceId
        email = accountEmail
        profile = chosen
        accounts.write(accountEmail, chosen)
        Log.i(TAG, "signed in", "profile" to (chosen != null))
        publish(newEpoch = true)
    }

    suspend fun profileChosen(chosen: ActiveProfile) = mutex.withLock {
        profile = chosen
        accounts.write(email, chosen)
        publish(newEpoch = true)
    }

    /**
     * The active profile was edited («Profile verwalten»: name, accent): the session shows the new
     * values without a new epoch, so the pages and their back stacks stay. Another id changes nothing.
     */
    suspend fun profileUpdated(updated: ActiveProfile) = mutex.withLock {
        if (profile?.id != updated.id) return@withLock
        profile = updated
        accounts.write(email, updated)
        publish()
    }

    suspend fun accountChanged(accountEmail: String) = mutex.withLock {
        email = accountEmail
        accounts.write(accountEmail, profile)
        publish()
    }

    /** Ends the session on this device: token, account and profile go, server and device id stay. */
    suspend fun signedOut() = mutex.withLock {
        clearLocked()
        Log.i(TAG, "signed out")
        publish(newEpoch = true)
    }

    private suspend fun clearLocked() {
        token = null
        email = null
        profile = null
        credentialsSuppressed = false
        tokens.write(StoredCredentials(null, device))
        accounts.write(null, null)
    }

    /** The state after a change; [newEpoch] in the same step, so nobody sees one without the other. */
    private fun publish(newEpoch: Boolean = false) {
        if (newEpoch) _epoch.value++
        val server = saved
        val mail = email
        val chosen = profile
        _state.value = when {
            server == null || server.needsSetup -> SessionState.NoServer
            token == null || mail == null -> SessionState.SignedOut(server)
            chosen == null -> SessionState.ChooseProfile(server, mail)
            else -> SessionState.Ready(server, mail, chosen)
        }
    }

    private companion object {
        const val TAG = "session"
    }
}
