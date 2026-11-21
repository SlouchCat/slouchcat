package com.slouchcat.app.tv.profiles

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.focusRestorer
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withStyle
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.common.Loadable
import com.slouchcat.app.tv.shell.tvPage
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.NoteTone
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDialog
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvIcons
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvNote
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.design.Avatar
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R

/**
 * «Wer schaut?» (mockup `/profiles`, photo 05): the profiles as large avatars in a row, a lock at
 * those with a PIN, «Profile verwalten» once that page exists ([onManage], stage 2b), and at the
 * foot which server and account this TV is connected with (K-9, required).
 */
@Composable
fun ProfilesScreen(
    state: ProfilesUiState,
    onPick: (ProfileSummary) -> Unit,
    onDigit: (Char) -> Unit,
    onDelete: () -> Unit,
    onSubmitPin: () -> Unit,
    onCancelPin: () -> Unit,
    onRetry: () -> Unit,
    onSignOut: () -> Unit,
    onManage: (() -> Unit)?,
    modifier: Modifier = Modifier,
) {
    val c = SlouchcatTheme.colors
    val focus = rememberFocusMemory()
    val profiles = state.profiles
    val first = (profiles as? Loadable.Ready)?.value?.let { list -> list.firstOrNull { it.id == state.current } ?: list.firstOrNull() }
    Box(modifier.fillMaxSize().tvPage()) {
        Column(Modifier.align(Alignment.Center), horizontalAlignment = Alignment.CenterHorizontally) {
            Text(stringResource(R.string.profiles_picker_title), style = TvType.h1, textAlign = TextAlign.Center, modifier = Modifier.semantics { heading() })
            LoadableContent(
                profiles,
                onRetry = onRetry,
                modifier = Modifier.padding(top = TvDimens.Profiles.RowTop),
                retryModifier = Modifier.focusKey(focus, "retry"),
            ) { list ->
                val rowFallback = remember { FocusRequester() }
                Row(
                    Modifier.padding(top = TvDimens.Profiles.RowTop).focusRestorer(rowFallback),
                    horizontalArrangement = Arrangement.spacedBy(TvDimens.Profiles.Gap),
                ) {
                    for (profile in list) {
                        ProfileTile(
                            profile,
                            busy = state.selecting == profile.id,
                            modifier = Modifier
                                .then(if (profile == first) Modifier.focusRequester(rowFallback) else Modifier)
                                .focusKey(focus, "profile-${profile.id}"),
                            onClick = { onPick(profile) },
                        )
                    }
                }
            }
            if (state.error != null) {
                TvNote(
                    stringResource(if (state.error == ProfilesError.Unreachable) R.string.app_state_offline else R.string.app_state_failed),
                    tone = NoteTone.Warn,
                    modifier = Modifier.padding(top = TvDimens.l),
                )
            }
            Row(Modifier.padding(top = TvDimens.Profiles.ActionsTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
                if (onManage != null && profiles is Loadable.Ready) {
                    TvButton(stringResource(R.string.profiles_picker_manage), onClick = onManage, icon = TvIcons.Edit, kind = TvButtonKind.Ghost, modifier = Modifier.focusKey(focus, "manage"))
                }
                if (profiles is Loadable.Failed) {
                    TvButton(
                        stringResource(R.string.tv_settings_signOut),
                        onClick = onSignOut,
                        icon = SlouchcatIcon.Logout.vector,
                        kind = TvButtonKind.Ghost,
                        enabled = !state.signingOut,
                        modifier = Modifier.focusKey(focus, "signout"),
                    )
                }
            }
        }
        Connected(state.server, state.email, Modifier.align(Alignment.BottomCenter))
    }
    state.pin?.let { pin -> PinDialog(pin, onDigit, onDelete, onSubmitPin, onCancelPin) }
    val start = when {
        first != null -> "profile-${first.id}"
        profiles is Loadable.Failed -> "retry"
        else -> "retry"
    }
    focus.StartFocus(start, profiles::class, first?.id, state.pin == null)
}

/** `.pf`: avatar, lock and name; in focus the ring of the mockup and the full colour. */
@Composable
private fun ProfileTile(profile: ProfileSummary, busy: Boolean, modifier: Modifier, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    val p = TvDimens.Profiles
    val source = remember { MutableInteractionSource() }
    val focused by source.collectIsFocusedAsState()
    val locked = stringResource(R.string.profiles_picker_locked)
    Surface(
        onClick = onClick,
        modifier = modifier.width(p.Tile).semantics { contentDescription = if (profile.hasPin) "${profile.name}, $locked" else profile.name },
        shape = ClickableSurfaceDefaults.shape(shape = RoundedCornerShape(TvDimens.radiusPanel)),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = Color.Transparent,
            contentColor = c.tx2,
            focusedContainerColor = Color.Transparent,
            focusedContentColor = c.tx,
            pressedContainerColor = Color.Transparent,
            pressedContentColor = c.tx,
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
        interactionSource = source,
    ) {
        Column(Modifier.padding(vertical = TvDimens.s), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(TvDimens.gap)) {
            Box(contentAlignment = Alignment.Center) {
                // The ring: a gap in the page colour, then a ring in the text colour (`.pf.is-f .pf-av`).
                Box(
                    Modifier
                        .size(p.Avatar + TvDimens.s * 2)
                        .border(BorderStroke(TvDimens.borderFocus, if (focused) c.tx else Color.Transparent), CircleShape),
                )
                Avatar(profile.name, profile.accent?.value, size = p.Avatar, modifier = Modifier.alpha(if (focused) 1f else TvMotion.profileRest))
                if (profile.hasPin) {
                    Box(
                        Modifier
                            .align(Alignment.BottomEnd)
                            .size(p.Lock)
                            .clip(CircleShape)
                            .background(c.bg)
                            .padding(TvDimens.borderThin)
                            .clip(CircleShape)
                            .background(c.elev),
                        contentAlignment = Alignment.Center,
                    ) { TvIcon(SlouchcatIcon.Lock, p.LockIcon, tint = c.tx) }
                }
            }
            Text(profile.name, style = TvType.largeStrong.copy(fontWeight = FontWeight.SemiBold), maxLines = 1)
            if (busy) Text(stringResource(R.string.app_state_loading), style = TvType.small, color = c.tx3)
        }
    }
}

/** `.pf-conn`: «Verbunden mit **Server** · Konto» (K-9). */
@Composable
private fun Connected(host: String, email: String, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val text = stringResource(R.string.tv_profiles_connected, host, email)
    val start = text.indexOf(host)
    Row(modifier.semantics(mergeDescendants = true) {}, horizontalArrangement = Arrangement.spacedBy(TvDimens.s), verticalAlignment = Alignment.CenterVertically) {
        TvIcon(SlouchcatIcon.Server, TvDimens.Badge.Icon, tint = c.tx3)
        Text(
            buildAnnotatedString {
                append(text)
                if (start >= 0 && host.isNotEmpty()) addStyle(SpanStyle(color = c.tx2, fontWeight = FontWeight.SemiBold), start, start + host.length)
            },
            style = TvType.small,
            color = c.tx3,
        )
    }
}

/**
 * The PIN pad (mockup `ACT.pfPick`, photos 56, 57): digits with the D-pad, also the digit keys of a
 * remote; the dots show how many digits are in (4 to 8, `SelectProfileRequest.pin`). «Öffnen»
 * sends the PIN; Back or «Abbrechen» closes the pad.
 */
@Composable
private fun PinDialog(pin: PinState, onDigit: (Char) -> Unit, onDelete: () -> Unit, onSubmit: () -> Unit, onCancel: () -> Unit) {
    val c = SlouchcatTheme.colors
    val p = TvDimens.Profiles
    val first = remember { FocusRequester() }
    val title = stringResource(R.string.profiles_pin_title, pin.profile.name)
    TvDialog(title = title, onDismiss = onCancel, initialFocus = first, showTitle = false) {
        Column(
            Modifier
                .align(Alignment.CenterHorizontally)
                .onPreviewKeyEvent { event ->
                    if (event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
                    val digit = DIGIT_KEYS[event.key]
                    when {
                        digit != null -> onDigit(digit).let { true }
                        event.key == Key.Backspace || event.key == Key.Delete -> onDelete().let { true }
                        else -> false
                    }
                },
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Avatar(pin.profile.name, pin.profile.accent?.value, size = p.PinAvatar)
            Text(title, style = TvType.h2, modifier = Modifier.padding(top = TvDimens.gap).semantics { heading() })
            val shown = maxOf(PinState.MIN_DIGITS, pin.digits.length)
            Row(
                Modifier.padding(top = TvDimens.gap),
                horizontalArrangement = Arrangement.spacedBy(p.PinDotsGap),
            ) {
                repeat(shown) { index ->
                    val on = index < pin.digits.length
                    Box(
                        Modifier
                            .size(p.PinDot)
                            .clip(CircleShape)
                            .background(if (on) c.tx else Color.Transparent)
                            .border(BorderStroke(TvDimens.borderThick, if (on) c.tx else c.tx2), CircleShape),
                    )
                }
            }
            val entered = pluralStringResource(R.plurals.app_profiles_pinEntered, pin.digits.length, pin.digits.length)
            val message = pinMessageText(pin)
            Row(
                Modifier
                    .heightIn(min = p.PinMessage)
                    .padding(top = TvDimens.s)
                    .semantics(mergeDescendants = true) { liveRegion = LiveRegionMode.Polite; contentDescription = "$message $entered" },
                horizontalArrangement = Arrangement.spacedBy(TvDimens.s),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                if (pin.message == PinMessage.Invalid || pin.message == PinMessage.Failed || pin.message == PinMessage.Unreachable) {
                    TvIcon(SlouchcatIcon.Alert, TvDimens.Badge.Icon, tint = c.danger)
                }
                if (pin.locked) TvIcon(SlouchcatIcon.Lock, TvDimens.Badge.Icon, tint = c.tx2)
                Text(message, style = TvType.body, color = c.tx2)
            }
            PinPad(pin, first, onDigit, onDelete, onSubmit)
            // After a wrong PIN «Öffnen» is off again: the focus goes back to the digits.
            LaunchedEffect(pin.message, pin.locked) {
                if (pin.message != PinMessage.Prompt && !pin.canSubmit) runCatching { first.requestFocus() }
            }
            TvButton(stringResource(R.string.common_cancel), onClick = onCancel, kind = TvButtonKind.Ghost, small = true, modifier = Modifier.padding(top = TvDimens.l))
        }
    }
}

@Composable
private fun pinMessageText(pin: PinState): String = when (pin.message) {
    PinMessage.Prompt -> stringResource(R.string.tv_pin_prompt)
    PinMessage.Invalid -> stringResource(R.string.profiles_pin_invalid)
    PinMessage.Locked -> stringResource(R.string.app_profiles_pinLocked, pin.lockedSeconds)
    PinMessage.Again -> stringResource(R.string.app_profiles_pinAgain)
    PinMessage.Unreachable -> stringResource(R.string.app_state_offline)
    PinMessage.Failed -> stringResource(R.string.app_state_failed)
}

/** `.pin-pad`: 1 to 9, then «löschen», 0 and «Öffnen» (a PIN has 4 to 8 digits, so it needs a key to send). */
@Composable
private fun PinPad(pin: PinState, first: FocusRequester, onDigit: (Char) -> Unit, onDelete: () -> Unit, onSubmit: () -> Unit) {
    val rows = listOf("123", "456", "789")
    val p = TvDimens.Profiles
    val usable = !pin.locked && !pin.checking
    Column(
        Modifier.padding(top = TvDimens.s).alpha(if (pin.locked) TvMotion.disabled else 1f).focusRestorer(first),
        verticalArrangement = Arrangement.spacedBy(p.PinGap),
    ) {
        for (row in rows) {
            Row(horizontalArrangement = Arrangement.spacedBy(p.PinGap)) {
                for (digit in row) {
                    PinKey(digit.toString(), onClick = { onDigit(digit) }, enabled = usable, modifier = if (digit == '1') Modifier.focusRequester(first) else Modifier)
                }
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(p.PinGap)) {
            PinKey(stringResource(R.string.app_profiles_pinDelete), onClick = onDelete, enabled = usable, icon = true, function = true)
            PinKey("0", onClick = { onDigit('0') }, enabled = usable)
            PinKey(stringResource(R.string.profiles_pin_submit), onClick = onSubmit, enabled = pin.canSubmit, function = true, accent = true)
        }
    }
}

@Composable
private fun PinKey(
    label: String,
    onClick: () -> Unit,
    enabled: Boolean,
    modifier: Modifier = Modifier,
    icon: Boolean = false,
    function: Boolean = false,
    accent: Boolean = false,
) {
    val c = SlouchcatTheme.colors
    val p = TvDimens.Profiles
    val shape = RoundedCornerShape(TvDimens.radiusField)
    Surface(
        onClick = onClick,
        enabled = enabled,
        modifier = modifier
            .size(p.PinKeyWidth, p.PinKeyHeight)
            .alpha(if (enabled || !function) 1f else TvMotion.disabled)
            .semantics { if (icon) contentDescription = label },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = if (accent) c.accent else if (function) c.s3 else c.s2,
            contentColor = if (accent) c.onAccent else c.tx,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
            disabledContainerColor = if (accent) c.accent else if (function) c.s3 else c.s2,
            disabledContentColor = if (accent) c.onAccent else c.tx,
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
    ) {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            if (icon) {
                TvIcon(TvIcons.Backspace, TvDimens.Menu.Icon)
            } else {
                Text(label, style = if (function) TvType.label else TvType.pin)
            }
        }
    }
}

/** The digit keys of a remote and of a keyboard. */
private val DIGIT_KEYS: Map<Key, Char> = mapOf(
    Key.Zero to '0', Key.One to '1', Key.Two to '2', Key.Three to '3', Key.Four to '4',
    Key.Five to '5', Key.Six to '6', Key.Seven to '7', Key.Eight to '8', Key.Nine to '9',
    Key.NumPad0 to '0', Key.NumPad1 to '1', Key.NumPad2 to '2', Key.NumPad3 to '3', Key.NumPad4 to '4',
    Key.NumPad5 to '5', Key.NumPad6 to '6', Key.NumPad7 to '7', Key.NumPad8 to '8', Key.NumPad9 to '9',
)
