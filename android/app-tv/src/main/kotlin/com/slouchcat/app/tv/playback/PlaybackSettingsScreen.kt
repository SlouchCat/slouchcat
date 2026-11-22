package com.slouchcat.app.tv.playback

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.toggleableState
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.LocalContentColor
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.player.PCM
import com.slouchcat.app.tv.player.channelLayout
import com.slouchcat.app.tv.player.reported
import com.slouchcat.app.tv.player.soundName
import com.slouchcat.app.tv.shell.tvPage
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvMenuItem
import com.slouchcat.app.tv.ui.TvSideMenu
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatTokens
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.player.AudioSetting
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.QualityChoice
import com.slouchcat.core.strings.R
import java.util.Locale

/** The measures of the page (tokens `tv.size.playback`). */
private val B = SlouchcatTokens.Tv.Size.Playback

interface PlaybackSettingsEvents {
    fun onRetry()
    fun onMenu(menu: PlaybackMenu?)
    fun onQuality(choice: QualityChoice)
    fun onAudioLanguage(code: String?)
    fun onSubtitleMode(mode: SubtitleMode)
    fun onSubtitleLanguage(code: String?)
    fun onAutoplay(on: Boolean)
    fun onSkipIntro(on: Boolean)
    fun onSoundAuto()
    fun onSoundStereo()
    fun onSoundManual()
    fun onFormat(format: String, on: Boolean)
    fun onChannels(channels: Int)
}

/** The name of a quality of the device's settings. */
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

/** The name of a sound setting («Automatisch», «Immer Stereo», «Formate von Hand»). */
@Composable
fun soundSettingName(setting: AudioSetting): String = stringResource(
    when (setting) {
        AudioSetting.Auto -> R.string.tv_playback_soundAuto
        AudioSetting.Stereo -> R.string.tv_playback_soundStereo
        is AudioSetting.Manual -> R.string.tv_playback_soundManual
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

/** The languages the menus offer: the frequent ones first, then every ISO 639-1 language by name. */
private val FREQUENT = listOf("de", "en", "fr", "it", "es")

@Composable
private fun languageChoices(): List<Pair<String, String>> {
    val locale = LocalConfiguration.current.locales[0]
    return remember(locale) {
        fun name(code: String) = Locale.forLanguageTag(code).getDisplayLanguage(locale).replaceFirstChar { it.titlecase(locale) }
        val all = Locale.getISOLanguages().filter { it.length == 2 }
        FREQUENT.map { it to name(it) } + all.filter { it !in FREQUENT }.map { it to name(it) }.filter { it.second != it.first }.sortedBy { it.second }
    }
}

@Composable
private fun languageName(code: String): String {
    val locale = LocalConfiguration.current.locales[0]
    return Locale.forLanguageTag(code).getDisplayLanguage(locale).replaceFirstChar { it.titlecase(locale) }.ifBlank { code }
}

/** The channel counts of «Von Hand». */
private val CHANNELS = listOf(2, 6, 8)

/**
 * «Wiedergabe» (mockup `/settings/playback`, photo `38-settings-playback`): «Bild und Sprache»
 * (quality, audio language, subtitles, subtitle language), «Serien» (next episode, skip intro),
 * «Ton» (automatic, always stereo, formats by hand, with what the TV found). A row opens a menu
 * from the right; a switch saves at once.
 */
@Composable
fun PlaybackSettingsScreen(state: PlaybackSettingsUiState, events: PlaybackSettingsEvents, profileName: String?) {
    val focus = rememberFocusMemory()
    val device = state.device
    Column(Modifier.fillMaxSize().tvPage().verticalScroll(rememberScrollState()).padding(bottom = B.Bottom)) {
        Text(stringResource(R.string.app_playback_title), style = TvType.h1, modifier = Modifier.semantics { heading() })
        Column(Modifier.widthIn(max = B.Width)) {
            Section(stringResource(R.string.tv_playback_pictureAndLanguage))
            ValueRow(stringResource(R.string.app_playback_quality), qualityName(device.qualityWifi), { events.onMenu(PlaybackMenu.Quality) }, Modifier.focusKey(focus, QUALITY))
            LoadableContent(state.prefs, onRetry = events::onRetry) { prefs ->
                Column {
                    val audio = prefs.audio.firstOrNull()?.let { languageName(it) } ?: stringResource(R.string.app_playback_original)
                    val subtitles = prefs.subtitles.firstOrNull()?.let { languageName(it) } ?: stringResource(R.string.app_playback_noLanguage)
                    ValueRow(stringResource(R.string.app_playback_audioLanguage), audio, { events.onMenu(PlaybackMenu.AudioLanguage) }, Modifier.focusKey(focus, "audio-language"))
                    ValueRow(stringResource(R.string.settings_playback_subtitleMode_label), modeName(prefs.mode), { events.onMenu(PlaybackMenu.SubtitleMode) }, Modifier.focusKey(focus, "subtitle-mode"))
                    ValueRow(stringResource(R.string.app_playback_subtitleLanguage), subtitles, { events.onMenu(PlaybackMenu.SubtitleLanguage) }, Modifier.focusKey(focus, "subtitle-language"))
                    if (profileName != null) Help(stringResource(R.string.app_playback_profileNote, profileName))
                }
            }
            Section(stringResource(R.string.tv_playback_series))
            SwitchRow(stringResource(R.string.app_playback_autoplay), stringResource(R.string.app_playback_autoplayHint), device.autoplayNext, events::onAutoplay, Modifier.focusKey(focus, "autoplay"))
            SwitchRow(stringResource(R.string.app_playback_skipIntro), stringResource(R.string.app_playback_skipIntroHint), device.offerSkipIntro, events::onSkipIntro, Modifier.focusKey(focus, "skip-intro"))

            Section(stringResource(R.string.tv_playback_sound))
            Help(stringResource(R.string.tv_playback_soundHelp))
            val audio = device.audio
            Column(Modifier.selectableGroup()) {
                RadioRow(stringResource(R.string.tv_playback_soundAuto), stringResource(R.string.tv_playback_soundAutoHint), audio is AudioSetting.Auto, events::onSoundAuto, Modifier.focusKey(focus, SOUND_AUTO))
                RadioRow(stringResource(R.string.tv_playback_soundStereo), stringResource(R.string.tv_playback_soundStereoHint), audio is AudioSetting.Stereo, events::onSoundStereo, Modifier.focusKey(focus, "sound-stereo"))
                RadioRow(stringResource(R.string.tv_playback_soundManual), stringResource(R.string.tv_playback_soundManualHint), audio is AudioSetting.Manual, events::onSoundManual, Modifier.focusKey(focus, "sound-manual"))
            }
            Detected(state)
            if (audio is AudioSetting.Manual) {
                Section(stringResource(R.string.tv_playback_formats))
                CapabilityRules.PASSTHROUGH.forEach { format ->
                    SwitchRow(soundName(format), null, format in audio.formats, { on -> events.onFormat(format, on) }, Modifier.focusKey(focus, "format-$format"))
                }
                ValueRow(stringResource(R.string.tv_playback_channels), channelLayout(audio.channels), { events.onMenu(PlaybackMenu.Channels) }, Modifier.focusKey(focus, "channels"))
            }
            Help(stringResource(R.string.tv_playback_deviceNote))
        }
    }
    focus.StartFocus(QUALITY)
    state.menu?.let { Menu(it, state, events) { focus.restore() } }
}

/** `.md-detect`: what the TV found at its output and what goes out with the chosen setting. */
@Composable
private fun Detected(state: PlaybackSettingsUiState) {
    val c = SlouchcatTheme.colors
    val found = state.detected ?: return
    val separator = stringResource(R.string.format_listSeparator)
    val formats = { list: List<String> -> list.joinToString(separator) { soundName(it) } }
    val title = if (found.surround) {
        stringResource(R.string.tv_playback_detected, formats(found.passthrough).ifEmpty { soundName(PCM) }, channelLayout(found.channels))
    } else {
        stringResource(R.string.tv_playback_detectedStereo)
    }
    val setting = state.device.audio
    val out = reported(setting, found)
    val detail = when (setting) {
        AudioSetting.Auto -> stringResource(if (found.passthrough.isNotEmpty()) R.string.tv_playback_passedAuto else R.string.tv_playback_passedNone)
        AudioSetting.Stereo -> stringResource(R.string.tv_playback_passedStereo)
        is AudioSetting.Manual -> stringResource(R.string.tv_playback_passedManual, formats(out.passthrough).ifEmpty { soundName(PCM) }, channelLayout(out.channels))
    }
    val shape = RoundedCornerShape(TvDimens.radiusPanel)
    Row(
        Modifier.padding(top = TvDimens.l).fillMaxWidth().clip(shape).background(c.s2).padding(B.DetectPad),
        horizontalArrangement = Arrangement.spacedBy(TvDimens.l),
    ) {
        TvIcon(SlouchcatIcon.Audio, TvDimens.Menu.Icon, tint = c.ok)
        Column {
            Text(title, style = TvType.bodyStrong)
            Text(detail, style = TvType.small, color = c.tx2)
        }
    }
}

@Composable
private fun Section(title: String) =
    Text(title, style = TvType.largeStrong, color = SlouchcatTheme.colors.tx2, modifier = Modifier.padding(top = B.SectionTop, bottom = B.SectionBottom).semantics { heading() })

@Composable
private fun Help(text: String) = Text(text, style = TvType.small, color = SlouchcatTheme.colors.tx3, modifier = Modifier.padding(top = TvDimens.m))

/** The colours of a row of the page (`.md-srow`, `.md-radio`): light in focus with a frame. */
@Composable
private fun rowColors() = SlouchcatTheme.colors.let { c ->
    ClickableSurfaceDefaults.colors(
        containerColor = c.s1,
        contentColor = c.tx,
        focusedContainerColor = c.tx,
        focusedContentColor = c.bg,
        pressedContainerColor = c.tx,
        pressedContentColor = c.bg,
    )
}

/** `.md-srow`: a label, the value and a chevron; opens a menu. */
@Composable
private fun ValueRow(label: String, value: String, onClick: () -> Unit, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusField)
    Surface(
        onClick = onClick,
        modifier = modifier.padding(top = B.RowGap).fillMaxWidth().heightIn(min = B.Row),
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = rowColors(),
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, c.line), shape = shape),
            focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = shape),
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
    ) {
        val focused = LocalContentColor.current == c.bg
        Row(Modifier.fillMaxWidth().align(Alignment.CenterStart).padding(horizontal = B.RowPad), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
            Text(label, style = TvType.body, modifier = Modifier.weight(1f))
            Text(value, style = TvType.bodyStrong, color = if (focused) c.bg else c.tx2, maxLines = 1, overflow = TextOverflow.Ellipsis)
            TvIcon(SlouchcatIcon.Right, TvDimens.Button.Icon, tint = if (focused) c.bg else c.tx3)
        }
    }
}

/** `.tsw.md-sw2`: a switch with a second line; told as a switch. */
@Composable
private fun SwitchRow(title: String, hint: String?, checked: Boolean, onChange: (Boolean) -> Unit, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusField)
    Surface(
        onClick = { onChange(!checked) },
        modifier = modifier
            .padding(top = B.RowGap)
            .fillMaxWidth()
            .heightIn(min = B.Row)
            .semantics(mergeDescendants = true) {
                role = Role.Switch
                toggleableState = ToggleableState(checked)
            },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = rowColors(),
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, c.line), shape = shape),
            focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = shape),
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
    ) {
        val focused = LocalContentColor.current == c.bg
        Row(Modifier.fillMaxWidth().align(Alignment.CenterStart).padding(horizontal = B.RowPad, vertical = TvDimens.s), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(title, style = TvType.body)
                if (hint != null) Text(hint, style = TvType.small, color = if (focused) c.bg else c.tx3)
            }
            val track = RoundedCornerShape(TvDimens.Switch.TrackHeight / 2)
            Box(
                Modifier
                    .size(TvDimens.Switch.TrackWidth, TvDimens.Switch.TrackHeight)
                    .clip(track)
                    .background(if (checked) c.accent else Color.Transparent)
                    .border(BorderStroke(TvDimens.borderThick, if (checked) c.accent else c.ctl), track)
                    .padding(horizontal = TvDimens.Switch.ThumbInset),
                contentAlignment = if (checked) Alignment.CenterEnd else Alignment.CenterStart,
            ) {
                Box(Modifier.size(TvDimens.Switch.Thumb).clip(CircleShape).background(if (checked) c.onAccent else c.tx3))
            }
        }
    }
}

/** `.md-radio`: a choice of a group with a dot; the chosen one with a frame of the accent. */
@Composable
private fun RadioRow(title: String, hint: String, selected: Boolean, onClick: () -> Unit, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusField)
    Surface(
        onClick = onClick,
        modifier = modifier
            .padding(top = B.RowGap)
            .fillMaxWidth()
            .heightIn(min = B.Radio)
            .semantics(mergeDescendants = true) {
                role = Role.RadioButton
                this.selected = selected
            },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = rowColors(),
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, if (selected) c.accent else c.line), shape = shape),
            focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = shape),
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
    ) {
        val focused = LocalContentColor.current == c.bg
        Row(Modifier.fillMaxWidth().align(Alignment.CenterStart).padding(horizontal = B.RowPad, vertical = TvDimens.s), horizontalArrangement = Arrangement.spacedBy(TvDimens.xl), verticalAlignment = Alignment.CenterVertically) {
            val ring = if (focused) c.bg else if (selected) c.accent else c.ctl
            Box(Modifier.size(B.RadioDot).border(BorderStroke(B.RadioRing, ring), CircleShape), contentAlignment = Alignment.Center) {
                if (selected) Box(Modifier.size(B.RadioDotInner).clip(CircleShape).background(if (focused) c.bg else c.accent))
            }
            Column(Modifier.weight(1f)) {
                Text(title, style = TvType.bodyStrong)
                Text(hint, style = TvType.small, color = if (focused) c.bg else c.tx3)
            }
        }
    }
}

/** The menus of the page: a list of choices from the right, the focus on the current one. */
@Composable
private fun Menu(menu: PlaybackMenu, state: PlaybackSettingsUiState, events: PlaybackSettingsEvents, restore: () -> Unit) {
    val first = remember(menu) { FocusRequester() }
    val close = {
        events.onMenu(null)
        restore()
    }
    @Composable
    fun Choice(title: String, subtitle: String?, selected: Boolean, onClick: () -> Unit) {
        TvMenuItem(title, onClick = {
            onClick()
            restore()
        }, subtitle = subtitle, selected = selected, modifier = if (selected) Modifier.focusRequester(first) else Modifier)
    }
    val prefs = (state.prefs as? Loadable.Ready)?.value
    when (menu) {
        PlaybackMenu.Quality -> TvSideMenu(stringResource(R.string.app_playback_quality), onDismiss = close, initialFocus = first) {
            QualityChoice.entries.forEach { choice -> Choice(qualityName(choice), null, choice == state.device.qualityWifi) { events.onQuality(choice) } }
            Help(stringResource(R.string.tv_playback_qualityHint))
        }
        PlaybackMenu.AudioLanguage -> if (prefs != null) {
            TvSideMenu(stringResource(R.string.app_playback_audioLanguage), onDismiss = close, initialFocus = first) {
                Choice(stringResource(R.string.app_playback_original), null, prefs.audio.isEmpty()) { events.onAudioLanguage(null) }
                languageChoices().forEach { (code, name) -> Choice(name, null, prefs.audio.firstOrNull() == code) { events.onAudioLanguage(code) } }
            }
        }
        PlaybackMenu.SubtitleMode -> if (prefs != null) {
            TvSideMenu(stringResource(R.string.settings_playback_subtitleMode_label), onDismiss = close, initialFocus = first) {
                listOf(SubtitleMode.OFF, SubtitleMode.FORCED, SubtitleMode.FOREIGN, SubtitleMode.ALWAYS).forEach { mode ->
                    Choice(modeName(mode), modeHint(mode), prefs.mode == mode) { events.onSubtitleMode(mode) }
                }
            }
        }
        PlaybackMenu.SubtitleLanguage -> if (prefs != null) {
            TvSideMenu(stringResource(R.string.app_playback_subtitleLanguage), onDismiss = close, initialFocus = first) {
                Choice(stringResource(R.string.app_playback_noLanguage), null, prefs.subtitles.isEmpty()) { events.onSubtitleLanguage(null) }
                languageChoices().forEach { (code, name) -> Choice(name, null, prefs.subtitles.firstOrNull() == code) { events.onSubtitleLanguage(code) } }
            }
        }
        PlaybackMenu.Channels -> TvSideMenu(stringResource(R.string.tv_playback_channels), onDismiss = close, initialFocus = first) {
            val now = (state.device.audio as? AudioSetting.Manual)?.channels
            CHANNELS.forEach { n -> Choice(channelLayout(n), null, now == n) { events.onChannels(n) } }
        }
    }
}

private const val QUALITY = "quality"
private const val SOUND_AUTO = "sound-auto"
