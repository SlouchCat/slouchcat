package com.slouchcat.app.mobile.shell

import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.toLoadable
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.session.LibrariesRepository
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.launch

/**
 * The libraries of the active profile (`listVisibleLibraries`) for the media choice of the shell.
 * Loads only when a feature registered media entries ([ShellRegistry.media]); with none, the app
 * sends no request for them. Reloads at every profile change and forgets them at the end of the
 * session. Pages of stage 4 may read [libraries] too instead of asking again.
 */
class LibrariesSource(
    private val session: SessionManager,
    private val repository: LibrariesRepository,
    private val registry: ShellRegistry,
    private val scope: CoroutineScope,
) {
    private val _libraries = MutableStateFlow<Loadable<List<LibraryNav>>>(Loadable.Empty)
    val libraries: StateFlow<Loadable<List<LibraryNav>>> = _libraries.asStateFlow()
    private var loading: Job? = null

    init {
        if (registry.media.isNotEmpty()) {
            scope.launch {
                session.state
                    .map { (it as? SessionState.Ready)?.profile?.id }
                    .distinctUntilChanged()
                    .collect { profile -> if (profile != null) reload() else clear() }
            }
        }
    }

    fun reload() {
        if (registry.media.isEmpty()) return
        val server = session.server ?: return
        loading?.cancel()
        _libraries.value = Loadable.Loading
        loading = scope.launch { _libraries.value = repository.libraries(server).toLoadable { it.isEmpty() } }
    }

    fun clear() {
        loading?.cancel()
        _libraries.value = Loadable.Empty
    }
}
