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
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.setup.ServerCard
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.EmptyState
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatDimens
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.Wordmark
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.strings.R

/**
 * «Verbunden mit …»: the plain page after the setup until the sign-in of stage 3 part 2 takes its
 * place. Shows the stored server with the version it reports now, and leads back to the address.
 * The example of a page that loads: [LoadableContent] draws loading, empty, failed and ready.
 */
@Composable
fun ConnectedScreen(
    state: ConnectedUiState,
    onRetry: () -> Unit,
    onChangeAddress: () -> Unit,
    modifier: Modifier = Modifier,
) {
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
            LoadableContent(
                state = state.server,
                onRetry = onRetry,
                empty = {
                    EmptyState(stringResource(R.string.app_connectedState_empty), icon = SlouchcatIcon.Server)
                    SlouchcatButton(stringResource(R.string.app_connectedState_setUp), onClick = onChangeAddress, block = true)
                },
            ) { server ->
                Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
                    Text(
                        stringResource(R.string.app_connected_title, server.address.host),
                        style = SlouchcatType.pageTitle,
                        color = c.tx,
                        modifier = Modifier.padding(top = 10.dp).semantics { heading() },
                    )
                    ServerCard(server.address, server.version, server.encrypted, found = false)
                }
            }
            if (state.server !is Loadable.Empty) {
                SlouchcatButton(stringResource(R.string.app_connected_change), onClick = onChangeAddress, kind = ButtonKind.Ghost, block = true)
            }
        }
    }
}
