package com.slouchcat.app.mobile.playback

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.common.RowGroup
import com.slouchcat.app.mobile.player.languageName
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.ListRow
import com.slouchcat.core.design.SectionTitle
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetHeadIcon
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.SwitchRow
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.player.QualityChoice
import com.slouchcat.core.strings.R
import java.util.Locale

/** The name of a quality of the device's settings (mockup `CHOICES.quality`). */
@Composable
fun qualityName(choice: QualityChoice): String = stringResource(
    when (choice) {
        QualityChoice.Auto -> R.string.app_playback_choice_auto
        QualityChoice.High -> R.string.app_playback_choice_high
        QualityChoice.Medium -> R.string.app_playback_choice_medium
        QualityChoice.Low -> R.string.app_playback_choice_low
        QualityChoice.Saver -> R.string.app_playback_choice_saver
    },
)

@Composable
private fun modeName(mode: SubtitleMode): String = stringResource(
    when (mode) {
        SubtitleMode.OFF -> R.string.settings_playback_subtitleMode_off
        SubtitleMode.FOREIGN -> R.string.settings_playback_subtitleMode_foreign
        SubtitleMode.ALWAYS -> R.string.settings_playback_subtitleMode_always
        else -> R.string.settings_playback_subtitleMode_forced
    },
)

@Composable
private fun modeHint(mode: SubtitleMode): String = stringResource(
    when (mode) {
        SubtitleMode.OFF -> R.string.settings_playback_subtitleMode_offHint
        SubtitleMode.FOREIGN -> R.string.settings_playback_subtitleMode_foreignHint
        SubtitleMode.ALWAYS -> R.string.settings_playback_subtitleMode_alwaysHint
        else -> R.string.settings_playback_subtitleMode_forcedHint
    },
)

/** The languages the sheets offer: the frequent ones first, then every ISO 639-1 language by name. */
private val FREQUENT = listOf("de", "en", "fr", "it", "es")

@Composable
private fun languageChoices(): List<Pair<String, String>> {
    val locale = LocalConfiguration.current.locales[0]
    return remember(locale) {
        val all = Locale.getISOLanguages().filter { it.length == 2 }
        fun name(code: String) = Locale.forLanguageTag(code).getDisplayLanguage(locale).replaceFirstChar { it.titlecase(locale) }
        val frequent = FREQUENT.map { it to name(it) }
        frequent + all.filter { it !in FREQUENT }.map { it to name(it) }.filter { it.second != it.first }.sortedBy { it.second }
    }
}

/**
 * «Wiedergabe» (mockup `/settings/playback`, reference photo handy 09): Qualität (mobile data,
 * Wi-Fi), Sprache (audio language, subtitles, subtitle language, and the web page's «Serienmerker
 * zurücksetzen»), Ablauf (next episode, skip intro), Downloads (only on Wi-Fi). Every choice opens
 * a sheet with radio rows; a switch saves at once.
 */
@Composable
fun PlaybackSettingsScreen(state: PlaybackSettingsUiState, events: PlaybackSettingsEvents, snackbar: SnackbarHostState, profileName: String?, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Box(modifier.fillMaxSize().background(c.bg)) {
        Column(Modifier.fillMaxSize()) {
            TopBar(stringResource(R.string.app_playback_title), navigationLabel = stringResource(R.string.common_back), onNavigation = events::onBack)
            Column(
                Modifier
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = gutter(), vertical = 8.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Column(Modifier.widthIn(max = 728.dp).fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    val device = state.device
                    SectionTitle(stringResource(R.string.app_playback_quality), inset = false)
                    RowGroup(
                        rows = listOf(
                            { Pick(SlouchcatIcon.Speed, stringResource(R.string.app_playback_mobile), qualityName(device.qualityMobile)) { events.onSheet(PlaybackSheet.QualityMobile) } },
                            { Pick(SlouchcatIcon.Wifi, stringResource(R.string.app_playback_wifi), qualityName(device.qualityWifi)) { events.onSheet(PlaybackSheet.QualityWifi) } },
                        ),
                    )
                    SectionTitle(stringResource(R.string.app_playback_language), inset = false)
                    LoadableContent(state.prefs, onRetry = events::onRetry) { prefs ->
                        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            LanguageGroup(prefs, state, events)
                            if (profileName != null) Text(stringResource(R.string.app_playback_profileNote, profileName), style = SlouchcatType.caption, color = c.tx3)
                        }
                    }
                    SectionTitle(stringResource(R.string.app_playback_flow), inset = false)
                    RowGroup(
                        rows = listOf(
                            { SwitchRow(stringResource(R.string.app_playback_autoplay), device.autoplayNext, events::onAutoplay, subtitle = stringResource(R.string.app_playback_autoplayHint)) },
                            { SwitchRow(stringResource(R.string.app_playback_skipIntro), device.offerSkipIntro, events::onSkipIntro, subtitle = stringResource(R.string.app_playback_skipIntroHint)) },
                        ),
                    )
                    SectionTitle(stringResource(R.string.app_playback_downloads), inset = false)
                    RowGroup(
                        rows = listOf(
                            {
                                SwitchRow(
                                    stringResource(R.string.app_playback_wifiOnly),
                                    device.downloadsWifiOnly,
                                    events::onWifiOnly,
                                    subtitle = stringResource(if (device.downloadsWifiOnly) R.string.app_playback_wifiOnlyOn else R.string.app_playback_wifiOnlyOff),
                                )
                            },
                        ),
                    )
                    Text(stringResource(R.string.app_playback_deviceNote), style = SlouchcatType.caption, color = c.tx2, modifier = Modifier.padding(top = 4.dp, bottom = 24.dp))
                }
            }
        }
        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter).padding(16.dp))
    }
    state.sheet?.let { ChoiceSheet(it, state, events) }
}

@Composable
private fun LanguageGroup(prefs: LanguagePrefs, state: PlaybackSettingsUiState, events: PlaybackSettingsEvents) {
    val audio = prefs.audio.firstOrNull()?.let { languageName(it) ?: it } ?: stringResource(R.string.app_playback_original)
    val subtitleLanguage = prefs.subtitles.firstOrNull()?.let { languageName(it) ?: it } ?: stringResource(R.string.app_playback_noLanguage)
    RowGroup(
        rows = listOf(
            { Pick(SlouchcatIcon.Audio, stringResource(R.string.app_playback_audioLanguage), audio) { events.onSheet(PlaybackSheet.AudioLanguage) } },
            { Pick(SlouchcatIcon.Subs, stringResource(R.string.settings_playback_subtitleMode_label), modeName(prefs.mode)) { events.onSheet(PlaybackSheet.SubtitleMode) } },
            { Pick(SlouchcatIcon.Globe, stringResource(R.string.app_playback_subtitleLanguage), subtitleLanguage) { events.onSheet(PlaybackSheet.SubtitleLanguage) } },
            {
                ListRow(
                    stringResource(R.string.settings_playback_series_reset),
                    icon = SlouchcatIcon.Refresh,
                    subtitle = stringResource(R.string.settings_playback_series_body),
                    onClick = if (state.resetting) null else ({ events.onSheet(PlaybackSheet.ResetSeries) }),
                )
            },
        ),
    )
}

/** `pick(...)`: icon, title, the value at the end and the arrow (mockup `.su-val`). */
@Composable
private fun Pick(icon: SlouchcatIcon, title: String, value: String, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    ListRow(title, icon = icon, onClick = onClick, end = {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(value, style = SlouchcatType.label.copy(fontSize = SlouchcatType.bodySmall.fontSize), color = c.tx2, maxLines = 2, modifier = Modifier.widthIn(max = 160.dp))
            SlouchcatIconImage(SlouchcatIcon.Right, contentDescription = null, size = 18.dp, tint = c.tx3)
        }
    })
}

@Composable
private fun ChoiceSheet(sheet: PlaybackSheet, state: PlaybackSettingsUiState, events: PlaybackSettingsEvents) {
    val close = { events.onSheet(null) }
    val prefs = (state.prefs as? com.slouchcat.core.media.Loadable.Ready)?.value
    when (sheet) {
        PlaybackSheet.ResetSeries -> SlouchcatSheet(stringResource(R.string.settings_playback_series_confirmTitle), onDismiss = close) {
            SheetHeadIcon(SlouchcatIcon.Refresh, warning = true)
            SheetTitle(stringResource(R.string.settings_playback_series_confirmTitle), centered = true)
            SheetText(stringResource(R.string.settings_playback_series_confirmBody), centered = true)
            SheetActions {
                SlouchcatButton(stringResource(R.string.common_cancel), onClick = close, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
                SlouchcatButton(stringResource(R.string.settings_playback_series_confirm), onClick = events::onResetSeries, kind = ButtonKind.Danger, modifier = Modifier.weight(1f))
            }
        }
        PlaybackSheet.QualityMobile, PlaybackSheet.QualityWifi -> {
            val mobile = sheet == PlaybackSheet.QualityMobile
            val title = stringResource(if (mobile) R.string.app_playback_mobileTitle else R.string.app_playback_wifiTitle)
            val current = if (mobile) state.device.qualityMobile else state.device.qualityWifi
            val choices = if (mobile) QualityChoice.entries else QualityChoice.entries - QualityChoice.Saver
            Choices(title, stringResource(if (mobile) R.string.app_playback_mobileHint else R.string.app_playback_wifiHint), close) {
                choices.forEach { choice ->
                    Option(qualityName(choice), null, choice == current) { if (mobile) events.onQualityMobile(choice) else events.onQualityWifi(choice) }
                }
            }
        }
        PlaybackSheet.AudioLanguage -> prefs?.let { p ->
            Choices(stringResource(R.string.app_playback_audioLanguage), stringResource(R.string.settings_playback_audio_hint), close) {
                Option(stringResource(R.string.app_playback_original), null, p.audio.isEmpty()) { events.onAudioLanguage(null) }
                languageChoices().forEach { (code, name) -> Option(name, null, p.audio.firstOrNull() == code) { events.onAudioLanguage(code) } }
            }
        }
        PlaybackSheet.SubtitleMode -> prefs?.let { p ->
            Choices(stringResource(R.string.settings_playback_subtitleMode_label), null, close) {
                listOf(SubtitleMode.OFF, SubtitleMode.FORCED, SubtitleMode.FOREIGN, SubtitleMode.ALWAYS).forEach { mode ->
                    Option(modeName(mode), modeHint(mode), p.mode == mode) { events.onSubtitleMode(mode) }
                }
            }
        }
        PlaybackSheet.SubtitleLanguage -> prefs?.let { p ->
            Choices(stringResource(R.string.app_playback_subtitleLanguage), stringResource(R.string.settings_playback_subtitles_hint), close) {
                Option(stringResource(R.string.app_playback_noLanguage), null, p.subtitles.isEmpty()) { events.onSubtitleLanguage(null) }
                languageChoices().forEach { (code, name) -> Option(name, null, p.subtitles.firstOrNull() == code) { events.onSubtitleLanguage(code) } }
            }
        }
    }
}

/** `suChoice`: the title, radio rows, the hint and «Abbrechen». */
@Composable
private fun Choices(title: String, hint: String?, onClose: () -> Unit, content: @Composable () -> Unit) {
    SlouchcatSheet(title, onDismiss = onClose) {
        SheetTitle(title)
        Column(Modifier.fillMaxWidth().selectableGroup()) { content() }
        if (hint != null) Text(hint, style = SlouchcatType.caption, color = SlouchcatTheme.colors.tx3, modifier = Modifier.padding(horizontal = gutter(), vertical = 8.dp))
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = onClose, kind = ButtonKind.Ghost, block = true, modifier = Modifier.weight(1f))
        }
    }
}

/** `.su-opt`: a radio row; the ring is filled for the chosen one. */
@Composable
private fun Option(label: String, detail: String?, selected: Boolean, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    Row(
        Modifier
            .fillMaxWidth()
            .defaultMinSize(minHeight = 56.dp)
            .selectable(selected = selected, role = Role.RadioButton, onClick = onClick)
            .padding(horizontal = gutter(), vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Column(Modifier.weight(1f)) {
            Text(label, style = SlouchcatType.rowTitle, color = c.tx)
            if (detail != null) Text(detail, style = SlouchcatType.caption, color = c.tx3)
        }
        Radio(selected)
    }
}

@Composable
private fun Radio(selected: Boolean) {
    val c = SlouchcatTheme.colors
    Box(
        Modifier
            .size(22.dp)
            .background(if (selected) c.accent else c.s2, shape = androidx.compose.foundation.shape.CircleShape),
        contentAlignment = Alignment.Center,
    ) {
        Box(Modifier.size(if (selected) 8.dp else 16.dp).background(if (selected) c.onAccent else c.bg, shape = androidx.compose.foundation.shape.CircleShape))
    }
}
