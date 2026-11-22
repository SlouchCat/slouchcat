package com.slouchcat.app.mobile.push

import android.app.Activity
import android.content.Intent
import android.os.Bundle
import com.slouchcat.app.mobile.MainActivity
import com.slouchcat.app.mobile.SlouchcatApplication

/**
 * The tap on a notification of the app (security review M-1): not exported and without an intent
 * filter, so only the app's own `PendingIntent` starts it; neither another app nor a notification
 * that Google's library draws itself reaches it. It hands the path of the decrypted message to the
 * in-app jumps, brings [MainActivity] to the front and closes.
 */
class PushTapActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val graph = (application as SlouchcatApplication).graph
        PushLinks.open(this, graph, intent?.getStringExtra(PushNotifier.EXTRA_PATH))
        // This activity runs in a task of its own (taskAffinity ""): NEW_TASK sends MainActivity to
        // the app's task, where CLEAR_TOP and SINGLE_TOP reuse the open instance instead of a second.
        startActivity(
            Intent(this, MainActivity::class.java)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP),
        )
        finish()
    }
}
