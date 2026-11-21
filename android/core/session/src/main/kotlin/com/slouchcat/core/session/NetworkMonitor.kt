package com.slouchcat.core.session

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import com.slouchcat.core.api.NetworkState
import com.slouchcat.core.api.log.Log
import java.util.concurrent.atomic.AtomicLong

/**
 * Counts the changes of the default network (concept 27 section 6: «Server weist sich aus» per
 * network). Every new default network and every loss of it is a new generation; the gate proves
 * the server again. [onChange] runs on the callback thread of the system (drop pooled connections there).
 */
class NetworkMonitor(private val onChange: () -> Unit = {}) : NetworkState {
    private val counter = AtomicLong(0)

    @Volatile private var current: Network? = null

    override fun generation(): Long = counter.get()

    /** Registers with the system once; call it from the Application. */
    fun start(context: Context) {
        val manager = context.getSystemService(ConnectivityManager::class.java) ?: return
        manager.registerDefaultNetworkCallback(
            object : ConnectivityManager.NetworkCallback() {
                override fun onAvailable(network: Network) {
                    if (network != current) changed(network)
                }

                override fun onLost(network: Network) {
                    if (network == current) changed(null)
                }
            },
        )
    }

    private fun changed(network: Network?) {
        current = network
        counter.incrementAndGet()
        Log.i(TAG, "network changed", "available" to (network != null))
        onChange()
    }

    private companion object {
        const val TAG = "network"
    }
}
