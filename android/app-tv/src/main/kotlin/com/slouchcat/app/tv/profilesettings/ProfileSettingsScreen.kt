package com.slouchcat.app.tv.profilesettings

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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.shell.tvPage
import com.slouchcat.app.tv.ui.BadgeTone
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.NoteTone
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvActions
import com.slouchcat.app.tv.ui.TvBadge
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDialog
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvIcons
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvNote
import com.slouchcat.app.tv.ui.TvSideMenu
import com.slouchcat.app.tv.ui.TvTextField
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.api.generated.models.Profile
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.browse.profilesettings.PinError
import com.slouchcat.core.browse.profilesettings.PinKind
import com.slouchcat.core.browse.profilesettings.ProfileEditor
import com.slouchcat.core.browse.profilesettings.ProfileSettingsUiState
import com.slouchcat.core.browse.profilesettings.ProfileSettingsViewModel
import com.slouchcat.core.design.Avatar
import com.slouchcat.core.design.ProfileAccents
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.strings.R

/** What «Profile verwalten» can be asked to do; the ViewModel does it, tests record it. */
interface ProfileSettingsEvents {
    fun onRetry()
    fun onSwitchProfile()
    fun onBack()
    fun onAdvice()
    fun onAdd()
    fun onEdit(profile: Profile)
    fun onDismiss()
    fun onName(name: String)
    fun onAccent(accent: Accent)
    fun onRating(age: Int)
    fun onPinForm(open: Boolean)
    fun onPinFirst(digits: String)
    fun onPinSecond(digits: String)
    fun onPinApply(): Boolean
    fun onPinRemove()
    fun onDeleteAsk(ask: Boolean)
    fun onDelete()
    fun onSave()
}

private const val ADD = "pr-add"
private const val RETRY = "retry"
private const val SWITCH = "pm-switch"

/**
 * «Profile verwalten» (concept 26, 3.3 and E-8; mockup `/settings/profiles`): with the primary
 * profile the profiles of the account, each opening «Profil bearbeiten» in a side menu (name,
 * colour, age rating, PIN, delete), «Profil hinzufügen», and the recommendation of a PIN for the
 * primary profile (E-9). Another profile reads why and may switch. No watch report and no offline
 * PIN on the TV (F-18, E-8). The server decides; a refusal shows in the menu (rule 12).
 */
@Composable
fun ProfileSettingsScreen(state: ProfileSettingsUiState, events: ProfileSettingsEvents) {
    val focus = rememberFocusMemory()
    val c = SlouchcatTheme.colors
    Column(Modifier.fillMaxSize().tvPage().verticalScroll(rememberScrollState())) {
        Text(stringResource(R.string.app_profiles_manage_title), style = TvType.h1, modifier = Modifier.semantics { heading() })
        if (!state.canManage) {
            val primary = (state.summaries as? Loadable.Ready)?.value?.firstOrNull { it.isPrimary }
            TvNote(
                stringResource(R.string.profiles_settings_onlyPrimary_body) + (primary?.let { " · ${it.name}" } ?: ""),
                Modifier.padding(top = TvDimens.xl).widthIn(max = TvDimens.Browse.ProfileList),
                tone = NoteTone.Warn,
                icon = SlouchcatIcon.Lock.vector,
            )
            Row(Modifier.padding(top = TvDimens.xl), horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
                TvButton(stringResource(R.string.profiles_settings_onlyPrimary_action), onClick = events::onSwitchProfile, icon = SlouchcatIcon.Users.vector, kind = TvButtonKind.Primary, modifier = Modifier.focusKey(focus, SWITCH))
                TvButton(stringResource(R.string.common_back), onClick = events::onBack, icon = SlouchcatIcon.Back.vector, kind = TvButtonKind.Ghost, modifier = Modifier.focusKey(focus, "pm-back"))
            }
            (state.summaries as? Loadable.Ready)?.value?.let { list ->
                Column(Modifier.padding(top = TvDimens.xl).widthIn(max = TvDimens.Browse.ProfileList), verticalArrangement = Arrangement.spacedBy(TvDimens.s)) {
                    list.forEach { summary -> SummaryRow(summary) }
                }
            }
        } else {
            if (state.advisePin) {
                Row(
                    Modifier
                        .padding(top = TvDimens.xl)
                        .widthIn(max = TvDimens.Browse.ProfileList)
                        .border(BorderStroke(TvDimens.borderThick, c.warn), RoundedCornerShape(TvDimens.radiusPanel))
                        .padding(TvDimens.l),
                    horizontalArrangement = Arrangement.spacedBy(TvDimens.l),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    TvIcon(SlouchcatIcon.Shield, TvDimens.Menu.Icon, tint = c.warn)
                    Column(Modifier.weight(1f)) {
                        Text(stringResource(R.string.profiles_pinAdvice_title), style = TvType.bodyStrong)
                        Text(stringResource(R.string.profiles_pinAdvice_body), style = TvType.small, color = c.tx2)
                    }
                    TvButton(stringResource(R.string.profiles_pinAdvice_action), onClick = events::onAdvice, icon = SlouchcatIcon.Lock.vector, small = true, modifier = Modifier.focusKey(focus, "pm-pin"))
                }
            }
            when (val profiles = state.profiles) {
                is Loadable.Ready -> Column(Modifier.padding(top = TvDimens.xl).widthIn(max = TvDimens.Browse.ProfileList), verticalArrangement = Arrangement.spacedBy(TvDimens.s)) {
                    profiles.value.forEach { profile ->
                        ProfileRow(
                            name = profile.name,
                            accent = profile.accent?.value,
                            subtitle = profileLine(profile, state),
                            locked = profile.hasPin,
                            onClick = { events.onEdit(profile) },
                            modifier = Modifier.focusKey(focus, "pr-${profile.id}"),
                        )
                    }
                    ProfileRow(
                        name = stringResource(R.string.profiles_settings_add),
                        accent = null,
                        subtitle = stringResource(R.string.tv_profiles_addHint),
                        locked = false,
                        onClick = events::onAdd,
                        add = true,
                        modifier = Modifier.focusKey(focus, ADD),
                    )
                }
                else -> LoadableContent(profiles, events::onRetry, Modifier.padding(top = TvDimens.xl), retryModifier = Modifier.focusKey(focus, RETRY)) {}
            }
        }
    }
    val start = when {
        !state.canManage -> SWITCH
        state.profiles is Loadable.Ready -> (state.profiles as Loadable.Ready).value.firstOrNull()?.let { "pr-${it.id}" } ?: ADD
        else -> RETRY
    }
    focus.StartFocus(start, state.profiles::class, state.canManage)
    state.editor?.let { Editor(it, state, events, focus) }
    // Saved or deleted: the menu is gone, the focus goes back to the row it came from.
    var hadEditor by remember { mutableStateOf(false) }
    LaunchedEffect(state.editor == null) {
        if (state.editor == null && hadEditor) focus.restore()
        hadEditor = state.editor != null
    }
}

/** «Hauptprofil · Freigabe 12 · mit PIN». */
@Composable
private fun profileLine(profile: Profile, state: ProfileSettingsUiState): String = listOfNotNull(
    if (profile.isPrimary) stringResource(R.string.profiles_settings_primary) else null,
    if (profile.maxRating >= ProfileSettingsUiState.NO_AGE_LIMIT) null else stringResource(R.string.app_profiles_manage_rating, state.ratingLabel(profile.maxRating) ?: profile.maxRating.toString()),
    stringResource(if (profile.hasPin) R.string.tv_profiles_pinSet else R.string.app_profiles_manage_noPin),
).joinToString(" · ")

@Composable
private fun SummaryRow(summary: ProfileSummary) {
    Row(horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
        Avatar(summary.name, summary.accent?.value, size = TvDimens.Browse.ProfileAvatar)
        Text(summary.name, style = TvType.largeStrong)
        if (summary.hasPin) TvIcon(SlouchcatIcon.Lock, TvDimens.Browse.Icon, tint = SlouchcatTheme.colors.tx2)
    }
}

/** `.md-prow`: avatar, name, the facts; in focus light. */
@Composable
private fun ProfileRow(name: String, accent: String?, subtitle: String, locked: Boolean, onClick: () -> Unit, modifier: Modifier, add: Boolean = false) {
    val c = SlouchcatTheme.colors
    Surface(
        onClick = onClick,
        modifier = modifier.fillMaxWidth().heightIn(min = TvDimens.Browse.ProfileRow).semantics { contentDescription = "$name, $subtitle" },
        shape = ClickableSurfaceDefaults.shape(shape = RoundedCornerShape(TvDimens.radiusMenuItem)),
        colors = ClickableSurfaceDefaults.colors(containerColor = c.s1, contentColor = c.tx, focusedContainerColor = c.tx, focusedContentColor = c.bg),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScaleSmall),
    ) {
        Row(Modifier.fillMaxWidth().padding(horizontal = TvDimens.l, vertical = TvDimens.s), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
            if (add) {
                Box(Modifier.size(TvDimens.Browse.ProfileAvatar).clip(CircleShape).background(c.s3), contentAlignment = Alignment.Center) { TvIcon(SlouchcatIcon.Plus, TvDimens.Browse.Icon) }
            } else {
                Avatar(name, accent, size = TvDimens.Browse.ProfileAvatar)
            }
            Column(Modifier.weight(1f)) {
                Text(name, style = TvType.largeStrong, maxLines = 1)
                Text(subtitle, style = TvType.small, color = androidx.tv.material3.LocalContentColor.current.copy(alpha = SUBTLE), maxLines = 1)
            }
            if (locked) TvIcon(SlouchcatIcon.Lock, TvDimens.Browse.Icon)
            if (!add) TvIcon(TvIcons.Edit, TvDimens.Browse.Icon)
        }
    }
}

/** «Profil bearbeiten» / «Neues Profil» (mockup `mdPEHtml`), a side menu; the server's refusal stays in it. */
@Composable
private fun Editor(editor: ProfileEditor, state: ProfileSettingsUiState, events: ProfileSettingsEvents, page: FocusMemory) {
    val c = SlouchcatTheme.colors
    val name = remember { FocusRequester() }
    val close = {
        events.onDismiss()
        page.restore()
    }
    // A question or the PIN pad comes instead of the menu (a menu keeps the focus inside), then the menu again.
    if (!editor.confirmDelete && editor.pinForm == null) TvSideMenu(
        stringResource(if (editor.isNew) R.string.profiles_settings_form_createTitle else R.string.profiles_settings_form_editTitle),
        onDismiss = close,
        initialFocus = name,
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
            Avatar(editor.name.ifBlank { "?" }, editor.accent.value, size = TvDimens.Browse.ProfileAvatar)
            Column {
                Text(editor.name.ifBlank { stringResource(R.string.profiles_settings_form_name) }, style = TvType.largeStrong)
                if (editor.isPrimary) Text(stringResource(R.string.profiles_settings_primary), style = TvType.small, color = c.tx3)
            }
        }
        TvTextField(
            value = editor.name,
            onValueChange = events::onName,
            label = stringResource(R.string.profiles_settings_form_name),
            placeholder = stringResource(R.string.profiles_settings_form_name),
            focusRequester = name,
            modifier = Modifier.padding(top = TvDimens.l),
        )
        if (editor.nameMissing) Text(stringResource(R.string.app_profiles_manage_nameMissing), style = TvType.small, color = c.danger)
        Label(stringResource(R.string.accent_label))
        Row(horizontalArrangement = Arrangement.spacedBy(TvDimens.s)) {
            ProfileSettingsViewModel.ACCENTS.forEach { accent -> Swatch(accent, accent == editor.accent) { events.onAccent(accent) } }
        }
        if (!editor.isPrimary) {
            Label(stringResource(R.string.profiles_settings_form_rating))
            Row(horizontalArrangement = Arrangement.spacedBy(TvDimens.s)) {
                state.ratings.filter { it.enabled }.forEach { choice ->
                    val label = choice.label ?: choice.age.toString()
                    TvButton(label, onClick = { events.onRating(choice.age) }, small = true, kind = if (choice.age == editor.rating) TvButtonKind.Primary else TvButtonKind.Standard, modifier = Modifier.semantics { selected = choice.age == editor.rating })
                }
            }
            Text(
                if (editor.rating >= ProfileSettingsUiState.NO_AGE_LIMIT) stringResource(R.string.app_profiles_manage_ratingAll) else stringResource(R.string.app_profiles_manage_ratingUpTo, state.ratingLabel(editor.rating) ?: editor.rating.toString()),
                style = TvType.small,
                color = c.tx2,
                modifier = Modifier.padding(top = TvDimens.s),
            )
        }
        Label(stringResource(R.string.profiles_pin_label))
        val pinOn = editor.pinOn()
        TvBadge(stringResource(if (pinOn) R.string.tv_profiles_pinSet else R.string.app_profiles_manage_noPin), tone = if (pinOn) BadgeTone.Ok else BadgeTone.Plain, icon = if (pinOn) SlouchcatIcon.Lock.vector else null)
        Row(Modifier.padding(top = TvDimens.m), horizontalArrangement = Arrangement.spacedBy(TvDimens.s)) {
            TvButton(stringResource(if (pinOn) R.string.profiles_settings_pin_change else R.string.profiles_settings_pin_set), onClick = { events.onPinForm(true) }, icon = SlouchcatIcon.Lock.vector, small = true)
            if (pinOn) TvButton(stringResource(R.string.profiles_settings_pin_remove_action), onClick = events::onPinRemove, icon = TvIcons.Unlock, small = true, kind = TvButtonKind.Ghost)
        }
        editor.error?.let { TvNote(stringResource(R.string.app_item_failed), Modifier.padding(top = TvDimens.l), tone = NoteTone.Bad) }
        TvActions {
            TvButton(
                stringResource(if (editor.isNew) R.string.profiles_settings_form_create else R.string.profiles_settings_form_save),
                onClick = events::onSave,
                icon = SlouchcatIcon.Check.vector,
                kind = TvButtonKind.Primary,
                enabled = !editor.busy,
            )
            if (!editor.isNew && !editor.isPrimary) TvButton(stringResource(R.string.profiles_settings_delete_action), onClick = { events.onDeleteAsk(true) }, icon = SlouchcatIcon.Trash.vector, kind = TvButtonKind.Ghost)
        }
    }
    if (editor.confirmDelete) {
        val keep = remember { FocusRequester() }
        TvDialog(stringResource(R.string.profiles_settings_delete_title, editor.name), onDismiss = { events.onDeleteAsk(false) }, initialFocus = keep) {
            Text(stringResource(R.string.app_profiles_manage_deleteWarning, editor.name), style = TvType.large, color = c.tx2)
            TvActions {
                TvButton(stringResource(R.string.profiles_settings_delete_confirm), onClick = events::onDelete, icon = SlouchcatIcon.Trash.vector)
                TvButton(stringResource(R.string.app_profiles_manage_keep), onClick = { events.onDeleteAsk(false) }, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(keep))
            }
        }
    }
    if (editor.pinForm != null) PinForm(editor, events)
}

@Composable
private fun Label(text: String) =
    Text(text, style = TvType.label, color = SlouchcatTheme.colors.tx2, modifier = Modifier.padding(top = TvDimens.xl, bottom = TvDimens.s))

/** `.md-sw`: a colour of a profile, the chosen one with a check. */
@Composable
private fun Swatch(accent: Accent, chosen: Boolean, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    val colors = ProfileAccents.of(accent.value)
    val name = accentName(accent)
    Surface(
        onClick = onClick,
        modifier = Modifier.size(TvDimens.Browse.Swatch).semantics {
            contentDescription = name
            selected = chosen
        },
        shape = ClickableSurfaceDefaults.shape(shape = CircleShape),
        colors = ClickableSurfaceDefaults.colors(containerColor = colors.base, focusedContainerColor = colors.base, pressedContainerColor = colors.base),
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, if (chosen) c.tx else Color.Transparent)),
            focusedBorder = Border(BorderStroke(TvDimens.Card.Ring, c.tx), inset = -TvDimens.Card.RingGap),
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
    ) {
        if (chosen) Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { TvIcon(SlouchcatIcon.Check, TvDimens.Browse.Icon, tint = colors.on) }
    }
}

@Composable
private fun accentName(accent: Accent): String = stringResource(
    when (accent) {
        Accent.CORAL -> R.string.accent_coral
        Accent.AMBER -> R.string.accent_amber
        Accent.LIME -> R.string.accent_lime
        Accent.TEAL -> R.string.accent_teal
        Accent.SKY -> R.string.accent_sky
        Accent.VIOLET -> R.string.accent_violet
        else -> R.string.accent_ember
    },
)

/**
 * The new PIN (mockup `mdPinPad`): four to eight digits, then the same again; «Übernehmen» takes it
 * for «Speichern». Digits of the remote count as well.
 */
@Composable
private fun PinForm(editor: ProfileEditor, events: ProfileSettingsEvents) {
    val c = SlouchcatTheme.colors
    var again by remember { mutableStateOf(false) }
    val first = remember { FocusRequester() }
    val digits = if (again) editor.second else editor.first
    val set = { value: String -> if (again) events.onPinSecond(value) else events.onPinFirst(value) }
    val cancel = { events.onPinForm(false) }
    TvDialog(stringResource(R.string.profiles_settings_pin_title, editor.name.ifBlank { "?" }), onDismiss = cancel, initialFocus = first) {
        Text(
            stringResource(if (again) R.string.app_profiles_manage_repeat else R.string.profiles_settings_pin_label),
            style = TvType.large,
            color = c.tx2,
        )
        Row(Modifier.padding(vertical = TvDimens.l), horizontalArrangement = Arrangement.spacedBy(TvDimens.Profiles.PinDotsGap)) {
            repeat(maxOf(PinKind.Pin.min, digits.length)) { index ->
                val on = index < digits.length
                Box(Modifier.size(TvDimens.Profiles.PinDot).clip(CircleShape).background(if (on) c.tx else Color.Transparent).border(BorderStroke(TvDimens.borderThick, if (on) c.tx else c.tx2), CircleShape))
            }
        }
        editor.pinError?.let {
            Text(
                stringResource(if (it == PinError.Mismatch) R.string.app_profiles_manage_mismatch else R.string.profiles_settings_pin_hint),
                style = TvType.body,
                color = c.danger,
            )
        }
        Column(verticalArrangement = Arrangement.spacedBy(TvDimens.Profiles.PinGap)) {
            listOf("123", "456", "789").forEach { row ->
                Row(horizontalArrangement = Arrangement.spacedBy(TvDimens.Profiles.PinGap)) {
                    row.forEach { d -> TvButton(d.toString(), onClick = { set(digits + d) }, modifier = if (d == '1') Modifier.focusRequester(first) else Modifier) }
                }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(TvDimens.Profiles.PinGap)) {
                TvButton(stringResource(R.string.app_profiles_pinDelete), onClick = { set(digits.dropLast(1)) }, icon = TvIcons.Backspace, small = true)
                TvButton("0", onClick = { set(digits + '0') })
            }
        }
        TvActions {
            if (!again) {
                TvButton(stringResource(R.string.tv_hand_next), onClick = { again = true }, kind = TvButtonKind.Primary, enabled = digits.length >= PinKind.Pin.min)
            } else {
                TvButton(stringResource(R.string.app_profiles_manage_apply), onClick = {
                    if (!events.onPinApply()) again = false
                }, kind = TvButtonKind.Primary, enabled = digits.length >= PinKind.Pin.min)
            }
            TvButton(stringResource(R.string.common_cancel), onClick = cancel, kind = TvButtonKind.Ghost)
        }
    }
}

private const val SUBTLE = 0.72f
