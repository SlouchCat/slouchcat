package com.slouchcat.core.offline

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.DownloadList
import com.slouchcat.core.api.generated.models.DownloadRequest
import com.slouchcat.core.api.generated.models.DownloadRules
import com.slouchcat.core.api.generated.models.DownloadStateValue
import com.slouchcat.core.api.generated.models.OfflineEvent
import com.slouchcat.core.api.generated.models.OfflineEventBatch
import com.slouchcat.core.api.generated.models.OfflineSyncRequest
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import java.time.Instant
import java.time.OffsetDateTime
import java.time.ZoneOffset

/** Who uses the device now: the account of the downloads ([key]) and the active profile, `null` before the choice. */
data class OfflineAccount(val key: String, val profileId: Long?)

/** The session as the offline engine needs it (the app adapts its SessionManager). */
fun interface OfflineSession {
    /** `null` without a session (signed out). */
    fun current(): OfflineAccount?
}

/** Whether the server answered the last attempt. */
enum class Contact { Unknown, Online, Offline }

/** The file coming in now. */
data class FetchProgress(val id: String, val bytes: Long, val total: Long)

/** What a check of the offline PIN found. */
sealed interface PinCheck {
    data object Ok : PinCheck

    data object Wrong : PinCheck

    /** Too many wrong PINs: the next attempt in [remainingMs]. */
    data class Locked(val remainingMs: Long) : PinCheck

    /** The profile has a PIN the device cannot check: enter it online once. */
    data object Unavailable : PinCheck
}

/** Starts the fetch where Android lets it run (user-initiated transfer, foreground service, short WorkManager runs). */
interface FetchScheduler {
    /** The user just started something (the app is visible): fetch now, with a notification. */
    fun startTransfer()

    /** Fetch later in the background in short runs ([unmetered]: only on Wi-Fi), and sync about every 6 hours (F-34). */
    fun scheduleBackground(unmetered: Boolean)

    /** Nothing more to do on this device (signed out, no downloads). */
    fun cancelAll()

    object None : FetchScheduler {
        override fun startTransfer() = Unit

        override fun scheduleBackground(unmetered: Boolean) = Unit

        override fun cancelAll() = Unit
    }
}

/**
 * The offline engine (concept 27 9.1 to 9.7; the PWA's engine.ts): it syncs the device with the
 * server (`syncOffline`), applies the answer (revoked downloads go at once, `allowedProfileIds`,
 * profile snapshots, the rule), asks the public revocation list without a session and for other
 * accounts' downloads, sends deletes made offline, uploads the progress of offline playback,
 * fetches ready downloads into the app's folder and resumes them at the durable length. One per
 * process; the pages, the background jobs and the offline player share it.
 */
class OfflineEngine(
    private val store: OfflineStore,
    val files: OfflineFiles,
    private val server: OfflineServer,
    private val fetcher: FileFetcher,
    private val session: OfflineSession,
    private val network: () -> NetworkKind,
    /** The device setting «Downloads nur im WLAN» (stage 5's DeviceSettings). */
    private val wifiOnly: suspend () -> Boolean,
    private val scope: CoroutineScope,
    private val clock: () -> Long = System::currentTimeMillis,
    /** Clock since boot and the boot count for the PIN brake. */
    private val elapsed: () -> Long = { System.nanoTime() / 1_000_000 },
    private val boot: () -> Int = { -1 },
) {
    var scheduler: FetchScheduler = FetchScheduler.None

    val downloads: Flow<List<StoredDownload>> = store.downloads
    val profiles: Flow<List<StoredProfile>> = store.profiles

    private val _contact = MutableStateFlow(Contact.Unknown)
    val contact: StateFlow<Contact> = _contact.asStateFlow()

    private val _progress = MutableStateFlow<FetchProgress?>(null)
    val progress: StateFlow<FetchProgress?> = _progress.asStateFlow()

    /** Device time of the last sync that the server answered. */
    private val _lastSync = MutableStateFlow<Long?>(null)
    val lastSync: StateFlow<Long?> = _lastSync.asStateFlow()

    private val syncLock = Mutex()
    private val fetchLock = Mutex()

    /** Downloads deleted here a moment ago: a sync answer in flight does not bring them back (K5). */
    private val recentDeletes = mutableMapOf<String, Long>()

    /** The fetch in progress (one at a time) and its download. */
    @Volatile private var current: Pair<String, Job>? = null

    /** A sync soon, off the caller (after a playback, a change of the network). */
    fun scheduleSync() {
        scope.launch { sync() }
    }

    fun reportOffline() {
        _contact.value = Contact.Offline
    }

    // ---------- sync ----------

    /** One sync (or the revocation check without a session); the contact follows the answer. */
    suspend fun sync(): Contact = syncLock.withLock {
        if (network() == NetworkKind.None) {
            _contact.value = Contact.Offline
            return@withLock Contact.Offline
        }
        val who = session.current()
        val outcome = try {
            when {
                who == null -> revocationsWithoutSession()
                who.profileId == null -> flushEvents(who.key)
                else -> syncAccount(who.key, who.profileId)
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: RuntimeException) {
            Log.w(TAG, "sync failed", error = e)
            Contact.Unknown
        }
        if (outcome != Contact.Unknown) _contact.value = outcome
        withContext(Dispatchers.IO) { runCatching { files.sweep(store.allDownloads().map { it.id }.toSet()) } }
        outcome
    }

    private suspend fun syncAccount(account: String, profileId: Long): Contact {
        rememberAccount(account)
        sendPendingDeletes(account)
        val held = heldDownloads(store.allDownloads(), account)
        val answer = when (val result = server.sync(OfflineSyncRequest(held = held, network = network().forRequest()))) {
            is ApiResult.Ok -> result.value
            ApiResult.Offline -> return Contact.Offline
            // The session ended (a revoked device or account): the app forgets it, and the next run asks by the device id.
            ApiResult.SignedOut -> return Contact.Online
            else -> return Contact.Online
        }
        val now = clock()
        val queued = store.allEvents()
        val deleted = synchronized(recentDeletes) {
            recentDeletes.entries.removeIf { now - it.value > RECENT_DELETE_MS }
            recentDeletes.keys.toSet()
        }
        val result = applySync(
            account = account,
            activeProfileId = profileId,
            now = now,
            downloads = store.allDownloads(),
            profiles = store.allProfiles(),
            queuedPlayedMs = { id, since, offset ->
                queued.filter { it.event.profileId == id && it.event.clientTime.toInstant().toEpochMilli() + offset >= since }.sumOf { it.event.playedMs }
            },
            answer = answer,
            recentlyDeleted = deleted,
        )
        removeDownloads(result.remove)
        // Per record, so a fetch writing its progress meanwhile keeps it.
        val active = SyncProfile(profileId, result.serverOffsetMs)
        val revoked = answer.revoked.toSet()
        for (state in answer.downloads) {
            if (state.id in revoked || state.id in deleted) continue
            store.updateDownload(state.id) { local -> mergeDownload(local, state, account, now, active) }
        }
        for (profile in result.putProfiles) {
            // The brake and a verifier derived meanwhile stay (a PIN check between read and write).
            store.updateProfile(account, profile.id) { local ->
                profile.copy(
                    pinVerifier = profile.pinVerifier ?: local?.pinVerifier?.takeIf { profile.snapshot.hasPin && profile.snapshot.pinSalt == it.salt },
                    lock = local?.lock ?: profile.lock,
                )
            }
        }
        store.deleteProfiles(account, result.removeProfiles)
        store.setMeta(META_OFFSET, result.serverOffsetMs.toString())
        _lastSync.value = now
        // A guest who used the device before: revoked even while another account is signed in (AN-4, K-7).
        checkRevocations(otherAccountDownloads(store.allDownloads(), account), keep = account)
        flushEvents(account)
        afterChange()
        return Contact.Online
    }

    /** The account signed in now is the device's account; downloads of another one wait for the revocation list. */
    private suspend fun rememberAccount(account: String) {
        if (store.meta(META_ACCOUNT) != account) store.setMeta(META_ACCOUNT, account)
    }

    /** Without a session the device names its downloads and learns by its device id which are revoked (AN-4). */
    private suspend fun revocationsWithoutSession(): Contact {
        val ids = store.allDownloads().map { it.id }
        if (ids.isEmpty()) return Contact.Unknown
        return checkRevocations(ids, keep = null)
    }

    private suspend fun checkRevocations(ids: List<String>, keep: String?): Contact {
        if (ids.isEmpty()) return Contact.Online
        val revoked = when (val result = server.revocations(ids)) {
            is ApiResult.Ok -> result.value.revoked
            ApiResult.Offline -> return Contact.Offline
            else -> return Contact.Online
        }
        if (revoked.isEmpty()) return Contact.Online
        removeDownloads(revoked)
        Log.i(TAG, "revoked", "count" to revoked.size)
        val left = store.allDownloads().map { it.account }.toMutableSet()
        keep?.let(left::add)
        store.meta(META_ACCOUNT)?.takeIf { keep != null }?.let(left::add)
        // Their profiles and events can never be used or uploaded again (K7).
        store.keepOnly(left)
        afterChange()
        return Contact.Online
    }

    /** Deletes made offline reach the server now (the files are gone already). */
    private suspend fun sendPendingDeletes(account: String) {
        for (download in store.allDownloads().filter { it.pendingDelete && it.account == account }) {
            when (server.delete(download.id)) {
                is ApiResult.Ok, ApiResult.NotFound -> store.deleteDownloads(listOf(download.id))
                else -> Unit
            }
        }
    }

    /** Removes downloads with their files (revoked, deleted). */
    private suspend fun removeDownloads(ids: Collection<String>) {
        if (ids.isEmpty()) return
        for (id in ids) {
            cancelFetch(id)
            withContext(Dispatchers.IO) { runCatching { files.remove(id) } }
        }
        store.deleteDownloads(ids)
    }

    /** PIN material stays only for profiles a download on the device names (AN-19). */
    private suspend fun afterChange() {
        val all = store.allDownloads()
        val named = profilesWithDownloads(all)
        for (profile in store.allProfiles()) {
            if ("${profile.account}:${profile.id}" in named) continue
            if (profile.pinVerifier == null && profile.snapshot.pinSalt == null && profile.snapshot.offlinePin == null) continue
            store.updateProfile(profile.account, profile.id) { it?.copy(snapshot = it.snapshot.withoutPinMaterial(), pinVerifier = null) }
        }
        if (all.isEmpty() && session.current() == null) scheduler.cancelAll()
    }

    // ---------- events ----------

    /** Uploads the queued progress events of [account] in batches of 500. */
    private suspend fun flushEvents(account: String): Contact {
        while (true) {
            val batch = store.events(account, EVENT_BATCH)
            if (batch.isEmpty()) return Contact.Online
            when (val result = server.upload(OfflineEventBatch(sentAt = now(), events = batch.map { it.event }))) {
                // Accepted, duplicate or rejected: the server decided; the queue lets go of them.
                is ApiResult.Ok -> store.deleteEvents(batch.map { it.key })
                ApiResult.Offline -> return Contact.Offline
                // 422 for a batch: the events can never be accepted.
                is ApiResult.Failed -> if (result.status == HTTP_UNPROCESSABLE) store.deleteEvents(batch.map { it.key }) else return Contact.Online
                else -> return Contact.Online
            }
        }
    }

    suspend fun queueEvent(account: String, event: OfflineEvent) = store.queue(account, event)

    // ---------- orders, deletes, pause ----------

    /** `getDownloadOptions`: qualities with sizes, tracks, subtitles, an existing download. */
    suspend fun options(itemId: String, mediaFileId: String? = null) = server.options(itemId, mediaFileId)

    /** `createDownloads` for [itemId] (a film, an episode, a season); the new downloads are stored at once. */
    suspend fun create(itemId: String, request: DownloadRequest): ApiResult<DownloadList> {
        val who = session.current() ?: return ApiResult.SignedOut
        val result = server.create(itemId, request)
        if (result is ApiResult.Ok) {
            val now = clock()
            for (state in result.value.downloads) {
                store.updateDownload(state.id) { local -> mergeDownload(local, state, who.key, now).copy(requested = true, paused = false, pendingDelete = false) }
                synchronized(recentDeletes) { recentDeletes.remove(state.id) }
            }
            scope.launch { sync() }
        }
        return result
    }

    /**
     * Deletes [ids] on the device at once and in the server (`deleteDownload`); without an answer the
     * record stays as `pendingDelete` and the delete goes out at the next contact.
     */
    suspend fun delete(ids: Collection<String>) {
        val now = clock()
        synchronized(recentDeletes) { ids.forEach { recentDeletes[it] = now } }
        for (id in ids) {
            cancelFetch(id)
            withContext(Dispatchers.IO) { runCatching { files.remove(id) } }
            when (server.delete(id)) {
                is ApiResult.Ok, ApiResult.NotFound -> store.deleteDownloads(listOf(id))
                else -> store.updateDownload(id) { it?.copy(pendingDelete = true, complete = false, bytesWritten = 0, manifest = null, subtitlesStored = emptyList(), trickplayStored = emptyList(), posterStored = false) }
            }
        }
        afterChange()
    }

    /** Stops fetching [id] on this device; the server keeps preparing it (it knows no pause). */
    suspend fun pause(id: String) {
        store.updateDownload(id) { it?.copy(paused = true) }
        cancelFetch(id)
    }

    suspend fun resume(id: String) {
        store.updateDownload(id) { it?.copy(paused = false, fetchError = null, fetchErrorAt = null) }
        scheduler.startTransfer()
    }

    // ---------- fetch ----------

    /** Whether something waits for its files on this device (and may be fetched on [kind]). */
    suspend fun pendingFetches(kind: NetworkKind = network()): List<String> {
        val all = store.allDownloads()
        val profiles = store.allProfiles()
        val only = wifiOnly()
        val who = session.current()
        return fetchOrder(all, clock())
            .mapNotNull { id -> all.firstOrNull { it.id == id } }
            .filter { it.account == who?.key && !waitsForWifi(it, profiles, kind, only) }
            // The manifest needs a profile that may see the title; another profile leaves it waiting.
            .filter { it.manifest != null || (who?.profileId != null && who.profileId in it.allowedProfileIds) }
            .map { it.id }
    }

    /** Whether a pending download waits only for Wi-Fi. */
    suspend fun waitsForWifiOnly(): Boolean = pendingFetches(NetworkKind.Wifi).isNotEmpty() && pendingFetches().isEmpty()

    /**
     * Fetches pending downloads one after the other until none is left, [deadline] (device time)
     * passed, or the coroutine is cancelled. Returns whether work is left.
     */
    suspend fun runFetches(deadline: Long? = null): Boolean = fetchLock.withLock {
        var left = pendingFetches()
        while (left.isNotEmpty() && (deadline == null || clock() < deadline) && network() != NetworkKind.None) {
            val next = left.first()
            val job = scope.launch(Dispatchers.IO) {
                try {
                    fetchOne(next)
                } catch (e: CancellationException) {
                    throw e
                } catch (e: RuntimeException) {
                    Log.w(TAG, "fetch broke", error = e)
                    fail(next, FetchError.STORAGE)
                }
            }
            current = next to job
            try {
                job.join()
            } finally {
                current = null
                _progress.value = null
            }
            left = pendingFetches()
            // The same download again at once: it made no progress (it waits for something); leave it to the next run.
            if (left.firstOrNull() == next && store.download(next)?.fetchError == null && !progressed(next)) break
        }
        left.isNotEmpty()
    }

    private val lastBytes = mutableMapOf<String, Long>()

    private suspend fun progressed(id: String): Boolean {
        val bytes = store.download(id)?.bytesWritten ?: return false
        val before = lastBytes.put(id, bytes)
        return before == null || bytes > before
    }

    private suspend fun cancelFetch(id: String) {
        val running = current?.takeIf { it.first == id }?.second ?: return
        running.cancel()
        running.join()
    }

    /** Stops the fetch in progress (network gone, the job stopped by the system). */
    suspend fun stopFetching() {
        current?.second?.let {
            it.cancel()
            it.join()
        }
    }

    private suspend fun fail(id: String, code: String) {
        Log.i(TAG, "fetch failed", "code" to code)
        store.updateDownload(id) { it?.copy(fetchError = code, fetchErrorAt = clock()) }
    }

    /** Manifest, poster, file, subtitles and preview sheets of one download; complete when all are there. */
    private suspend fun fetchOne(id: String) {
        var download = store.download(id) ?: return
        val who = session.current() ?: return
        // A copy prepared again (another readyAt): the old bytes belong to another file.
        if (download.manifest != null && download.fileReadyAt != null && download.fileReadyAt != download.readyAt) {
            withContext(Dispatchers.IO) { files.remove(id) }
            download = store.updateDownload(id) { it?.copy(manifest = null, bytesWritten = 0, fileEtag = null, subtitlesStored = emptyList(), trickplayStored = emptyList(), posterStored = false, complete = false) } ?: return
        }
        if (download.manifest == null) {
            // The manifest needs a profile that may see the title (scoped); the file itself only the token.
            if (who.profileId == null || who.profileId !in download.allowedProfileIds) return
            val manifest = when (val result = server.manifest(id)) {
                is ApiResult.Ok -> result.value
                ApiResult.NotFound -> return fail(id, FetchError.GONE)
                is ApiResult.Failed -> return fail(id, if (result.status == HTTP_GONE) FetchError.EVICTED else FetchError.NETWORK)
                ApiResult.Offline -> return fail(id, FetchError.NETWORK)
                else -> return fail(id, FetchError.UNAUTHORIZED)
            }
            download = store.updateDownload(id) { it?.copy(manifest = manifest, fileReadyAt = it.readyAt, bytesWritten = 0, fileEtag = null) } ?: return
        }
        val manifest = download.manifest ?: return
        withContext(Dispatchers.IO) { files.folder(id).mkdirs() }
        // The poster first: it needs the session, the rest only the download token (9.3). Best effort:
        // without it the offline library draws the title's initial.
        if (!download.posterStored) {
            val poster = download.item?.images?.let { it.posterSmall ?: it.poster ?: it.backdropSmall ?: it.backdrop }?.url?.let(server::absolute)
            val stored = poster == null || fetcher.fetchSmall(poster, files.poster(id))
            if (stored) download = store.updateDownload(id) { it?.copy(posterStored = true) } ?: return
        }
        val url = server.absolute(manifest.fileUrl) ?: return fail(id, FetchError.GONE)
        // After a crash the file may be longer than what was durably written: cut it back (stage 0 point 6).
        withContext(Dispatchers.IO) { files.truncateMedia(id, download.bytesWritten) }
        val from = withContext(Dispatchers.IO) { files.media(id).length() }.coerceAtMost(download.bytesWritten)
        _progress.value = FetchProgress(id, from, manifest.sizeBytes)
        val outcome = fetcher.fetch(
            url = url,
            target = files.media(id),
            from = from,
            total = manifest.sizeBytes,
            etag = download.fileEtag,
            onDurable = { length, etag -> store.updateDownload(id) { it?.copy(bytesWritten = length, fileEtag = etag) } },
            onProgress = { bytes -> _progress.value = FetchProgress(id, bytes, manifest.sizeBytes) },
        )
        when (outcome) {
            is FetchOutcome.Failed -> return fail(id, outcome.code)
            is FetchOutcome.Done -> Unit
        }
        for (subtitle in manifest.subtitles) {
            if (subtitle.track.id in download.subtitlesStored) continue
            val address = server.absolute(subtitle.url) ?: continue
            var code: String? = null
            if (!fetcher.fetchSmall(address, files.subtitle(id, subtitle.track.id)) { code = it }) return fail(id, code ?: FetchError.NETWORK)
            download = store.updateDownload(id) { it?.copy(subtitlesStored = it.subtitlesStored + subtitle.track.id) } ?: return
        }
        for (sheet in manifest.trickplay?.sheets.orEmpty()) {
            if (sheet.index in download.trickplayStored) continue
            val address = server.absolute(sheet.url) ?: continue
            var code: String? = null
            if (!fetcher.fetchSmall(address, files.trickplay(id, sheet.index)) { code = it }) return fail(id, code ?: FetchError.NETWORK)
            download = store.updateDownload(id) { it?.copy(trickplayStored = it.trickplayStored + sheet.index) } ?: return
        }
        if (!Playlist.coversFile(manifest)) return fail(id, FetchError.SIZE)
        withContext(Dispatchers.IO) { files.writePlaylist(id, Playlist.build(manifest)) }
        store.updateDownload(id) { it?.copy(complete = true, fetchError = null, fetchErrorAt = null) }
        Log.i(TAG, "download complete")
        // The next sync reports it as complete; the server clears its copy 24 h later (9.2 point 4).
        scope.launch { sync() }
    }

    // ---------- profiles and the offline PIN ----------

    /**
     * After a correct PIN online (the profile choice): the device derives its own verifier of the
     * normal PIN with the salt of the last sync, only when it holds a download of the profile (9.5).
     */
    suspend fun pinEntered(profileId: Long, pin: CharArray) {
        val account = session.current()?.key ?: store.meta(META_ACCOUNT) ?: return
        val profile = store.profile(account, profileId) ?: return
        val salt = profile.snapshot.pinSalt ?: return
        if (profile.pinVerifier?.salt == salt) return
        if ("$account:$profileId" !in profilesWithDownloads(store.allDownloads())) return
        val verifier = withContext(Dispatchers.Default) { OfflinePin.deriveLocal(pin, salt, profile.snapshot.pinIterations ?: OFFLINE_PIN_ITERATIONS) } ?: return
        store.updateProfile(account, profileId) { it?.copy(pinVerifier = verifier) }
    }

    /** The offline PIN of [profileId] with the brake against trying (9.5); never on the main thread. */
    suspend fun checkPin(profileId: Long, pin: CharArray): PinCheck {
        val account = session.current()?.key ?: return PinCheck.Unavailable
        val profile = store.profile(account, profileId) ?: return PinCheck.Unavailable
        val remaining = OfflinePin.remainingMs(profile.lock, clock(), elapsed(), boot())
        if (remaining > 0) return PinCheck.Locked(remaining)
        if (OfflinePin.requirement(profile) == OfflinePin.Requirement.Unavailable) return PinCheck.Unavailable
        val ok = withContext(Dispatchers.Default) { OfflinePin.verify(profile, pin) }
        val updated = store.updateProfile(account, profileId) { stored ->
            stored?.copy(lock = if (ok) PinLock() else OfflinePin.registerFailure(stored.lock, clock(), elapsed(), boot()))
        }
        if (ok) return PinCheck.Ok
        val wait = updated?.let { OfflinePin.remainingMs(it.lock, clock(), elapsed(), boot()) } ?: 0
        return if (wait > 0) PinCheck.Locked(wait) else PinCheck.Wrong
    }

    // ---------- rules ----------

    suspend fun rules(): ApiResult<DownloadRules> = server.rules()

    suspend fun setRules(rules: DownloadRules): ApiResult<DownloadRules> {
        val result = server.setRules(rules)
        val who = session.current()
        if (result is ApiResult.Ok && who?.profileId != null) {
            store.updateProfile(who.key, who.profileId) { it?.copy(rules = result.value) }
            scope.launch { sync() }
        }
        return result
    }

    // ---------- sign-out ----------

    /** Downloads of the signed-in account on this device (the question of the sign-out sheet). */
    suspend fun accountDownloads(): List<StoredDownload> {
        val account = session.current()?.key ?: return emptyList()
        return store.allDownloads().filter { it.account == account && !it.pendingDelete }
    }

    /**
     * Before the sign-out (9.7, F-25): [deleteAll] deletes the account's downloads in the server
     * (while the token still works) and on the device; otherwise they stay locked until the same
     * account signs in again (the PWA's rule). Either way the background work stops.
     */
    suspend fun beforeSignOut(deleteAll: Boolean) {
        stopFetching()
        if (deleteAll) {
            val ids = accountDownloads().map { it.id }
            delete(ids)
            // What the server did not confirm goes on the device anyway; the server learns it from «Meine Geräte».
            removeDownloads(store.allDownloads().filter { it.id in ids }.map { it.id })
        }
        scheduler.cancelAll()
    }

    // ---------- helpers ----------

    /** The server's clock offset of the last sync (server minus device), 0 before the first. */
    suspend fun serverOffsetMs(): Long = store.meta(META_OFFSET)?.toLongOrNull() ?: 0

    fun now(): OffsetDateTime = OffsetDateTime.ofInstant(Instant.ofEpochMilli(clock()), ZoneOffset.UTC)

    suspend fun download(id: String): StoredDownload? = store.download(id)

    suspend fun allDownloads(): List<StoredDownload> = store.allDownloads()

    suspend fun profile(account: String, id: Long): StoredProfile? = store.profile(account, id)

    suspend fun updateDownload(id: String, change: (StoredDownload?) -> StoredDownload?) = store.updateDownload(id, change)

    suspend fun updateProfile(account: String, id: Long, change: (StoredProfile?) -> StoredProfile?) = store.updateProfile(account, id, change)

    fun account(): OfflineAccount? = session.current()

    private fun NetworkKind.forRequest() = when (this) {
        NetworkKind.Wifi -> OfflineSyncRequest.Network.WIFI
        NetworkKind.Cellular -> OfflineSyncRequest.Network.CELLULAR
        else -> OfflineSyncRequest.Network.UNKNOWN
    }

    companion object {
        private const val TAG = "offline"
        private const val EVENT_BATCH = 500
        private const val HTTP_GONE = 410
        private const val HTTP_UNPROCESSABLE = 422
        const val RECENT_DELETE_MS = 10 * 60_000L
        const val META_ACCOUNT = "account"
        const val META_OFFSET = "serverOffsetMs"

        /** States of the server in which a download waits for the server, not for the device. */
        val WAITING = setOf(DownloadStateValue.QUEUED, DownloadStateValue.PREPARING, DownloadStateValue.PAUSED, DownloadStateValue.EVICTED)
    }
}
