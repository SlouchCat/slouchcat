package com.slouchcat.core.session

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStoreFile
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.map
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import java.util.concurrent.ConcurrentHashMap

/**
 * The address of the pairing relay (concept 27 F-21, K26 4.7): built in, changeable in the advanced
 * settings by whoever runs a relay of their own. Phone and TV need the same one: the phone refuses a
 * code of another relay (concept 27 section 10 step 2). Shared by both apps; a seam for tests.
 */
interface RelaySettings {
    /** The relay in use; the built-in one until another is set. */
    val relay: Flow<HttpUrl>

    /** Sets another relay; `null` returns to the built-in one. */
    suspend fun set(address: HttpUrl?)

    companion object {
        /** The relay built into the apps (F-37); the same as `PairingProtocol.DEFAULT_RELAY`. */
        val DEFAULT: HttpUrl = "https://pair.slouchcat.com/".toHttpUrl()
    }
}

/**
 * A relay address as the user typed it; `null` when it is none. Only `https`: over plain HTTP whoever
 * reads along sees the meeting point and the pickup secret (K26 2a review N-7, K26 4.6). Plain HTTP
 * only with [allowPlainHttp]: the apps pass `BuildConfig.DEBUG` (a local relay in development).
 */
fun parseRelay(text: String, allowPlainHttp: Boolean): HttpUrl? {
    val url = (ServerAddress.parse(text) as? ParsedAddress.Valid)?.url ?: return null
    return url.takeIf { it.isHttps || allowPlainHttp }
}

/**
 * [RelaySettings] in the DataStore file [name] of the app (the phone `pairing`, the TV `tv`): only
 * the address, nothing of a pairing; neither app takes the file into its backup.
 */
class DataStoreRelaySettings(context: Context, name: String) : RelaySettings {
    private val store = stores.getOrPut(name) {
        val app = context.applicationContext
        PreferenceDataStoreFactory.create { app.preferencesDataStoreFile(name) }
    }

    override val relay: Flow<HttpUrl> = store.data.map { prefs -> prefs[RELAY]?.toHttpUrlOrNull() ?: RelaySettings.DEFAULT }

    override suspend fun set(address: HttpUrl?) {
        store.edit { if (address == null) it.remove(RELAY) else it[RELAY] = address.toString() }
    }

    private companion object {
        val RELAY = stringPreferencesKey("relay")

        /** One DataStore per file and process, as DataStore demands. */
        val stores = ConcurrentHashMap<String, DataStore<Preferences>>()
    }
}

/** [RelaySettings] in memory (tests, the showcases, the emulator flows). */
class MemoryRelaySettings(initial: HttpUrl = RelaySettings.DEFAULT) : RelaySettings {
    val flow = MutableStateFlow(initial)
    override val relay: Flow<HttpUrl> = flow

    override suspend fun set(address: HttpUrl?) {
        flow.value = address ?: RelaySettings.DEFAULT
    }
}
