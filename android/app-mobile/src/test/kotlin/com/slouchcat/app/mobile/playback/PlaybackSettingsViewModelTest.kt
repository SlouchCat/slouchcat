package com.slouchcat.app.mobile.playback

import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.testing.FakePrefs
import com.slouchcat.app.mobile.testing.settle
import com.slouchcat.core.api.generated.models.PlaybackPrefs
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.player.DeviceSettings
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.player.QualityChoice
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class PlaybackSettingsViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val store = MemoryDeviceSettingsStore()
    private val prefs = FakePrefs()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun ready(vm: PlaybackSettingsViewModel) = (vm.state.value.prefs as Loadable.Ready).value

    @Test
    fun theDeviceSettingsSaveAtOnce() = runTest(dispatcher) {
        val vm = PlaybackSettingsViewModel(store, prefs)
        settle()
        assertEquals(DeviceSettings(), vm.state.value.device)
        vm.onSheet(PlaybackSheet.QualityMobile)
        vm.onQualityMobile(QualityChoice.Saver)
        vm.onAutoplay(false)
        vm.onSkipIntro(false)
        vm.onWifiOnly(false)
        settle()
        assertNull(vm.state.value.sheet)
        val saved = store.settings.first()
        assertEquals(DeviceSettings(qualityMobile = QualityChoice.Saver, autoplayNext = false, offerSkipIntro = false, downloadsWifiOnly = false), saved)
        assertEquals(saved, vm.state.value.device)
    }

    @Test
    fun theLanguagesOfTheProfileGoToTheServer() = runTest(dispatcher) {
        val vm = PlaybackSettingsViewModel(store, prefs)
        settle()
        val messages = mutableListOf<PlaybackMessage>()
        backgroundScope.launch { vm.messages.collect { messages += it } }
        vm.onAudioLanguage("en")
        settle()
        assertEquals(listOf("en", "de"), ready(vm).audio)
        vm.onAudioLanguage(null)
        vm.onSubtitleMode(SubtitleMode.OFF)
        vm.onSubtitleLanguage("fr")
        settle()
        assertEquals(LanguagePrefs(emptyList(), SubtitleMode.OFF, listOf("fr", "de")), LanguagePrefs.of(prefs.stored))
        assertEquals(LanguagePrefs.of(prefs.stored), ready(vm))
        // The three quick choices went to the server as one, the last state.
        assertEquals(2, prefs.saved.size)
        assertEquals(2, messages.count { it == PlaybackMessage.Saved })
    }

    @Test
    fun quickChoicesGoOutOneAfterTheOtherAndTheLastOneWins() = runTest(dispatcher) {
        val vm = PlaybackSettingsViewModel(store, prefs)
        settle()
        vm.onSubtitleMode(SubtitleMode.ALWAYS)
        vm.onSubtitleMode(SubtitleMode.FOREIGN)
        settle()
        assertEquals(listOf(SubtitleMode.FOREIGN), prefs.saved.map { it.subtitleMode })
        assertEquals(SubtitleMode.FOREIGN, ready(vm).mode)
        // A refusal goes back to what the server holds.
        prefs.refuse = true
        vm.onSubtitleMode(SubtitleMode.OFF)
        settle()
        assertEquals(SubtitleMode.FOREIGN, ready(vm).mode)
    }

    @Test
    fun aRefusalPutsTheOldValueBack() = runTest(dispatcher) {
        val vm = PlaybackSettingsViewModel(store, prefs)
        settle()
        val messages = mutableListOf<PlaybackMessage>()
        backgroundScope.launch { vm.messages.collect { messages += it } }
        prefs.refuse = true
        vm.onSubtitleMode(SubtitleMode.ALWAYS)
        assertEquals(SubtitleMode.ALWAYS, ready(vm).mode)
        settle()
        assertEquals(SubtitleMode.FORCED, ready(vm).mode)
        assertEquals(listOf(PlaybackMessage.Failed), messages)
    }

    @Test
    fun theSeriesAreForgottenAfterTheQuestion() = runTest(dispatcher) {
        val vm = PlaybackSettingsViewModel(store, prefs)
        settle()
        val messages = mutableListOf<PlaybackMessage>()
        backgroundScope.launch { vm.messages.collect { messages += it } }
        vm.onSheet(PlaybackSheet.ResetSeries)
        assertEquals(0, prefs.resets)
        vm.onResetSeries()
        settle()
        assertEquals(1, prefs.resets)
        assertEquals(listOf(PlaybackMessage.SeriesReset), messages)
    }

    @Test
    fun withFirstKeepsTheOrderAndTheLimit() {
        assertEquals(listOf("fr", "de", "en"), PlaybackSettingsViewModel.withFirst(listOf("de", "fr", "en"), "fr"))
        val ten = (0 until 10).map { "l$it" }
        assertEquals(10, PlaybackSettingsViewModel.withFirst(ten, "x").size)
    }
}
