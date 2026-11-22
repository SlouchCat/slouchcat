package com.slouchcat.app.tv.playback

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.tv.testing.Dpad
import com.slouchcat.app.tv.testing.EMULATOR_TV
import com.slouchcat.app.tv.testing.FakePrefs
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.player.AudioSetting
import com.slouchcat.core.player.DeviceSettings
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.player.QualityChoice
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * «Einstellungen › Wiedergabe» with the remote (mockup `/settings/playback`): the start focus on
 * «Qualität», every row reached, a menu chooses and closes, the switches, the sound: automatic,
 * always stereo, formats and channels by hand, prefilled with what the TV found.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class PlaybackSettingsScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val remote = Dpad(compose)
    private val context: Context = ApplicationProvider.getApplicationContext()
    private val store = MemoryDeviceSettingsStore(DeviceSettings(qualityMobile = QualityChoice.Auto, qualityWifi = QualityChoice.Auto))
    private val prefs = FakePrefs()
    private fun s(id: Int, vararg args: Any) = context.getString(id, *args)
    private val menu = hasAnyAncestor(SemanticsMatcher.keyIsDefined(SemanticsProperties.PaneTitle))
    private val soundbar = EMULATOR_TV.copy(passthrough = setOf("ac3", "eac3"))

    private fun show() {
        val vm = PlaybackSettingsViewModel(store, prefs) { soundbar }
        remote.show {
            val state by vm.state.collectAsState()
            PlaybackSettingsScreen(state, events(vm), profileName = "Primary")
        }
        remote.settle()
    }

    private fun device() = runBlocking { store.settings.first() }

    @Test
    fun everyRowIsReachedAndTheQualityIsChosenInAMenu() {
        show()
        remote.assertFocused(s(R.string.app_playback_quality))
        remote.assertEveryControlReachable()
        remote.ok()
        remote.assertFocused(s(R.string.app_playback_choice_auto))
        remote.down(2)
        remote.ok()
        remote.settle()
        assertEquals(QualityChoice.Medium, device().qualityWifi)
        assertEquals(QualityChoice.Medium, device().qualityMobile)
        remote.assertFocused(s(R.string.app_playback_quality))
    }

    @Test
    fun theSubtitlesOfTheProfileGoToTheServer() {
        show()
        remote.down(2)
        remote.assertFocused(s(R.string.settings_playback_subtitleMode_label))
        remote.ok()
        remote.assertFocused(s(R.string.settings_playback_subtitleMode_forced))
        remote.assertEveryControlReachable(menu)
        remote.down()
        remote.ok()
        remote.settle()
        assertEquals(SubtitleMode.FOREIGN, prefs.saved.single().subtitleMode)
    }

    @Test
    fun theSoundAutomaticAlwaysStereoOrByHand() {
        show()
        // What the TV found: Dolby Digital and Dolby Digital Plus, 5.1.
        compose.onNode(hasText(s(R.string.tv_playback_detected, "Dolby Digital, Dolby Digital Plus", "5.1"))).assertExists()
        // With the remote: down (or up) to the row, OK.
        fun press(text: String) {
            repeat(ROWS) { if (!remote.focusedLabel().contains(text)) remote.down() }
            repeat(ROWS) { if (!remote.focusedLabel().contains(text)) remote.up() }
            remote.assertFocused(text)
            remote.ok()
            remote.settle()
        }
        press(s(R.string.tv_playback_soundStereo))
        assertEquals(AudioSetting.Stereo, device().audio)
        compose.onNode(hasText(s(R.string.tv_playback_passedStereo))).assertExists()
        // By hand: filled with the decoders and what passed through, then DTS on, 7.1.
        press(s(R.string.tv_playback_soundManual))
        assertEquals(AudioSetting.Manual(setOf("aac", "flac", "opus", "ac3", "eac3"), 6), device().audio)
        press("DTS")
        assertTrue("dts" in (device().audio as AudioSetting.Manual).formats)
        press(s(R.string.tv_playback_channels))
        remote.down()
        remote.ok()
        remote.settle()
        assertEquals(8, (device().audio as AudioSetting.Manual).channels)
        press(s(R.string.tv_playback_soundAuto))
        assertEquals(AudioSetting.Auto, device().audio)
    }

    private companion object {
        const val ROWS = 16
    }
}
