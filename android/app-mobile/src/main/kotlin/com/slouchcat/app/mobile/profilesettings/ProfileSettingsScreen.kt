package com.slouchcat.app.mobile.profilesettings

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.common.RowGroup
import com.slouchcat.core.api.generated.models.Accent
import com.slouchcat.core.api.generated.models.Language
import com.slouchcat.core.api.generated.models.Profile
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.browse.profilesettings.PinChange
import com.slouchcat.core.browse.profilesettings.PinError
import com.slouchcat.core.browse.profilesettings.PinKind
import com.slouchcat.core.browse.profilesettings.ProfileEditor
import com.slouchcat.core.browse.profilesettings.ProfileSettingsUiState
import com.slouchcat.core.browse.profilesettings.ProfileSettingsViewModel
import com.slouchcat.core.design.Avatar
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.ProfileAccents
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatChip
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTextField
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.SwitchRow
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.strings.R
import java.util.Locale

/** The events of the page and its sheet, one object so the screen keeps a short signature. */
interface ProfileSettingsEvents {
    fun onBack()
    fun onRetry()
    fun onSwitchProfile()
    fun onAdvice()
    fun onAdd()
    fun onEdit(profile: Profile)
    fun onDismiss()
    fun onName(name: String)
    fun onAccent(accent: Accent)
    fun onRating(age: Int)
    fun onToggleLanguages()
    fun onLanguage(language: Language)
    fun onAllLibraries(all: Boolean)
    fun onLibrary(id: Long, on: Boolean)
    fun onWatchLimit(text: String)
    fun onCanRequest(on: Boolean)
    fun onPinForm(kind: PinKind?)
    fun onPinFirst(digits: String)
    fun onPinSecond(digits: String)
    fun onPinApply()
    fun onPinRemove(kind: PinKind)
    fun onDeleteAsk(ask: Boolean)
    fun onDelete()
    fun onSave()
}

/**
 * «Profile verwalten» (mockup `/settings/profiles`, reference photos handy 08 and 68): the PIN
 * recommendation (E-9, F-19, the web app's texts), the profiles as rows (avatar, name, «Hauptprofil ·
 * Freigabe 18 · ohne PIN», the lock), «Profil hinzufügen»; a row opens the sheet. Another profile
 * than the primary one sees the rows only, with the note and «Profil wechseln».
 */
@Composable
fun ProfileSettingsScreen(state: ProfileSettingsUiState, snackbar: SnackbarHostState, events: ProfileSettingsEvents, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Box(modifier.fillMaxSize().background(c.bg)) {
        Column(Modifier.fillMaxSize()) {
            TopBar(stringResource(R.string.app_profiles_manage_title), navigationLabel = stringResource(R.string.common_back), onNavigation = events::onBack)
            Column(
                Modifier
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = gutter())
                    .padding(top = 8.dp, bottom = 32.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Column(Modifier.widthIn(max = 728.dp).fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(14.dp)) {
                    if (state.canManage) Manage(state, events) else ReadOnly(state, events)
                }
            }
        }
        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter))
    }
    state.editor?.let { Editor(it, state, events) }
}

@Composable
private fun Manage(state: ProfileSettingsUiState, events: ProfileSettingsEvents) {
    LoadableContent(state.profiles, onRetry = events::onRetry) { list ->
        Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
            if (state.advisePin) Advice(events::onAdvice)
            RowGroup(
                rows = list.map { profile ->
                    {
                        ProfileRow(
                            name = profile.name,
                            accent = profile.accent?.value,
                            subtitle = profileLine(profile.isPrimary, ratingText(state, profile.maxRating), profile.hasPin, profile.hasOfflinePin),
                            locked = profile.hasPin,
                            onClick = { events.onEdit(profile) },
                        )
                    }
                },
            )
            SlouchcatButton(stringResource(R.string.profiles_settings_add), onClick = events::onAdd, kind = ButtonKind.Secondary, leadingIcon = SlouchcatIcon.Plus)
        }
    }
}

@Composable
private fun ReadOnly(state: ProfileSettingsUiState, events: ProfileSettingsEvents) {
    Note(stringResource(R.string.profiles_settings_onlyPrimary_body), icon = SlouchcatIcon.Lock, title = stringResource(R.string.profiles_settings_onlyPrimary_title))
    LoadableContent(state.summaries, onRetry = events::onRetry) { list: List<ProfileSummary> ->
        RowGroup(
            rows = list.map { profile ->
                { ProfileRow(profile.name, profile.accent?.value, profileLine(profile.isPrimary, null, profile.hasPin, false), profile.hasPin, onClick = null) }
            },
        )
    }
    SlouchcatButton(stringResource(R.string.profiles_settings_onlyPrimary_action), onClick = events::onSwitchProfile, kind = ButtonKind.Secondary, leadingIcon = SlouchcatIcon.Users)
}

/** `.su-rec`: the recommendation with «PIN setzen» (web `profiles.pinAdvice`). */
@Composable
private fun Advice(onSet: () -> Unit) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(14.dp)
    Row(
        Modifier
            .fillMaxWidth()
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.warn), shape)
            .padding(16.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        SlouchcatIconImage(SlouchcatIcon.Shield, contentDescription = null, size = 20.dp, tint = c.warn)
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Text(stringResource(R.string.profiles_pinAdvice_title), style = SlouchcatType.strong, color = c.tx)
            Text(stringResource(R.string.profiles_pinAdvice_body), style = SlouchcatType.bodySmall, color = c.tx2)
            SlouchcatButton(stringResource(R.string.profiles_pinAdvice_action), onClick = onSet, kind = ButtonKind.Secondary, small = true, leadingIcon = SlouchcatIcon.Lock)
        }
    }
}

/** «Freigabe 16» with the label of the rating country where it has one («FSK 16»). */
@Composable
private fun ratingText(state: ProfileSettingsUiState, age: Int): String =
    stringResource(R.string.app_profiles_manage_rating, state.ratingLabel(age) ?: age.toString())

/** «Hauptprofil · Freigabe 18 · ohne PIN · Offline-PIN» (mockup `profSub`). */
@Composable
private fun profileLine(primary: Boolean, rating: String?, pin: Boolean, offline: Boolean): String = listOfNotNull(
    if (primary) stringResource(R.string.profiles_settings_primary) else null,
    rating,
    stringResource(if (pin) R.string.profiles_settings_hasPin else R.string.app_profiles_manage_noPin),
    if (offline) stringResource(R.string.offline_pin_offlineLabel) else null,
).joinToString(stringResource(R.string.format_separator))

@Composable
private fun ProfileRow(name: String, accent: String?, subtitle: String, locked: Boolean, onClick: (() -> Unit)?) {
    val c = SlouchcatTheme.colors
    val label = stringResource(R.string.app_profiles_manage_editLabel, name)
    val said = label + stringResource(R.string.format_listSeparator) + subtitle
    Row(
        Modifier
            .fillMaxWidth()
            .then(if (onClick != null) Modifier.clickable(role = Role.Button, onClickLabel = label, onClick = onClick) else Modifier)
            .semantics(mergeDescendants = true) { if (onClick != null) contentDescription = said }
            .defaultMinSize(minHeight = 60.dp)
            .padding(horizontal = 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Avatar(name, accent, size = 40.dp)
        Column(Modifier.weight(1f)) {
            Text(name, style = SlouchcatType.rowTitle, color = c.tx)
            Text(subtitle, style = SlouchcatType.caption, color = c.tx3)
        }
        if (locked) SlouchcatIconImage(SlouchcatIcon.Lock, contentDescription = null, size = 18.dp, tint = c.tx3)
        if (onClick != null) SlouchcatIconImage(SlouchcatIcon.Right, contentDescription = null, size = 18.dp, tint = c.tx3)
    }
}

/** The name of a language in that language («Deutsch», «Français»), as the web app's list. */
fun languageName(language: Language): String {
    val locale = Locale.forLanguageTag(language.value)
    return locale.getDisplayName(locale).replaceFirstChar { it.titlecase(locale) }
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
        Accent.ROSE -> R.string.accent_rose
        else -> R.string.accent_ember
    },
)

/** The text of a refused request: the server's code where the web app has one, else the general one. */
@Composable
private fun errorText(code: String): String = stringResource(
    when (code) {
        "validation_failed" -> R.string.errors_code_validation_failed
        "primary_profile_required" -> R.string.errors_code_primary_profile_required
        "primary_profile" -> R.string.errors_code_primary_profile
        ProfileSettingsViewModel.NOT_FOUND -> R.string.errors_code_not_found
        ProfileSettingsViewModel.OFFLINE -> R.string.app_state_offline
        else -> R.string.app_state_failed
    },
)

@Composable
private fun Label(text: String) {
    Text(text, style = SlouchcatType.label, color = SlouchcatTheme.colors.tx2, modifier = Modifier.padding(start = 2.dp, top = 6.dp).semantics { heading() })
}

/**
 * «Profil bearbeiten» / «Neues Profil» (mockup `editHtml`, reference photo handy 68): name, colour,
 * rating with its hint, language, libraries, watch time per day, «Wünsche» with «Darf Titel
 * anfordern» (while the wishes are on, as the web form), «Schutz» with PIN and offline PIN
 * (each with its own form), «Profil löschen» with its warning, «Abbrechen» and «Speichern».
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Editor(editor: ProfileEditor, state: ProfileSettingsUiState, events: ProfileSettingsEvents) {
    val c = SlouchcatTheme.colors
    val title = stringResource(if (editor.isNew) R.string.profiles_settings_form_createTitle else R.string.profiles_settings_form_editTitle)
    SlouchcatSheet(title, onDismiss = events::onDismiss) {
        Column(Modifier.padding(horizontal = gutter()).imePadding(), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                Avatar(editor.name.trim(), editor.accent.value, size = 56.dp)
                Text(title, style = SlouchcatType.sheetTitle, color = c.tx, modifier = Modifier.semantics { heading() })
            }
            SlouchcatTextField(
                value = editor.name,
                onValueChange = events::onName,
                label = stringResource(R.string.profiles_settings_form_name),
                error = if (editor.nameMissing) stringResource(R.string.app_profiles_manage_nameMissing) else null,
            )
            Label(stringResource(R.string.accent_label))
            FlowRow(Modifier.selectableGroup(), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                ProfileSettingsViewModel.ACCENTS.forEach { accent -> Swatch(accent, accentName(accent), editor.accent == accent) { events.onAccent(accent) } }
            }
            Label(stringResource(R.string.profiles_settings_form_rating))
            FlowRow(Modifier.selectableGroup(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                val ages = (state.ratings.map { it.age } + editor.rating).distinct().sorted()
                ages.forEach { age ->
                    val choice = state.ratings.firstOrNull { it.age == age }
                    val label = choice?.label ?: age.toString()
                    val enabled = choice?.enabled ?: true
                    val above = label + stringResource(R.string.format_listSeparator) + stringResource(R.string.app_profiles_manage_aboveAccount)
                    // Above the account's ceiling: no control (disabled, code review m-7). Only a mirror of
                    // what the server enforces (`validation_failed`); the server stays the authority (L-11).
                    SlouchcatChip(
                        label,
                        selected = editor.rating == age,
                        onClick = { events.onRating(age) },
                        icon = if (editor.rating == age) SlouchcatIcon.Check else null,
                        enabled = enabled,
                        modifier = Modifier
                            .heightIn(min = 48.dp)
                            .then(if (!enabled) Modifier.semantics { contentDescription = above } else Modifier),
                    )
                }
            }
            Text(
                if (editor.rating >= ProfileSettingsUiState.NO_AGE_LIMIT) {
                    stringResource(R.string.app_profiles_manage_ratingAll)
                } else {
                    stringResource(R.string.app_profiles_manage_ratingUpTo, state.ratingLabel(editor.rating) ?: editor.rating.toString())
                },
                style = SlouchcatType.caption,
                color = c.tx2,
            )
            Label(stringResource(R.string.profiles_settings_form_language))
            Languages(editor, events)
            if (state.libraries.isNotEmpty()) {
                Label(stringResource(R.string.profiles_settings_form_libraries))
                Framed {
                    SwitchRow(stringResource(R.string.app_profiles_manage_allLibraries), checked = editor.allLibraries, onCheckedChange = events::onAllLibraries)
                    if (!editor.allLibraries) {
                        state.libraries.forEach { library ->
                            SwitchRow(library.name, checked = library.id in editor.libraryIds, onCheckedChange = { events.onLibrary(library.id, it) })
                        }
                    }
                }
            }
            SlouchcatTextField(
                value = editor.watchLimit,
                onValueChange = events::onWatchLimit,
                label = stringResource(R.string.limits_watch),
                hint = stringResource(R.string.limits_hint, "1", ProfileSettingsViewModel.MAX_WATCH_MINUTES.toString()),
                error = if (editor.watchLimitInvalid) stringResource(R.string.app_profiles_manage_watchLimitInvalid) else null,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
            )
            if (state.requestShown) {
                Label(stringResource(R.string.profiles_settings_form_wishes))
                Framed {
                    SwitchRow(
                        stringResource(R.string.profiles_settings_form_canRequest),
                        checked = state.requestAllowed && editor.canRequest,
                        onCheckedChange = events::onCanRequest,
                        enabled = state.requestAllowed,
                    )
                }
                Text(
                    stringResource(if (state.requestAllowed) R.string.profiles_settings_form_canRequestHelp else R.string.profiles_settings_form_canRequestCapped),
                    style = SlouchcatType.caption,
                    color = c.tx2,
                )
            }
            Label(stringResource(R.string.app_profiles_manage_protection))
            PinRow(PinKind.Pin, editor, events)
            PinRow(PinKind.Offline, editor, events)
            if (!editor.isNew && !editor.isPrimary && !editor.confirmDelete) {
                SlouchcatButton(
                    stringResource(R.string.profiles_settings_delete_confirm),
                    onClick = { events.onDeleteAsk(true) },
                    kind = ButtonKind.Danger,
                    block = true,
                    leadingIcon = SlouchcatIcon.Trash,
                    modifier = Modifier.padding(top = 6.dp),
                )
            }
            if (editor.isPrimary && !editor.isNew) {
                Text(stringResource(R.string.app_profiles_manage_primaryNoDelete), style = SlouchcatType.caption, color = c.tx2)
            }
            if (editor.confirmDelete) {
                Note(stringResource(R.string.app_profiles_manage_deleteWarning, editor.name), warning = true, icon = SlouchcatIcon.Trash)
            }
            editor.error?.let { Note(errorText(it), warning = true) }
        }
        SheetActions {
            if (editor.confirmDelete) {
                SlouchcatButton(stringResource(R.string.app_profiles_manage_keep), onClick = { events.onDeleteAsk(false) }, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
                SlouchcatButton(stringResource(R.string.profiles_settings_delete_action), onClick = events::onDelete, kind = ButtonKind.Danger, busy = editor.busy, modifier = Modifier.weight(1f))
            } else {
                SlouchcatButton(stringResource(R.string.common_cancel), onClick = events::onDismiss, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
                SlouchcatButton(
                    stringResource(if (editor.isNew) R.string.profiles_settings_form_create else R.string.profiles_settings_form_save),
                    onClick = events::onSave,
                    kind = ButtonKind.Accent,
                    busy = editor.busy,
                    modifier = Modifier.weight(1f),
                )
            }
        }
    }
}

@Composable
private fun Framed(content: @Composable ColumnScope.() -> Unit) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(14.dp)
    Column(Modifier.fillMaxWidth().clip(shape).background(c.s1).border(BorderStroke(1.dp, c.frame), shape), content = content)
}

/** `.su-swatch`: a round colour of the contract's accents, the chosen one with a check. */
@Composable
private fun Swatch(accent: Accent, name: String, selected: Boolean, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    val colors = ProfileAccents.of(accent.value)
    Box(
        Modifier
            .size(48.dp)
            .clip(CircleShape)
            .selectable(selected = selected, role = Role.RadioButton, onClick = onClick)
            .semantics { contentDescription = name },
        contentAlignment = Alignment.Center,
    ) {
        Box(
            Modifier
                .size(if (selected) 40.dp else 36.dp)
                .clip(CircleShape)
                .background(colors.base)
                .then(if (selected) Modifier.border(BorderStroke(2.dp, c.tx), CircleShape) else Modifier),
            contentAlignment = Alignment.Center,
        ) {
            if (selected) SlouchcatIconImage(SlouchcatIcon.Check, contentDescription = null, size = 18.dp, tint = colors.on)
        }
    }
}

/** «Sprache: Deutsch ›»; open, the languages of the contract to choose from. */
@Composable
private fun Languages(editor: ProfileEditor, events: ProfileSettingsEvents) {
    val c = SlouchcatTheme.colors
    val languages = remember { Language.entries.filter { it != Language.UNKNOWN_DEFAULT_OPEN_API } }
    val said = stringResource(R.string.app_profiles_manage_languageLabel, languageName(editor.language))
    Framed {
        Row(
            Modifier
                .fillMaxWidth()
                .clickable(role = Role.Button, onClick = events::onToggleLanguages)
                .semantics(mergeDescendants = true) { contentDescription = said }
                .heightIn(min = 52.dp)
                .padding(horizontal = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(languageName(editor.language), style = SlouchcatType.body, color = c.tx, modifier = Modifier.weight(1f))
            SlouchcatIconImage(if (editor.languagesOpen) SlouchcatIcon.Up else SlouchcatIcon.Down, contentDescription = null, size = 18.dp, tint = c.tx3)
        }
        if (editor.languagesOpen) {
            Column(Modifier.selectableGroup()) {
                languages.forEach { language ->
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .selectable(selected = language == editor.language, role = Role.RadioButton, onClick = { events.onLanguage(language) })
                            .heightIn(min = 48.dp)
                            .padding(horizontal = 16.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(languageName(language), style = SlouchcatType.body, color = c.tx, modifier = Modifier.weight(1f))
                        if (language == editor.language) SlouchcatIconImage(SlouchcatIcon.Check, contentDescription = null, size = 18.dp, tint = c.accent)
                    }
                }
            }
        }
    }
}

/** `.su-pinrow` with its form (`.su-pinset`): PIN or offline PIN, set, change, remove. */
@Composable
private fun PinRow(kind: PinKind, editor: ProfileEditor, events: ProfileSettingsEvents) {
    val c = SlouchcatTheme.colors
    val label = stringResource(if (kind == PinKind.Pin) R.string.profiles_settings_hasPin else R.string.offline_pin_offlineLabel)
    val on = editor.pinOn(kind)
    val change = if (kind == PinKind.Pin) editor.pin else editor.offlinePin
    val shape = RoundedCornerShape(14.dp)
    Column(
        Modifier
            .fillMaxWidth()
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.frame), shape)
            .padding(14.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        if (editor.pinForm == kind) {
            Text(
                stringResource(if (on) R.string.app_profiles_manage_changeTitle else R.string.app_profiles_manage_setTitle, label),
                style = SlouchcatType.strong,
                color = c.tx,
            )
            if (kind == PinKind.Offline) Text(stringResource(R.string.app_profiles_manage_offlineExplain), style = SlouchcatType.caption, color = c.tx2)
            val error = when (editor.pinError) {
                PinError.Format -> stringResource(if (kind == PinKind.Pin) R.string.errors_field_pin_format else R.string.offline_pin_offlineFormat)
                PinError.Mismatch -> stringResource(R.string.app_profiles_manage_mismatch)
                null -> null
            }
            SlouchcatTextField(
                value = editor.first,
                onValueChange = events::onPinFirst,
                label = stringResource(R.string.profiles_settings_pin_label),
                hint = stringResource(if (kind == PinKind.Pin) R.string.profiles_pin_hint else R.string.offline_pin_offlineHint),
                error = error,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
                visualTransformation = PasswordVisualTransformation(),
            )
            SlouchcatTextField(
                value = editor.second,
                onValueChange = events::onPinSecond,
                label = stringResource(R.string.app_profiles_manage_repeat),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
                visualTransformation = PasswordVisualTransformation(),
            )
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                SlouchcatButton(stringResource(R.string.common_cancel), onClick = { events.onPinForm(null) }, kind = ButtonKind.Ghost, small = true)
                SlouchcatButton(stringResource(R.string.app_profiles_manage_apply), onClick = events::onPinApply, kind = ButtonKind.Primary, small = true)
            }
        } else {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Box(Modifier.size(40.dp).clip(RoundedCornerShape(12.dp)).background(c.s2), contentAlignment = Alignment.Center) {
                    SlouchcatIconImage(if (on) SlouchcatIcon.Lock else SlouchcatIcon.Key, contentDescription = null, size = 20.dp, tint = c.tx2)
                }
                Column(Modifier.weight(1f)) {
                    Text(label, style = SlouchcatType.rowTitle, color = c.tx)
                    Text(
                        when {
                            change is PinChange.Set -> stringResource(R.string.app_profiles_manage_pendingSet)
                            change == PinChange.Remove -> stringResource(R.string.app_profiles_manage_pendingRemove)
                            on -> stringResource(R.string.app_profiles_manage_pinOn)
                            kind == PinKind.Pin -> stringResource(R.string.app_profiles_manage_pinOff)
                            else -> stringResource(R.string.app_profiles_manage_offlineOff)
                        },
                        style = SlouchcatType.caption,
                        color = c.tx2,
                    )
                }
                if (!on) SlouchcatButton(stringResource(R.string.app_profiles_manage_set), onClick = { events.onPinForm(kind) }, kind = ButtonKind.Secondary, small = true)
            }
            if (on) {
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    SlouchcatButton(stringResource(R.string.app_profiles_manage_change), onClick = { events.onPinForm(kind) }, kind = ButtonKind.Ghost, small = true)
                    SlouchcatButton(stringResource(R.string.app_profiles_manage_remove), onClick = { events.onPinRemove(kind) }, kind = ButtonKind.Ghost, small = true)
                }
            }
        }
    }
}

/** «2 Profile» under «Profile verwalten» in «Mehr». */
@Composable
fun profilesSubtitle(count: Int?, canManage: Boolean): String = when {
    !canManage -> stringResource(R.string.app_profiles_manage_readOnly)
    count == null -> stringResource(R.string.profiles_settings_title)
    else -> pluralStringResource(R.plurals.app_profiles_manage_subtitle, count, count)
}
