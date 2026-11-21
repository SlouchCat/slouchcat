package com.slouchcat.app.tv

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import com.slouchcat.app.tv.navigation.TvNavHost

/** The one activity of the TV app; every page is a Compose destination of [TvNavHost]. */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val graph = (application as TvApplication).graph
        setContent {
            // The second Back on the first page leaves the app (TV-DB).
            TvNavHost(graph, onExit = ::finish)
        }
    }
}
