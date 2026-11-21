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
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.slouchcat.app.mobile.AppGraph
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
    NavHost(navController = nav, startDestination = first) {
        composable<SetupRoute> {
            val vm = viewModel { SetupViewModel(graph.serverProbe, graph.hostResolver, graph.serverStore) }
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(state.saved) {
                if (state.saved) {
                    nav.navigate(ConnectedRoute) { popUpTo(nav.graph.id) { inclusive = true } }
                }
            }
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
        composable<ConnectedRoute> {
            val vm = viewModel { ConnectedViewModel(graph.serverStore) }
            val server by vm.server.collectAsStateWithLifecycle()
            ConnectedScreen(server = server, onChangeAddress = { nav.navigate(SetupRoute) })
        }
    }
}
