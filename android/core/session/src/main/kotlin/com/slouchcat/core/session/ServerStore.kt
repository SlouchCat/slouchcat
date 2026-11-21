package com.slouchcat.core.session

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/** The server the app is set up with. */
data class SavedServer(val address: HttpUrl, val version: String) {
    val encrypted: Boolean get() = address.isHttps
}

/** Where the server address lives; a seam for tests. */
interface ServerStore {
    /** The saved server, `null` before the first setup. */
    val server: Flow<SavedServer?>

    suspend fun save(server: SavedServer)

    suspend fun clear()
}

private val Context.serverPreferences: DataStore<Preferences> by preferencesDataStore(name = "server")

/** [ServerStore] in a DataStore of the app (concept 27, 5.2: settings in DataStore). */
class DataStoreServerStore(context: Context) : ServerStore {
    private val store = context.applicationContext.serverPreferences

    override val server: Flow<SavedServer?> = store.data.map { prefs ->
        val address = prefs[ADDRESS]?.toHttpUrlOrNull() ?: return@map null
        SavedServer(address, prefs[VERSION].orEmpty())
    }

    override suspend fun save(server: SavedServer) {
        store.edit {
            it[ADDRESS] = server.address.toString()
            it[VERSION] = server.version
        }
    }

    override suspend fun clear() {
        store.edit { it.clear() }
    }

    private companion object {
        val ADDRESS = stringPreferencesKey("address")
        val VERSION = stringPreferencesKey("version")
    }
}
