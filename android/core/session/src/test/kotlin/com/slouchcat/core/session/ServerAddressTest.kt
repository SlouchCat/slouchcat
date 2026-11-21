package com.slouchcat.core.session

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ServerAddressTest {
    private fun valid(input: String): ParsedAddress.Valid =
        ServerAddress.parse(input) as? ParsedAddress.Valid ?: error("$input should be valid")

    @Test
    fun withoutSchemeItIsHttps() {
        val parsed = valid("  medien.example.test/ ")
        assertEquals("https://medien.example.test", ServerAddress.display(parsed.url))
        assertTrue(parsed.encrypted)
    }

    @Test
    fun httpPortsPathsAndBareHostsStay() {
        assertEquals("http://192.0.2.10:8096", ServerAddress.display(valid("http://192.0.2.10:8096/").url))
        assertFalse(valid("HTTP://192.0.2.10:8096").encrypted)
        assertEquals("https://example.test/media", ServerAddress.display(valid("https://example.test//media/").url))
        assertEquals("http://nas:8096", ServerAddress.display(valid("http://nas:8096").url))
        assertEquals("http://[fd00::5]:8096", ServerAddress.display(valid("http://[fd00::5]:8096").url))
    }

    @Test
    fun emptyAndBrokenAddressesAreTold() {
        assertEquals(ParsedAddress.Empty, ServerAddress.parse("   "))
        for (bad in listOf("ftp://example.test", "https://", "exa mple.test", "https://user:pw@example.test", "https://example.test/?a=1", "https://example.test/#x", "http//x")) {
            assertEquals(bad, ParsedAddress.Invalid, ServerAddress.parse(bad))
        }
    }
}
