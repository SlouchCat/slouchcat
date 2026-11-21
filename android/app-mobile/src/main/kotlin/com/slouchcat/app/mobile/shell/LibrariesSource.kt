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
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.launch

/**
 * The libraries of the active profile (`listVisibleLibraries`) for the media choice of the shell.
 * Loads only when a feature registered media entries ([ShellRegistry.media]); with none, the app
 * sends no request for them. Loads again with every session epoch that has a profile (also when the
 * same profile is chosen again, code review M-2) and forgets them without one. While the same
 * profile reloads, its entries stay (no flicker of the menu); another profile starts empty (8.4).
 * Pages of stage 4 may read [libraries] instead of asking again.
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
    private var shownFor: Long? = null

    init {
        if (registry.media.isNotEmpty()) {
            scope.launch {
                combine(session.epoch, session.state) { epoch, state -> epoch to (state as? SessionState.Ready)?.profile?.id }
                    .distinctUntilChanged()
                    .collect { (_, profile) ->
                        if (profile == null) {
                            clear()
                        } else {
                            if (profile != shownFor) clear()
                            shownFor = profile
                            reload()
                        }
                    }
            }
        }
    }

    fun reload() {
        if (registry.media.isEmpty()) return
        val server = session.server ?: return
        loading?.cancel()
        if (_libraries.value !is Loadable.Ready) _libraries.value = Loadable.Loading
        loading = scope.launch {
            val result = repository.libraries(server).toLoadable { it.isEmpty() }
            // A failed reload keeps the entries the menu shows.
            if (result is Loadable.Failed && _libraries.value is Loadable.Ready) return@launch
            _libraries.value = result
        }
    }

    fun clear() {
        loading?.cancel()
        shownFor = null
        _libraries.value = Loadable.Empty
    }
}
