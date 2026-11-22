package com.slouchcat.core.media

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult

/**
 * The state of what a page loads, the same on every page of both apps: loading, nothing there,
 * there, failed. A `XUiState` holds one per thing it loads (`val items: Loadable<List<ItemRef>>`).
 * Each app draws it itself (`LoadableContent` in the phone app and in the TV app).
 */
sealed interface Loadable<out T> {
    data object Loading : Loadable<Nothing>

    /** Loaded, and there is nothing: the page shows its empty state. */
    data object Empty : Loadable<Nothing>

    data class Ready<T>(val value: T) : Loadable<T>

    data class Failed(val error: LoadError) : Loadable<Nothing>
}

/** What a page tells about a failed load; each app has the text for it. */
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
