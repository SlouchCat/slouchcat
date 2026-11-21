package com.slouchcat.core.session

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/**
 * The server the app is set up with. [serverKey]: its Ed25519 key, remembered at the setup and
 * checked in every network for a server over HTTP («Server weist sich aus», F-38).
 */
data class SavedServer(
    val address: HttpUrl,
    val version: String,
    val userCertificate: Boolean = false,
    val serverKey: String? = null,
) {
    val encrypted: Boolean get() = address.isHttps

    /** A server over HTTP without a remembered key cannot prove itself: the setup has to run again. */
    val needsSetup: Boolean get() = !encrypted && serverKey == null
}

/** Where the server address lives; a seam for tests. */
interface ServerStore {
    /** The saved server, `null` before the first setup. */
    val server: Flow<SavedServer?>

    suspend fun save(server: SavedServer)

    /** The user confirmed a new key of the server by signing in again (concept 27 section 6). */
    suspend fun confirmKey(serverKey: String)

    /** The version the server reported last. */
    suspend fun updateVersion(version: String)

    suspend fun clear()
}

private val Context.serverPreferences: DataStore<Preferences> by preferencesDataStore(name = "server")

/** [ServerStore] in a DataStore of the app (concept 27, 5.2: settings in DataStore). */
class DataStoreServerStore(context: Context) : ServerStore {
    private val store = context.applicationContext.serverPreferences

    override val server: Flow<SavedServer?> = store.data.map { prefs ->
        val address = prefs[ADDRESS]?.toHttpUrlOrNull() ?: return@map null
        SavedServer(address, prefs[VERSION].orEmpty(), prefs[USER_CERTIFICATE] ?: false, prefs[SERVER_KEY])
    }

    override suspend fun save(server: SavedServer) {
        store.edit {
            it[ADDRESS] = server.address.toString()
            it[VERSION] = server.version
            it[USER_CERTIFICATE] = server.userCertificate
            if (server.serverKey != null) it[SERVER_KEY] = server.serverKey else it.remove(SERVER_KEY)
        }
    }

    override suspend fun confirmKey(serverKey: String) {
        store.edit { it[SERVER_KEY] = serverKey }
    }

    override suspend fun updateVersion(version: String) {
        store.edit { it[VERSION] = version }
    }

    override suspend fun clear() {
        store.edit { it.clear() }
    }

    private companion object {
        val ADDRESS = stringPreferencesKey("address")
        val VERSION = stringPreferencesKey("version")
        val USER_CERTIFICATE = booleanPreferencesKey("userCertificate")
        val SERVER_KEY = stringPreferencesKey("serverKey")
    }
}
