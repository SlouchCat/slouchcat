package com.slouchcat.app.mobile.audio

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.player.timecode
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.gutter
import com.slouchcat.core.player.AudioRules
import com.slouchcat.core.player.SleepChoice
import com.slouchcat.core.player.SleepState
import com.slouchcat.core.strings.R

/** «Tempo» (mockup photo 60): the presets of the web player and «Langsamer», «Schneller» in tenths. */
@Composable
fun SpeedSheet(speed: Float, events: ListenEvents, onClose: () -> Unit) {
    val title = stringResource(R.string.app_audio_speedTitle)
    SlouchcatSheet(title, onDismiss = onClose) {
        SheetTitle(title)
        Column(Modifier.fillMaxWidth().selectableGroup()) {
            AudioRules.SPEEDS.forEach { preset ->
                Choice(rateText(preset), selected = preset == speed) {
                    events.onSpeed(preset)
                    onClose()
                }
            }
        }
        SheetActions {
            SlouchcatButton(
                stringResource(R.string.player_audio_slower),
                onClick = { events.onSpeed(AudioRules.clampSpeed(speed - AudioRules.SPEED_STEP)) },
                kind = ButtonKind.Ghost,
                enabled = speed > AudioRules.MIN_SPEED,
                modifier = Modifier.weight(1f),
            )
            SlouchcatButton(
                stringResource(R.string.player_audio_faster),
                onClick = { events.onSpeed(AudioRules.clampSpeed(speed + AudioRules.SPEED_STEP)) },
                kind = ButtonKind.Ghost,
                enabled = speed < AudioRules.MAX_SPEED,
                modifier = Modifier.weight(1f),
            )
        }
    }
}

/** «Schlaftimer» (mockup photo 61, web `SleepMenu`): off, 15 to 60 minutes, the end of the chapter or title. */
@Composable
fun SleepSheet(sleep: SleepState?, hasChapters: Boolean, events: ListenEvents, onClose: () -> Unit) {
    val title = stringResource(R.string.app_audio_sleepTitle)
    SlouchcatSheet(title, onDismiss = onClose) {
        SheetTitle(title)
        Column(Modifier.fillMaxWidth().selectableGroup()) {
            fun choose(choice: SleepChoice) {
                events.onSleep(choice)
                onClose()
            }
            Choice(stringResource(R.string.player_audio_sleep_off), selected = sleep == null) { choose(SleepChoice.Off) }
            SleepChoice.MINUTES.forEach { minutes ->
                val running = (sleep?.choice as? SleepChoice.Minutes)?.minutes == minutes
                Choice(sleepMinutes(minutes), selected = running, detail = if (running) stringResource(R.string.app_audio_sleepLeft, timecode(sleep!!.leftMs)) else null) {
                    choose(SleepChoice.Minutes(minutes))
                }
            }
            val end = sleep?.choice == SleepChoice.EndOfChapter
            Choice(
                stringResource(if (hasChapters) R.string.player_audio_sleep_chapter else R.string.player_audio_sleep_end),
                selected = end,
                detail = if (end) stringResource(R.string.app_audio_sleepLeft, timecode(sleep!!.leftMs)) else null,
            ) { choose(SleepChoice.EndOfChapter) }
        }
    }
}

/** «Kapitel» (mockup photo 62, web `player.chapters`): each with its start; the current one marked. */
@Composable
fun ChaptersSheet(chapters: List<Chapter>, positionMs: Long, events: ListenEvents, onClose: () -> Unit) {
    val title = stringResource(R.string.player_chapters_button)
    val current = AudioRules.chapterIndex(chapters, positionMs)
    SlouchcatSheet(title, onDismiss = onClose) {
        SheetTitle(title)
        Column(Modifier.fillMaxWidth().selectableGroup()) {
            chapters.forEachIndexed { index, chapter ->
                Choice(chapterTitle(chapter, index), selected = index == current, detail = timecode(chapter.startMs)) {
                    events.onChapter(chapter)
                    onClose()
                }
            }
        }
    }
}

/** `.li.bx-opt`: a radio row, the chosen one in the accent colour with a check mark. */
@Composable
internal fun Choice(label: String, selected: Boolean, detail: String? = null, onClick: () -> Unit) {
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
            Text(label, style = SlouchcatType.rowTitle, color = if (selected) c.accent else c.tx)
            if (detail != null) Text(detail, style = SlouchcatType.caption, color = c.tx3)
        }
        if (selected) SlouchcatIconImage(SlouchcatIcon.Check, contentDescription = null, size = 22.dp, tint = c.accent)
    }
}
