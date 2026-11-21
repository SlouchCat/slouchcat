package com.slouchcat.app.mobile.navigation

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.createSavedStateHandle
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.common.SessionEventHandler
import com.slouchcat.app.mobile.connected.ConnectedScreen
import com.slouchcat.app.mobile.connected.ConnectedViewModel
import com.slouchcat.app.mobile.setup.SetupScreen
import com.slouchcat.app.mobile.setup.SetupViewModel
import com.slouchcat.core.design.SlouchcatTheme
import kotlinx.coroutines.flow.first
import kotlinx.serialization.Serializable

/** The pages of the app as typed routes (Navigation Compose). */
@Serializable
data object SetupRoute

@Serializable
data object ConnectedRoute

/**
 * The navigation of the app. The first page depends on the stored server: none yet, the address;
 * otherwise the page after it. Each page gets its ViewModel from [graph].
 */
@Composable
fun SlouchcatNavHost(graph: AppGraph) {
    val start by produceState<Any?>(initialValue = null) {
        value = if (graph.serverStore.server.first() == null) SetupRoute else ConnectedRoute
    }
    val first = start
    if (first == null) {
        Box(Modifier.fillMaxSize().background(SlouchcatTheme.colors.bg))
        return
    }
    val nav = rememberNavController()
    // «Sitzung weg» and «App zu alt» from any call (concept 27, 11.5). Until the sign-in of stage 3
    // part 2 exists, a lost session leads back to the server address, the start of the sign-in.
    SessionEventHandler(graph.sessionEvents, onSignedOut = {
        nav.navigate(SetupRoute) { popUpTo(nav.graph.id) { inclusive = true } }
    })
    NavHost(navController = nav, startDestination = first) {
        setupDestination(graph, onDone = { nav.navigate(ConnectedRoute) { popUpTo(nav.graph.id) { inclusive = true } } })
        connectedDestination(graph, onChangeAddress = { nav.navigate(SetupRoute) })
    }
}

/** One destination per page: the ViewModel from [AppGraph], the state, the screen. */
private fun NavGraphBuilder.setupDestination(graph: AppGraph, onDone: () -> Unit) {
    composable<SetupRoute> {
        val vm = viewModel { SetupViewModel(createSavedStateHandle(), graph.serverProbe, graph.hostResolver, graph.serverStore) }
        val state by vm.state.collectAsStateWithLifecycle()
        LaunchedEffect(vm) { vm.done.collect { onDone() } }
        SetupScreen(
            state = state,
            onAddressChange = vm::onAddressChange,
            onConnect = vm::onConnect,
            onConfirmInsecure = vm::onConfirmInsecure,
            onDismissInsecure = vm::onDismissInsecure,
            onChangeAddress = vm::onChangeAddress,
            onNext = vm::onNext,
        )
    }
}

private fun NavGraphBuilder.connectedDestination(graph: AppGraph, onChangeAddress: () -> Unit) {
    composable<ConnectedRoute> {
        val vm = viewModel { ConnectedViewModel(graph.serverStore, graph.serverRepository) }
        val state by vm.state.collectAsStateWithLifecycle()
        ConnectedScreen(state = state, onRetry = vm::retry, onChangeAddress = onChangeAddress)
    }
}
