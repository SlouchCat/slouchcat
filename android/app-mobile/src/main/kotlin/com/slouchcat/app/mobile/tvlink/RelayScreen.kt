package com.slouchcat.app.mobile.tvlink

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.BuildConfig
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTextField
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.session.RelaySettings
import com.slouchcat.core.session.ServerAddress
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.session.parseRelay
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import okhttp3.HttpUrl

/**
 * «Adresse der Vermittlung» (concept 27 F-21: in the advanced settings; security review L-4): under
 * «Mehr › Einstellungen», for whoever runs a relay of their own. Only the primary profile changes it,
 * so a child cannot point the pairing elsewhere; the others read it. A setting of this phone, never
 * one of the server.
 */
class RelayViewModel(session: SessionManager, private val settings: RelaySettings) : ViewModel() {
    val primary: Boolean = (session.state.value as? SessionState.Ready)?.profile?.isPrimary == true
    val relay: StateFlow<HttpUrl> = settings.relay.stateIn(viewModelScope, SharingStarted.Eagerly, RelaySettings.DEFAULT)

    /** A new relay, `null` for the built-in one; then [done]. */
    fun onSave(address: HttpUrl?, done: () -> Unit) {
        if (!primary) return
        viewModelScope.launch {
            settings.set(address)
            done()
        }
    }
}

@Composable
fun RelayScreen(relay: HttpUrl, primary: Boolean, onBack: () -> Unit, onSave: (HttpUrl?) -> Unit, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    var text by rememberSaveable(relay) { mutableStateOf(ServerAddress.display(relay)) }
    var invalid by rememberSaveable { mutableStateOf(false) }
    Column(modifier.fillMaxSize().background(c.bg)) {
        TopBar(stringResource(R.string.tv_relay_title), navigationLabel = stringResource(R.string.common_back), onNavigation = onBack)
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = gutter(), vertical = 8.dp).imePadding(),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Column(Modifier.widthIn(max = 528.dp).fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Text(stringResource(R.string.tv_relay_lead), style = SlouchcatType.bodySmall, color = c.tx2)
                if (!primary) Note(stringResource(R.string.app_tv_relay_primaryOnly), icon = SlouchcatIcon.Lock)
                SlouchcatTextField(
                    value = text,
                    onValueChange = {
                        text = it
                        invalid = false
                    },
                    label = stringResource(R.string.tv_relay_label),
                    placeholder = RelaySettings.DEFAULT.host,
                    error = if (invalid) stringResource(R.string.tv_relay_invalid) else null,
                    readOnly = !primary,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, autoCorrectEnabled = false),
                )
                if (primary) {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        SlouchcatButton(stringResource(R.string.tv_relay_reset), onClick = { onSave(null) }, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
                        SlouchcatButton(
                            stringResource(R.string.tv_relay_save),
                            onClick = {
                                val parsed = parseRelay(text, allowPlainHttp = BuildConfig.DEBUG)
                                if (parsed == null) invalid = true else onSave(parsed)
                            },
                            modifier = Modifier.weight(1f),
                        )
                    }
                }
            }
        }
    }
}
