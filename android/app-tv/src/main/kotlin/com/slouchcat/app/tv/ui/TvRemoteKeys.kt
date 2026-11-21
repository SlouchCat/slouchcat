package com.slouchcat.app.tv.ui

import android.view.KeyEvent
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberUpdatedState
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow

/** The media keys of a remote (and of a keyboard or headset) that a page can take. */
enum class RemoteKey { PlayPause, Play, Pause, Stop, Next, Previous, FastForward, Rewind }

/**
 * The media keys of the remote as events (review N-13): the activity hands every key to
 * [dispatch] before Compose; a page subscribes with [OnRemoteKey]. Keys go to the page that is
 * seen (its lifecycle started) whatever has the focus. A key nobody takes goes on to the system
 * (later a `MediaSession` for sound in the background, stage 4).
 */
class TvRemoteKeys {
    private val events = MutableSharedFlow<RemoteKey>(extraBufferCapacity = BUFFER)
    val keys: SharedFlow<RemoteKey> = events.asSharedFlow()

    /** `true` when a page takes the key; the release of a taken key is taken as well. */
    fun dispatch(event: KeyEvent): Boolean {
        val key = KEYS[event.keyCode] ?: return false
        if (events.subscriptionCount.value == 0) return false
        if (event.action == KeyEvent.ACTION_DOWN && event.repeatCount == 0) events.tryEmit(key)
        return true
    }

    private companion object {
        const val BUFFER = 8
        val KEYS = mapOf(
            KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE to RemoteKey.PlayPause,
            KeyEvent.KEYCODE_MEDIA_PLAY to RemoteKey.Play,
            KeyEvent.KEYCODE_MEDIA_PAUSE to RemoteKey.Pause,
            KeyEvent.KEYCODE_MEDIA_STOP to RemoteKey.Stop,
            KeyEvent.KEYCODE_MEDIA_NEXT to RemoteKey.Next,
            KeyEvent.KEYCODE_MEDIA_PREVIOUS to RemoteKey.Previous,
            KeyEvent.KEYCODE_MEDIA_FAST_FORWARD to RemoteKey.FastForward,
            KeyEvent.KEYCODE_MEDIA_REWIND to RemoteKey.Rewind,
        )
    }
}

/**
 * Takes the media keys while this page is seen (its back stack entry started): the player, the
 * audio player. A page below another one or the app in the background gets none.
 *
 * ```
 * OnRemoteKey(app.graph.remoteKeys) { key -> if (key == RemoteKey.PlayPause) vm.toggle() }
 * ```
 */
@Composable
fun OnRemoteKey(keys: TvRemoteKeys, onKey: (RemoteKey) -> Unit) {
    val current by rememberUpdatedState(onKey)
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    LaunchedEffect(keys, lifecycle) {
        lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
            keys.keys.collect { current(it) }
        }
    }
}
