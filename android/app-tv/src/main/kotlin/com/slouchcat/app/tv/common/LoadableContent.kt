package com.slouchcat.app.tv.common

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import com.slouchcat.app.tv.ui.NoteTone
import com.slouchcat.app.tv.ui.TvBusy
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvNote
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.strings.R

@Composable
fun LoadError.text(): String = stringResource(
    when (this) {
        LoadError.Offline -> R.string.app_state_offline
        LoadError.NotFound -> R.string.app_state_notFound
        LoadError.NotAllowed -> R.string.app_state_notAllowed
        LoadError.ServerUnconfirmed -> R.string.tv_settings_unconfirmedBody
        LoadError.Failed -> R.string.app_state_failed
    },
)

/**
 * Draws a [Loadable] on the TV: the spinner while it loads, [empty] when there is nothing, the
 * error with «Erneut versuchen» (focusable, [retryModifier] gives it a focus key), [content] when ready.
 */
@Composable
fun <T> LoadableContent(
    state: Loadable<T>,
    onRetry: () -> Unit,
    modifier: Modifier = Modifier,
    retryModifier: Modifier = Modifier,
    empty: @Composable () -> Unit = {},
    content: @Composable (T) -> Unit,
) {
    when (state) {
        Loadable.Loading -> TvBusy(stringResource(R.string.app_state_loading), modifier.padding(vertical = TvDimens.xl))
        Loadable.Empty -> empty()
        is Loadable.Failed -> Column(modifier, verticalArrangement = Arrangement.spacedBy(TvDimens.l)) {
            TvNote(state.error.text(), tone = if (state.error == LoadError.NotFound) NoteTone.Plain else NoteTone.Warn, icon = SlouchcatIcon.Alert.vector)
            if (state.error != LoadError.NotFound) {
                TvButton(stringResource(R.string.app_state_retry), onClick = onRetry, icon = SlouchcatIcon.Refresh.vector, modifier = retryModifier)
            }
        }
        is Loadable.Ready -> content(state.value)
    }
}
