package com.slouchcat.core.player

import android.annotation.SuppressLint
import android.content.Context
import android.hardware.display.DisplayManager
import android.media.AudioDeviceInfo
import android.media.AudioFormat
import android.media.AudioManager
import android.media.MediaCodecList
import android.os.Build
import android.view.Display
import androidx.media3.common.AudioAttributes
import androidx.annotation.OptIn
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.audio.AudioCapabilities

/**
 * Reads [DeviceFacts] from the device (concept 26 stage 0, `Capabilities.kt` of the spike): the
 * decoders of `MediaCodecList`, the encodings the audio output passes through, the channels of an
 * external output, the largest display mode and its HDR formats. Read again before every start: a
 * headphone or an HDMI adapter changes the answer.
 */
@OptIn(UnstableApi::class)
class DeviceFactsReader(private val context: Context) {
    fun read(type: DeviceType): DeviceFacts {
        val display = context.getSystemService(DisplayManager::class.java).getDisplay(Display.DEFAULT_DISPLAY)
        return DeviceFacts(
            decoders = decoders(),
            passthrough = passthrough(),
            outputChannels = outputChannels(),
            display = display(display),
            type = type,
            name = "${Build.MODEL} · Android ${Build.VERSION.RELEASE}".take(NAME_MAX),
        )
    }

    private fun decoders(): List<DecoderInfo> = MediaCodecList(MediaCodecList.ALL_CODECS).codecInfos
        .filter { !it.isEncoder }
        .flatMap { info ->
            info.supportedTypes.map { type ->
                DecoderInfo(type, info.getCapabilitiesForType(type).profileLevels.map { it.profile }.toSet())
            }
        }

    private fun passthrough(): Set<String> {
        // A device whose audio service does not answer passes nothing through.
        val capabilities = runCatching { AudioCapabilities.getCapabilities(context, AudioAttributes.DEFAULT, null) }.getOrNull() ?: return emptySet()
        return ENCODINGS.filter { (_, encoding) -> capabilities.supportsEncoding(encoding) }.keys
    }

    /**
     * The channels of an external output (HDMI, ARC, USB, dock); 2 for the speaker and every
     * headphone, wired or Bluetooth (concept 27 8.2: a phone reports stereo).
     */
    private fun outputChannels(): Int {
        val audio = context.getSystemService(AudioManager::class.java)
        return audio.getDevices(AudioManager.GET_DEVICES_OUTPUTS)
            .filter { it.type in MULTICHANNEL_OUTPUTS }
            .maxOfOrNull { it.channelCounts.maxOrNull() ?: 2 } ?: 2
    }

    @Suppress("DEPRECATION")
    private fun display(display: Display): DisplayInfo {
        val mode = display.supportedModes.maxByOrNull { it.physicalWidth * it.physicalHeight }
        val metrics = context.resources.displayMetrics
        val width = mode?.physicalWidth ?: metrics.widthPixels
        val height = mode?.physicalHeight ?: metrics.heightPixels
        val types = if (Build.VERSION.SDK_INT >= 34) display.mode.supportedHdrTypes.toList() else display.hdrCapabilities?.supportedHdrTypes?.toList().orEmpty()
        // Landscape: video fills the screen sideways, so the short side is the height that counts.
        return DisplayInfo(
            maxWidth = maxOf(width, height),
            maxHeight = minOf(width, height),
            hdrTypes = types.mapNotNull {
                when (it) {
                    Display.HdrCapabilities.HDR_TYPE_HDR10 -> HdrType.Hdr10
                    Display.HdrCapabilities.HDR_TYPE_HLG -> HdrType.Hlg
                    Display.HdrCapabilities.HDR_TYPE_DOLBY_VISION -> HdrType.DolbyVision
                    Display.HdrCapabilities.HDR_TYPE_HDR10_PLUS -> HdrType.Hdr10Plus
                    else -> null
                }
            }.toSet(),
        )
    }

    private companion object {
        const val NAME_MAX = 80
        val ENCODINGS = linkedMapOf(
            "ac3" to AudioFormat.ENCODING_AC3,
            "eac3" to AudioFormat.ENCODING_E_AC3,
            "dts" to AudioFormat.ENCODING_DTS,
            "truehd" to AudioFormat.ENCODING_DOLBY_TRUEHD,
        )
        // TYPE_HDMI_EARC is API 31: an older device never reports the value, the comparison is harmless.
        @SuppressLint("InlinedApi")
        val MULTICHANNEL_OUTPUTS = setOf(
            AudioDeviceInfo.TYPE_HDMI,
            AudioDeviceInfo.TYPE_HDMI_ARC,
            AudioDeviceInfo.TYPE_HDMI_EARC,
            AudioDeviceInfo.TYPE_USB_DEVICE,
            AudioDeviceInfo.TYPE_DOCK,
        )
    }
}
