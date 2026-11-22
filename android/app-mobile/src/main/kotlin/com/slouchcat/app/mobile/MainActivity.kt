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
        if (savedInstanceState == null) openPushLink(intent)
    }

    /** A tapped notification while the app runs (stage 8). */
    override fun onNewIntent(intent: android.content.Intent) {
        super.onNewIntent(intent)
        openPushLink(intent)
    }

    /** The path of a tapped notification, opened inside the app (concept 27, 7.3 point 6). */
    private fun openPushLink(intent: android.content.Intent?) {
        val path = intent?.getStringExtra(com.slouchcat.app.mobile.push.PushNotifier.EXTRA_PATH) ?: return
        intent.removeExtra(com.slouchcat.app.mobile.push.PushNotifier.EXTRA_PATH)
        com.slouchcat.app.mobile.push.PushLinks.open(this, (application as SlouchcatApplication).graph, path)
    }
}
