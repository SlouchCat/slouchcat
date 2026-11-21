package com.slouchcat.app.tv

import android.os.Bundle
import android.view.KeyEvent
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import com.slouchcat.app.tv.navigation.TvNavHost

/** The one activity of the TV app; every page is a Compose destination of [TvNavHost]. */
class MainActivity : ComponentActivity() {
    private val graph get() = (application as TvApplication).graph

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            // The second Back on the first page leaves the app (TV-DB).
            TvNavHost(graph, onExit = ::finish)
        }
    }

    /** The media keys go to the page that takes them ([com.slouchcat.app.tv.ui.OnRemoteKey]) before Compose. */
    override fun dispatchKeyEvent(event: KeyEvent): Boolean = graph.remoteKeys.dispatch(event) || super.dispatchKeyEvent(event)
}
