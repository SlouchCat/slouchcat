package com.slouchcat.app.mobile.signin

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.disabled
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.onClick
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.EntryLayout
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatDimens
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatTextField
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.Wordmark
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.strings.R

/**
 * «Anmelden» (mockup `/setup/login`, reference photos 03-setup-login): the server with «Ändern»,
 * e-mail and password, the button and the line how the device will appear under «Meine Geräte».
 */
@Composable
fun SignInScreen(
    state: SignInUiState,
    onEmailChange: (String) -> Unit,
    onPasswordChange: (String) -> Unit,
    onTogglePassword: () -> Unit,
    onSubmit: () -> Unit,
    onChangeServer: () -> Unit,
    modifier: Modifier = Modifier,
) {
    EntryLayout(modifier) { wide ->
        val c = SlouchcatTheme.colors
        Wordmark(stringResource(R.string.app_name), wide = wide)
        Spacer(Modifier.height(22.dp))
        state.server?.let { ServerRow(it, onChangeServer, enabled = !state.busy) }
        if (state.confirmKey) {
            Note(stringResource(R.string.app_serverKey_confirm), warning = true, icon = SlouchcatIcon.Shield, modifier = Modifier.padding(top = 14.dp))
        }
        Text(
            stringResource(R.string.login_title),
            style = SlouchcatType.pageTitle,
            color = c.tx,
            modifier = Modifier.padding(top = 26.dp, bottom = 16.dp).semantics { heading() },
        )
        SlouchcatTextField(
            value = state.email,
            onValueChange = onEmailChange,
            label = stringResource(R.string.login_email),
            error = state.emailError?.let { fieldText(it) },
            enabled = !state.busy,
            keyboardOptions = KeyboardOptions(
                keyboardType = KeyboardType.Email,
                imeAction = ImeAction.Next,
                autoCorrectEnabled = false,
                capitalization = KeyboardCapitalization.None,
            ),
        )
        Spacer(Modifier.height(16.dp))
        val toggle = stringResource(if (state.showPassword) R.string.app_signIn_hidePassword else R.string.app_signIn_showPassword)
        SlouchcatTextField(
            value = state.password,
            onValueChange = onPasswordChange,
            label = stringResource(R.string.login_password),
            error = state.passwordError?.let { fieldText(it) },
            enabled = !state.busy,
            visualTransformation = if (state.showPassword) VisualTransformation.None else PasswordVisualTransformation(),
            keyboardOptions = KeyboardOptions(
                keyboardType = KeyboardType.Password,
                imeAction = ImeAction.Go,
                autoCorrectEnabled = false,
                capitalization = KeyboardCapitalization.None,
            ),
            keyboardActions = KeyboardActions(onGo = { onSubmit() }),
            trailing = {
                SlouchcatIconButton(
                    icon = if (state.showPassword) SlouchcatIcon.EyeOff else SlouchcatIcon.Eye,
                    contentDescription = toggle,
                    onClick = onTogglePassword,
                    tint = c.tx2,
                )
            },
        )
        state.error?.let { Note(errorText(it), warning = true, modifier = Modifier.padding(top = 16.dp)) }
        SlouchcatButton(
            text = stringResource(if (state.busy) R.string.login_submitting else R.string.login_submit),
            onClick = onSubmit,
            block = true,
            busy = state.busy,
            enabled = state.error !is SignInError.ServerTooOld,
            modifier = Modifier.padding(top = 18.dp),
        )
        Row(Modifier.padding(top = 18.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            SlouchcatIconImage(SlouchcatIcon.Phone, contentDescription = null, size = 16.dp, tint = c.tx2, modifier = Modifier.padding(top = 2.dp))
            Text(stringResource(R.string.app_signIn_deviceHint, state.deviceName), style = SlouchcatType.caption, color = c.tx2)
        }
    }
}

/** `.su-srv`: the server the app is set up with; the whole row leads back to the address. */
@Composable
private fun ServerRow(server: SavedServer, onChange: () -> Unit, enabled: Boolean) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(SlouchcatDimens.radius)
    val host = server.address.host
    val label = stringResource(R.string.app_signIn_serverLabel, host)
    val state = stringResource(if (server.encrypted) R.string.app_signIn_encrypted else R.string.app_signIn_notEncrypted)
    Row(
        Modifier
            .fillMaxWidth()
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.frame), shape)
            .clickable(enabled = enabled, role = Role.Button, onClick = onChange)
            .clearAndSetSemantics {
                contentDescription = label
                role = Role.Button
                if (enabled) onClick { onChange(); true } else disabled()
            }
            .heightIn(min = SlouchcatDimens.touch)
            .padding(horizontal = 12.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box(Modifier.size(44.dp).clip(RoundedCornerShape(12.dp)).background(c.s3), contentAlignment = Alignment.Center) {
            SlouchcatIconImage(SlouchcatIcon.Server, contentDescription = null, size = 20.dp, tint = c.tx)
        }
        Column(Modifier.weight(1f)) {
            Text(host, style = SlouchcatType.strong, color = c.tx, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                SlouchcatIconImage(
                    if (server.encrypted) SlouchcatIcon.Lock else SlouchcatIcon.Alert,
                    contentDescription = null,
                    size = 13.dp,
                    tint = if (server.encrypted) c.tx2 else c.warn,
                )
                Text(
                    stringResource(R.string.app_signIn_serverLine, state, server.version),
                    style = SlouchcatType.caption,
                    color = c.tx2,
                    maxLines = 2,
                )
            }
        }
        Text(stringResource(R.string.app_signIn_change), style = SlouchcatType.button, color = c.tx)
    }
}

@Composable
private fun fieldText(error: FieldError): String = stringResource(
    when (error) {
        FieldError.EmailEmpty -> R.string.app_signIn_error_emailEmpty
        FieldError.EmailInvalid -> R.string.app_signIn_error_emailInvalid
        FieldError.PasswordEmpty -> R.string.app_signIn_error_passwordEmpty
    },
)

@Composable
private fun errorText(error: SignInError): String = when (error) {
    SignInError.InvalidCredentials -> stringResource(R.string.errors_code_invalid_credentials)
    is SignInError.TooManyAttempts -> stringResource(R.string.app_signIn_error_tooMany, error.seconds.toString())
    SignInError.Unreachable -> stringResource(R.string.app_setup_error_unreachable)
    SignInError.AppTooOld -> stringResource(R.string.app_update_body)
    is SignInError.ServerTooOld -> stringResource(R.string.app_setup_error_tooOld, error.serverVersion, error.required)
    SignInError.Storage -> stringResource(R.string.app_signIn_error_storage)
    SignInError.Failed -> stringResource(R.string.app_state_failed)
}
