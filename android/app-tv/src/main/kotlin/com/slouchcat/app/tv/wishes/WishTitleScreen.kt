package com.slouchcat.app.tv.wishes

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.toggleableState
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.LocalContentColor
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.EmptyAction
import com.slouchcat.app.tv.browse.EmptyState
import com.slouchcat.app.tv.browse.KindLine
import com.slouchcat.app.tv.browse.MetaLine
import com.slouchcat.app.tv.browse.ProgressBar
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.browse.runtimeText
import com.slouchcat.app.tv.browse.screenWidth
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.common.text
import com.slouchcat.app.tv.shell.tvPage
import com.slouchcat.app.tv.ui.BackdropMode
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.NoteTone
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvActions
import com.slouchcat.app.tv.ui.TvBackdrop
import com.slouchcat.app.tv.ui.TvBadge
import com.slouchcat.app.tv.ui.TvBusy
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDialog
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvImage
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvNote
import com.slouchcat.app.tv.ui.TvSideMenu
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.ExternalTitle
import com.slouchcat.core.api.generated.models.WishSeasonOffer
import com.slouchcat.core.browse.wishes.WishSheet
import com.slouchcat.core.browse.wishes.WishTitleUiState
import com.slouchcat.core.browse.wishes.WishTitleViewModel
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.strings.R

/** Focus keys of the page of a title outside the library. */
object WishTitleKeys {
    const val REQUEST = "request"
    const val MINE = "mine"
    const val WITHDRAW = "withdraw"
    const val RETRY = "retry"
    const val BACK = "back"
}

/**
 * A title outside the library (mockup `/wish/:i`, reference photos tv 33, 34, 60, 61): the art layer
 * darkened, the poster, «Nicht in der Bibliothek», title, year, kind, length or seasons, age; the
 * summary; an own wish with its state and progress, «Meine Wünsche» and «Zurückziehen» (asks
 * first); without one «Anfordern» and the limit, or why the profile cannot request. «Anfordern»
 * opens a side menu, for a series with its seasons. [onMine] opens «Meine Wünsche»; [onBack] leaves
 * a title that is gone.
 */
@Composable
fun WishTitleScreen(state: WishTitleUiState, images: ServerImages, actions: WishTitleViewModel, onMine: () -> Unit, onBack: () -> Unit) {
    val focus = rememberFocusMemory()
    val title = (state.title as? Loadable.Ready)?.value
    Box(Modifier.fillMaxSize()) {
        TvBackdrop(title?.let { images.externalBackdrops(it.images).forWidth(screenWidth()) }, mode = BackdropMode.Dim)
        if (title == null) {
            val failed = state.title as? Loadable.Failed
            if (failed?.error == LoadError.NotFound) {
                EmptyState(
                    SlouchcatIcon.Wish.vector,
                    stringResource(R.string.titles_notFound_title),
                    stringResource(R.string.titles_notFound_body),
                    Modifier.tvPage(),
                    action = { EmptyAction(stringResource(R.string.common_back), SlouchcatIcon.Back.vector, onBack, Modifier.focusKey(focus, WishTitleKeys.BACK)) },
                )
            } else {
                LoadableContent(state.title, onRetry = actions::retry, modifier = Modifier.tvPage(), retryModifier = Modifier.focusKey(focus, WishTitleKeys.RETRY)) {}
            }
        } else {
            Page(title, images, actions, onMine, focus)
        }
    }
    val first = when {
        title == null -> if ((state.title as? Loadable.Failed)?.error == LoadError.NotFound) WishTitleKeys.BACK else WishTitleKeys.RETRY
        title.wish == null && title.canRequest -> WishTitleKeys.REQUEST
        else -> WishTitleKeys.MINE
    }
    // The actions change with the wish (requested, withdrawn): the focus goes to the first one there.
    focus.StartFocus(first, state.title::class, title?.wish?.state, title?.canRequest, first)
    // A side menu or the question closed: back to the element of before (2b review M-4).
    LaunchedEffect(state.sheet == null) { if (state.sheet == null) focus.restore() }
    if (title != null) {
        when (state.sheet) {
            WishSheet.Request -> RequestMenu(title, state, images, actions)
            WishSheet.Withdraw -> WithdrawDialog(title, state, actions)
            null -> Unit
        }
    }
}

@Composable
private fun Page(title: ExternalTitle, images: ServerImages, actions: WishTitleViewModel, onMine: () -> Unit, focus: FocusMemory) {
    val c = SlouchcatTheme.colors
    Row(Modifier.fillMaxSize().tvPage(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(W.PageGap)) {
        Box(Modifier.width(W.Poster).aspectRatio(POSTER).clip(RoundedCornerShape(TvDimens.radiusPanel)).background(c.s2)) {
            images.externalPosters(title.images).forWidth(W.Poster)?.let { TvImage(it, Modifier.fillMaxSize()) }
        }
        Column(Modifier.weight(1f).widthIn(max = W.Info)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(W.KickerGap), modifier = Modifier.padding(bottom = TvDimens.s)) {
                TvIcon(SlouchcatIcon.Wish, TvDimens.Badge.Icon, tint = c.accent)
                KindLine(stringResource(R.string.titles_missing))
            }
            Text(title.title, style = TvType.xl, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.semantics { heading() })
            MetaLine(facts(title), Modifier.padding(top = TvDimens.s), rating = title.minAge)
            title.overview?.takeIf { it.isNotBlank() }?.let {
                Text(it, style = TvType.body, color = c.tx2, maxLines = OVERVIEW_LINES, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = TvDimens.l).widthIn(max = W.Overview))
            }
            val wish = title.wish
            if (wish != null) {
                val state = wishStateText(wish.state, wish.stateDetail)
                val progress = wish.progress?.let { progressText(it) }
                Row(Modifier.padding(top = W.StateTop), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
                    TvBadge(state, tone = toneOf(wish.state))
                    progress?.let { Text(it, style = TvType.body, color = c.tx2, maxLines = 1) }
                }
                wish.progress?.percent?.let { ProgressBar(it / PERCENT, W.Bar, Modifier.padding(top = TvDimens.m)) }
            } else if (!title.canRequest) {
                TvNote(reasonText(title.requestReason ?: title.request.reason), Modifier.padding(top = W.StateTop), small = true)
            }
            Row(Modifier.padding(top = W.ActionsTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
                if (wish == null && title.canRequest) {
                    TvButton(stringResource(R.string.app_wishes_request), onClick = actions::onRequest, icon = SlouchcatIcon.Plus.vector, kind = TvButtonKind.Primary, modifier = Modifier.focusKey(focus, WishTitleKeys.REQUEST))
                } else {
                    TvButton(stringResource(R.string.app_wishes_toMine), onClick = onMine, icon = SlouchcatIcon.Wish.vector, kind = TvButtonKind.Primary, modifier = Modifier.focusKey(focus, WishTitleKeys.MINE))
                    if (wish != null) {
                        TvButton(stringResource(R.string.wishes_withdraw_action), onClick = actions::onWithdraw, icon = SlouchcatIcon.Close.vector, modifier = Modifier.focusKey(focus, WishTitleKeys.WITHDRAW))
                    }
                }
            }
            if (wish == null && title.canRequest) title.request.quota?.let { quota -> Text(quotaLine(title.media, quota), style = TvType.small, color = c.tx3, modifier = Modifier.padding(top = W.LineTop)) }
            title.lastWish?.takeIf { wish == null }?.let { last ->
                Text(stringResource(R.string.titles_last_title, wishStateText(last.state)), style = TvType.small, color = c.tx3, modifier = Modifier.padding(top = TvDimens.m))
                last.reason?.let { reason -> failText(reason)?.let { Text(it, style = TvType.small, color = c.tx3) } }
            }
        }
    }
}

/** «1984 · Film · 1 Std. 53 Min.» or «2024 · Serie · 3 Staffeln». */
@Composable
private fun facts(title: ExternalTitle): List<String> = buildList {
    title.year?.let { add(it.toString()) }
    add(mediaKind(title.media))
    if (title.media == ExternalMedia.SHOW && title.seasons.isNotEmpty()) {
        add(pluralCount(R.plurals.titles_seasonCount, title.seasons.size))
    } else {
        title.runtimeMinutes?.takeIf { it > 0 }?.let { add(runtimeText(it)) }
    }
}

/**
 * «Anfordern» (mockup tv 61, `mdReqHtml`): the title, for a series its seasons with «Alle
 * Staffeln» (the available ones ticked and not choosable), the limit, that the wish goes to Seerr,
 * a refusal of the server; «Anfordern» (focus) and «Abbrechen».
 */
@Composable
private fun RequestMenu(title: ExternalTitle, state: WishTitleUiState, images: ServerImages, actions: WishTitleViewModel) {
    val c = SlouchcatTheme.colors
    val submit = remember { FocusRequester() }
    val series = title.media == ExternalMedia.SHOW
    TvSideMenu(stringResource(R.string.app_wishes_request), onDismiss = actions::onDismiss, initialFocus = submit) {
        Row(Modifier.padding(bottom = TvDimens.l), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.width(W.SheetPoster).aspectRatio(POSTER).clip(RoundedCornerShape(TvDimens.radiusCard)).background(c.s2)) {
                images.externalPosters(title.images).forWidth(W.SheetPoster)?.let { TvImage(it, Modifier.fillMaxSize()) }
            }
            Column {
                Text(title.title, style = TvType.largeStrong, maxLines = 2, overflow = TextOverflow.Ellipsis)
                Text(externalCaption(title.year, title.media), style = TvType.small, color = c.tx3)
            }
        }
        if (series) {
            Text(stringResource(R.string.wishes_dialog_seasons), style = TvType.bodyStrong, color = c.tx2, modifier = Modifier.padding(bottom = TvDimens.s).semantics { heading() })
            when (val seasons = state.seasons) {
                is Loadable.Ready -> {
                    if (state.choosable.isEmpty()) {
                        Text(stringResource(R.string.wishes_dialog_nothingLeft_body), style = TvType.body, color = c.tx2)
                    } else {
                        CheckRow(stringResource(R.string.app_wishes_allSeasons), pluralCount(R.plurals.app_wishes_seasonsCount, state.choosable.size), state.all, enabled = true, onToggle = actions::onAllSeasons)
                        seasons.value.forEach { season -> SeasonRow(season, season.number in state.chosen) { actions.onSeason(season.number) } }
                    }
                }
                is Loadable.Failed -> {
                    TvNote(seasons.error.text(), tone = NoteTone.Warn, small = true)
                    TvButton(stringResource(R.string.app_state_retry), onClick = actions::onRetrySeasons, icon = SlouchcatIcon.Refresh.vector, small = true, modifier = Modifier.padding(top = TvDimens.m))
                }
                else -> TvBusy(stringResource(R.string.app_state_loading))
            }
        }
        Column(
            Modifier
                .padding(top = TvDimens.l)
                .fillMaxWidth()
                .clip(RoundedCornerShape(TvDimens.radiusField))
                .background(c.s2)
                .padding(horizontal = W.InfoPadX, vertical = W.InfoPadY)
                .semantics(mergeDescendants = true) {},
            verticalArrangement = Arrangement.spacedBy(TvDimens.xs),
        ) {
            title.request.quota?.let { quota -> Text(quotaLine(title.media, quota), style = TvType.small, color = c.tx2) }
            Text(stringResource(R.string.app_wishes_requestNote), style = TvType.small, color = c.tx2)
        }
        state.error?.let { TvNote(stringResource(wishErrorId(it)), Modifier.padding(top = TvDimens.m), tone = NoteTone.Warn, small = true) }
        val label = when {
            state.sending -> stringResource(R.string.wishes_dialog_submitting)
            !series -> stringResource(R.string.app_wishes_request)
            state.chosen.isEmpty() -> stringResource(R.string.wishes_dialog_chooseSeasons)
            state.fresh == 0 -> stringResource(R.string.wishes_dialog_join)
            else -> pluralCount(R.plurals.wishes_dialog_submitSeasons, state.chosen.size)
        }
        TvActions {
            // Stays focusable while it cannot send (no season, on its way): the focus never falls out of the menu.
            TvButton(label, onClick = { if (!series || state.chosen.isNotEmpty()) actions.onSubmit() }, icon = SlouchcatIcon.Check.vector, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(submit))
            TvButton(stringResource(R.string.common_cancel), onClick = actions::onDismiss, kind = TvButtonKind.Ghost)
        }
    }
}

@Composable
private fun SeasonRow(season: WishSeasonOffer, checked: Boolean, onToggle: () -> Unit) {
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
        enabled = !available,
        onToggle = onToggle,
    )
}

/** `.mi.md-chk`: a row with a box that OK ticks (`role=checkbox`); a season that is there already is ticked and not focusable. */
@Composable
private fun CheckRow(title: String, subtitle: String?, checked: Boolean, enabled: Boolean, onToggle: () -> Unit) {
    val c = SlouchcatTheme.colors
    Surface(
        onClick = onToggle,
        enabled = enabled,
        modifier = Modifier
            .focusProperties { canFocus = enabled }
            .fillMaxWidth()
            .heightIn(min = W.Check)
            .semantics(mergeDescendants = true) {
                role = Role.Checkbox
                toggleableState = ToggleableState(checked)
            },
        shape = ClickableSurfaceDefaults.shape(shape = RoundedCornerShape(TvDimens.radiusMenuItem)),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = Color.Transparent,
            contentColor = c.tx,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
            disabledContainerColor = Color.Transparent,
            disabledContentColor = c.tx2,
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScaleSmall),
    ) {
        val focused = LocalContentColor.current == c.bg
        Row(
            Modifier.fillMaxWidth().align(Alignment.CenterStart).padding(horizontal = TvDimens.Menu.ItemPad, vertical = TvDimens.xs),
            horizontalArrangement = Arrangement.spacedBy(TvDimens.l),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.weight(1f)) {
                Text(title, style = TvType.largeStrong, maxLines = 1)
                if (subtitle != null) Text(subtitle, style = TvType.small, color = if (focused) c.bg else c.tx3, maxLines = 2)
            }
            val box = RoundedCornerShape(TvDimens.radiusIconBox)
            Box(
                Modifier
                    .size(W.CheckBox)
                    .clip(box)
                    .background(if (checked) c.accent else Color.Transparent)
                    .border(BorderStroke(TvDimens.borderThick, if (checked) c.accent else if (focused) c.bg else c.ctl), box),
                contentAlignment = Alignment.Center,
            ) { if (checked) TvIcon(SlouchcatIcon.Check, TvDimens.Badge.Icon, tint = c.onAccent) }
        }
    }
}

/** «Wunsch zurückziehen?» (mockup tv 60): «Zurückziehen» and «Behalten» (the focus starts there). */
@Composable
private fun WithdrawDialog(title: ExternalTitle, state: WishTitleUiState, actions: WishTitleViewModel) {
    val keep = remember { FocusRequester() }
    TvDialog(stringResource(R.string.wishes_withdraw_title), onDismiss = actions::onDismiss, initialFocus = keep) {
        Text(stringResource(R.string.app_wishes_withdrawBody, title.title), style = TvType.large, color = SlouchcatTheme.colors.tx2)
        TvActions {
            TvButton(
                stringResource(if (state.sending) R.string.app_state_loading else R.string.wishes_withdraw_confirm),
                onClick = actions::onConfirmWithdraw,
                icon = SlouchcatIcon.Close.vector,
            )
            TvButton(stringResource(R.string.app_wishes_keep), onClick = actions::onDismiss, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(keep))
        }
    }
}

/** `.md-ov`: four lines of the summary. */
private const val OVERVIEW_LINES = 4
private const val POSTER = 2f / 3f
private const val PERCENT = 100f
