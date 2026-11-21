package com.slouchcat.core.api.log

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException

class LogTest {
    private val lines = mutableListOf<String>()

    private fun capture(block: () -> Unit): String {
        val before = Log.sink
        Log.sink = object : LogSink {
            override fun write(priority: Int, tag: String, message: String) {
                lines += "$priority $tag $message"
            }
        }
        try {
            block()
        } finally {
            Log.sink = before
        }
        return lines.joinToString("\n")
    }

    @Test
    fun secretFieldsAreRedactedByName() {
        val out = capture {
            Log.i("auth", "signed in", "token" to "abc123", "device-id" to "dev-9", "Voucher" to "v-1", "version" to "1.42.0")
        }
        assertEquals("4 auth signed in token=[redacted] device-id=[redacted] Voucher=[redacted] version=1.42.0", out)
    }

    @Test
    fun messagesAndErrorsAreScrubbed() {
        val error = IOException(
            "failed: http://192.0.2.10:8096/api/v1/items/01J/stream?st=STREAMTOKEN&x=1",
            IllegalStateException("header Authorization: Bearer abc.def and X-Slouchcat-Device: dev-9"),
        )
        val out = capture { Log.w("player", "start failed for ?dt=DLTOKEN", error = error) }
        for (secret in listOf("STREAMTOKEN", "abc.def", "dev-9", "DLTOKEN")) {
            assertFalse("$secret leaked: $out", secret in out)
        }
        assertTrue(out, "st=[redacted]&x=1" in out)
        assertTrue(out, "error=java.io.IOException" in out)
        assertTrue(out, "cause=java.lang.IllegalStateException" in out)
    }

    @Test
    fun secretKeysIgnoreCaseAndSeparators() {
        for (key in listOf("token", "Bearer_Token", "X-Slouchcat-Device", "pickupSecret", "newPassword", "fcm_token")) {
            assertTrue(key, Redaction.isSecret(key))
        }
        assertFalse(Redaction.isSecret("version"))
    }
}
