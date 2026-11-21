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

    /** The media keys Compose did not take go to the page that takes them ([com.slouchcat.app.tv.ui.OnRemoteKey]). */
    override fun onKeyDown(keyCode: Int, event: KeyEvent): Boolean = graph.remoteKeys.dispatch(event) || super.onKeyDown(keyCode, event)

    override fun onKeyUp(keyCode: Int, event: KeyEvent): Boolean = graph.remoteKeys.dispatch(event) || super.onKeyUp(keyCode, event)
}
