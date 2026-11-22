package com.slouchcat.app.mobile.setup

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.EntryLayout
import com.slouchcat.app.mobile.common.KeyCheckSheet
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetHeadIcon
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatTextField
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.Wordmark
import com.slouchcat.core.design.gutter
import com.slouchcat.core.strings.R

/**
 * The page «Serveradresse» (mockup `/setup`, reference photos 02-setup): the backdrop above the
 * form on a phone, beside it from 700 dp. Stateless: it shows [state] and reports what the user does.
 */
@Composable
fun SetupScreen(
    state: SetupUiState,
    onAddressChange: (String) -> Unit,
    onConnect: () -> Unit,
    onConfirmInsecure: () -> Unit,
    onDismissInsecure: () -> Unit,
    onChangeAddress: () -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier,
    onOwnNetwork: (Boolean) -> Unit = {},
    onAcceptKey: () -> Unit = {},
    onDismissKey: () -> Unit = {},
) {
    EntryLayout(modifier) { wide ->
        SetupPane(state, onAddressChange, onConnect, onChangeAddress, onNext, wide = wide)
    }
    val insecure = state.insecureAddress
    if (insecure != null) {
        InsecureSheet(insecure, onConfirm = onConfirmInsecure, onDismiss = onDismissInsecure)
    }
    val change = state.keyChange
    if (change != null) {
        KeyCheckSheet(
            remembered = change.first,
            presented = change.second,
            ownNetwork = state.ownNetwork,
            onOwnNetwork = onOwnNetwork,
            confirmLabel = stringResource(R.string.app_serverKey_accept),
            onConfirm = onAcceptKey,
            onDismiss = onDismissKey,
        )
    }
}

@Composable
private fun SetupPane(
    state: SetupUiState,
    onAddressChange: (String) -> Unit,
    onConnect: () -> Unit,
    onChangeAddress: () -> Unit,
    onNext: () -> Unit,
    wide: Boolean,
) {
    val c = SlouchcatTheme.colors
    Column(Modifier.fillMaxWidth()) {
        Wordmark(stringResource(R.string.app_name), wide = wide)
        Text(
            stringResource(R.string.app_setup_tagline),
            style = SlouchcatType.body,
            color = c.tx2,
            modifier = Modifier.padding(top = 10.dp).widthIn(max = 300.dp),
        )
        Spacer(Modifier.height(26.dp))
        Text(
            stringResource(R.string.app_setup_title),
            style = SlouchcatType.pageTitle,
            color = c.tx,
            modifier = Modifier.padding(bottom = 16.dp).semantics { heading() },
        )
        val example = stringResource(R.string.app_setup_addressExample)
        SlouchcatTextField(
            value = state.address,
            onValueChange = onAddressChange,
            label = stringResource(R.string.app_setup_addressLabel),
            placeholder = example,
            hint = stringResource(R.string.app_setup_addressHint),
            error = state.error?.let { errorText(it, example) },
            readOnly = state.found != null,
            keyboardOptions = KeyboardOptions(
                keyboardType = KeyboardType.Uri,
                imeAction = ImeAction.Go,
                autoCorrectEnabled = false,
                capitalization = KeyboardCapitalization.None,
            ),
            keyboardActions = KeyboardActions(onGo = { onConnect() }),
        )
        Spacer(Modifier.height(14.dp))
        val found = state.found
        if (found == null) {
            SlouchcatButton(
                text = stringResource(if (state.busy) R.string.app_setup_connecting else R.string.app_setup_connect),
                onClick = onConnect,
                block = true,
                busy = state.busy,
            )
        } else {
            ServerCard(
                address = found.address,
                version = found.version,
                encrypted = found.encrypted,
                found = true,
                userCertificate = found.userCertificate,
                modifier = Modifier.padding(top = 4.dp, bottom = 16.dp),
            )
            SlouchcatButton(stringResource(R.string.app_setup_next), onClick = onNext, block = true, trailingIcon = SlouchcatIcon.Right)
            SlouchcatButton(
                stringResource(R.string.app_setup_otherAddress),
                onClick = onChangeAddress,
                kind = ButtonKind.Ghost,
                block = true,
                modifier = Modifier.padding(top = 8.dp),
            )
        }
    }
}

@Composable
private fun errorText(error: SetupError, example: String): String = when (error) {
    SetupError.Empty -> stringResource(R.string.app_setup_error_empty)
    SetupError.Invalid -> stringResource(R.string.app_setup_error_invalid, example)
    SetupError.Unreachable -> stringResource(R.string.app_setup_error_unreachable)
    SetupError.NotSlouchcat -> stringResource(R.string.app_setup_error_notSlouchcat)
    is SetupError.TooOld -> stringResource(R.string.app_setup_error_tooOld, error.serverVersion, error.required)
    is SetupError.HttpsRequired -> stringResource(R.string.app_tls_httpsRequired, error.address)
}

/** The warning before plain HTTP outside the home network (concept 27, 6; E-18). */
@Composable
private fun InsecureSheet(address: String, onConfirm: () -> Unit, onDismiss: () -> Unit) {
    val c = SlouchcatTheme.colors
    val title = stringResource(R.string.app_setup_insecure_title)
    SlouchcatSheet(title = title, onDismiss = onDismiss) {
        SheetHeadIcon(SlouchcatIcon.Alert, warning = true)
        SheetTitle(title, centered = true)
        SheetText(stringResource(R.string.app_setup_insecure_body), centered = true)
        Row(
            Modifier
                .fillMaxWidth()
                .padding(start = gutter(), end = gutter(), top = 12.dp),
            horizontalArrangement = Arrangement.Center,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            SlouchcatIconImage(SlouchcatIcon.Globe, contentDescription = null, size = 16.dp, tint = c.tx3)
            Text(address, style = SlouchcatType.bodySmall, color = c.tx3, textAlign = TextAlign.Center, modifier = Modifier.padding(start = 6.dp))
        }
        SheetActions {
            SlouchcatButton(stringResource(R.string.app_setup_insecure_cancel), onClick = onDismiss, modifier = Modifier.weight(1f))
            SlouchcatButton(
                stringResource(R.string.app_setup_insecure_confirm),
                onClick = onConfirm,
                kind = ButtonKind.Ghost,
                modifier = Modifier.weight(1f),
            )
        }
    }
}
