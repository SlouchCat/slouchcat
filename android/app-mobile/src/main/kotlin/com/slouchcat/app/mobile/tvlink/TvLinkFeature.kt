package com.slouchcat.app.mobile.tvlink

import android.content.Context
import androidx.compose.runtime.getValue
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
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable
import okhttp3.HttpUrl
import okhttp3.OkHttpClient
import java.time.Duration

@Serializable
data object TvLinkRoute

/**
 * «Mehr › Fernseher verbinden» (concept 27 section 10, stage 9; mockup `/tv`): the row under
 * «Geräte» before «Meine Geräte», and the page. [scanner] is the flavour's ([flavorScanner]); tests
 * and the emulator flow pass their own, as [relaySettings] (the relay in memory).
 */
class TvLinkFeature(
    private val scanner: CodeScanner = flavorScanner(),
    private val relaySettings: (Context) -> RelaySettings = ::DataStoreRelaySettings,
    /** The steps of a pairing; `null`: over the relay and the server. The showcase scripts its own. */
    private val pairing: ((AppGraph, relay: HttpUrl, server: HttpUrl) -> PhonePairingFlow)? = null,
) : Feature {
    override val more = listOf(
        MoreEntry("tv", MoreSection.Devices, 10, R.string.app_tv_title, SlouchcatIcon.Tv, TvLinkRoute, subtitle = { stringResource(R.string.app_tv_moreHint) }),
    )

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<TvLinkRoute> {
            val context = LocalContext.current.applicationContext
            val vm = viewModel {
                // The relay never sees the credentials of the server and follows no redirect: a 307
                // would carry the packet to another host (as on the TV, review N-5).
                val relayHttp = SlouchcatHttp.client(graph.identity, builder = OkHttpClient.Builder().callTimeout(RELAY_TIMEOUT).followRedirects(false))
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
                    onScannerUnavailable = vm::onScannerUnavailable,
                    onConfirm = vm::onConfirm,
                    onBackToIntro = vm::onBackToIntro,
                    onSwitchProfile = navigator::toProfiles,
                    onDevices = { navigator.navigate(DevicesRoute) },
                    onFinish = navigator::back,
                    onEditRelay = vm::onEditRelay,
                    onRelay = vm::onRelay,
                ),
            )
        }
    }

    private companion object {
        val RELAY_TIMEOUT: Duration = Duration.ofSeconds(15)
    }
}
