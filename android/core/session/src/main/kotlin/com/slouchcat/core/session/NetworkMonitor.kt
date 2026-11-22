package com.slouchcat.core.session

import android.content.Context
import android.net.ConnectivityManager
import android.net.LinkProperties
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import com.slouchcat.core.api.NetworkState
import com.slouchcat.core.api.log.Log
import java.util.concurrent.atomic.AtomicLong

/**
 * Counts the changes of the networks the device uses (concept 27 section 6: «Server weist sich aus»
 * per network). A new generation, and with it a new proof of the server, comes with
 * - a new default network or its loss;
 * - a change of the default network's addresses or routes;
 * - any network appearing or going that is not a VPN (WLAN, mobile, Ethernet), whatever the
 *   default is: under an always-on VPN that lets the local network through, the default network
 *   stays the VPN while the WLAN below it changes (security review M-1). The default request of
 *   [NetworkRequest.Builder] leaves VPNs out, so it sees exactly those networks.
 * A false alarm costs one proof. What stays hidden (a VPN that carries the local network and
 * changes nothing visible below it) is a known limit in ADR 0040; the gate also proves again after
 * ten minutes and after a broken connection. [onChange] runs on the callback thread of the system.
 */
class NetworkMonitor(private val onChange: () -> Unit = {}) : NetworkState {
    private val counter = AtomicLong(0)

    @Volatile private var current: Network? = null

    @Volatile private var routes: LinkProperties? = null

    override fun generation(): Long = counter.get()

    /** Registers with the system once; call it from the Application. */
    fun start(context: Context) {
        val manager = context.getSystemService(ConnectivityManager::class.java) ?: return
        manager.registerDefaultNetworkCallback(
            object : ConnectivityManager.NetworkCallback() {
                override fun onAvailable(network: Network) {
                    if (network != current) changed("default", network)
                }

                override fun onLost(network: Network) {
                    if (network == current) changed("default lost", null)
                }

                override fun onLinkPropertiesChanged(network: Network, properties: LinkProperties) {
                    val before = routes
                    routes = properties
                    if (before != null && (before.linkAddresses != properties.linkAddresses || before.routes != properties.routes)) {
                        changed("addresses", current)
                    }
                }
            },
        )
        val any = NetworkRequest.Builder().addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET).build()
        manager.registerNetworkCallback(
            any,
            object : ConnectivityManager.NetworkCallback() {
                override fun onAvailable(network: Network) = bump("network available")

                override fun onLost(network: Network) = bump("network lost")
            },
        )
    }

    private fun changed(reason: String, network: Network?) {
        current = network
        bump(reason)
    }

    private fun bump(reason: String) {
        counter.incrementAndGet()
        Log.i(TAG, "network changed", "reason" to reason)
        onChange()
    }

    private companion object {
        const val TAG = "network"
    }
}
