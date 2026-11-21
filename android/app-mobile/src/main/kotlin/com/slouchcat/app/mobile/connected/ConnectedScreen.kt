package com.slouchcat.app.mobile.connected

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.setup.ServerCard
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatDimens
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.Wordmark
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn

/** The stored server, `null` while it loads. */
class ConnectedViewModel(store: ServerStore) : ViewModel() {
    val server: StateFlow<SavedServer?> = store.server.stateIn(viewModelScope, SharingStarted.Eagerly, null)
}

/**
 * «Verbunden mit …»: the plain page after the setup until the sign-in of stage 3 part 2 takes its
 * place. Shows the stored server and leads back to the address.
 */
@Composable
fun ConnectedScreen(server: SavedServer?, onChangeAddress: () -> Unit, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val wide = isWideLayout()
    Box(
        modifier
            .fillMaxSize()
            .background(c.bg)
            .verticalScroll(rememberScrollState())
            .systemBarsPadding()
            .padding(horizontal = 24.dp, vertical = if (wide) 48.dp else 32.dp),
        contentAlignment = Alignment.TopCenter,
    ) {
        Column(Modifier.widthIn(max = SlouchcatDimens.formMax).fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(16.dp)) {
            Wordmark(stringResource(R.string.app_name), wide = wide)
            if (server != null) {
                Text(
                    stringResource(R.string.app_connected_title, server.address.host),
                    style = SlouchcatType.pageTitle,
                    color = c.tx,
                    modifier = Modifier.padding(top = 10.dp).semantics { heading() },
                )
                ServerCard(server.address, server.version, server.encrypted, found = false)
                SlouchcatButton(stringResource(R.string.app_connected_change), onClick = onChangeAddress, kind = ButtonKind.Ghost, block = true)
            }
        }
    }
}
