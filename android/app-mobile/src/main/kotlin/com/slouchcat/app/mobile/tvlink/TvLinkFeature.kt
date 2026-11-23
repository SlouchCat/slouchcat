package com.slouchcat.app.mobile.tvlink

import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.LifecycleStartEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.more.DevicesRoute
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.MoreEntry
import com.slouchcat.app.mobile.shell.MoreSection
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.pairing.ApiPairingServer
import com.slouchcat.core.pairing.PhonePairing
import com.slouchcat.core.pairing.PhonePairingFlow
import com.slouchcat.core.pairing.RelayClient
import com.slouchcat.core.session.DataStoreRelaySettings
import com.slouchcat.core.session.RelaySettings
import com.slouchcat.core.strings.R
import java.time.Duration
import kotlinx.serialization.Serializable
import okhttp3.HttpUrl
import okhttp3.OkHttpClient

@Serializable
data object TvLinkRoute

/** «Adresse der Vermittlung» (F-21), under «Mehr › Einstellungen». */
@Serializable
data object RelayRoute

/**
 * «Mehr › Fernseher verbinden» (concept 27 section 10, stage 9; mockup `/tv`): the row under
 * «Geräte» before «Meine Geräte», and the page. [scanner] is the flavour's ([flavorScanner]); tests
 * and the emulator flow pass their own, as [relaySettings] (the relay in memory).
 */
class TvLinkFeature(
    private val scanner: CodeScanner = flavorScanner(),
    private val relaySettings: (Context) -> RelaySettings = { DataStoreRelaySettings(it, "pairing") },
    /** The steps of a pairing; `null`: over the relay and the server. The showcase scripts its own. */
    private val pairing: ((AppGraph, relay: HttpUrl, server: HttpUrl) -> PhonePairingFlow)? = null,
) : Feature {
    override val more = listOf(
        MoreEntry("tv", MoreSection.Devices, 10, R.string.app_tv_title, SlouchcatIcon.Tv, TvLinkRoute, subtitle = { stringResource(R.string.app_tv_moreHint) }),
        MoreEntry("relay", MoreSection.Settings, 80, R.string.tv_relay_title, SlouchcatIcon.Link, RelayRoute, subtitle = { relayHost(it) }),
    )

    @Composable
    private fun relayHost(graph: AppGraph): String {
        val context = LocalContext.current.applicationContext
        val settings = remember(graph) { relaySettings(context) }
        val relay by settings.relay.collectAsStateWithLifecycle(RelaySettings.DEFAULT)
        return relay.host
    }

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<TvLinkRoute> {
            val context = LocalContext.current.applicationContext
            val vm = viewModel {
                // The relay never sees the credentials of the server and follows no redirect: a 307
                // would carry the packet to another host (as on the TV, review N-5).
                // No repeat of a request after a lost connection either: a second `PUT` of the own
                // packet would answer 409/410 and look like a foreign packet (security review L-1).
                val relayHttp = SlouchcatHttp.client(
                    graph.identity,
                    builder = OkHttpClient.Builder().callTimeout(RELAY_TIMEOUT).followRedirects(false).retryOnConnectionFailure(false),
                )
                TvLinkViewModel(graph.session, relaySettings(context), graph.devices, graph.serverRepository) { relay, server ->
                    pairing?.invoke(graph, relay, server) ?: PhonePairing(RelayClient(relayHttp, relay), ApiPairingServer(graph.api, graph.http, server))
                }
            }
            val state by vm.state.collectAsStateWithLifecycle()
            // The pairing lives only while the page is seen (K26 stage 2a review): another page, the
            // home screen or the screen off stop it.
            LifecycleStartEffect(vm) {
                vm.onShown()
                onStopOrDispose { vm.onHidden() }
            }
            TvLinkScreen(
                state = state,
                scanner = scanner,
                actions = TvLinkActions(
                    onBack = navigator::back,
                    onScan = vm::onScan,
                    onScanned = vm::onScanned,
                    onScanClosed = vm::onScanClosed,
                    accepts = vm::accepts,
                    onScannerLoading = vm::onScannerLoading,
                    onScannerTrouble = vm::onScannerTrouble,
                    onConfirm = vm::onConfirm,
                    onBackToIntro = vm::onBackToIntro,
                    onSwitchProfile = navigator::toProfiles,
                    onDevices = { navigator.navigate(DevicesRoute) },
                    onFinish = navigator::back,
                    onNotMine = vm::onNotMine,
                    onNotMineConfirm = vm::onNotMineConfirm,
                    onNotMineDismiss = vm::onNotMineDismiss,
                ),
            )
        }
        composable<RelayRoute> {
            val context = LocalContext.current.applicationContext
            val vm = viewModel { RelayViewModel(graph.session, relaySettings(context)) }
            val relay by vm.relay.collectAsStateWithLifecycle()
            RelayScreen(relay = relay, primary = vm.primary, onBack = navigator::back, onSave = { vm.onSave(it) { navigator.back() } })
        }
    }

    private companion object {
        val RELAY_TIMEOUT: Duration = Duration.ofSeconds(15)
    }
}
