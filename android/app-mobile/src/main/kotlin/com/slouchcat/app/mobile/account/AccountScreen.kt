package com.slouchcat.app.mobile.account

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.autofill.ContentType
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.RowGroup
import com.slouchcat.app.mobile.common.StaticRow
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.ListRow
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SectionTitle
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTextField
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.strings.R

/**
 * «Konto» (mockup `/account`, reference photos 11-account and 69): the e-mail address and «Passwort
 * ändern» with their sheets, the server, and that the administration is only in the browser.
 */
@Composable
fun AccountScreen(
    state: AccountUiState,
    snackbar: SnackbarHostState,
    onBack: () -> Unit,
    onEmailAsk: () -> Unit,
    onPasswordAsk: () -> Unit,
    onField: (String, String) -> Unit,
    onSave: () -> Unit,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val c = SlouchcatTheme.colors
    Box(modifier.fillMaxSize().background(c.bg)) {
        Column(Modifier.fillMaxSize()) {
            TopBar(stringResource(R.string.nav_account), navigationLabel = stringResource(R.string.common_back), onNavigation = onBack)
            Column(
                Modifier
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = gutter(), vertical = 8.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Column(Modifier.widthIn(max = 728.dp).fillMaxWidth()) {
                    SectionTitle(stringResource(R.string.app_account_signInSection), inset = false)
                    RowGroup(
                        rows = listOf(
                            { ListRow(state.email, icon = SlouchcatIcon.User, subtitle = stringResource(R.string.app_account_emailSub), onClick = onEmailAsk) },
                            { ListRow(stringResource(R.string.app_account_password), icon = SlouchcatIcon.Key, subtitle = stringResource(R.string.app_account_passwordSub), onClick = onPasswordAsk) },
                        ),
                    )
                    state.server?.let { server ->
                        SectionTitle(stringResource(R.string.app_more_section_server), inset = false)
                        RowGroup(rows = listOf({ StaticRow(server.address.host, icon = SlouchcatIcon.Server, subtitle = stringResource(R.string.app_setup_versionValue, server.version)) }))
                    }
                    Text(
                        stringResource(R.string.app_account_adminHint),
                        style = SlouchcatType.caption,
                        color = c.tx2,
                        modifier = Modifier.padding(top = 14.dp, bottom = 24.dp),
                    )
                }
            }
        }
        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter).padding(16.dp))
    }
    when (state.sheet) {
        AccountSheet.Email -> FormSheet(R.string.app_account_emailTitle, state.form, onSave, onDismiss) {
            AccountField(AccountViewModel.NEW_EMAIL, R.string.app_account_newEmail, state.form, onField, password = false, contentType = ContentType.NewUsername)
            AccountField(AccountViewModel.CURRENT, R.string.app_account_currentPassword, state.form, onField, contentType = ContentType.Password)
        }
        AccountSheet.Password -> FormSheet(R.string.app_account_password, state.form, onSave, onDismiss) {
            Note(stringResource(R.string.app_account_passwordWarning), warning = true)
            AccountField(AccountViewModel.CURRENT, R.string.app_account_currentPassword, state.form, onField, contentType = ContentType.Password)
            AccountField(
                AccountViewModel.NEW_PASSWORD,
                R.string.app_account_newPassword,
                state.form,
                onField,
                hint = pluralStringResource(R.plurals.app_account_passwordHint, AccountViewModel.MIN_PASSWORD, AccountViewModel.MIN_PASSWORD),
                contentType = ContentType.NewPassword,
            )
            AccountField(AccountViewModel.REPEAT, R.string.app_account_repeatPassword, state.form, onField, contentType = ContentType.NewPassword)
        }
        null -> Unit
    }
}

@Composable
private fun FormSheet(title: Int, form: AccountForm, onSave: () -> Unit, onDismiss: () -> Unit, fields: @Composable () -> Unit) {
    val text = stringResource(title)
    SlouchcatSheet(title = text, onDismiss = onDismiss) {
        SheetTitle(text)
        Column(Modifier.padding(horizontal = gutter()), verticalArrangement = Arrangement.spacedBy(14.dp)) {
            fields()
            form.error?.let {
                Note(
                    when (it) {
                        is AccountError.TooManyAttempts -> stringResource(R.string.app_signIn_error_tooMany, it.seconds.toString())
                        AccountError.Unreachable -> stringResource(R.string.app_setup_error_unreachable)
                        AccountError.Failed -> stringResource(R.string.app_state_failed)
                    },
                    warning = true,
                )
            }
        }
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = onDismiss, kind = ButtonKind.Ghost, enabled = !form.busy, modifier = Modifier.weight(1f))
            SlouchcatButton(
                stringResource(if (form.busy) R.string.app_account_saving else R.string.app_account_save),
                onClick = onSave,
                kind = ButtonKind.Accent,
                busy = form.busy,
                modifier = Modifier.weight(1f),
            )
        }
    }
}

@Composable
private fun AccountField(
    key: String,
    label: Int,
    form: AccountForm,
    onField: (String, String) -> Unit,
    password: Boolean = true,
    hint: String? = null,
    contentType: ContentType? = null,
) {
    val value = when (key) {
        AccountViewModel.NEW_EMAIL -> form.newEmail
        AccountViewModel.CURRENT -> form.current
        AccountViewModel.NEW_PASSWORD -> form.newPassword
        else -> form.repeat
    }
    SlouchcatTextField(
        value = value,
        onValueChange = { onField(key, it) },
        label = stringResource(label),
        hint = hint,
        error = form.errors[key]?.let { fieldText(it) },
        enabled = !form.busy,
        contentType = contentType,
        visualTransformation = if (password) PasswordVisualTransformation() else androidx.compose.ui.text.input.VisualTransformation.None,
        keyboardOptions = KeyboardOptions(
            keyboardType = if (password) KeyboardType.Password else KeyboardType.Email,
            autoCorrectEnabled = false,
            capitalization = KeyboardCapitalization.None,
        ),
    )
}

@Composable
private fun fieldText(error: AccountFieldError): String = when (error) {
    AccountFieldError.EmailEmpty -> stringResource(R.string.app_signIn_error_emailEmpty)
    AccountFieldError.EmailInvalid -> stringResource(R.string.app_signIn_error_emailInvalid)
    AccountFieldError.SameEmail -> stringResource(R.string.app_account_error_sameEmail)
    AccountFieldError.EmailTaken -> stringResource(R.string.errors_code_email_taken)
    AccountFieldError.CurrentEmpty -> stringResource(R.string.app_account_error_currentEmpty)
    AccountFieldError.WrongPassword -> stringResource(R.string.app_account_error_wrongPassword)
    // The length comes from the contract's minLength, in one place (code review minor 25).
    AccountFieldError.TooShort -> pluralStringResource(R.plurals.app_account_error_tooShort, AccountViewModel.MIN_PASSWORD, AccountViewModel.MIN_PASSWORD)
    AccountFieldError.SamePassword -> stringResource(R.string.app_account_error_same)
    AccountFieldError.Mismatch -> stringResource(R.string.app_account_error_mismatch)
}
