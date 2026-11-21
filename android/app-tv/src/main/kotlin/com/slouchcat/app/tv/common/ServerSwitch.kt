package com.slouchcat.app.tv.common

import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.withTimeoutOrNull

/**
 * Stores [server] and waits until the session has taken it (it forgets the session of any other
 * server first). `false` when that did not happen within [timeoutMs]. Without UI and without
 * Android: the same step of the phone app (review N-15) can share it once it moves to `:core:session`.
 */
suspend fun SessionManager.takeServer(servers: ServerStore, server: SavedServer, timeoutMs: Long = SERVER_SWITCH_MS): Boolean {
    servers.save(server)
    return withTimeoutOrNull(timeoutMs) {
        state.first { s -> s is SessionState.SignedOut && s.server.address == server.address && s.server.serverKey == server.serverKey }
    } != null
}

private const val SERVER_SWITCH_MS = 5_000L
