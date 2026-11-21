package com.slouchcat.core.session

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.net.InetAddress

class HomeNetworkTest {
    private fun home(literal: String) = HomeNetwork.isHome(InetAddress.getByName(literal))

    @Test
    fun theRangesOfConcept27AreHome() {
        for (address in listOf(
            "10.20.30.40", "10.0.2.2", "10.255.255.255",
            "172.16.0.1", "172.31.255.254",
            "192.168.0.255", "192.168.77.20",
            "169.254.1.1",
            "100.64.0.1", "100.127.255.254",
            "127.0.0.1",
        )) {
            assertTrue(address, home(address))
        }
    }

    @Test
    fun theEdgesOfTheRangesAreNot() {
        for (address in listOf(
            "172.15.255.255", "172.32.0.0",
            "192.167.255.255", "192.169.0.0",
            "100.63.255.255", "100.128.0.0",
            "169.253.255.255",
            "11.0.0.1", "9.255.255.255",
        )) {
            assertFalse(address, home(address))
        }
    }

    @Test
    fun documentationAndPublicAddressesAreNot() {
        for (address in listOf("192.0.2.10", "198.51.100.7", "203.0.113.1", "8.8.8.8", "2001:db8::1", "2a00:1450::1")) {
            assertFalse(address, home(address))
        }
    }

    @Test
    fun ipv6OfAHomeNetworkIs() {
        for (address in listOf("::1", "fe80::1", "febf::1", "fd12:3456::1", "fc00::1", "::ffff:192.168.77.5")) {
            assertTrue(address, home(address))
        }
        assertFalse("fec0::1", home("fec0::1"))
        assertFalse("::ffff:8.8.8.8", home("::ffff:8.8.8.8"))
    }
}
