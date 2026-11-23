package com.slouchcat.app.tv.setup

import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.tv.material3.Text
import com.slouchcat.app.tv.BuildConfig
import com.slouchcat.core.session.RelaySettings
import com.slouchcat.app.tv.ui.NoteTone
import com.slouchcat.app.tv.ui.TvActions
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvNote
import com.slouchcat.app.tv.ui.TvSideMenu
import com.slouchcat.app.tv.ui.TvTextField
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.session.ServerAddress
import com.slouchcat.core.session.parseRelay
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl

/**
 * «Adresse der Vermittlung» (advanced, F-21): the relay the QR code names. Only for whoever runs a
 * relay of their own; phone and TV need the same one. [onSave] gets the new relay, `null` for the
 * built-in one.
 */
@Composable
fun RelayMenu(current: HttpUrl, onSave: (HttpUrl?) -> Unit, onDismiss: () -> Unit) {
    val c = SlouchcatTheme.colors
    var text by rememberSaveable { mutableStateOf(ServerAddress.display(current)) }
    var invalid by rememberSaveable { mutableStateOf(false) }
    val first = remember { FocusRequester() }
    TvSideMenu(stringResource(R.string.tv_relay_title), onDismiss = onDismiss, initialFocus = first) {
        Text(stringResource(R.string.tv_relay_lead), style = TvType.body, color = c.tx2)
        TvTextField(
            value = text,
            onValueChange = {
                text = it
                invalid = false
            },
            label = stringResource(R.string.tv_relay_label),
            placeholder = RelaySettings.DEFAULT.host,
            keyboardType = KeyboardType.Uri,
            focusRequester = first,
            modifier = Modifier.padding(top = TvDimens.xl),
        )
        if (invalid) TvNote(stringResource(R.string.tv_relay_invalid), tone = NoteTone.Bad, modifier = Modifier.padding(top = TvDimens.Note.Top))
        TvActions(vertical = true) {
            TvButton(
                stringResource(R.string.tv_relay_save),
                onClick = {
                    val relay = parseRelay(text, allowPlainHttp = BuildConfig.DEBUG)
                    if (relay == null) invalid = true else onSave(relay)
                },
                icon = SlouchcatIcon.Check.vector,
                kind = TvButtonKind.Primary,
            )
            TvButton(stringResource(R.string.tv_relay_reset), onClick = { onSave(null) }, icon = SlouchcatIcon.Refresh.vector, kind = TvButtonKind.Ghost)
            TvButton(stringResource(R.string.common_cancel), onClick = onDismiss, kind = TvButtonKind.Ghost)
        }
    }
}
