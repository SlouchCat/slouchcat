package com.slouchcat.app.mobile.common

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.EmptyState
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.strings.R

/** The phone app's text of a [LoadError]. */
@Composable
fun LoadError.text(): String = stringResource(
    when (this) {
        LoadError.Offline -> R.string.app_state_offline
        LoadError.NotFound -> R.string.app_state_notFound
        LoadError.NotAllowed -> R.string.app_state_notAllowed
        LoadError.ServerUnconfirmed -> R.string.app_serverKey_unconfirmedBody
        LoadError.Failed -> R.string.app_state_failed
    },
)

/**
 * Draws a [Loadable]: a spinner while it loads, [empty] when there is nothing, the error with
 * «Erneut versuchen» when it failed ([onRetry]; not offered for «not found»), [content] when ready.
 */
@Composable
fun <T> LoadableContent(
    state: Loadable<T>,
    onRetry: () -> Unit,
    modifier: Modifier = Modifier,
    empty: @Composable () -> Unit = {},
    content: @Composable (T) -> Unit,
) {
    when (state) {
        Loadable.Loading -> {
            val label = stringResource(R.string.app_state_loading)
            Box(modifier.fillMaxWidth().padding(vertical = 48.dp), contentAlignment = Alignment.Center) {
                CircularProgressIndicator(color = SlouchcatTheme.colors.tx2, modifier = Modifier.semantics { contentDescription = label })
            }
        }
        Loadable.Empty -> empty()
        is Loadable.Failed -> Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            if (state.error == LoadError.NotFound) {
                EmptyState(state.error.text(), icon = SlouchcatIcon.Info)
            } else {
                Note(state.error.text(), warning = true)
                SlouchcatButton(stringResource(R.string.app_state_retry), onClick = onRetry, kind = ButtonKind.Secondary, block = true)
            }
        }
        is Loadable.Ready -> content(state.value)
    }
}
