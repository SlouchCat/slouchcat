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
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.EmptyState
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R

/**
 * The state of what a page loads, the same on every page: loading, nothing there, there, failed.
 * A `XUiState` holds one per thing it loads (`val items: Loadable<List<ItemRef>>`).
 */
sealed interface Loadable<out T> {
    data object Loading : Loadable<Nothing>

    /** Loaded, and there is nothing: the page shows its empty state. */
    data object Empty : Loadable<Nothing>

    data class Ready<T>(val value: T) : Loadable<T>

    data class Failed(val error: LoadError) : Loadable<Nothing>
}

/** What a page tells about a failed load; the text comes from [LoadError.text]. */
enum class LoadError {
    /** No answer: «try again» helps. */
    Offline,

    /** 404: gone or not visible to this profile (rules 2 and 12: the server decides). */
    NotFound,

    /** 403: not with this session. */
    NotAllowed,

    /** The server's address answers with another key (ADR 0040): nothing was sent; see «Mehr › Server». */
    ServerUnconfirmed,

    /** Anything else, also a session the app is already handling (signed out, app too old). */
    Failed,
}

/** [ApiResult] to [Loadable]; [isEmpty] decides when a loaded value counts as nothing. */
fun <T> ApiResult<T>.toLoadable(isEmpty: (T) -> Boolean = { false }): Loadable<T> = when (this) {
    is ApiResult.Ok -> if (isEmpty(value)) Loadable.Empty else Loadable.Ready(value)
    ApiResult.NotFound -> Loadable.Failed(LoadError.NotFound)
    is ApiResult.NotAllowed -> Loadable.Failed(LoadError.NotAllowed)
    ApiResult.Offline -> Loadable.Failed(LoadError.Offline)
    // The app shows the sign-in or the update notice itself (SessionEvents); the page only stops.
    is ApiResult.Failed -> Loadable.Failed(if (code == ApiCaller.SERVER_UNCONFIRMED) LoadError.ServerUnconfirmed else LoadError.Failed)
    ApiResult.SignedOut, ApiResult.AppTooOld -> Loadable.Failed(LoadError.Failed)
}

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
