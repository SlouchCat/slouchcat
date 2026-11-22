package com.slouchcat.core.player

import android.content.Context
import android.net.ConnectivityManager
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.slouchcat.core.api.generated.models.QualityCap
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.update

/**
 * The quality a device asks for on a network (mockup `/settings/playback`): automatic or a cap. A
 * cap goes to the server as `qualityCap` (V-17): a file within it still plays directly.
 */
enum class QualityChoice(val key: String, val cap: QualityCap?) {
    Auto("auto", null),
    High("1080p", QualityCap._1080P),
    Medium("720p", QualityCap._720P),
    Low("480p", QualityCap._480P),
    Saver("360p", QualityCap._360P),
    ;

    companion object {
        fun of(key: String?): QualityChoice? = entries.firstOrNull { it.key == key }
    }
}

/**
 * The playback settings of this device (mockup `/settings/playback`: «Diese Einstellungen gelten
 * für dieses Gerät»), apart from the languages, which belong to the profile on the server.
 * [downloadsWifiOnly] is read by the downloads of stage 6.
 */
data class DeviceSettings(
    val qualityMobile: QualityChoice = QualityChoice.Medium,
    val qualityWifi: QualityChoice = QualityChoice.High,
    val autoplayNext: Boolean = true,
    val offerSkipIntro: Boolean = true,
    val downloadsWifiOnly: Boolean = true,
    val audio: AudioSetting = AudioSetting.Auto,
) {
    /** The cap of the network the device uses now. */
    fun cap(metered: Boolean): QualityCap? = (if (metered) qualityMobile else qualityWifi).cap
}

/** Where [DeviceSettings] live; a seam for tests. */
interface DeviceSettingsStore {
    val settings: Flow<DeviceSettings>

    suspend fun update(change: (DeviceSettings) -> DeviceSettings)
}

/** [DeviceSettingsStore] in memory (tests, previews). */
class MemoryDeviceSettingsStore(initial: DeviceSettings = DeviceSettings()) : DeviceSettingsStore {
    private val state = MutableStateFlow(initial)
    override val settings: Flow<DeviceSettings> = state

    override suspend fun update(change: (DeviceSettings) -> DeviceSettings) = state.update(change)
}

private val Context.playbackPreferences: DataStore<Preferences> by preferencesDataStore(name = "playback")

/** [DeviceSettingsStore] in a DataStore of the app (concept 27 5.2). */
class DataStoreDeviceSettingsStore(context: Context) : DeviceSettingsStore {
    private val store = context.applicationContext.playbackPreferences

    override val settings: Flow<DeviceSettings> = store.data.map(::read)

    private fun read(prefs: Preferences): DeviceSettings {
        val defaults = DeviceSettings()
        return DeviceSettings(
            qualityMobile = QualityChoice.of(prefs[QUALITY_MOBILE]) ?: defaults.qualityMobile,
            qualityWifi = QualityChoice.of(prefs[QUALITY_WIFI]) ?: defaults.qualityWifi,
            autoplayNext = prefs[AUTOPLAY] ?: defaults.autoplayNext,
            offerSkipIntro = prefs[SKIP_INTRO] ?: defaults.offerSkipIntro,
            downloadsWifiOnly = prefs[DOWNLOADS_WIFI] ?: defaults.downloadsWifiOnly,
            audio = when (prefs[AUDIO]) {
                "stereo" -> AudioSetting.Stereo
                "manual" -> AudioSetting.Manual(
                    prefs[AUDIO_FORMATS].orEmpty().split(',').filter { it.isNotBlank() }.toSet(),
                    prefs[AUDIO_CHANNELS] ?: 2,
                )
                else -> AudioSetting.Auto
            },
        )
    }

    override suspend fun update(change: (DeviceSettings) -> DeviceSettings) {
        store.edit { prefs ->
            val next = change(read(prefs))
            prefs[QUALITY_MOBILE] = next.qualityMobile.key
            prefs[QUALITY_WIFI] = next.qualityWifi.key
            prefs[AUTOPLAY] = next.autoplayNext
            prefs[SKIP_INTRO] = next.offerSkipIntro
            prefs[DOWNLOADS_WIFI] = next.downloadsWifiOnly
            when (val audio = next.audio) {
                AudioSetting.Auto -> prefs[AUDIO] = "auto"
                AudioSetting.Stereo -> prefs[AUDIO] = "stereo"
                is AudioSetting.Manual -> {
                    prefs[AUDIO] = "manual"
                    prefs[AUDIO_FORMATS] = audio.formats.sorted().joinToString(",")
                    prefs[AUDIO_CHANNELS] = audio.channels
                }
            }
        }
    }

    private companion object {
        val QUALITY_MOBILE = stringPreferencesKey("quality_mobile")
        val QUALITY_WIFI = stringPreferencesKey("quality_wifi")
        val AUTOPLAY = booleanPreferencesKey("autoplay_next")
        val SKIP_INTRO = booleanPreferencesKey("offer_skip_intro")
        val DOWNLOADS_WIFI = booleanPreferencesKey("downloads_wifi_only")
        val AUDIO = stringPreferencesKey("audio")
        val AUDIO_FORMATS = stringPreferencesKey("audio_formats")
        val AUDIO_CHANNELS = intPreferencesKey("audio_channels")
    }
}

/** Whether the network in use counts data (mobile data, a metered hotspot). */
fun interface MeteredNetwork {
    fun metered(): Boolean

    companion object {
        fun of(context: Context): MeteredNetwork {
            val connectivity = context.applicationContext.getSystemService(ConnectivityManager::class.java)
            return MeteredNetwork { connectivity.isActiveNetworkMetered }
        }
    }
}
