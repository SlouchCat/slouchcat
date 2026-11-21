package com.slouchcat.app.tv.settings

import androidx.compose.runtime.getValue
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.slouchcat.app.tv.BuildConfig
import com.slouchcat.app.tv.shell.Label
import com.slouchcat.app.tv.shell.RailEntry
import com.slouchcat.app.tv.shell.RailSection
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable

@Serializable
data object SettingsRoute

/**
 * «Einstellungen» (stage 2a): the entry of the rail (Bottom, order 50) and the page. The rows of
 * later stages come from `TvFeature.settings` (Registry); this feature brings only its own.
 */
object SettingsFeature : TvFeature {
    override val rail = listOf(
        RailEntry("settings", RailSection.Bottom, 50, Label.Res(R.string.tv_settings_title), SlouchcatIcon.Settings.vector, SettingsRoute),
    )

    override fun NavGraphBuilder.register(app: TvScope) {
        composable<SettingsRoute> {
            val graph = app.graph
            val vm = viewModel {
                SettingsViewModel(
                    session = graph.session,
                    actions = graph.sessionActions,
                    relaySettings = graph.relaySettings,
                    newKey = graph.identityGate.newKey,
                    tvName = graph.device.name,
                    appVersion = BuildConfig.VERSION_NAME,
                    onSignedOut = { confirmed -> graph.toasts.show(if (confirmed) R.string.tv_settings_signedOut else R.string.tv_settings_signedOutOffline) },
                    appScope = graph.scope,
                )
            }
            val state by vm.state.collectAsStateWithLifecycle()
            val extra = graph.registry.settings.map { entry ->
                SettingsRow(
                    key = entry.key,
                    order = entry.order,
                    title = stringResource(entry.title),
                    subtitle = entry.subtitle?.invoke(graph),
                    icon = entry.icon,
                    onClick = { app.navigator.navigate(entry.route) },
                    pane = entry.pane?.let { pane -> { pane(graph) } },
                )
            }
            SettingsScreen(
                state = state,
                extra = extra,
                onSwitchProfile = app.navigator::toProfiles,
                onSignOut = vm::onSignOut,
                onRelay = vm::onRelay,
            )
        }
    }
}
