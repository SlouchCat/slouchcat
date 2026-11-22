package com.slouchcat.app.mobile.playback

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.material3.SnackbarHostState
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isSelectable
import androidx.compose.ui.test.isToggleable
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.more.MoreFeature
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.app.mobile.testing.FakePrefs
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeAccounts
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import com.slouchcat.core.browse.testing.HTTPS_SERVER
import com.slouchcat.core.browse.testing.PRIMARY
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.player.DeviceSettings
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.player.QualityChoice
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private const val PHONE = "w412dp-h915dp"
private const val TABLET = "w1280dp-h800dp-land"

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/** «Wiedergabe» (rule 8): two widths, 200 % font, every control labelled, the sheets. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class PlaybackSettingsScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val recorder = object : PlaybackSettingsEvents {
        override fun onBack() { events += "back" }
        override fun onRetry() { events += "retry" }
        override fun onSheet(sheet: PlaybackSheet?) { events += "sheet:$sheet" }
        override fun onQualityMobile(choice: QualityChoice) { events += "mobile:$choice" }
        override fun onQualityWifi(choice: QualityChoice) { events += "wifi:$choice" }
        override fun onAutoplay(on: Boolean) { events += "autoplay:$on" }
        override fun onSkipIntro(on: Boolean) { events += "intro:$on" }
        override fun onWifiOnly(on: Boolean) { events += "wifiOnly:$on" }
        override fun onAudioLanguage(code: String?) { events += "audio:$code" }
        override fun onSubtitleMode(mode: SubtitleMode) { events += "mode:${mode.value}" }
        override fun onSubtitleLanguage(code: String?) { events += "subtitle:$code" }
        override fun onResetSeries() { events += "reset" }
    }

    private val prefs = LanguagePrefs(listOf("de", "en"), SubtitleMode.OFF, emptyList())

    private fun show(state: PlaybackSettingsUiState) = compose.setContent {
        SlouchcatTheme { PlaybackSettingsScreen(state, recorder, SnackbarHostState(), profileName = "Marta") }
    }

    private fun page() {
        show(PlaybackSettingsUiState(DeviceSettings(), Loadable.Ready(prefs)))
        compose.onNodeWithText(s(R.string.app_playback_title)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_playback_choice_medium)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_playback_choice_high)).assertIsDisplayed()
        compose.onNodeWithText("German").assertIsDisplayed()
        compose.onNodeWithText(s(R.string.settings_playback_subtitleMode_off)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_playback_noLanguage)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_playback_profileNote, "Marta")).performScrollTo().assertIsDisplayed()
        compose.onNode(hasText(s(R.string.app_playback_autoplay)) and isToggleable()).performScrollTo().assertIsOn()
        compose.onNode(hasText(s(R.string.app_playback_wifiOnly), substring = true) and isToggleable()).performScrollTo().assertIsOn()
        compose.onNodeWithText(s(R.string.app_playback_deviceNote)).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.app_playback_mobile)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.app_playback_audioLanguage)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.settings_playback_series_reset)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.app_playback_skipIntro)) and isToggleable())).performClick()
        compose.onNode(hasContentDescription(s(R.string.common_back)) and hasClickAction()).performClick()
        assertEquals(listOf("sheet:QualityMobile", "sheet:AudioLanguage", "sheet:ResetSeries", "intro:false", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun pagePhone() = page()

    @Test @Config(qualifiers = TABLET)
    fun pageTablet() = page()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun pagePhoneLargeFont() = page()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun pageTabletLargeFont() = page()

    @Test @Config(qualifiers = PHONE)
    fun theLanguagesFailToLoad() {
        show(PlaybackSettingsUiState(prefs = Loadable.Failed(com.slouchcat.core.media.LoadError.Offline)))
        compose.onNode(hasText(s(R.string.app_state_retry)) and hasClickAction()).performScrollTo().performClick()
        assertEquals(listOf("retry"), events)
    }

    private fun qualitySheet() {
        show(PlaybackSettingsUiState(DeviceSettings(), Loadable.Ready(prefs), sheet = PlaybackSheet.QualityMobile))
        compose.onNode(hasText(s(R.string.app_playback_choice_medium)) and isSelectable()).assertIsSelected()
        compose.onNodeWithText(s(R.string.app_playback_mobileHint)).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNode(hasText(s(R.string.app_playback_choice_saver)) and isSelectable()).performScrollTo().performClick()
        assertEquals(listOf("mobile:Saver"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun qualitySheetPhone() = qualitySheet()

    @Test @Config(qualifiers = TABLET)
    fun qualitySheetTablet() = qualitySheet()

    @Test @Config(qualifiers = PHONE)
    fun wifiOffersNoDataSaver() {
        show(PlaybackSettingsUiState(DeviceSettings(), Loadable.Ready(prefs), sheet = PlaybackSheet.QualityWifi))
        compose.onNode(hasText(s(R.string.app_playback_choice_high)) and isSelectable()).assertIsSelected()
        compose.onNodeWithText(s(R.string.app_playback_choice_saver)).assertDoesNotExist()
    }

    @Test @Config(qualifiers = PHONE)
    fun subtitleModeSheet() {
        show(PlaybackSettingsUiState(DeviceSettings(), Loadable.Ready(prefs), sheet = PlaybackSheet.SubtitleMode))
        compose.onNode(hasText(s(R.string.settings_playback_subtitleMode_off)) and isSelectable()).assertIsSelected()
        compose.onNodeWithText(s(R.string.settings_playback_subtitleMode_foreignHint)).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNode(hasText(s(R.string.settings_playback_subtitleMode_always)) and isSelectable()).performScrollTo().performClick()
        assertEquals(listOf("mode:always"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun audioLanguageSheet() {
        show(PlaybackSettingsUiState(DeviceSettings(), Loadable.Ready(prefs), sheet = PlaybackSheet.AudioLanguage))
        compose.onNode(hasText("German") and isSelectable()).assertIsSelected()
        compose.onNode(hasText(s(R.string.app_playback_original)) and isSelectable()).performClick()
        compose.onNode(hasText("English") and isSelectable()).performScrollTo().performClick()
        assertEquals(listOf("audio:null", "audio:en"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun resetAsksFirst() {
        show(PlaybackSettingsUiState(DeviceSettings(), Loadable.Ready(prefs), sheet = PlaybackSheet.ResetSeries))
        compose.onNodeWithText(s(R.string.settings_playback_series_confirmBody)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNode(hasText(s(R.string.settings_playback_series_confirm)) and hasClickAction()).performClick()
        assertEquals(listOf("reset"), events)
    }
}

/** «Mehr › Einstellungen › Wiedergabe» through the real NavHost; Back returns to «Mehr» and leads out. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class PlaybackSettingsBackTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val store = MemoryDeviceSettingsStore(DeviceSettings(qualityMobile = QualityChoice.Low))

    private val graph = AppGraph(
        context,
        features = listOf(MoreFeature, PlaybackSettingsFeature(settings = { store }, repository = { FakePrefs() })),
        serverStore = FakeServers(HTTPS_SERVER),
        tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
        accountStore = FakeAccounts(EMAIL, PRIMARY),
    )

    /** The state after the main looper is idle (fakes answer on the main thread): no clock. */
    private fun waitFor(matcher: androidx.compose.ui.test.SemanticsMatcher) {
        compose.waitForIdle()
        assertTrue("not shown: $matcher", compose.onAllNodes(matcher).fetchSemanticsNodes().isNotEmpty())
    }

    private fun back() = compose.activityRule.scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }

    private fun backLeadsOut() {
        compose.setContent { SlouchcatNavHost(graph) }
        val row = hasText(s(R.string.app_playback_title)) and hasClickAction()
        waitFor(row)
        // The second line names the qualities of both networks.
        waitFor(hasText(s(R.string.app_playback_summary, s(R.string.app_playback_choice_low), s(R.string.app_playback_choice_high)), substring = true))
        compose.reveal(compose.onNode(row)).performClick()
        waitFor(hasText(s(R.string.app_playback_autoplay)))
        back()
        waitFor(row)
        back()
        var finishing = false
        compose.activityRule.scenario.onActivity { finishing = it.isFinishing }
        assertTrue(finishing)
    }

    @Test @Config(qualifiers = PHONE)
    fun backLeadsOutPhone() = backLeadsOut()

    @Test @Config(qualifiers = TABLET)
    fun backLeadsOutTablet() = backLeadsOut()
}
