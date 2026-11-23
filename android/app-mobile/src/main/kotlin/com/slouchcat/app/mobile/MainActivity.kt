package com.slouchcat.app.mobile

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.core.design.SlouchcatTheme

/** The one activity of the phone app; every page is a Compose destination of [SlouchcatNavHost]. */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        // The app is always dark: light symbols in the system bars, whatever the system's theme.
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(android.graphics.Color.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(android.graphics.Color.TRANSPARENT),
        )
        super.onCreate(savedInstanceState)
        val graph = (application as SlouchcatApplication).graph
        setContent {
            SlouchcatTheme {
                SlouchcatNavHost(graph)
            }
        }
    }
}
