package com.slouchcat.core.offline

import com.slouchcat.core.api.generated.models.DownloadStateValue
import com.slouchcat.core.api.generated.models.OfflineWatchTime
import com.slouchcat.core.api.generated.models.Progress
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.math.BigDecimal

/** The rules of sync.test.ts of the PWA, for the app (concept 27 12, «Unit»). */
class SyncTest {
    private val now = 5_000L

    @Test
    fun `revoked downloads go, the rest is merged and the ready ones are fetched oldest first`() {
        val local = listOf(newDownload(state(D1), ACCOUNT, 0), newDownload(state(D2, created = 500), ACCOUNT, 0))
        val result = applySync(
            ACCOUNT, 1, now, local, emptyList(), { _, _, _ -> 0 },
            answer(downloads = listOf(state(D2, created = 500), state(D3, created = 100)), revoked = listOf(D1)),
        )
        assertEquals(listOf(D1), result.remove)
        assertEquals(setOf(D2, D3), result.put.map { it.id }.toSet())
        assertEquals(listOf(D3, D2), result.fetch)
        assertEquals(10_000 - now, result.serverOffsetMs)
    }

    @Test
    fun `the device keeps its bytes, manifest and title when the answer has none`() {
        val local = newDownload(state(D1), ACCOUNT, 0).copy(bytesWritten = 500, manifest = manifest(), complete = true)
        val merged = mergeDownload(local, state(D1, item = null, allowed = listOf(1, 2)), ACCOUNT, now)
        assertEquals(500, merged.bytesWritten)
        assertEquals("Morgenrot", merged.item?.title)
        assertTrue(merged.complete)
        assertEquals(listOf(1L, 2L), merged.allowedProfileIds)
    }

    @Test
    fun `a ready answer clears a server-side fetch error but a device error waits`() {
        val evicted = newDownload(state(D1), ACCOUNT, 0).copy(fetchError = FetchError.EVICTED, fetchErrorAt = now)
        assertNull(mergeDownload(evicted, state(D1), ACCOUNT, now).fetchError)
        val full = evicted.copy(fetchError = FetchError.QUOTA)
        assertEquals(FetchError.QUOTA, mergeDownload(full, state(D1), ACCOUNT, now).fetchError)
        assertTrue(fetchOrder(listOf(full.copy(fetchErrorAt = now)), now).isEmpty())
        assertEquals(listOf(D1), fetchOrder(listOf(full.copy(fetchErrorAt = now)), now + FETCH_RETRY_MS))
    }

    @Test
    fun `paused, complete and deleted downloads are not fetched`() {
        val base = newDownload(state(D1), ACCOUNT, 0)
        assertTrue(fetchOrder(listOf(base.copy(paused = true)), now).isEmpty())
        assertTrue(fetchOrder(listOf(base.copy(complete = true)), now).isEmpty())
        assertTrue(fetchOrder(listOf(base.copy(pendingDelete = true)), now).isEmpty())
        assertTrue(fetchOrder(listOf(newDownload(state(D1, value = DownloadStateValue.PREPARING), ACCOUNT, 0)), now).isEmpty())
    }

    @Test
    fun `a profile sees only downloads the server allowed for it (rule 12)`() {
        val list = listOf(newDownload(state(D1, allowed = listOf(1)), ACCOUNT, 0), newDownload(state(D2, allowed = listOf(1, 2)), ACCOUNT, 0))
        assertEquals(listOf(D2), downloadsForProfile(list, ACCOUNT, 2).map { it.id })
        assertTrue(downloadsForProfile(list, "other", 1).isEmpty())
        val profiles = listOf(StoredProfile(2, ACCOUNT, profile(2, "Kim"), 0), StoredProfile(3, ACCOUNT, profile(3, "Noa"), 0), StoredProfile(1, ACCOUNT, profile(1), 0))
        assertEquals(listOf(1L, 2L), offlineProfiles(profiles, list, ACCOUNT).map { it.id })
        assertTrue(offlineProfiles(profiles, list, null).isEmpty())
    }

    @Test
    fun `pin material stays only with downloads of the profile, a new salt voids the verifier`() {
        val withPin = profile(1, hasPin = true, salt = "00112233445566778899aabbccddeeff")
        val verifier = LocalPinVerifier("00112233445566778899aabbccddeeff", 1, "x")
        val local = StoredProfile(1, ACCOUNT, withPin, 0, pinVerifier = verifier, lock = PinLock(failures = 2))
        val kept = mergeProfile(local, withPin, ACCOUNT, now, 0, null, holdsDownloads = true)
        assertEquals(verifier, kept.pinVerifier)
        assertEquals(2, kept.lock.failures)
        val stripped = mergeProfile(local, withPin, ACCOUNT, now, 0, null, holdsDownloads = false)
        assertNull(stripped.snapshot.pinSalt)
        assertNull(stripped.pinVerifier)
        val changed = mergeProfile(local, withPin.copy(pinSalt = "ffeeddccbbaa99887766554433221100"), ACCOUNT, now, 0, null, holdsDownloads = true)
        assertNull(changed.pinVerifier)
    }

    @Test
    fun `the server progress of the active profile is kept on the device clock`() {
        val progress = Progress(positionMs = 42_000, durationMs = 90_000, percent = BigDecimal.ONE, completed = false, updatedAt = at(9_000))
        val merged = mergeDownload(null, state(D1, item = item().copy(progress = progress)), ACCOUNT, now, SyncProfile(1, offsetMs = 1_000))
        assertEquals(ServerPosition(42_000, false, 8_000), merged.serverPositions["1"])
    }

    @Test
    fun `auto-downloads wait on cellular when a rule says Wi-Fi only, user downloads when the device does`() {
        val auto = newDownload(state(D1, auto = true), ACCOUNT, 0)
        val user = newDownload(state(D2), ACCOUNT, 0)
        val rule = StoredProfile(1, ACCOUNT, profile(), 0, rules = RULES.copy(enabled = true, wifiOnly = true))
        assertTrue(waitsForWifi(auto, listOf(rule), NetworkKind.Cellular, deviceWifiOnly = false))
        assertFalse(waitsForWifi(auto, listOf(rule), NetworkKind.Wifi, deviceWifiOnly = false))
        assertFalse(waitsForWifi(user, listOf(rule), NetworkKind.Cellular, deviceWifiOnly = false))
        assertTrue(waitsForWifi(user, listOf(rule), NetworkKind.Cellular, deviceWifiOnly = true))
    }

    @Test
    fun `watch time counts down by the device clock and starts again after the reset`() {
        val watch = WatchTime.fromSnapshot(OfflineWatchTime(limitMinutes = 60, remainingMinutes = 10, resetsAt = at(1_000_000)), 0, pendingMs = 60_000)!!
        assertEquals(9 * 60_000L, WatchTime.remainingMs(watch, 0))
        val played = WatchTime.addPlayed(watch, 9 * 60_000L, 10)
        assertEquals(0L, WatchTime.remainingMs(played, 20))
        assertEquals(60 * 60_000L, WatchTime.remainingMs(played, 1_000_001))
        val next = WatchTime.addPlayed(played, 30 * 60_000L, 1_000_001)
        assertEquals(30 * 60_000L, WatchTime.remainingMs(next, 1_000_002))
    }

    @Test
    fun `the playlist names every segment as a byte range of the one file`() {
        val text = Playlist.build(manifest())
        assertTrue(text.contains("#EXT-X-MAP:URI=\"media.mp4\",BYTERANGE=\"10@0\""))
        assertTrue(text.contains("#EXT-X-BYTERANGE:100@910"))
        assertTrue(text.contains("#EXTINF:2.000,"))
        assertTrue(text.trimEnd().endsWith("#EXT-X-ENDLIST"))
        assertTrue(Playlist.coversFile(manifest()))
        assertFalse(Playlist.coversFile(manifest().copy(sizeBytes = 2000)))
    }

    @Test
    fun `ulids fit the contract's pattern`() {
        repeat(50) { assertTrue(Regex("^[0-9A-HJKMNP-TV-Z]{26}$").matches(Ulid.next())) }
    }
}
