package com.slouchcat.app.tv.playback

import android.content.Context
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.tv.material3.Text
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.player.TvFactsReader
import com.slouchcat.app.tv.shell.SettingsEntry
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.player.ApiPlaybackPrefsRepository
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DeviceSettings
import com.slouchcat.core.player.DeviceSettingsStore
import com.slouchcat.core.player.PlaybackPrefsRepository
import com.slouchcat.core.player.QualityChoice
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlin.coroutines.CoroutineContext

/** «Einstellungen › Wiedergabe» (mockup `/settings/playback`). */
@Serializable
data object PlaybackSettingsRoute

/**
 * The page «Wiedergabe» and its row in the settings (order 30, after «Profile verwalten»; mockup
 * `MD_SET`) with «Automatisch · Ton Automatisch» as its second line. The menu «Ton» of the player
 * opens it as well. [settings], [repository], [facts]: tests bring fakes.
 */
class PlaybackSettingsFeature(
    private val deviceSettings: (Context) -> DeviceSettingsStore = ::tvDeviceSettings,
    private val repository: (TvGraph) -> PlaybackPrefsRepository = { graph -> ApiPlaybackPrefsRepository(graph.api, graph.http) { graph.session.server } },
    private val facts: (Context, TvGraph) -> DeviceFacts = { context, graph -> TvFactsReader(context) { graph.device.name }.read() },
    private val background: CoroutineContext = Dispatchers.Default,
) : TvFeature {
    override val settings = listOf(
        SettingsEntry(
            key = "playback",
            order = ORDER,
            title = R.string.app_playback_title,
            icon = SlouchcatIcon.Play.vector,
            route = PlaybackSettingsRoute,
            subtitle = { summary() },
            pane = { Pane() },
        ),
    )

    @Composable
    private fun summary(): String {
        val context = LocalContext.current.applicationContext
        val store = remember(context) { deviceSettings(context) }
        val device by store.settings.collectAsStateWithLifecycle(DeviceSettings(qualityMobile = QualityChoice.Auto, qualityWifi = QualityChoice.Auto))
        return stringResource(R.string.tv_playback_summary, qualityName(device.qualityWifi), soundSettingName(device.audio))
    }

    @Composable
    private fun Pane() {
        Column(verticalArrangement = Arrangement.spacedBy(TvDimens.l)) {
            Text(stringResource(R.string.app_playback_title), style = TvType.h2)
            Text(stringResource(R.string.tv_playback_pane), style = TvType.body, color = SlouchcatTheme.colors.tx2)
        }
    }

    override fun NavGraphBuilder.register(app: TvScope) {
        val graph = app.graph
        composable<PlaybackSettingsRoute> {
            val context = LocalContext.current.applicationContext
            val vm = viewModel { PlaybackSettingsViewModel(deviceSettings(context), repository(graph)) { withContext(background) { facts(context, graph) } } }
            val state by vm.state.collectAsStateWithLifecycle()
            val session by graph.session.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    when (message) {
                        PlaybackMessage.Saved -> graph.toasts.show(R.string.settings_playback_saved)
                        PlaybackMessage.Failed -> graph.toasts.show(R.string.app_playback_failed)
                    }
                }
            }
            val events = remember(vm) { events(vm) }
            PlaybackSettingsScreen(state, events, profileName = (session as? SessionState.Ready)?.profile?.name)
        }
    }

    private companion object {
        /** The place of the row (Registry: Profile verwalten 20, Wiedergabe 30). */
        const val ORDER = 30
    }
}

/** The events of the page, bound to its ViewModel. */
internal fun events(vm: PlaybackSettingsViewModel) = object : PlaybackSettingsEvents {
    override fun onRetry() = vm.retry()
    override fun onMenu(menu: PlaybackMenu?) = vm.onMenu(menu)
    override fun onQuality(choice: QualityChoice) = vm.onQuality(choice)
    override fun onAudioLanguage(code: String?) = vm.onAudioLanguage(code)
    override fun onSubtitleMode(mode: SubtitleMode) = vm.onSubtitleMode(mode)
    override fun onSubtitleLanguage(code: String?) = vm.onSubtitleLanguage(code)
    override fun onAutoplay(on: Boolean) = vm.onAutoplay(on)
    override fun onSkipIntro(on: Boolean) = vm.onSkipIntro(on)
    override fun onSoundAuto() = vm.onSoundAuto()
    override fun onSoundStereo() = vm.onSoundStereo()
    override fun onSoundManual() = vm.onSoundManual()
    override fun onFormat(format: String, on: Boolean) = vm.onFormat(format, on)
    override fun onChannels(channels: Int) = vm.onChannels(channels)
}
