package com.slouchcat.app.mobile.more

import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.BuildConfig
import com.slouchcat.app.mobile.account.AccountMessage
import com.slouchcat.app.mobile.account.AccountScreen
import com.slouchcat.app.mobile.account.AccountViewModel
import com.slouchcat.app.mobile.devices.DevicesScreen
import com.slouchcat.app.mobile.devices.DevicesViewModel
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.MoreEntry
import com.slouchcat.app.mobile.shell.MoreSection
import com.slouchcat.app.mobile.shell.TopDestination
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable

@Serializable
data object MoreRoute

@Serializable
data object DevicesRoute

@Serializable
data object AccountRoute

/**
 * «Mehr» with «Meine Geräte» and «Konto» (stage 3): the target «Mehr» of the bar and two rows of
 * the page. Later stages add their rows to [more] of their own feature.
 */
object MoreFeature : Feature {
    override val destinations = listOf(TopDestination("more", 50, MoreRoute, R.string.app_more_title, SlouchcatIcon.Dots))

    override val more = listOf(
        MoreEntry("devices", MoreSection.Devices, 20, R.string.nav_devices, SlouchcatIcon.Phone, DevicesRoute, subtitle = { devicesSubtitle(it) }),
        MoreEntry("account", MoreSection.Settings, 90, R.string.nav_account, SlouchcatIcon.User, AccountRoute, subtitle = { accountSubtitle(it) }),
    )

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<MoreRoute> {
            val vm = viewModel { MoreViewModel(graph.session, graph.hooks, graph.sessionActions) }
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) { vm.signedOut.collect { navigator.toSignIn() } }
            val rows = graph.registry.more.map { entry ->
                MoreRow(
                    key = entry.key,
                    section = entry.section,
                    icon = entry.icon,
                    title = stringResource(entry.title),
                    subtitle = entry.subtitle?.invoke(graph),
                    onClick = { navigator.navigate(entry.route) },
                )
            }
            MoreScreen(
                state = state,
                rows = rows,
                appVersion = BuildConfig.VERSION_NAME,
                onSwitchProfile = navigator::toProfiles,
                onServer = vm::onServer,
                onSignOutAsk = vm::onSignOutAsk,
                onChoice = vm::onChoice,
                onSignOut = vm::onSignOut,
                onDismissSheet = vm::onDismissSheet,
            )
        }
        composable<DevicesRoute> {
            val vm = viewModel { DevicesViewModel(graph.session, graph.devices) }
            val state by vm.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            val resources = LocalResources.current
            LaunchedEffect(vm) {
                vm.signedOut.collect { name -> snackbar.showSnackbar(resources.getString(R.string.settings_devices_signedOut, name)) }
            }
            DevicesScreen(
                state = state,
                snackbar = snackbar,
                onBack = navigator::back,
                onRetry = vm::retry,
                onAsk = vm::onAsk,
                onConfirm = vm::onConfirm,
                onDismiss = vm::onDismiss,
            )
        }
        composable<AccountRoute> {
            val vm = viewModel { AccountViewModel(graph.session, graph.auth) }
            val state by vm.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            val resources = LocalResources.current
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    val text = when (message) {
                        AccountMessage.EmailChanged -> R.string.app_account_emailChanged
                        AccountMessage.PasswordChanged -> R.string.app_account_passwordChanged
                    }
                    snackbar.showSnackbar(resources.getString(text))
                }
            }
            AccountScreen(
                state = state,
                snackbar = snackbar,
                onBack = navigator::back,
                onEmailAsk = vm::onEmailAsk,
                onPasswordAsk = vm::onPasswordAsk,
                onField = vm::onField,
                onSave = vm::onSave,
                onDismiss = vm::onDismiss,
            )
        }
    }
}

/** «2 Geräte angemeldet» under «Meine Geräte» (mockup); the hint while it loads or without answer. */
@Composable
private fun devicesSubtitle(graph: AppGraph): String {
    val count by produceState<Int?>(null, graph) {
        val server = graph.session.server ?: return@produceState
        value = (graph.devices.devices(server) as? ApiResult.Ok)?.value?.size
    }
    val n = count
    return if (n == null) stringResource(R.string.app_more_devicesHint) else pluralStringResource(R.plurals.app_more_devicesCount, n, n)
}

@Composable
private fun accountSubtitle(graph: AppGraph): String? {
    val state by graph.session.state.collectAsStateWithLifecycle()
    return (state as? SessionState.Ready)?.email
}
