package com.slouchcat.core.api

import com.slouchcat.core.api.generated.models.Health
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.PlaybackRequest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * Concept 27, 11.5: a newer server may add fields and values of an enumeration; the client of an
 * older app must read such answers without failing.
 */
class ClientToleranceTest {
    private val json = SlouchcatApi.json

    @Test
    fun unknownFieldsAreIgnored() {
        val health = json.decodeFromString(
            Health.serializer(),
            """{"status":"ok","version":"9.9.9","uptimeSeconds":3,"brandNew":{"x":[1,2]},"contractVersion":"9.9.0","serverKey":"k"}""",
        )
        assertEquals("9.9.9", health.version)
        assertEquals(Health.Status.OK, health.status)
    }

    @Test
    fun unknownEnumValuesBecomeTheUnknownCase() {
        val health = json.decodeFromString(
            Health.serializer(),
            """{"status":"degraded-in-future","version":"1.0.0","contractVersion":"1.42.0","serverKey":"k","uptimeSeconds":3}""",
        )
        assertEquals(Health.Status.UNKNOWN_DEFAULT_OPEN_API, health.status)

        // An enumeration by $ref inside an object (ItemRef.kind is ItemKind), next to a new field.
        val ref = json.decodeFromString(
            ItemRef.serializer(),
            """{"id":"01J","kind":"hologram","title":"T","inWatchlist":false,"images":{},"newField":1}""",
        )
        assertEquals(ItemKind.UNKNOWN_DEFAULT_OPEN_API, ref.kind)
    }

    @Test
    fun absentValuesStayAbsent() {
        val text = json.encodeToString(PlaybackRequest.serializer(), PlaybackRequest(subtitleTrack = null, positionMs = 5))
        assertFalse(text, "subtitleTrack" in text)
        assertNull(json.decodeFromString(PlaybackRequest.serializer(), """{"subtitleTrack":null}""").subtitleTrack)
    }
}
