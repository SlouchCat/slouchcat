package com.slouchcat.app.mobile

import android.graphics.Bitmap
import androidx.activity.ComponentActivity
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.mobile.player.PlayerEvents
import com.slouchcat.app.mobile.player.PlayerScreen
import com.slouchcat.core.api.generated.models.ItemImages
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.Quality
import com.slouchcat.core.api.generated.models.Track
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.player.PlayerError
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.strings.R
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/**
 * The error pictures of the player on the device (K27 stage 5): the states a real server gives
 * only by chance (a title gone, the limits, a stop elsewhere, no network, an unplayable file),
 * drawn from a state and photographed (prefix `pe`; upright: a turn would recreate the test's activity) with `slouchcatShots`.
 */
@RunWith(AndroidJUnit4::class)
class PlayerPicturesTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private val shots = InstrumentationRegistry.getArguments().getString("slouchcatShots") != null

    private val events = object : PlayerEvents {
        override fun onBack() = Unit
        override fun onTogglePlay() = Unit
        override fun onSeekTo(positionMs: Long) = Unit
        override fun onSeekBy(deltaMs: Long) = Unit
        override fun onSpeed(speed: Float) = Unit
        override fun onAudio(track: Track) = Unit
        override fun onSubtitle(track: Track?) = Unit
        override fun onQuality(quality: Quality) = Unit
        override fun onSkipIntro() = Unit
        override fun onSkipOutro() = Unit
        override fun onPrevious() = Unit
        override fun onNext() = Unit
        override fun onEpisode(id: String) = Unit
        override fun onCancelCountdown() = Unit
        override fun onRetry() = Unit
        override fun onEpisodesOpened() = Unit
    }

    private val film = ItemRef(id = "film", kind = ItemKind.MOVIE, title = "Two Voices", inWatchlist = false, images = ItemImages(), year = 2024, runtimeMinutes = 1)

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        InstrumentationRegistry.getInstrumentation().waitForIdleSync()
        val bitmap = InstrumentationRegistry.getInstrumentation().uiAutomation.takeScreenshot() ?: return
        val dir = File(context.getExternalFilesDir(null), "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    @Test
    fun errorPictures() {
        var state by mutableStateOf(PlayerState())
        compose.setContent { SlouchcatTheme { PlayerScreen(state, events) } }
        val cases = listOf(
            Triple(PlayerError.NotFound, R.string.app_player_error_notFoundTitle, "pe01-not-found"),
            Triple(PlayerError.WatchLimit, R.string.player_watchLimit_title, "pe02-watch-limit"),
            Triple(PlayerError.StreamLimit, R.string.player_streamLimit_title, "pe03-stream-limit"),
            Triple(PlayerError.StoppedElsewhere, R.string.player_ended_title, "pe04-stopped-elsewhere"),
            Triple(PlayerError.Network(retrying = true), R.string.app_player_error_networkTitle, "pe05-network"),
            Triple(PlayerError.Unplayable, R.string.app_player_error_unplayableTitle, "pe06-unplayable"),
        )
        for ((error, title, name) in cases) {
            state = PlayerState(item = film, loading = false, durationMs = 60_000, error = error)
            compose.waitUntil(5_000) { compose.isShown(hasText(context.getString(title)) and isHeading()) }
            shot(name)
        }
    }
}
