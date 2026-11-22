package com.slouchcat.app.tv.player

import android.annotation.SuppressLint
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.media.AudioDeviceCallback
import android.media.AudioDeviceInfo
import android.media.AudioManager
import android.os.Handler
import android.os.Looper
import androidx.core.content.ContextCompat
import com.slouchcat.core.player.AudioSetting
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DeviceFactsReader
import com.slouchcat.core.player.DeviceType
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow

/*
 * What the TV reports about its sound (concept 26 5.2, 5.3; ADR 0041): the decoders of the device,
 * the encodings the output passes through (HDMI, ARC, eARC through Media3's AudioCapabilities) and
 * the channels of the output. The rules are in :core:player (`CapabilityRules`); the TV adds two
 * things: its own name («Shield · Android TV 11», the name of the setup) and headphones. A headphone
 * on a TV box takes the sound while the HDMI output stays listed: then the TV reports stereo and
 * nothing passed through, so the server sends what a headphone plays.
 */

/** Outputs that take the sound from the HDMI output when they are connected: headphones, Bluetooth speakers and hearing aids. */
@SuppressLint("InlinedApi")
val HEADPHONE_OUTPUTS: Set<Int> = setOf(
    AudioDeviceInfo.TYPE_WIRED_HEADPHONES,
    AudioDeviceInfo.TYPE_WIRED_HEADSET,
    AudioDeviceInfo.TYPE_BLUETOOTH_A2DP,
    AudioDeviceInfo.TYPE_USB_HEADSET,
    AudioDeviceInfo.TYPE_BLE_HEADSET,
    AudioDeviceInfo.TYPE_BLE_SPEAKER,
    AudioDeviceInfo.TYPE_BLE_BROADCAST,
    AudioDeviceInfo.TYPE_HEARING_AID,
)

/** [facts] as the TV reports them: the setup's [name], stereo and no passthrough while a headphone listens ([outputs]: the types of the outputs). */
fun tvFacts(facts: DeviceFacts, outputs: Set<Int>, name: String): DeviceFacts {
    val headphone = outputs.any { it in HEADPHONE_OUTPUTS }
    return facts.copy(
        type = DeviceType.Tv,
        name = name,
        passthrough = if (headphone) emptySet() else facts.passthrough,
        outputChannels = if (headphone) STEREO else facts.outputChannels,
    )
}

/** What the TV hands to Media3's sound now: the formats passed through and the channels (the menu «Ton», the settings). */
data class SoundOutput(val passthrough: List<String>, val channels: Int) {
    /** Surround sound reaches a sound system: passed through or as PCM over several channels. */
    val surround: Boolean get() = passthrough.isNotEmpty() || channels > STEREO

    companion object {
        fun of(facts: DeviceFacts): SoundOutput =
            SoundOutput(CapabilityRules.PASSTHROUGH.filter { it in facts.passthrough }, CapabilityRules.channels(facts))
    }
}

/** What reaches the server for [setting]: the formats and channels of «Von Hand», the output's for «Automatisch», stereo else. */
fun reported(setting: AudioSetting, output: SoundOutput): SoundOutput = when (setting) {
    AudioSetting.Auto -> output
    AudioSetting.Stereo -> SoundOutput(emptyList(), STEREO)
    is AudioSetting.Manual -> SoundOutput(CapabilityRules.PASSTHROUGH.filter { it in setting.formats }, setting.channels.takeIf { it == SURROUND || it == SURROUND_WIDE } ?: STEREO)
}

/** Reads the facts of this TV (decoders, display, output): a moment's work, not on the main thread. */
class TvFactsReader(private val context: Context, private val name: () -> String) {
    fun read(): DeviceFacts {
        val outputs = context.getSystemService(AudioManager::class.java)
            ?.getDevices(AudioManager.GET_DEVICES_OUTPUTS)?.map { it.type }?.toSet().orEmpty()
        return tvFacts(DeviceFactsReader(context).read(DeviceType.Tv), outputs, name())
    }
}

/**
 * A signal whenever an output comes or goes (an HDMI sound system switched off or on, a
 * headphone): the player reads its facts again, the server decides again.
 */
fun outputChanges(context: Context): Flow<Unit> = callbackFlow {
    val audio = context.getSystemService(AudioManager::class.java)
    val main = Handler(Looper.getMainLooper())
    var first = true
    val callback = object : AudioDeviceCallback() {
        // The system names every present output once at registration: that is no change.
        override fun onAudioDevicesAdded(addedDevices: Array<out AudioDeviceInfo>) {
            if (first) first = false else trySend(Unit)
        }

        override fun onAudioDevicesRemoved(removedDevices: Array<out AudioDeviceInfo>) {
            trySend(Unit)
        }
    }
    // HDMI: a sound system that changes what it takes (eARC, switched to another input) sends this.
    val hdmi = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            if (!isInitialStickyBroadcast) trySend(Unit)
        }
    }
    audio?.registerAudioDeviceCallback(callback, main)
    ContextCompat.registerReceiver(context, hdmi, IntentFilter(AudioManager.ACTION_HDMI_AUDIO_PLUG), ContextCompat.RECEIVER_NOT_EXPORTED)
    awaitClose {
        audio?.unregisterAudioDeviceCallback(callback)
        context.unregisterReceiver(hdmi)
    }
}

private const val STEREO = 2
private const val SURROUND = 6
private const val SURROUND_WIDE = 8
