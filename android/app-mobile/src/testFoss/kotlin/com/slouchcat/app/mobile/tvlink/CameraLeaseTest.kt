package com.slouchcat.app.mobile.tvlink

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** The camera of the own scanner (build `foss`, code review M-1): never bound after the scanner closed. */
class CameraLeaseTest {
    private val events = mutableListOf<String>()

    private fun lease(binds: Boolean = true) = CameraLease<String>(
        bind = { provider -> events += "bind:$provider"; binds },
        unbind = { provider -> events += "unbind:$provider" },
        release = { events += "release" },
    )

    @Test
    fun theProviderArrivesWhileTheScannerIsOpen() {
        val lease = lease()
        lease.onProvider("camera")
        assertTrue(lease.isBound)
        lease.close()
        assertFalse(lease.isBound)
        // Only the own use cases, before the analysis thread stops.
        assertEquals(listOf("bind:camera", "unbind:camera", "release"), events)
    }

    @Test
    fun aProviderThatArrivesAfterTheScannerClosedBindsNothing() {
        val lease = lease()
        lease.close()
        lease.onProvider("camera")
        assertFalse(lease.isBound)
        assertEquals(listOf("release"), events)
    }

    @Test
    fun aCameraThatCouldNotBeBoundIsNotUnbound() {
        val lease = lease(binds = false)
        lease.onProvider("camera")
        lease.close()
        assertEquals(listOf("bind:camera", "release"), events)
    }

    @Test
    fun closingTwiceAndASecondProviderChangeNothing() {
        val lease = lease()
        lease.onProvider("a")
        lease.onProvider("b")
        lease.close()
        lease.close()
        assertEquals(listOf("bind:a", "unbind:a", "release"), events)
    }
}
