package com.slouchcat.app.tv.common

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult

/**
 * The state of what a TV page loads, the same on every page (the pattern of the phone app):
 * loading, nothing there, there, failed. A `XUiState` holds one per thing it loads. Without UI and
 * without Android, so it can move to a `:core` module with the phone app's copy (review N-15);
 * the drawing is in `LoadableContent.kt`.
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
