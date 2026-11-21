package com.slouchcat.core.offline

import com.slouchcat.core.api.generated.models.DownloadManifest
import com.slouchcat.core.api.generated.models.DownloadQuality
import com.slouchcat.core.api.generated.models.DownloadRules
import com.slouchcat.core.api.generated.models.DownloadStateValue
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.OfflineEvent
import com.slouchcat.core.api.generated.models.OfflineProfile
import kotlinx.serialization.Serializable

// What the device keeps of the offline contract (concept 27 9.1, the same as the PWA's
// web/src/features/offline/types.ts). The server decides; the device stores its answers and looks
// them up (plan principle 1, rule 12).

/** Iterations of the PBKDF2 verifiers when the server names none (`model.OfflinePinIterations`). */
const val OFFLINE_PIN_ITERATIONS = 600_000

/** A position of one profile in a download: where offline playback stopped (`at` on the device's clock). */
@Serializable
data class LocalPosition(val positionMs: Long, val at: Long)

/** The server's progress of one profile from the last sync made with that profile active (`at` on the device's clock). */
@Serializable
data class ServerPosition(val positionMs: Long, val completed: Boolean, val at: Long, val playbackRate: Double? = null)

/** The neighbour episodes the server named for one profile (concept 23 3.3); the player only looks them up. */
@Serializable
data class Neighbours(val next: String? = null, val previous: String? = null)

/**
 * A download as the device keeps it: the server's last state (the server owns state, sizes and
 * [allowedProfileIds]), the title data, the manifest, how far the files are and the positions per
 * profile. Files: `<offline folder>/<id>/` ([OfflineFiles]).
 */
@Serializable
data class StoredDownload(
    val id: String,
    /** The account the download belongs to ([OfflineAccount.key]). */
    val account: String,
    val state: DownloadStateValue,
    val quality: DownloadQuality,
    val auto: Boolean,
    /** Profiles that may see the title, computed by the server (plan 3.3). The only filter offline. */
    val allowedProfileIds: List<Long>,
    val estimatedBytes: Long,
    val sizeBytes: Long? = null,
    val segmentsDone: Int = 0,
    val segmentsTotal: Int? = null,
    val error: String? = null,
    val createdAt: String,
    val readyAt: String? = null,
    /** Title data from the last answer that carried it (another profile's sync has none). */
    val item: ItemRef? = null,
    val neighbours: Map<String, Neighbours> = emptyMap(),
    val manifest: DownloadManifest? = null,
    /** Bytes of `media.mp4` that are durably written (synced), from offset 0. */
    val bytesWritten: Long = 0,
    val subtitlesStored: List<Long> = emptyList(),
    val trickplayStored: List<Int> = emptyList(),
    val posterStored: Boolean = false,
    /** File, subtitles, preview sheets and poster are on the device (`complete` of the next sync). */
    val complete: Boolean = false,
    /** Code of the last local fetch failure ([FetchError]); device-side ones wait [FETCH_RETRY_MS]. */
    val fetchError: String? = null,
    val fetchErrorAt: Long? = null,
    /** `readyAt` of the server copy the bytes come from; another one means start over. */
    val fileReadyAt: String? = null,
    /** Strong ETag of that copy; a resume asks with If-Range. */
    val fileEtag: String? = null,
    /** Deleted here while offline; the delete goes to the server at the next contact. */
    val pendingDelete: Boolean = false,
    /** Paused by the user on this device: the fetch leaves it alone (the server knows nothing of it). */
    val paused: Boolean = false,
    /** Started by a tap on «Herunterladen» (a user-initiated transfer), not by a rule. */
    val requested: Boolean = false,
    val positions: Map<String, LocalPosition> = emptyMap(),
    val serverPositions: Map<String, ServerPosition> = emptyMap(),
    val updatedAt: Long = 0,
)

/** Remaining watch time of a profile for the offline player (AN-18): all times are server time except [syncedAt]. */
@Serializable
data class StoredWatchTime(
    val limitMinutes: Int,
    val remainingMinutes: Int,
    /** Server time (ms) of the next reset; the full limit applies from then on, every 24 h. */
    val resetsAt: Long,
    val syncedAt: Long,
    /** -1: before [resetsAt], else the number of the 24-h period after it. */
    val period: Int,
    /** Played milliseconds in [period] that the snapshot does not include yet. */
    val playedMs: Long,
)

/** PBKDF2-SHA256 verifier the device derived from the normal PIN (salt from the server, hex). */
@Serializable
data class LocalPinVerifier(val salt: String, val iterations: Int, val hash: String) {
    override fun toString() = "LocalPinVerifier(iterations=$iterations)"
}

/**
 * The brake of the offline PIN (30 s after the third failure, doubling up to 15 min). [until] is
 * device time; [untilElapsed] the same moment on the clock since boot of boot [boot], so moving
 * the wall clock forward does not end a pause early.
 */
@Serializable
data class PinLock(val failures: Int = 0, val until: Long = 0, val untilElapsed: Long = 0, val boot: Int = -1)

/** A profile as the offline library needs it, from the last sync that listed it. */
@Serializable
data class StoredProfile(
    val id: Long,
    val account: String,
    val snapshot: OfflineProfile,
    val syncedAt: Long,
    val pinVerifier: LocalPinVerifier? = null,
    val watch: StoredWatchTime? = null,
    val lock: PinLock = PinLock(),
    /** The profile's auto-download rule on this device, when it was active at a sync. */
    val rules: DownloadRules? = null,
) {
    /** Without PIN material: a profile's verifiers never reach a log (rule 10). */
    override fun toString() = "StoredProfile(id=$id, hasPin=${snapshot.hasPin}, verifier=${pinVerifier != null}, failures=${lock.failures})"
}

/** A progress event of offline playback waiting for the upload. */
data class QueuedEvent(val key: Long, val account: String, val event: OfflineEvent)

/** Codes of a failed fetch (the PWA's FetchErrorCode plus the device's own). */
object FetchError {
    const val EVICTED = "evicted"
    const val GONE = "gone"
    const val UNAUTHORIZED = "unauthorized"
    const val SIZE = "size"
    const val CHANGED = "changed"
    const val NETWORK = "network"
    const val RANGE = "range"
    const val QUOTA = "quota"
    const val STORAGE = "storage"

    /** Resolved on the server side (a new preparation brings a new manifest): the next `ready` clears them. */
    val SERVER = setOf(EVICTED, GONE, UNAUTHORIZED, SIZE, CHANGED)
}

/** Device-side fetch errors are tried again after this long (a full disk does not pull a block per minute). */
const val FETCH_RETRY_MS = 5 * 60_000L
