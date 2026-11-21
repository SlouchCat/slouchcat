package com.slouchcat.app.tv.setup

import androidx.activity.compose.BackHandler
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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.Text
import com.slouchcat.app.tv.shell.tvPage
import com.slouchcat.app.tv.ui.ConnectionBadge
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.NoteTone
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvBusy
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvNote
import com.slouchcat.app.tv.ui.TvSwitch
import com.slouchcat.app.tv.ui.TvTextField
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.TvWordmark
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R

/**
 * «Von Hand einrichten» (mockup `/setup/hand`, photos 04, 53 to 55): the steps on the left, the
 * current one on the right. Text goes in through the input of the system (A-7). Back goes one step
 * back, on the first step out of the page.
 */
@Composable
fun HandSetupScreen(
    state: HandUiState,
    onAddress: (String) -> Unit,
    onConsent: (Boolean) -> Unit,
    onNext: () -> Unit,
    onEmail: (String) -> Unit,
    onPassword: (String) -> Unit,
    onTogglePassword: () -> Unit,
    onSignIn: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val c = SlouchcatTheme.colors
    val focus = rememberFocusMemory()
    BackHandler(enabled = state.step != HandStep.Server || state.busy) { onBack() }
    Column(modifier.fillMaxSize().tvPage()) {
        Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
            TvWordmark(stringResource(R.string.app_name))
            Text(stringResource(R.string.tv_hand_subtitle), style = TvType.body, color = c.tx3)
        }
        Row(Modifier.padding(top = TvDimens.Setup.HandTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.Setup.ColsGap)) {
            Progress(state, Modifier.width(TvDimens.Setup.ProgressWidth))
            Column(Modifier.weight(1f)) {
                when (state.step) {
                    HandStep.Server -> ServerStep(state, focus, onAddress, onConsent, onNext)
                    HandStep.Email -> EmailStep(state, focus, onEmail, onNext)
                    HandStep.Password -> PasswordStep(state, focus, onPassword, onTogglePassword, onSignIn)
                }
            }
        }
    }
    val start = when (state.step) {
        HandStep.Server -> if (state.serverReady) "next" else if (state.check is AddressCheck.Found) "consent" else "url"
        HandStep.Email -> if (state.emailReady) "next" else "email"
        HandStep.Password -> if (state.password.isNotEmpty()) "signin" else "password"
    }
    focus.StartFocus(start, state.step)
    // A failed sign-in goes back to the password (mockup: `focusTo('su-pw')`).
    LaunchedEffect(state.error) {
        if (state.error != null && state.step == HandStep.Password) focus.focus("password")
    }
    // A finished check moves on like the mockup: to «Weiter», to the switch, or back to the field.
    LaunchedEffect(state.check?.let { it::class }, state.httpConsent) {
        if (state.step == HandStep.Server && state.check != null && state.check != AddressCheck.Busy) focus.focus(start)
    }
}

/** `.su-prog`: the three steps; the current one framed, the done ones with their value. */
@Composable
private fun Progress(state: HandUiState, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val p = TvDimens.Setup
    val steps = listOf(
        Triple(HandStep.Server, R.string.tv_hand_stepServer, R.string.tv_hand_stepServerHint),
        Triple(HandStep.Email, R.string.tv_hand_stepEmail, R.string.tv_hand_stepAccount),
        Triple(HandStep.Password, R.string.tv_hand_stepPassword, R.string.tv_hand_stepAccount),
    )
    Column(modifier.semantics(mergeDescendants = true) {}, verticalArrangement = Arrangement.spacedBy(TvDimens.s / 2 + TvDimens.xs / 2)) {
        steps.forEachIndexed { index, (step, title, hint) ->
            val current = step == state.step
            val done = step.ordinal < state.step.ordinal
            val value = when {
                !done -> null
                step == HandStep.Server -> state.found?.address?.host
                step == HandStep.Email -> state.email
                else -> null
            }
            Row(
                Modifier
                    .fillMaxWidth()
                    .heightIn(min = p.ProgressItem)
                    .clip(RoundedCornerShape(TvDimens.radiusMenuItem))
                    .background(if (current) c.s1 else c.bg)
                    .padding(horizontal = TvDimens.l / 2),
                horizontalArrangement = Arrangement.spacedBy(TvDimens.Menu.ItemPad - TvDimens.xs / 2),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                val circle = Modifier.size(p.ProgressNumber).clip(CircleShape)
                when {
                    current -> Box(circle.background(c.accent), contentAlignment = Alignment.Center) { Text("${index + 1}", style = TvType.bodyStrong, color = c.onAccent) }
                    done -> Box(circle.border(BorderStroke(TvDimens.borderThick, c.ok), CircleShape), contentAlignment = Alignment.Center) { TvIcon(SlouchcatIcon.Check, TvDimens.Button.Icon, tint = c.ok) }
                    else -> Box(circle.border(BorderStroke(TvDimens.borderThick, c.ctl), CircleShape), contentAlignment = Alignment.Center) { Text("${index + 1}", style = TvType.bodyStrong, color = c.tx3) }
                }
                Column {
                    Text(stringResource(title), style = TvType.largeStrong, color = if (current || done) c.tx else c.tx3)
                    Text(value ?: stringResource(hint), style = TvType.small, color = c.tx3, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
        }
    }
}

@Composable
private fun Heading(title: Int, lead: String) {
    val c = SlouchcatTheme.colors
    Text(stringResource(title), style = TvType.h1, modifier = Modifier.semantics { heading() })
    Text(lead, style = TvType.large, color = c.tx2, modifier = Modifier.padding(top = TvDimens.Setup.LeadTop).widthIn(max = TvDimens.Setup.LeadMax))
}

@Composable
private fun ServerStep(state: HandUiState, focus: FocusMemory, onAddress: (String) -> Unit, onConsent: (Boolean) -> Unit, onNext: () -> Unit) {
    val c = SlouchcatTheme.colors
    Heading(R.string.tv_hand_serverTitle, stringResource(R.string.tv_hand_serverLead))
    TvTextField(
        value = state.address,
        onValueChange = onAddress,
        label = stringResource(R.string.tv_hand_serverLabel),
        placeholder = stringResource(R.string.tv_hand_serverPlaceholder),
        keyboardType = KeyboardType.Uri,
        modifier = Modifier.padding(top = TvDimens.Field.Top).focusKey(focus, "url"),
    )
    when (val check = state.check) {
        null -> Unit
        AddressCheck.Busy -> TvBusy(stringResource(R.string.tv_hand_checking), Modifier.padding(top = TvDimens.Note.Top))
        is AddressCheck.Found -> {
            Row(
                Modifier
                    .padding(top = TvDimens.Note.Top)
                    .widthIn(max = TvDimens.Note.Max)
                    .fillMaxWidth()
                    .background(c.s1, RoundedCornerShape(TvDimens.radiusMenuItem))
                    .border(BorderStroke(TvDimens.borderThin, c.line), RoundedCornerShape(TvDimens.radiusMenuItem))
                    .padding(TvDimens.Setup.ResultPad / 2)
                    .semantics(mergeDescendants = true) {},
                horizontalArrangement = Arrangement.spacedBy(TvDimens.Setup.ResultPad / 2 - TvDimens.xs / 2),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                TvIcon(SlouchcatIcon.Server, TvDimens.Menu.IconBox - TvDimens.xs, tint = c.tx2)
                Column(Modifier.weight(1f)) {
                    Text(check.info.address.host, style = TvType.largeStrong, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Text(stringResource(R.string.app_setup_versionValue, check.info.version), style = TvType.small, color = c.tx2)
                }
                ConnectionBadge(check.info.encrypted, stringResource(R.string.tv_hand_encrypted), stringResource(R.string.tv_hand_notEncrypted))
            }
            if (check.needsConsent) {
                TvNote(stringResource(R.string.app_setup_insecure_body), tone = NoteTone.Warn, modifier = Modifier.padding(top = TvDimens.Note.Top))
                TvSwitch(
                    stringResource(R.string.tv_hand_consent),
                    checked = state.httpConsent,
                    onCheckedChange = onConsent,
                    modifier = Modifier.padding(top = TvDimens.Switch.Top).focusKey(focus, "consent"),
                )
            }
        }
        else -> TvNote(addressError(check), tone = NoteTone.Bad, modifier = Modifier.padding(top = TvDimens.Note.Top))
    }
    Row(Modifier.padding(top = TvDimens.Setup.ActionsTop)) {
        TvButton(
            stringResource(R.string.tv_hand_next),
            onClick = onNext,
            icon = SlouchcatIcon.Right.vector,
            kind = TvButtonKind.Primary,
            enabled = state.serverReady,
            modifier = Modifier.focusKey(focus, "next"),
        )
    }
}

@Composable
private fun addressError(check: AddressCheck): String = when (check) {
    AddressCheck.Empty -> stringResource(R.string.app_setup_error_empty)
    AddressCheck.Invalid -> stringResource(R.string.app_setup_error_invalid, stringResource(R.string.app_setup_addressExample))
    AddressCheck.Unreachable -> stringResource(R.string.app_setup_error_unreachable)
    AddressCheck.NotSlouchcat -> stringResource(R.string.app_setup_error_notSlouchcat)
    is AddressCheck.TooOld -> stringResource(R.string.app_setup_error_tooOld, check.serverVersion, check.required)
    is AddressCheck.HttpsRequired -> stringResource(R.string.app_tls_httpsRequired, check.address)
    AddressCheck.Busy, is AddressCheck.Found -> ""
}

@Composable
private fun EmailStep(state: HandUiState, focus: FocusMemory, onEmail: (String) -> Unit, onNext: () -> Unit) {
    Heading(R.string.tv_hand_emailTitle, stringResource(R.string.tv_hand_emailLead))
    TvTextField(
        value = state.email,
        onValueChange = onEmail,
        label = stringResource(R.string.tv_hand_emailLabel),
        placeholder = stringResource(R.string.tv_hand_emailPlaceholder),
        keyboardType = KeyboardType.Email,
        modifier = Modifier.padding(top = TvDimens.Field.Top).focusKey(focus, "email"),
        // On to «Weiter»; while it is off (no valid address) the start focus falls back to the field.
        onDone = {
            focus.focus("next")
            true
        },
    )
    if (state.error == HandError.EmailInvalid) {
        TvNote(stringResource(R.string.app_signIn_error_emailInvalid), tone = NoteTone.Bad, modifier = Modifier.padding(top = TvDimens.Note.Top))
    }
    Row(Modifier.padding(top = TvDimens.Setup.ActionsTop)) {
        TvButton(
            stringResource(R.string.tv_hand_next),
            onClick = onNext,
            icon = SlouchcatIcon.Right.vector,
            kind = TvButtonKind.Primary,
            enabled = state.emailReady,
            modifier = Modifier.focusKey(focus, "next"),
        )
    }
}

@Composable
private fun PasswordStep(state: HandUiState, focus: FocusMemory, onPassword: (String) -> Unit, onTogglePassword: () -> Unit, onSignIn: () -> Unit) {
    val host = state.found?.address?.host ?: state.address
    Heading(R.string.tv_hand_passwordTitle, stringResource(R.string.tv_hand_passwordLead, host))
    Row(Modifier.padding(top = TvDimens.Field.Top).widthIn(max = TvDimens.Field.Max), verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
        TvTextField(
            value = state.password,
            onValueChange = onPassword,
            label = stringResource(R.string.tv_hand_passwordLabel),
            placeholder = stringResource(R.string.tv_hand_passwordPlaceholder),
            keyboardType = KeyboardType.Password,
            password = true,
            showPassword = state.showPassword,
            modifier = Modifier.weight(1f).focusKey(focus, "password"),
            onDone = {
                focus.focus("signin")
                true
            },
        )
        TvButton(
            stringResource(if (state.showPassword) R.string.tv_hand_hide else R.string.tv_hand_show),
            onClick = onTogglePassword,
            icon = if (state.showPassword) SlouchcatIcon.EyeOff.vector else SlouchcatIcon.Eye.vector,
            kind = TvButtonKind.Ghost,
            modifier = Modifier.focusKey(focus, "show"),
        )
    }
    val error = state.error
    if (error != null && error != HandError.EmailInvalid) {
        TvNote(signInError(error), tone = NoteTone.Bad, modifier = Modifier.padding(top = TvDimens.Note.Top))
    }
    if (state.busy) TvBusy(stringResource(R.string.tv_hand_signingIn), Modifier.padding(top = TvDimens.Note.Top))
    Row(Modifier.padding(top = TvDimens.Setup.ActionsTop)) {
        TvButton(
            stringResource(R.string.tv_hand_signIn),
            onClick = onSignIn,
            icon = SlouchcatIcon.Check.vector,
            kind = TvButtonKind.Primary,
            enabled = state.password.isNotEmpty() && !state.busy,
            modifier = Modifier.focusKey(focus, "signin"),
        )
    }
}

@Composable
private fun signInError(error: HandError): String = when (error) {
    HandError.InvalidCredentials -> stringResource(R.string.tv_hand_wrong)
    is HandError.TooManyAttempts -> stringResource(R.string.app_signIn_error_tooMany, error.seconds)
    HandError.Unreachable -> stringResource(R.string.app_setup_error_unreachable)
    HandError.ServerUnconfirmed -> stringResource(R.string.tv_settings_unconfirmedBody)
    HandError.AppTooOld -> stringResource(R.string.app_update_body)
    HandError.Storage -> stringResource(R.string.app_signIn_error_storage)
    HandError.EmailInvalid -> stringResource(R.string.app_signIn_error_emailInvalid)
    HandError.Failed -> stringResource(R.string.app_state_failed)
}
