package com.slouchcat.app.mobile.tvlink

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.slouchcat.app.mobile.BuildConfig
import com.slouchcat.core.pairing.PairingProtocol
import com.slouchcat.core.session.ParsedAddress
import com.slouchcat.core.session.ServerAddress
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.map
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/**
 * The address of the relay (F-21): built in, changeable for whoever runs a relay of their own.
 * Phone and TV need the same one: the phone refuses a code of another relay (concept 27 section 10
 * step 2). The same as the TV app's `RelaySettings`; both apps keep their own (no Android in
 * :core:pairing). A seam for tests.
 */
interface RelaySettings {
    /** The relay in use; the built-in one until another is set. */
    val relay: Flow<HttpUrl>

    /** Sets another relay; `null` returns to the built-in one. */
    suspend fun set(address: HttpUrl?)

    companion object {
        val DEFAULT: HttpUrl = PairingProtocol.DEFAULT_RELAY.toHttpUrl()
    }
}

/**
 * A relay address as the user typed it; `null` when it is none. Only `https`: over plain HTTP whoever
 * reads along sees the meeting point and could lay a packet down first (K26 4.6). Plain HTTP only in
 * the debug build (a local relay in development) or with [allowPlainHttp].
 */
fun parseRelay(text: String, allowPlainHttp: Boolean = BuildConfig.DEBUG): HttpUrl? {
    val url = (ServerAddress.parse(text) as? ParsedAddress.Valid)?.url ?: return null
    return url.takeIf { it.isHttps || allowPlainHttp }
}

private val Context.pairingPreferences: DataStore<Preferences> by preferencesDataStore(name = "pairing")

/** [RelaySettings] in a DataStore of the app: only the address, never anything of a pairing. */
class DataStoreRelaySettings(context: Context) : RelaySettings {
    private val store = context.applicationContext.pairingPreferences

    override val relay: Flow<HttpUrl> = store.data.map { prefs -> prefs[RELAY]?.toHttpUrlOrNull() ?: RelaySettings.DEFAULT }

    override suspend fun set(address: HttpUrl?) {
        store.edit { if (address == null) it.remove(RELAY) else it[RELAY] = address.toString() }
    }

    private companion object {
        val RELAY = stringPreferencesKey("relay")
    }
}

/** [RelaySettings] in memory (tests, the showcase, the emulator flow). */
class MemoryRelaySettings(initial: HttpUrl = RelaySettings.DEFAULT) : RelaySettings {
    val flow = MutableStateFlow(initial)
    override val relay: Flow<HttpUrl> = flow

    override suspend fun set(address: HttpUrl?) {
        flow.value = address ?: RelaySettings.DEFAULT
    }
}
