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
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.strings.R

/**
 * The state of what a TV page loads, the same on every page (the pattern of the phone app):
 * loading, nothing there, there, failed. A `XUiState` holds one per thing it loads.
 */
sealed interface Loadable<out T> {
    data object Loading : Loadable<Nothing>

    data object Empty : Loadable<Nothing>

    data class Ready<T>(val value: T) : Loadable<T>

    data class Failed(val error: LoadError) : Loadable<Nothing>
}

/** What a page tells about a failed load. */
enum class LoadError {
    Offline,

    /** 404: gone or not visible to this profile (rules 2 and 12: the server decides). */
    NotFound,
    NotAllowed,

    /** The server's address answers with another key (ADR 0040): nothing was sent. */
    ServerUnconfirmed,
    Failed,
}

fun <T> ApiResult<T>.toLoadable(isEmpty: (T) -> Boolean = { false }): Loadable<T> = when (this) {
    is ApiResult.Ok -> if (isEmpty(value)) Loadable.Empty else Loadable.Ready(value)
    ApiResult.NotFound -> Loadable.Failed(LoadError.NotFound)
    is ApiResult.NotAllowed -> Loadable.Failed(LoadError.NotAllowed)
    ApiResult.Offline -> Loadable.Failed(LoadError.Offline)
    is ApiResult.Failed -> Loadable.Failed(if (code == ApiCaller.SERVER_UNCONFIRMED) LoadError.ServerUnconfirmed else LoadError.Failed)
    // The shell shows the setup or the update notice itself; the page only stops.
    ApiResult.SignedOut, ApiResult.AppTooOld -> Loadable.Failed(LoadError.Failed)
}

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
