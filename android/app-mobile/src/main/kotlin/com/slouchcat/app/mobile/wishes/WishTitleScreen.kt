package com.slouchcat.app.mobile.wishes

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CheckboxDefaults
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.text
import com.slouchcat.app.mobile.media.DetailState
import com.slouchcat.app.mobile.media.FskBadge
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.media.RoundIcon
import com.slouchcat.app.mobile.media.pluralCount
import com.slouchcat.app.mobile.media.runtimeText
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.ExternalTitle
import com.slouchcat.core.api.generated.models.WishQuota
import com.slouchcat.core.api.generated.models.WishSeasonOffer
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.browse.wishes.WishSheet
import com.slouchcat.core.browse.wishes.WishTitleUiState
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SectionTitle
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WishStep
import com.slouchcat.core.media.wishStep
import com.slouchcat.core.strings.R

/** What the wish page does on a tap. */
interface WishTitleEvents {
    fun onBack()
    fun onRetry()
    fun onRequest()
    fun onRetrySeasons()
    fun onSeason(number: Int)
    fun onAllSeasons()
    fun onSubmit()
    fun onWithdraw()
    fun onConfirmWithdraw()
    fun onDismiss()

    /** «Meine Wünsche»; `null` when the page is opened from there. */
    val onMine: (() -> Unit)?
}

/**
 * The page of a title outside the library (mockup `/wish/:i`, reference photos handy 41, 42, 64,
 * 65): the backdrop, the poster with «Nicht in der Bibliothek», title, year, kind, runtime or seasons
 * and age; the description; with an own wish its four steps, its state, «Zurückziehen» and «Meine
 * Wünsche»; without one «Anfordern» and the limit, or why the profile cannot request.
 */
@Composable
fun WishTitleScreen(state: WishTitleUiState, images: ServerImages, events: WishTitleEvents, snackbar: SnackbarHostState, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Box(modifier.fillMaxSize().background(c.bg)) {
        val title = (state.title as? Loadable.Ready)?.value
        if (title == null) {
            DetailState(state.title, stringResource(R.string.titles_notFound_title), stringResource(R.string.titles_notFound_body), SlouchcatIcon.Wish, events::onRetry)
        } else {
            Page(title, images, events)
        }
        RoundIcon(SlouchcatIcon.Back, stringResource(R.string.common_back), onClick = events::onBack, modifier = Modifier.padding(start = 8.dp, top = 8.dp))
        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter))
        val shown = title
        if (shown != null) {
            when (state.sheet) {
                WishSheet.Request -> RequestSheet(shown, state, images, events)
                WishSheet.Withdraw -> WithdrawSheet(shown, state, events)
                null -> Unit
            }
        }
    }
}

@Composable
private fun Page(title: ExternalTitle, images: ServerImages, events: WishTitleEvents) {
    val c = SlouchcatTheme.colors
    val wide = isWideLayout()
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
        Box(Modifier.fillMaxWidth().aspectRatio(if (wide) 3.2f else 16f / 10f)) {
            MediaImage(images.externalBackdrops(title.images), title.title, Modifier.fillMaxSize(), placeholder = false)
            Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(c.scrim.copy(alpha = 0f), c.bg))))
        }
        Column(
            Modifier
                .padding(horizontal = gutter())
                .offset(y = (-56).dp)
                .then(if (wide) Modifier.widthIn(max = 900.dp) else Modifier),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                MediaImage(images.externalPosters(title.images), title.title, Modifier.width(if (wide) 160.dp else 112.dp).aspectRatio(2f / 3f).clip(RoundedCornerShape(10.dp)))
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    StateTag(stringResource(R.string.titles_missing))
                    Text(title.title, style = SlouchcatType.pageTitle, color = c.tx, modifier = Modifier.semantics { heading() })
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text(meta(title), style = SlouchcatType.caption, color = c.tx2)
                        title.minAge?.let { FskBadge(it) }
                    }
                }
            }
            Text(title.overview?.takeIf { it.isNotBlank() } ?: stringResource(R.string.app_item_noOverview), style = SlouchcatType.body, color = c.tx2)
            val wish = title.wish
            if (wish != null) {
                Steps(wish.state)
                StateTag(wishStateText(wish.state, wish.stateDetail), strong = true)
                wish.progress?.let { progress -> progressText(progress)?.let { Text(it, style = SlouchcatType.caption, color = c.tx2) } }
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    SlouchcatButton(stringResource(R.string.wishes_withdraw_action), onClick = events::onWithdraw, kind = ButtonKind.Secondary)
                    events.onMine?.let { SlouchcatButton(stringResource(R.string.app_wishes_toMine), onClick = it, kind = ButtonKind.Secondary) }
                }
            } else if (title.canRequest) {
                SlouchcatButton(stringResource(R.string.app_wishes_request), onClick = events::onRequest, kind = ButtonKind.Accent, leadingIcon = SlouchcatIcon.Wish, block = !wide)
                title.request.quota?.let { quota -> Text(quotaLine(title.media, quota), style = SlouchcatType.caption, color = c.tx2) }
            } else {
                Note(reasonText(title.requestReason ?: title.request.reason))
                events.onMine?.takeIf { title.requestReason == com.slouchcat.core.api.generated.models.WishRequestReason.ALREADY_REQUESTED }?.let {
                    SlouchcatButton(stringResource(R.string.app_wishes_toMine), onClick = it, kind = ButtonKind.Secondary)
                }
            }
            title.lastWish?.takeIf { wish == null }?.let { last ->
                Text(stringResource(R.string.titles_last_title, wishStateText(last.state)), style = SlouchcatType.caption, color = c.tx2)
                last.reason?.let { reason -> failText(reason)?.let { Text(it, style = SlouchcatType.caption, color = c.tx2) } }
            }
            Spacer(Modifier.height(24.dp))
        }
    }
}

/** «1984 · Film · 1 Std. 53 Min.» or «2024 · Serie · 3 Staffeln». */
@Composable
private fun meta(title: ExternalTitle): String {
    val parts = mutableListOf<String>()
    title.year?.let { parts += it.toString() }
    parts += mediaKind(title.media)
    if (title.media == ExternalMedia.SHOW && title.seasons.isNotEmpty()) {
        parts += pluralCount(R.plurals.titles_seasonCount, title.seasons.size)
    } else {
        title.runtimeMinutes?.takeIf { it > 0 }?.let { parts += runtimeText(it) }
    }
    return parts.joinToString(stringResource(R.string.format_separator))
}

/** The four steps (mockup `.steps`): done ones in the accent, the current one too, open ones muted. */
@Composable
private fun Steps(state: WishState) {
    val c = SlouchcatTheme.colors
    val step = wishStep(state) ?: return
    val labels = listOf(
        WishStep.Requested to R.string.app_wishes_steps_requested,
        WishStep.Approved to R.string.app_wishes_steps_approved,
        WishStep.Loading to R.string.app_wishes_steps_loading,
        WishStep.Available to R.string.app_wishes_steps_available,
    )
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        labels.forEach { (s, label) ->
            val text = stringResource(label)
            val reached = s.ordinal <= step.ordinal
            val spoken = stringResource(
                when {
                    s == step -> R.string.app_wishes_steps_current
                    reached -> R.string.app_wishes_steps_done
                    else -> R.string.app_wishes_steps_open
                },
                text,
            )
            Column(Modifier.weight(1f).clearAndSetSemantics { contentDescription = spoken }, horizontalAlignment = Alignment.CenterHorizontally) {
                Box(Modifier.fillMaxWidth().height(4.dp).clip(RoundedCornerShape(2.dp)).background(if (reached) c.accent else c.s3))
                Text(text, style = SlouchcatType.label, color = if (s == step) c.tx else c.tx2, modifier = Modifier.padding(top = 6.dp), maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

@Composable
private fun progressText(progress: com.slouchcat.core.api.generated.models.WishProgress): String? = when (progress.stage) {
    com.slouchcat.core.api.generated.models.WishProgress.Stage.SEARCH -> stringResource(R.string.wishes_progress_search)
    com.slouchcat.core.api.generated.models.WishProgress.Stage.DOWNLOAD -> listOfNotNull(
        stringResource(R.string.wishes_progress_download),
        progress.percent?.let { stringResource(R.string.wishes_progress_percent, it.toString()) },
    ).joinToString(stringResource(R.string.format_separator))
    com.slouchcat.core.api.generated.models.WishProgress.Stage.IMPORT -> stringResource(R.string.wishes_progress_import)
    com.slouchcat.core.api.generated.models.WishProgress.Stage.STALLED -> stringResource(R.string.wishes_progress_stalled)
    else -> null
}

/** «Filme: 1 von 5 in 7 Tagen» (web `quotaText`); without limit «Filme ohne Limit». */
@Composable
fun quotaLine(media: ExternalMedia, quota: WishQuota): String {
    val part = if (media == ExternalMedia.SHOW) quota.tv else quota.movie
    return when {
        quota.unlimited || part == null -> stringResource(if (media == ExternalMedia.SHOW) R.string.wishes_quota_tvNone else R.string.wishes_quota_movieNone)
        media == ExternalMedia.SHOW -> stringResource(R.string.wishes_quota_tv, part.used.toString(), part.limit.toString(), part.days.toString())
        else -> stringResource(R.string.wishes_quota_movie, part.used.toString(), part.limit.toString(), part.days.toString())
    }
}

/** «Anfordern» (mockup handy 65): the title, for a series its seasons with «Alle Staffeln», the limit and the note. */
@Composable
private fun RequestSheet(title: ExternalTitle, state: WishTitleUiState, images: ServerImages, events: WishTitleEvents) {
    val c = SlouchcatTheme.colors
    val series = title.media == ExternalMedia.SHOW
    val heading = stringResource(R.string.app_wishes_request)
    SlouchcatSheet(heading, onDismiss = events::onDismiss) {
        SheetTitle(heading)
        Row(Modifier.fillMaxWidth().padding(horizontal = gutter(), vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
            MediaImage(images.externalPosters(title.images), title.title, Modifier.width(48.dp).aspectRatio(2f / 3f).clip(RoundedCornerShape(6.dp)))
            Column(Modifier.weight(1f)) {
                Text(title.title, style = SlouchcatType.strong, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
                Text(externalCaption(title.year, title.media), style = SlouchcatType.caption, color = c.tx2)
            }
        }
        if (series) {
            SectionTitle(stringResource(R.string.wishes_dialog_seasons))
            when (val seasons = state.seasons) {
                is Loadable.Ready -> {
                    val choosable = state.choosable
                    if (choosable.isEmpty()) {
                        SheetText(stringResource(R.string.wishes_dialog_nothingLeft_body))
                    } else {
                        CheckRow(stringResource(R.string.app_wishes_allSeasons), pluralCount(R.plurals.app_wishes_seasonsCount, choosable.size), checked = state.all, enabled = !state.sending, onToggle = events::onAllSeasons)
                        seasons.value.forEach { season -> SeasonRow(season, season.number in state.chosen, !state.sending) { events.onSeason(season.number) } }
                    }
                }
                is Loadable.Failed -> Column(Modifier.padding(horizontal = gutter())) {
                    Note(seasons.error.text(), warning = true)
                    SlouchcatButton(stringResource(R.string.app_state_retry), onClick = events::onRetrySeasons, kind = ButtonKind.Secondary, block = true, modifier = Modifier.padding(top = 8.dp))
                }
                else -> SheetText(stringResource(R.string.app_state_loading))
            }
        }
        title.request.quota?.let { quota -> InfoLine(SlouchcatIcon.Info, quotaLine(title.media, quota)) }
        InfoLine(SlouchcatIcon.Bell, stringResource(R.string.app_wishes_requestNote))
        state.error?.let { Note(wishErrorText(it), warning = true, modifier = Modifier.padding(horizontal = gutter(), vertical = 4.dp)) }
        val submit = when {
            state.sending -> stringResource(R.string.wishes_dialog_submitting)
            !series -> stringResource(R.string.app_wishes_request)
            state.chosen.isEmpty() -> stringResource(R.string.wishes_dialog_chooseSeasons)
            state.fresh == 0 -> stringResource(R.string.wishes_dialog_join)
            else -> pluralCount(R.plurals.wishes_dialog_submitSeasons, state.chosen.size)
        }
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = events::onDismiss, kind = ButtonKind.Secondary, modifier = Modifier.weight(1f), enabled = !state.sending)
            SlouchcatButton(
                submit,
                onClick = events::onSubmit,
                kind = ButtonKind.Accent,
                leadingIcon = SlouchcatIcon.Wish,
                busy = state.sending,
                enabled = !state.sending && (!series || state.chosen.isNotEmpty()),
                modifier = Modifier.weight(1f),
            )
        }
    }
}

@Composable
private fun SeasonRow(season: WishSeasonOffer, checked: Boolean, enabled: Boolean, onToggle: () -> Unit) {
    val parts = mutableListOf(pluralCount(R.plurals.wishes_dialog_episodes, season.episodeCount))
    when (season.state) {
        WishSeasonOffer.State.REQUESTED -> parts += stringResource(R.string.wishes_dialog_seasonState_requested)
        WishSeasonOffer.State.AVAILABLE -> parts += stringResource(R.string.wishes_dialog_seasonState_available)
        else -> Unit
    }
    val available = season.state == WishSeasonOffer.State.AVAILABLE
    CheckRow(
        stringResource(R.string.wishes_dialog_season, season.number.toString()),
        parts.joinToString(stringResource(R.string.format_separator)),
        checked = checked || available,
        enabled = enabled && !available,
        onToggle = onToggle,
    )
}

@Composable
private fun CheckRow(title: String, subtitle: String?, checked: Boolean, enabled: Boolean, onToggle: () -> Unit) {
    val c = SlouchcatTheme.colors
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 56.dp)
            .toggleable(value = checked, enabled = enabled, role = Role.Checkbox, onValueChange = { onToggle() })
            .padding(horizontal = gutter()),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Checkbox(
            checked = checked,
            onCheckedChange = null,
            enabled = enabled,
            colors = CheckboxDefaults.colors(checkedColor = c.accent, uncheckedColor = c.ctl, checkmarkColor = c.onAccent, disabledCheckedColor = c.s3),
        )
        Column(Modifier.weight(1f)) {
            Text(title, style = SlouchcatType.rowTitle, color = if (enabled) c.tx else c.tx2)
            subtitle?.let { Text(it, style = SlouchcatType.caption, color = c.tx2) }
        }
    }
}

@Composable
private fun InfoLine(icon: SlouchcatIcon, text: String) {
    val c = SlouchcatTheme.colors
    Row(Modifier.fillMaxWidth().padding(horizontal = gutter(), vertical = 6.dp), horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
        com.slouchcat.core.design.SlouchcatIconImage(icon, contentDescription = null, size = 18.dp, tint = c.tx2)
        Text(text, style = SlouchcatType.caption, color = c.tx2, modifier = Modifier.weight(1f))
    }
}

/** «Wunsch zurückziehen?» (mockup handy 64): «Behalten» and «Zurückziehen». */
@Composable
private fun WithdrawSheet(title: ExternalTitle, state: WishTitleUiState, events: WishTitleEvents) {
    val heading = stringResource(R.string.wishes_withdraw_title)
    SlouchcatSheet(heading, onDismiss = events::onDismiss) {
        SheetTitle(heading)
        SheetText(stringResource(R.string.app_wishes_withdrawBody, title.title))
        SheetActions {
            SlouchcatButton(stringResource(R.string.app_wishes_keep), onClick = events::onDismiss, kind = ButtonKind.Secondary, modifier = Modifier.weight(1f), enabled = !state.sending)
            SlouchcatButton(stringResource(R.string.wishes_withdraw_confirm), onClick = events::onConfirmWithdraw, kind = ButtonKind.Danger, busy = state.sending, modifier = Modifier.weight(1f))
        }
    }
}
