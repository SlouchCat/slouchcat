package com.slouchcat.core.session

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.net.Inet4Address
import java.net.Inet6Address
import java.net.InetAddress

/**
 * Whether an address lies in a home network (concept 27, 6: the warning about plain HTTP outside
 * it). Decided on the resolved address, not on the name: the ranges Android 17 treats as local
 * (2.7: 10/8, 172.16/12, 192.168/16, 169.254/16, 100.64/10), plus the device itself (127/8, ::1)
 * and the IPv6 counterparts of a home network (link-local fe80::/10, unique local fc00::/7).
 */
object HomeNetwork {
    fun isHome(address: InetAddress): Boolean = when (address) {
        is Inet4Address -> isHomeV4(address.address)
        is Inet6Address -> isHomeV6(address.address)
        else -> false
    }

    private fun isHomeV4(b: ByteArray): Boolean {
        val a0 = b[0].toInt() and 0xff
        val a1 = b[1].toInt() and 0xff
        return a0 == 10 ||
            a0 == 127 ||
            (a0 == 172 && a1 in 16..31) ||
            (a0 == 192 && a1 == 168) ||
            (a0 == 169 && a1 == 254) ||
            (a0 == 100 && a1 in 64..127)
    }

    private fun isHomeV6(b: ByteArray): Boolean {
        val first = b[0].toInt() and 0xff
        val second = b[1].toInt() and 0xff
        val loopback = b.take(15).all { it.toInt() == 0 } && b[15].toInt() == 1
        val mappedV4 = b.take(10).all { it.toInt() == 0 } && (b[10].toInt() and 0xff) == 0xff && (b[11].toInt() and 0xff) == 0xff
        return loopback ||
            (first == 0xfe && (second and 0xc0) == 0x80) ||
            (first and 0xfe) == 0xfc ||
            (mappedV4 && isHomeV4(b.copyOfRange(12, 16)))
    }
}

/** Resolves a host name; a seam for tests. */
fun interface HostResolver {
    /** All addresses of [host]; throws [java.net.UnknownHostException] when there are none. */
    suspend fun resolve(host: String): List<InetAddress>

    companion object {
        val System = HostResolver { host -> withContext(Dispatchers.IO) { InetAddress.getAllByName(host).toList() } }
    }
}
