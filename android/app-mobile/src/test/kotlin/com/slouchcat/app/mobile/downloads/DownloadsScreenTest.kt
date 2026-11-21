package com.slouchcat.app.mobile.downloads

import android.content.Context
import androidx.compose.material3.SnackbarHostState
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isSelectable
import androidx.compose.ui.test.isToggleable
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.core.api.generated.models.DownloadQuality
import com.slouchcat.core.api.generated.models.DownloadRules
import com.slouchcat.core.api.generated.models.Language
import com.slouchcat.core.api.generated.models.OfflineProfile
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.offline.StoredProfile
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

private fun row(id: String, title: String, state: RowState, group: String? = null, episode: Int? = null) =
    DownloadRowUi(id, "item-$id", title, group, group?.let { "Sternfahrt" }, DownloadQuality.MEDIUM, 1_000_000_000, state, auto = false, poster = null, season = episode?.let { 1 }, episode = episode)

/** «Downloads» (rule 8): two widths, 200 % font, every control labelled and reachable, the sheets. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class DownloadsScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val recorder = object : DownloadsEvents {
        override fun onPause(id: String) { events += "pause:$id" }
        override fun onResume(id: String) { events += "resume:$id" }
        override fun onDeleteAsk(rows: List<DownloadRowUi>) { events += "delete:${rows.map { it.id }}" }
        override fun onDeleteConfirm() { events += "deleteConfirm" }
        override fun onDismissSheet() { events += "dismiss" }
        override fun onToggleAuto() { events += "auto" }
        override fun onToggleSettings() { events += "settings" }
        override fun onWifiOnly(on: Boolean) { events += "wifi:$on" }
        override fun onRule(change: (DownloadRules) -> DownloadRules) { events += "rule" }
        override fun onSaveRule() { events += "saveRule" }
        override fun onSwitchProfile() { events += "switch" }
        override fun onPickProfile(profile: StoredProfile) { events += "pick:${profile.id}" }
        override fun onPinDigit(digit: Char) { events += "digit:$digit" }
        override fun onPinDelete() { events += "pinDelete" }
        override fun onPinSubmit() { events += "pinSubmit" }
    }

    private val links = DownloadsLinks(play = { events += "play:${it.id}" }, item = { events += "item:$it" }, start = { events += "start" })

    private val rows = listOf(
        row("a", "Morgenrot", RowState.Done),
        row("b", "Lichtjahre", RowState.Fetching(40, 400_000_000, 1_000_000_000), group = "s", episode = 2),
        row("c", "Nordlicht", RowState.Queued),
    )

    private val storage = Storage(5_600_000_000, 36_000_000_000, 128_000_000_000)

    private val rule = DownloadRules(enabled = true, nextEpisodes = true, newPodcastEpisodes = false, wifiOnly = true, maxCount = 3, quality = DownloadQuality.MEDIUM)

    private fun show(state: DownloadsUiState) = compose.setContent {
        SlouchcatTheme { DownloadsScreen(state, recorder, links, SnackbarHostState()) }
    }

    private fun page() {
        show(DownloadsUiState(rows = rows, loaded = true, storage = storage, profileName = "Mara", settingsOpen = true, autoOpen = true, rule = RuleForm(rule)))
        compose.onNodeWithText(s(R.string.app_downloads_sectionActive)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_downloads_sectionWaiting)).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_downloads_sectionDone)).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText("Sternfahrt").performScrollTo().assertIsDisplayed()
        compose.onNode(hasText(s(R.string.offline_rules_enabled)) and isToggleable()).performScrollTo().assertIsDisplayed()
        compose.onNode(hasText(s(R.string.app_downloads_wifiOnly)) and isToggleable()).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasContentDescription(s(R.string.app_downloads_play, "Morgenrot")))).performClick()
        compose.reveal(compose.onNode(hasContentDescription(s(R.string.app_downloads_pauseLabel, "Lichtjahre")))).performClick()
        compose.reveal(compose.onNode(hasContentDescription(s(R.string.app_downloads_deleteLabel, "Nordlicht")))).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.offline_rules_save)) and hasClickAction())).performClick()
        assertEquals(listOf("play:a", "pause:b", "delete:[c]", "saveRule"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun pagePhone() = page()

    @Test @Config(qualifiers = TABLET)
    fun pageTablet() = page()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun pagePhoneLargeFont() = page()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun pageTabletLargeFont() = page()

    private fun empty() {
        show(DownloadsUiState(loaded = true, storage = storage))
        compose.onNodeWithText(s(R.string.app_downloads_emptyTitle)).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.app_downloads_toStart)) and hasClickAction())).performClick()
        assertEquals(listOf("start"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun emptyPhone() = empty()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun emptyTabletLargeFont() = empty()

    @Test @Config(qualifiers = PHONE)
    fun offlineShowsTheBannerAndKeepsPlayable() {
        val profiles = listOf(1L to "Mara", 2L to "Kim").map { (id, name) ->
            StoredProfile(id, "acc", OfflineProfile(id = id, name = name, language = Language.DE, isPrimary = id == 1L, hasPin = id == 2L), 0)
        }
        show(DownloadsUiState(rows = rows, loaded = true, offline = true, storage = storage, profileName = "Mara", offlineProfiles = profiles))
        compose.onNodeWithText(s(R.string.app_downloads_offlineBanner)).assertIsDisplayed()
        compose.reveal(compose.onNode(hasContentDescription(s(R.string.app_downloads_pauseLabel, "Lichtjahre")))).assertIsNotEnabled()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.app_downloads_viewing, "Mara")) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasContentDescription(s(R.string.app_downloads_play, "Morgenrot")))).performClick()
        assertEquals(listOf("switch", "play:a"), events)
    }

    private fun deleteSheet() {
        show(DownloadsUiState(rows = rows, loaded = true, sheet = DownloadsSheet.Delete, deleting = listOf(rows[0])))
        compose.onNodeWithText(s(R.string.app_downloads_deleteBody, "Morgenrot")).assertIsDisplayed()
        compose.onNode(hasText(s(R.string.app_downloads_deleteConfirm)) and hasClickAction()).performClick()
        assertEquals(listOf("deleteConfirm"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun deleteSheetPhone() = deleteSheet()

    @Test @Config(qualifiers = TABLET)
    fun deleteSheetTablet() = deleteSheet()

    private fun pinSheet() {
        val kim = StoredProfile(2, "acc", OfflineProfile(id = 2, name = "Kim", language = Language.DE, isPrimary = false, hasPin = true), 0)
        show(DownloadsUiState(loaded = true, sheet = DownloadsSheet.Pin, pin = OfflinePinUi(kim, digits = "1234", lockedSeconds = 30)))
        compose.onNodeWithText(s(R.string.app_downloads_pinTitle, "Kim")).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_downloads_pinLocked, "30")).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_downloads_pinWeak)).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNode(hasText("7") and hasClickAction()).performScrollTo().performClick()
        compose.onNode(hasContentDescription(s(R.string.app_downloads_pinDelete))).performScrollTo().performClick()
        compose.onNode(hasText(s(R.string.app_downloads_pinOpen)) and hasClickAction()).performScrollTo().performClick()
        assertEquals(listOf("digit:7", "pinDelete", "pinSubmit"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun pinSheetPhone() = pinSheet()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun pinSheetTabletLargeFont() = pinSheet()

    @Test @Config(qualifiers = PHONE)
    fun profileSheetListsTheProfilesWithDownloads() {
        val profiles = listOf(1L to "Mara", 2L to "Kim").map { (id, name) ->
            StoredProfile(id, "acc", OfflineProfile(id = id, name = name, language = Language.DE, isPrimary = id == 1L, hasPin = false), 0)
        }
        show(DownloadsUiState(loaded = true, profileName = "Mara", offlineProfiles = profiles, sheet = DownloadsSheet.Profiles))
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNode(hasText("Kim") and isSelectable()).performClick()
        assertTrue(events.contains("pick:2"))
    }
}
