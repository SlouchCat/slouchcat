package com.slouchcat.app.mobile.audio

import android.app.PendingIntent
import android.content.Intent
import androidx.annotation.OptIn
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.session.CommandButton
import androidx.media3.session.DefaultMediaNotificationProvider
import androidx.media3.session.MediaSession
import androidx.media3.session.MediaSessionService
import com.google.common.collect.ImmutableList
import com.slouchcat.app.mobile.MainActivity
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.player.AudioPlayback
import com.slouchcat.core.player.Media3Engine
import com.slouchcat.core.player.PlayerEngine
import com.slouchcat.core.player.TrustedControllers
import com.slouchcat.core.strings.R
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch

/**
 * The sound in the background (concept 27 8.3): a `MediaSessionService` over the player of what
 * [AudioPlayback] plays. Media3 makes it a foreground service of the type `mediaPlayback` while
 * it plays, with the notification (play, pause, −15 s, +30 s), the lock screen, the headphone and
 * Bluetooth buttons; audio focus and «becoming noisy» belong to the player. A change between a
 * title and a station moves the session to the other player; when nothing is loaded any more the
 * service ends. Only the app's own controller, the notification and the system's trusted
 * controllers connect, without commands that set media (security review stage 5, L-1).
 *
 * The video player keeps its own session and stops in the background (stage 5); it never comes here.
 */
/** The application that holds the one [AudioPlayback] of the process (the app; a test application). */
interface AudioHost {
    val audioPlayback: AudioPlayback
}

@OptIn(UnstableApi::class)
class AudioService : MediaSessionService() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var session: MediaSession? = null

    override fun onCreate() {
        super.onCreate()
        setMediaNotificationProvider(DefaultMediaNotificationProvider.Builder(this).setChannelName(R.string.app_audio_channel).build())
        val playback = (application as AudioHost).audioPlayback
        // A start that answers only after the app left the foreground: Android 12 and later refuse
        // the foreground service then (code review m-5). The sound plays on without it; the log says so.
        setListener(
            object : Listener {
                override fun onForegroundServiceStartNotAllowedException() {
                    Log.w(TAG, "foreground service not allowed, sound without notification")
                }
            },
        )
        follow(playback.engine.value)
        scope.launch { playback.engine.collect(::follow) }
    }

    private fun follow(engine: PlayerEngine?) {
        val player = (engine as? Media3Engine)?.player
        val current = session
        when {
            player == null -> {
                current?.let {
                    removeSession(it)
                    it.release()
                }
                session = null
                stopSelf()
            }
            // Added at once: the notification follows the session before a controller connects.
            current == null -> session = build(player).also(::addSession)
            current.player !== player -> current.player = player
        }
    }

    private fun build(player: Player): MediaSession {
        val open = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        return MediaSession.Builder(this, player)
            .setId("$SESSION_ID-${SESSIONS.incrementAndGet()}")
            .setCallback(TrustedControllers(packageName))
            // The cover comes as bytes (AudioData, security review L-1); the default loader decodes them.
            .setSessionActivity(open)
            .setMediaButtonPreferences(
                ImmutableList.of(
                    CommandButton.Builder(CommandButton.ICON_SKIP_BACK_15)
                        .setPlayerCommand(Player.COMMAND_SEEK_BACK)
                        .setDisplayName(getString(R.string.player_audio_back15))
                        .setSlots(CommandButton.SLOT_BACK)
                        .build(),
                    CommandButton.Builder(CommandButton.ICON_SKIP_FORWARD_30)
                        .setPlayerCommand(Player.COMMAND_SEEK_FORWARD)
                        .setDisplayName(getString(R.string.player_audio_forward30))
                        .setSlots(CommandButton.SLOT_FORWARD)
                        .build(),
                ),
            )
            .build()
    }

    override fun onGetSession(controllerInfo: MediaSession.ControllerInfo): MediaSession? = session

    /**
     * Swiped away while paused: the sound ends (its session at the server, the listen of the station,
     * the notification) and the service with it (code review m-3); «Zuletzt gehört» stays. Playing
     * on is the point of the service: while it plays, the swipe changes nothing.
     */
    override fun onTaskRemoved(rootIntent: Intent?) {
        val player = session?.player
        if (player == null || !player.playWhenReady || player.mediaItemCount == 0) {
            (application as AudioHost).audioPlayback.stop(forget = false)
            stopSelf()
        }
    }

    override fun onDestroy() {
        scope.cancel()
        // The player belongs to AudioPlayback; only the session goes.
        session?.release()
        session = null
        super.onDestroy()
    }

    private companion object {
        const val SESSION_ID = "slouchcat-audio"
        const val TAG = "audio"

        /** Media3 keeps the ids of the process; a new session of a new service takes the next one. */
        val SESSIONS = java.util.concurrent.atomic.AtomicInteger()
    }
}
