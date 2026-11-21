package com.slouchcat.app.mobile.playback

import android.content.Context
import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.MoreEntry
import com.slouchcat.app.mobile.shell.MoreSection
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.player.ApiPlaybackPrefsRepository
import com.slouchcat.core.player.DataStoreDeviceSettingsStore
import com.slouchcat.core.player.DeviceSettings
import com.slouchcat.core.player.DeviceSettingsStore
import com.slouchcat.core.player.PlaybackPrefsRepository
import com.slouchcat.core.player.QualityChoice
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable

/** «Einstellungen › Wiedergabe» (mockup `/settings/playback`). */
@Serializable
data object PlaybackSettingsRoute

/**
 * The page «Wiedergabe» and its row in «Mehr › Einstellungen» (order 20, after «Profile
 * verwalten»), with the qualities as its second line (mockup `su-m-play`). [settings],
 * [repository]: tests bring fakes.
 */
class PlaybackSettingsFeature(
    private val settings: (Context) -> DeviceSettingsStore = { DataStoreDeviceSettingsStore(it) },
    private val repository: (AppGraph) -> PlaybackPrefsRepository = { graph -> ApiPlaybackPrefsRepository(graph.api, graph.http) { graph.session.server } },
) : Feature {
    override val more = listOf(
        MoreEntry("playback", MoreSection.Settings, 20, R.string.app_playback_title, SlouchcatIcon.Play, PlaybackSettingsRoute, subtitle = { summary() }),
    )

    /** `settings/playback`: the page under «Mehr» (as the web app's address). */
    override fun link(address: String): AppLink? =
        if (address.removePrefix("/") == ADDRESS) AppLink(MORE, listOf(PlaybackSettingsRoute)) else null

    @Composable
    private fun summary(): String {
        val context = LocalContext.current.applicationContext
        val store = remember(context) { settings(context) }
        val device by store.settings.collectAsStateWithLifecycle(DeviceSettings())
        return stringResource(R.string.app_playback_summary, qualityName(device.qualityMobile), qualityName(device.qualityWifi))
    }

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        composable<PlaybackSettingsRoute> {
            val context = LocalContext.current.applicationContext
            val vm = viewModel { PlaybackSettingsViewModel(settings(context), repository(graph)) }
            val state by vm.state.collectAsStateWithLifecycle()
            val session by graph.session.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            val resources = LocalResources.current
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    snackbar.showSnackbar(
                        resources.getString(
                            when (message) {
                                PlaybackMessage.Saved -> R.string.settings_playback_saved
                                PlaybackMessage.Failed -> R.string.app_playback_failed
                                PlaybackMessage.SeriesReset -> R.string.settings_playback_series_done
                            },
                        ),
                    )
                }
            }
            val events = remember(vm) { events(vm, app.navigator::back) }
            PlaybackSettingsScreen(state, events, snackbar, profileName = (session as? SessionState.Ready)?.profile?.name)
        }
    }

    private companion object {
        const val ADDRESS = "settings/playback"
        const val MORE = "more"
    }
}

/** The events of the page, bound to its ViewModel and the navigation. */
internal fun events(vm: PlaybackSettingsViewModel, onBack: () -> Unit) = object : PlaybackSettingsEvents {
    override fun onBack() = onBack()
    override fun onRetry() = vm.retry()
    override fun onSheet(sheet: PlaybackSheet?) = vm.onSheet(sheet)
    override fun onQualityMobile(choice: QualityChoice) = vm.onQualityMobile(choice)
    override fun onQualityWifi(choice: QualityChoice) = vm.onQualityWifi(choice)
    override fun onAutoplay(on: Boolean) = vm.onAutoplay(on)
    override fun onSkipIntro(on: Boolean) = vm.onSkipIntro(on)
    override fun onWifiOnly(on: Boolean) = vm.onWifiOnly(on)
    override fun onAudioLanguage(code: String?) = vm.onAudioLanguage(code)
    override fun onSubtitleMode(mode: SubtitleMode) = vm.onSubtitleMode(mode)
    override fun onSubtitleLanguage(code: String?) = vm.onSubtitleLanguage(code)
    override fun onResetSeries() = vm.onResetSeries()
}
