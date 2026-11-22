package com.slouchcat.core.offline

import com.slouchcat.core.api.generated.models.DownloadRules
import com.slouchcat.core.api.generated.models.DownloadState
import com.slouchcat.core.api.generated.models.DownloadStateValue
import com.slouchcat.core.api.generated.models.OfflineProfile
import com.slouchcat.core.api.generated.models.OfflineSync
import com.slouchcat.core.api.generated.models.OfflineSyncRequestHeldInner

// What a sync answer (`syncOffline`) changes on the device: pure functions, the same rules as the
// PWA's web/src/features/offline/sync.ts. The engine reads the database, calls these and writes the
// result. No visibility rule of its own (rule 12): the device stores `allowedProfileIds` as the
// server computed them and filters by them.

/** Most ids one sync or revocation check may name (`OfflineSyncRequest.held`, `RevocationCheck.ids`). */
const val MAX_HELD = 2000

/** The server's clock offset (server time minus device time) and the active profile of a sync. */
data class SyncProfile(val profileId: Long, val offsetMs: Long)

private fun retryDue(download: StoredDownload, now: Long): Boolean {
    val error = download.fetchError ?: return true
    return error !in FetchError.SERVER && now - (download.fetchErrorAt ?: 0) >= FETCH_RETRY_MS
}

/** The downloads the device reports for an account, with whether each is complete. */
fun heldDownloads(downloads: List<StoredDownload>, account: String): List<OfflineSyncRequestHeldInner> =
    downloads.filter { it.account == account && !it.pendingDelete }
        .take(MAX_HELD)
        .map { OfflineSyncRequestHeldInner(id = it.id, complete = it.complete) }

/** A download the device has not seen before (created now, by a rule, on another page). */
fun newDownload(state: DownloadState, account: String, now: Long): StoredDownload = StoredDownload(
    id = state.id,
    account = account,
    state = state.state,
    quality = state.quality,
    auto = state.auto,
    allowedProfileIds = state.allowedProfileIds,
    estimatedBytes = state.estimatedBytes,
    sizeBytes = state.sizeBytes,
    segmentsDone = state.segmentsDone,
    segmentsTotal = state.segmentsTotal,
    error = state.error,
    createdAt = state.createdAt.toInstant().toString(),
    readyAt = state.readyAt?.toInstant()?.toString(),
    item = state.item,
    updatedAt = now,
)

/** The server's progress of the active profile after an answer (with the item only), on the device's clock. */
private fun mergeServerPositions(stored: Map<String, ServerPosition>, state: DownloadState, active: SyncProfile?): Map<String, ServerPosition> {
    if (active == null || state.item == null) return stored
    val key = active.profileId.toString()
    val rest = stored - key
    val progress = state.item?.progress ?: return rest
    return rest + (
        key to ServerPosition(
            positionMs = progress.positionMs,
            completed = progress.completed,
            at = progress.updatedAt.toInstant().toEpochMilli() - active.offsetMs,
            playbackRate = progress.playbackRate?.toDouble(),
        )
        )
}

/** The neighbour episodes of the active profile, named only with the title of a copy the device can play. */
private fun mergeNeighbours(stored: Map<String, Neighbours>, state: DownloadState, active: SyncProfile?): Map<String, Neighbours> {
    val named = state.state == DownloadStateValue.READY || state.state == DownloadStateValue.EVICTED
    if (active == null || state.item == null || !named) return stored
    return stored + (active.profileId.toString() to Neighbours(state.nextEpisodeId, state.previousEpisodeId))
}

/**
 * The server's state of a download merged into what the device keeps: state, sizes and
 * `allowedProfileIds` from the server; manifest, bytes, files, positions from the device; the title
 * from the answer, or the kept one when the answer has none (another profile is active).
 */
fun mergeDownload(local: StoredDownload?, state: DownloadState, account: String, now: Long, active: SyncProfile? = null): StoredDownload {
    val fresh = newDownload(state, account, now)
    val serverPositions = mergeServerPositions(local?.serverPositions.orEmpty(), state, active)
    val neighbours = mergeNeighbours(local?.neighbours.orEmpty(), state, active)
    if (local == null) return fresh.copy(serverPositions = serverPositions, neighbours = neighbours)
    val error = local.fetchError
    val preparedAgain = fresh.readyAt != null && fresh.readyAt != local.readyAt
    val cleared = error == null || (state.state == DownloadStateValue.READY && (error in FetchError.SERVER || preparedAgain))
    return local.copy(
        state = fresh.state,
        quality = fresh.quality,
        auto = fresh.auto,
        allowedProfileIds = fresh.allowedProfileIds,
        estimatedBytes = fresh.estimatedBytes,
        sizeBytes = fresh.sizeBytes,
        segmentsDone = fresh.segmentsDone,
        segmentsTotal = fresh.segmentsTotal,
        error = fresh.error,
        createdAt = fresh.createdAt,
        readyAt = fresh.readyAt,
        account = account,
        item = state.item ?: local.item,
        neighbours = neighbours,
        serverPositions = serverPositions,
        fetchError = if (cleared) null else error,
        fetchErrorAt = if (cleared) null else local.fetchErrorAt,
        updatedAt = now,
    )
}

/** `account:profileId` of every profile a download on the device names (not being deleted). */
fun profilesWithDownloads(downloads: List<StoredDownload>): Set<String> =
    downloads.filterNot { it.pendingDelete }.flatMapTo(mutableSetOf()) { d -> d.allowedProfileIds.map { "${d.account}:$it" } }

/** A snapshot without PIN material: a profile keeps it only on a device with downloads it may play (AN-19). */
fun OfflineProfile.withoutPinMaterial(): OfflineProfile = copy(pinSalt = null, pinIterations = null, offlinePin = null)

/** The stored profile of a sync answer; a new PIN salt voids the device's verifier. */
fun mergeProfile(
    local: StoredProfile?,
    snapshot: OfflineProfile,
    account: String,
    now: Long,
    pendingPlayedMs: Long,
    rules: DownloadRules?,
    holdsDownloads: Boolean,
): StoredProfile {
    val kept = if (holdsDownloads) snapshot else snapshot.withoutPinMaterial()
    val verifier = local?.pinVerifier?.takeIf { kept.hasPin && kept.pinSalt != null && it.salt == kept.pinSalt }
    return StoredProfile(
        id = snapshot.id,
        account = account,
        snapshot = kept,
        syncedAt = now,
        pinVerifier = verifier,
        watch = WatchTime.fromSnapshot(snapshot.watchTime, now, pendingPlayedMs),
        lock = local?.lock ?: PinLock(),
        rules = rules ?: local?.rules,
    )
}

/** What an answer changes: downloads to store, revoked ones to delete, profiles to store and drop. */
data class SyncResult(
    val put: List<StoredDownload>,
    val remove: List<String>,
    val putProfiles: List<StoredProfile>,
    val removeProfiles: List<Long>,
    val serverOffsetMs: Long,
    val fetch: List<String>,
)

fun applySync(
    account: String,
    activeProfileId: Long,
    now: Long,
    downloads: List<StoredDownload>,
    profiles: List<StoredProfile>,
    queuedPlayedMs: (profileId: Long, sinceServerMs: Long, offsetMs: Long) -> Long,
    answer: OfflineSync,
    /** Downloads deleted on this device after the answer was written: they do not come back (K5). */
    recentlyDeleted: Set<String> = emptySet(),
): SyncResult {
    val offset = answer.serverTime.toInstant().toEpochMilli() - now
    val revoked = answer.revoked.toSet()
    val local = downloads.associateBy { it.id }
    val active = SyncProfile(activeProfileId, offset)
    val put = answer.downloads
        .filter { it.id !in revoked && it.id !in recentlyDeleted }
        .map { mergeDownload(local[it.id], it, account, now, active) }
    val merged = put.map { it.id }.toSet()
    val withDownloads = profilesWithDownloads(put + downloads.filter { it.id !in merged && it.id !in revoked })
    val putProfiles = answer.profiles.map { snapshot ->
        val pending = snapshot.watchTime?.let { queuedPlayedMs(snapshot.id, WatchTime.snapshotDayStart(it), offset) } ?: 0
        mergeProfile(
            local = profiles.firstOrNull { it.account == account && it.id == snapshot.id },
            snapshot = snapshot,
            account = account,
            now = now,
            pendingPlayedMs = pending,
            rules = if (snapshot.id == activeProfileId) answer.rules else null,
            holdsDownloads = "$account:${snapshot.id}" in withDownloads,
        )
    }
    val listed = answer.profiles.map { it.id }.toSet()
    return SyncResult(
        put = put,
        remove = revoked.filter { it in local },
        putProfiles = putProfiles,
        removeProfiles = profiles.filter { it.account == account && it.id !in listed }.map { it.id },
        serverOffsetMs = offset,
        fetch = fetchOrder(put, now),
    )
}

/** Downloads ready on the server whose files the device lacks, oldest first (paused ones wait). */
fun fetchOrder(downloads: List<StoredDownload>, now: Long): List<String> = downloads
    .filter { it.state == DownloadStateValue.READY && !it.complete && !it.pendingDelete && !it.paused && retryDue(it, now) }
    .sortedBy { it.createdAt }
    .map { it.id }

/** The downloads a profile may see on this device: the server listed it in `allowedProfileIds` (the only filter). */
fun downloadsForProfile(downloads: List<StoredDownload>, account: String, profileId: Long): List<StoredDownload> =
    downloads.filter { it.account == account && !it.pendingDelete && profileId in it.allowedProfileIds }
        .sortedByDescending { it.createdAt }

/** Profiles that may open the offline library: of the device's account, listed by the server, allowed for a download. */
fun offlineProfiles(profiles: List<StoredProfile>, downloads: List<StoredDownload>, account: String?): List<StoredProfile> {
    if (account == null) return emptyList()
    return profiles
        .filter { p -> p.account == account && downloads.any { d -> d.account == account && !d.pendingDelete && p.id in d.allowedProfileIds } }
        .sortedWith(compareByDescending<StoredProfile> { it.snapshot.isPrimary }.thenBy { it.snapshot.name })
}

/** Downloads of other accounts on this device: the public revocation list is asked about them after every sync. */
fun otherAccountDownloads(downloads: List<StoredDownload>, account: String): List<String> =
    downloads.filter { it.account != account }.take(MAX_HELD).map { it.id }

/** What the device knows of its connection. */
enum class NetworkKind { Wifi, Cellular, None, Unknown }

/**
 * An auto-download waits on a metered connection while an enabled rule of its account on this
 * device says «nur WLAN» (AN-13); a download the user started waits when the device setting
 * «Downloads nur im WLAN» is on (stage 5's `downloadsWifiOnly`). The client applies `wifiOnly`.
 */
fun waitsForWifi(download: StoredDownload, profiles: List<StoredProfile>, network: NetworkKind, deviceWifiOnly: Boolean): Boolean {
    if (network != NetworkKind.Cellular) return false
    if (!download.auto) return deviceWifiOnly
    return deviceWifiOnly || profiles.any { it.account == download.account && it.rules?.enabled == true && it.rules.wifiOnly }
}
