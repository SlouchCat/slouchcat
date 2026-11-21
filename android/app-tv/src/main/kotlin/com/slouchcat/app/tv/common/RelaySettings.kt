package com.slouchcat.app.tv.common

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.slouchcat.core.pairing.PairingProtocol
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.map
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

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
