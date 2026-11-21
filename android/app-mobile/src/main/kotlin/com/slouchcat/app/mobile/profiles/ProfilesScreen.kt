package com.slouchcat.app.mobile.profiles

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.onClick
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.core.api.generated.models.ProfileSummary
import com.slouchcat.core.design.Avatar
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.Wordmark
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.strings.R

/** «Wer schaut?» (mockup `/profiles`, reference photos 04-profiles): the tiles of the account's profiles. */
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
    modifier: Modifier = Modifier,
) {
    val c = SlouchcatTheme.colors
    BoxWithConstraints(modifier.fillMaxSize().background(c.bg)) {
        Column(
            Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .heightIn(min = maxHeight)
                .systemBarsPadding()
                .padding(horizontal = 24.dp, vertical = 12.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            // Wordmark at the top, the choice in the middle, the account at the bottom (mockup `.su-who`).
            verticalArrangement = Arrangement.SpaceBetween,
        ) {
            Wordmark(stringResource(R.string.app_name), modifier = Modifier.padding(top = 4.dp))
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Text(
                    stringResource(R.string.profiles_picker_title),
                    style = SlouchcatType.wordmark.copy(fontSize = 32.sp, lineHeight = 38.sp, fontWeight = FontWeight.Bold),
                    color = c.tx,
                    textAlign = TextAlign.Center,
                    modifier = Modifier.padding(top = 24.dp, bottom = 32.dp).semantics { heading() },
                )
                LoadableContent(state.profiles, onRetry = onRetry, modifier = Modifier.widthIn(max = 560.dp)) { profiles ->
                    Tiles(profiles, state, onPick)
                }
                state.error?.let {
                    Note(
                        stringResource(if (it == ProfilesError.Unreachable) R.string.app_setup_error_unreachable else R.string.app_state_failed),
                        warning = true,
                        modifier = Modifier.padding(top = 16.dp).widthIn(max = 460.dp),
                    )
                }
                SlouchcatButton(
                    stringResource(R.string.nav_logout),
                    onClick = onSignOut,
                    kind = ButtonKind.Ghost,
                    busy = state.signingOut,
                    leadingIcon = SlouchcatIcon.Logout,
                    modifier = Modifier.padding(top = 28.dp),
                )
            }
            Row(
                Modifier.padding(top = 24.dp, bottom = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                if (state.email.isNotEmpty()) {
                    SlouchcatIconImage(SlouchcatIcon.User, contentDescription = null, size = 16.dp, tint = c.tx2)
                    Text(stringResource(R.string.app_profiles_account, state.email, state.host), style = SlouchcatType.caption, color = c.tx2, textAlign = TextAlign.Center)
                }
            }
        }
    }
    state.pin?.let { PinSheet(it, onDigit, onDelete, onSubmitPin, onCancelPin) }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Tiles(profiles: List<ProfileSummary>, state: ProfilesUiState, onPick: (ProfileSummary) -> Unit) {
    val wide = isWideLayout()
    FlowRow(
        Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(if (wide) 40.dp else 28.dp, Alignment.CenterHorizontally),
        verticalArrangement = Arrangement.spacedBy(28.dp),
    ) {
        for (profile in profiles) Tile(profile, current = profile.id == state.current, busy = profile.id == state.selecting, onClick = { onPick(profile) })
    }
}

/** `.su-tile`: avatar (ring when it was active last, lock when it has a PIN), name, role. */
@Composable
private fun Tile(profile: ProfileSummary, current: Boolean, busy: Boolean, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    val parts = buildList {
        add(profile.name)
        if (profile.isPrimary) add(stringResource(R.string.profiles_picker_primary))
        if (profile.hasPin) add(stringResource(R.string.profiles_picker_locked))
        if (current) add(stringResource(R.string.profiles_picker_active))
    }
    val label = parts.joinToString(", ")
    Column(
        Modifier
            .width(128.dp)
            .clip(androidx.compose.foundation.shape.RoundedCornerShape(16.dp))
            .clickable(role = Role.Button, onClick = onClick)
            .clearAndSetSemantics {
                contentDescription = label
                role = Role.Button
                onClick { onClick(); true }
            }
            .padding(vertical = 8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(contentAlignment = Alignment.Center) {
            Box(
                Modifier
                    .size(104.dp)
                    .then(if (current) Modifier.border(BorderStroke(3.dp, c.tx), CircleShape) else Modifier),
                contentAlignment = Alignment.Center,
            ) {
                Avatar(profile.name, profile.accent?.value, size = 92.dp)
            }
            if (profile.hasPin) {
                Box(
                    Modifier
                        .align(Alignment.BottomEnd)
                        .padding(end = 2.dp, bottom = 4.dp)
                        .size(30.dp)
                        .clip(CircleShape)
                        .background(c.s3)
                        .border(BorderStroke(2.dp, c.bg), CircleShape),
                    contentAlignment = Alignment.Center,
                ) {
                    SlouchcatIconImage(SlouchcatIcon.Lock, contentDescription = null, size = 15.dp, tint = c.tx)
                }
            }
            if (busy) CircularProgressIndicator(color = c.tx, modifier = Modifier.size(104.dp))
        }
        Text(profile.name, style = SlouchcatType.strong, color = c.tx, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 10.dp))
        if (profile.isPrimary) {
            Text(stringResource(R.string.profiles_picker_primary), style = SlouchcatType.caption, color = c.tx2, textAlign = TextAlign.Center)
        }
    }
}

/** The PIN pad (mockup `#su-pin`): dots, the message, digits, «Öffnen» and delete. */
@Composable
private fun PinSheet(pin: PinState, onDigit: (Char) -> Unit, onDelete: () -> Unit, onSubmit: () -> Unit, onCancel: () -> Unit) {
    val c = SlouchcatTheme.colors
    val title = stringResource(R.string.profiles_pin_title, pin.profile.name)
    SlouchcatSheet(title = title, onDismiss = onCancel) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                Avatar(pin.profile.name, pin.profile.accent?.value, size = 56.dp)
                Text(title, style = SlouchcatType.sheetTitle, color = c.tx, modifier = Modifier.semantics { heading() })
            }
            val entered = pluralStringResource(R.plurals.app_profiles_pinEntered, pin.digits.length, pin.digits.length)
            Row(
                Modifier.padding(top = 22.dp).clearAndSetSemantics { contentDescription = entered },
                horizontalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                repeat(maxOf(PinState.MIN_DIGITS, pin.digits.length)) { n ->
                    val on = n < pin.digits.length
                    Box(
                        Modifier
                            .size(14.dp)
                            .clip(CircleShape)
                            .background(if (on) c.tx else c.bg)
                            .border(BorderStroke(2.dp, if (on) c.tx else c.ctl), CircleShape),
                    )
                }
            }
            val message = when (pin.message) {
                PinMessage.Prompt -> stringResource(R.string.profiles_pin_hint)
                PinMessage.Invalid -> stringResource(R.string.profiles_pin_invalid)
                PinMessage.Locked -> stringResource(R.string.app_profiles_pinLocked, pin.lockedSeconds.toString())
                PinMessage.Again -> stringResource(R.string.app_profiles_pinAgain)
                PinMessage.Unreachable -> stringResource(R.string.app_setup_error_unreachable)
                PinMessage.Failed -> stringResource(R.string.app_state_failed)
            }
            val bad = pin.message in setOf(PinMessage.Invalid, PinMessage.Locked, PinMessage.Unreachable, PinMessage.Failed)
            Text(
                message,
                style = SlouchcatType.bodySmall,
                color = if (bad) c.danger else c.tx2,
                textAlign = TextAlign.Center,
                modifier = Modifier.padding(top = 16.dp, bottom = 12.dp).semantics { liveRegion = LiveRegionMode.Polite },
            )
            val enabled = !pin.checking && !pin.locked
            val rows = listOf(listOf('1', '2', '3'), listOf('4', '5', '6'), listOf('7', '8', '9'))
            for (keys in rows) {
                Row(horizontalArrangement = Arrangement.spacedBy(14.dp), modifier = Modifier.padding(vertical = 5.dp)) {
                    for (key in keys) Key(key.toString(), enabled = enabled, onClick = { onDigit(key) })
                }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(14.dp), modifier = Modifier.padding(vertical = 5.dp)) {
                IconKey(SlouchcatIcon.Check, stringResource(R.string.profiles_pin_submit), enabled = pin.canSubmit, onClick = onSubmit, strong = true)
                Key("0", enabled = enabled, onClick = { onDigit('0') })
                IconKey(SlouchcatIcon.Back, stringResource(R.string.app_profiles_pinDelete), enabled = enabled && pin.digits.isNotEmpty(), onClick = onDelete)
            }
        }
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = onCancel, kind = ButtonKind.Ghost, enabled = !pin.checking, modifier = Modifier.weight(1f))
        }
    }
}

@Composable
private fun Key(text: String, enabled: Boolean, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    Box(
        Modifier
            .size(72.dp)
            .clip(CircleShape)
            .background(c.s2)
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Text(text, style = SlouchcatType.pageTitle.copy(fontSize = 26.sp), color = if (enabled) c.tx else c.tx3)
    }
}

@Composable
private fun IconKey(icon: SlouchcatIcon, label: String, enabled: Boolean, onClick: () -> Unit, strong: Boolean = false) {
    val c = SlouchcatTheme.colors
    Box(
        Modifier
            .size(72.dp)
            .clip(CircleShape)
            .background(if (strong && enabled) c.tx else c.bg)
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick)
            .semantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) {
        SlouchcatIconImage(icon, contentDescription = null, size = 26.dp, tint = if (strong && enabled) c.bg else if (enabled) c.tx else c.tx3)
    }
}
