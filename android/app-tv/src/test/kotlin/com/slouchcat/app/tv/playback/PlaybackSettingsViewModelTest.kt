package com.slouchcat.app.tv.playback

import com.slouchcat.app.tv.testing.EMULATOR_TV
import com.slouchcat.app.tv.testing.FakePrefs
import com.slouchcat.core.player.AudioSetting
import com.slouchcat.core.player.DecoderInfo
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test

/** «Formate von Hand» starts with what «Automatisch» reports (K26 stage 0 point 2, review m-7). */
@OptIn(ExperimentalCoroutinesApi::class)
class PlaybackSettingsViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    @Test
    fun byHandKeepsDolbyTheTvDecodesItself() = runTest(dispatcher) {
        // A TV with its own speakers that decodes Dolby Digital and passes nothing through.
        val tv = EMULATOR_TV.copy(decoders = EMULATOR_TV.decoders + DecoderInfo("audio/ac3") + DecoderInfo("audio/eac3"))
        val store = MemoryDeviceSettingsStore()
        val vm = PlaybackSettingsViewModel(store, FakePrefs()) { tv }
        runCurrent()
        vm.onSoundManual()
        runCurrent()
        assertEquals(AudioSetting.Manual(setOf("aac", "opus", "flac", "ac3", "eac3"), 2), store.settings.first().audio)
        // A format off and on again: the decoders of the TV without Dolby stay.
        vm.onFormat("ac3", false)
        runCurrent()
        assertEquals(setOf("aac", "opus", "flac", "eac3"), (store.settings.first().audio as AudioSetting.Manual).formats)
    }
}
