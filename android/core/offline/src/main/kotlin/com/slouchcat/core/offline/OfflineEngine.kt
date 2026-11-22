package com.slouchcat.core.offline

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.models.DownloadList
import com.slouchcat.core.api.generated.models.DownloadRequest
import com.slouchcat.core.api.generated.models.DownloadRules
import com.slouchcat.core.api.generated.models.DownloadStateValue
import com.slouchcat.core.api.generated.models.OfflineEvent
import com.slouchcat.core.api.generated.models.OfflineEventBatch
import com.slouchcat.core.api.generated.models.OfflineSyncRequest
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineExceptionHandler
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.cancelAndJoin
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.builtins.MapSerializer
import kotlinx.serialization.builtins.serializer
import java.io.IOException
import java.time.Instant
import java.time.OffsetDateTime
import java.time.ZoneOffset

/**
 * Who uses the app now, as the session knows it: the server ([server] = its key, or its address when
 * it has none), the e-mail address of the account and the active profile (`null` before the choice).
 */
data class OfflineIdentity(val server: String, val email: String, val profileId: Long?)

/**
 * The owner of the downloads and the active profile. [key] = the server's key and the account's id
 * (security review L-3): another server under the same address, or another account with the same
 * e-mail address, is another owner; a changed e-mail address is the same one.
 */
data class OfflineAccount(val key: String, val profileId: Long?)

/** The session as the offline engine needs it (the app adapts its SessionManager). */
fun interface OfflineSession {
    /** Waits until the app has read its stored session (a cold process woken by the system, code review B-2); `null` without a session. */
    suspend fun current(): OfflineIdentity?
}

/** Whether the server answered the last attempts. */
enum class Contact { Unknown, Online, Offline }

/** The file coming in now; [title] only when the active profile may see the download (security review L-4). */
data class FetchProgress(val id: String, val bytes: Long, val total: Long, val title: String? = null)

/** How a run of fetches ended. */
sealed interface FetchRun {
    /** Nothing left to fetch now. */
    data object Done : FetchRun

    /** Work left that can go on at once (the run's deadline, the network gone). */
    data object Left : FetchRun

    /** Only downloads whose last fetch failed on the device are left: again at [at] (device time). */
    data class RetryAt(val at: Long) : FetchRun
}

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
    fun scheduleBackground(unmetered: Boolean, delayMs: Long = 0)

    /** Nothing more to do on this device (signed out, no downloads). */
    fun cancelAll()

    object None : FetchScheduler {
        override fun startTransfer() = Unit

        override fun scheduleBackground(unmetered: Boolean, delayMs: Long) = Unit

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
 *
 * A fetch runs as a child of the caller of [runFetches] (code review B-1): a stopped job or worker
 * stops its fetch and waits for its file to close, so two writers on one file cannot happen.
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
    scope: CoroutineScope,
    private val clock: () -> Long = System::currentTimeMillis,
    /** Clock since boot and the boot count for the PIN brake. */
    private val elapsed: () -> Long = { System.nanoTime() / 1_000_000 },
    private val boot: () -> Int = { -1 },
) {
    /** The engine's own work (a sync after an order) never takes the app down with an unexpected error (code review m-1). */
    private val scope = CoroutineScope(scope.coroutineContext + CoroutineExceptionHandler { _, e -> Log.w(TAG, "offline work broke", error = e) })

    var scheduler: FetchScheduler = FetchScheduler.None

    val downloads: Flow<List<StoredDownload>> = store.downloads
    val profiles: Flow<List<StoredProfile>> = store.profiles

    private val _contact = MutableStateFlow(Contact.Unknown)

    /** Offline only after two failed attempts in a row or without any network (code review M-3). */
    val contact: StateFlow<Contact> = _contact.asStateFlow()
    private var failures = 0

    private val _progress = MutableStateFlow<FetchProgress?>(null)
    val progress: StateFlow<FetchProgress?> = _progress.asStateFlow()

    private val _account = MutableStateFlow<OfflineAccount?>(null)

    /** The owner and profile of the last resolution (for drawing without suspending). */
    val lastAccount: StateFlow<OfflineAccount?> = _account.asStateFlow()

    private val syncLock = Mutex()
    private val fetchLock = Mutex()
    private var repaired = false

    /** Downloads deleted here a moment ago: a sync answer in flight does not bring them back (K5). */
    private val recentDeletes = mutableMapOf<String, Long>()

    /** The fetch in progress (one at a time) and its download. */
    @Volatile private var current: Pair<String, Job>? = null

    private val json = SlouchcatApi.json
    private val tombstoneSerializer = MapSerializer(String.serializer(), ListSerializer(String.serializer()))

    /** A sync soon, off the caller (after a playback, a change of the network). */
    fun scheduleSync() {
        scope.launch { sync() }
    }

    fun reportOffline() {
        failures = OFFLINE_AFTER
        _contact.value = Contact.Offline
    }

    // ---------- who ----------

    /** The owner of the downloads and the active profile now; learns the account id online once per e-mail and server. */
    suspend fun account(): OfflineAccount? {
        val who = session.current() ?: return null.also { _account.value = null }
        val name = "$META_ACCOUNT_ID${who.server}|${who.email.lowercase()}"
        val id = store.meta(name)?.toLongOrNull() ?: when (val answer = server.accountId()) {
            is ApiResult.Ok -> answer.value.also { store.setMeta(name, it.toString()) }
            else -> null
        } ?: return null.also { _account.value = null }
        return OfflineAccount("${who.server}|#$id", who.profileId).also { _account.value = it }
    }

    // ---------- sync ----------

    private fun contactAfter(outcome: Contact) {
        when (outcome) {
            Contact.Online -> {
                failures = 0
                _contact.value = Contact.Online
            }
            Contact.Offline -> {
                failures++
                if (failures >= OFFLINE_AFTER) _contact.value = Contact.Offline
            }
            Contact.Unknown -> Unit
        }
    }

    /** One sync (or the revocation check without a session); the contact follows the answers. */
    suspend fun sync(): Contact = syncLock.withLock {
        if (network() == NetworkKind.None) {
            reportOffline()
            return@withLock Contact.Offline
        }
        val outcome = try {
            val who = account()
            when {
                who == null -> if (session.current() == null) revocationsWithoutSession() else Contact.Offline
                who.profileId == null -> flushEvents(who.key)
                else -> syncAccount(who.key, who.profileId)
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: RuntimeException) {
            Log.w(TAG, "sync failed", error = e)
            Contact.Unknown
        } catch (e: IOException) {
            Log.w(TAG, "sync failed", error = e)
            Contact.Unknown
        }
        contactAfter(outcome)
        outcome
    }

    private suspend fun syncAccount(account: String, profileId: Long): Contact {
        rememberAccount(account)
        if (!sendTombstones(account)) return Contact.Offline
        sendPendingDeletes(account)
        val held = heldDownloads(store.allDownloads(), account)
        val answer = when (val result = server.sync(OfflineSyncRequest(held = held, network = network().forRequest()))) {
            is ApiResult.Ok -> result.value
            ApiResult.Offline -> return Contact.Offline
            // The session ended (a revoked device or account): the app forgets it, and the next run asks by the device id.
            else -> return Contact.Online
        }
        val now = clock()
        val queued = store.allEvents()
        val deleted = synchronized(recentDeletes) {
            recentDeletes.entries.removeIf { now - it.value > RECENT_DELETE_MS }
            recentDeletes.keys.toSet()
        } + tombstones()[account].orEmpty()
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
            store.updateDownload(state.id) { local -> if (local?.pendingDelete == true) null else mergeDownload(local, state, account, now, active) }
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

    // ---------- tombstones of a sign-out without server (code review M-2) ----------

    private suspend fun tombstones(): Map<String, List<String>> =
        store.meta(META_TOMBSTONES)?.let { runCatching { json.decodeFromString(tombstoneSerializer, it) }.getOrNull() }.orEmpty()

    private suspend fun saveTombstones(all: Map<String, List<String>>) =
        store.setMeta(META_TOMBSTONES, all.filterValues { it.isNotEmpty() }.takeIf { it.isNotEmpty() }?.let { json.encodeToString(tombstoneSerializer, it) })

    /** Sends the deletes the sign-out could not; `false` when the server did not answer (nothing else goes then). */
    private suspend fun sendTombstones(account: String): Boolean {
        val all = tombstones()
        val pending = all[account].orEmpty()
        if (pending.isEmpty()) return true
        val left = pending.toMutableList()
        val now = clock()
        for (id in pending) {
            // An answer in flight of this sync may still list it: it counts as deleted here (K5).
            synchronized(recentDeletes) { recentDeletes[id] = now }
            when (server.delete(id)) {
                is ApiResult.Ok, ApiResult.NotFound -> left.remove(id)
                ApiResult.Offline -> {
                    saveTombstones(all + (account to left))
                    return false
                }
                else -> left.remove(id)
            }
        }
        saveTombstones(all + (account to left))
        return true
    }

    /**
     * Removes downloads with their files (revoked, deleted): first the mark, so no fetch starts them
     * again, then the running fetch stops, then the files, then the record (code review M-1, M-7).
     */
    private suspend fun removeDownloads(ids: Collection<String>) {
        if (ids.isEmpty()) return
        for (id in ids) {
            store.updateDownload(id) { it?.copy(pendingDelete = true) }
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
        val who = account() ?: return ApiResult.SignedOut
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
     * record stays as `pendingDelete` (no files) and the delete goes out at the next contact. The mark
     * comes first, so a crash never leaves a «complete» record without its files (code review M-7).
     */
    suspend fun delete(ids: Collection<String>) {
        val now = clock()
        synchronized(recentDeletes) { ids.forEach { recentDeletes[it] = now } }
        for (id in ids) {
            store.updateDownload(id) { it?.copy(pendingDelete = true, complete = false, bytesWritten = 0, subtitlesStored = emptyList(), trickplayStored = emptyList(), posterStored = false) }
            cancelFetch(id)
            withContext(Dispatchers.IO) { runCatching { files.remove(id) } }
            when (server.delete(id)) {
                is ApiResult.Ok, ApiResult.NotFound -> store.deleteDownloads(listOf(id))
                else -> Unit
            }
        }
        afterChange()
    }

    /** Stops fetching [id] on this device; the server keeps preparing it (it knows no pause). */
    suspend fun pause(id: String) {
        store.updateDownload(id) { it?.copy(paused = true) }
        cancelFetch(id)
    }

    /** «Anhalten» of the notification: every download this run would fetch waits until the user resumes it (code review m-8). */
    suspend fun pauseAll() {
        for (id in pendingFetches(NetworkKind.Wifi)) store.updateDownload(id) { it?.copy(paused = true) }
        current?.first?.let { id -> store.updateDownload(id) { it?.copy(paused = true) } }
        stopFetching()
    }

    suspend fun resume(id: String) {
        store.updateDownload(id) { it?.copy(paused = false, fetchError = null, fetchErrorAt = null) }
        scheduler.startTransfer()
    }

    // ---------- fetch ----------

    /** Whether something waits for its files on this device (and may be fetched on [kind]). */
    suspend fun pendingFetches(kind: NetworkKind = network()): List<String> {
        val who = account() ?: return emptyList()
        return pendingFor(who, kind, clock())
    }

    private suspend fun pendingFor(who: OfflineAccount, kind: NetworkKind, now: Long): List<String> {
        val all = store.allDownloads()
        val profiles = store.allProfiles()
        val only = runCatching { wifiOnly() }.getOrDefault(true)
        return fetchOrder(all, now)
            .mapNotNull { id -> all.firstOrNull { it.id == id } }
            .filter { it.account == who.key && !waitsForWifi(it, profiles, kind, only) }
            // The manifest needs a profile that may see the title; another profile leaves it waiting.
            .filter { it.manifest != null || (who.profileId != null && who.profileId in it.allowedProfileIds) }
            .map { it.id }
    }

    /** Whether a pending download waits only for Wi-Fi. */
    suspend fun waitsForWifiOnly(): Boolean = pendingFetches(NetworkKind.Wifi).isNotEmpty() && pendingFetches().isEmpty()

    /** The next moment a download whose fetch failed on the device is due again (code review m-11); `null`: none. */
    private suspend fun nextRetry(who: OfflineAccount): Long? = store.allDownloads()
        .filter { it.account == who.key && it.state == DownloadStateValue.READY && !it.complete && !it.pendingDelete && !it.paused }
        .mapNotNull { d -> d.fetchError?.takeIf { it !in FetchError.SERVER }?.let { (d.fetchErrorAt ?: 0) + FETCH_RETRY_MS } }
        .minOrNull()

    /**
     * Fetches pending downloads one after the other until none is left, [deadline] (device time)
     * passes, the network goes or the caller is cancelled. Each fetch is a child of the caller: a
     * cancelled caller cancels the fetch and returns only after its file is closed.
     */
    suspend fun runFetches(deadline: Long? = null): FetchRun = fetchLock.withLock {
        repairOnce()
        val who = account() ?: return@withLock FetchRun.Done
        var left = pendingFor(who, network(), clock())
        while (left.isNotEmpty() && (deadline == null || clock() < deadline) && network() != NetworkKind.None) {
            val next = left.first()
            val before = store.download(next)?.bytesWritten
            coroutineScope {
                val job = launch(Dispatchers.IO) { safeFetch(next, who, deadline) }
                current = next to job
                try {
                    job.join()
                } finally {
                    current = null
                    _progress.value = null
                }
            }
            left = pendingFor(who, network(), clock())
            // The same download again without progress and without an error: it waits for something; the next run.
            val after = store.download(next)
            if (left.firstOrNull() == next && after?.fetchError == null && after?.bytesWritten == before) break
        }
        when {
            left.isNotEmpty() -> FetchRun.Left
            else -> nextRetry(who)?.let { FetchRun.RetryAt(it) } ?: FetchRun.Done
        }
    }

    private suspend fun safeFetch(id: String, who: OfflineAccount, deadline: Long?) {
        try {
            fetchOne(id, who, deadline)
        } catch (e: CancellationException) {
            throw e
        } catch (e: IOException) {
            Log.w(TAG, "fetch broke", error = e)
            fail(id, FetchError.STORAGE)
        } catch (e: RuntimeException) {
            Log.w(TAG, "fetch broke", error = e)
            fail(id, FetchError.STORAGE)
        }
    }

    private suspend fun cancelFetch(id: String) {
        current?.takeIf { it.first == id }?.second?.cancelAndJoin()
    }

    /** Stops the fetch in progress (network gone, sign-out) and waits until its file is closed. */
    suspend fun stopFetching() {
        current?.second?.cancelAndJoin()
    }

    private suspend fun fail(id: String, code: String) {
        Log.i(TAG, "fetch failed", "code" to code)
        store.updateDownload(id) { it?.copy(fetchError = code, fetchErrorAt = clock()) }
    }

    /**
     * Once per process: folders without a record go, and a record «complete» whose files are missing
     * (a crash between the steps of a delete in an older build, a cleared folder) is fetched again
     * (code review M-7, L-9: under the fetch lock, never while a fetch creates a folder).
     */
    private suspend fun repairOnce() {
        if (repaired) return
        repaired = true
        withContext(Dispatchers.IO) {
            val all = store.allDownloads()
            runCatching { files.sweep(all.map { it.id }.toSet()) }
            for (d in all) {
                if (d.complete && !d.pendingDelete && !(files.media(d.id).exists() && files.playlist(d.id).exists())) {
                    store.updateDownload(d.id) { it?.copy(complete = false, bytesWritten = 0, fileEtag = null, subtitlesStored = emptyList(), trickplayStored = emptyList()) }
                }
            }
        }
    }

    /** Manifest, poster, file, subtitles and preview sheets of one download; complete when all are there. */
    private suspend fun fetchOne(id: String, who: OfflineAccount, deadline: Long?) {
        var download = store.download(id) ?: return
        // A copy prepared again (another readyAt): the old bytes belong to another file.
        if (download.manifest != null && download.fileReadyAt != null && download.fileReadyAt != download.readyAt) {
            withContext(Dispatchers.IO) { files.remove(id) }
            download = reset(id) ?: return
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
            download = store.updateDownload(id) { it?.takeUnless { d -> d.pendingDelete }?.copy(manifest = manifest, fileReadyAt = it.readyAt, bytesWritten = 0, fileEtag = null) } ?: return
        }
        val manifest = download.manifest ?: return
        // The title only for a profile that may see it (notification, security review L-4).
        val title = download.item?.title?.takeIf { who.profileId != null && who.profileId in download.allowedProfileIds }
        withContext(Dispatchers.IO) { files.folder(id).mkdirs() }
        // The poster first: it needs the session, the rest only the download token (9.3). Best effort.
        if (!download.posterStored) {
            val poster = download.item?.images?.let { it.posterSmall ?: it.poster ?: it.backdropSmall ?: it.backdrop }?.url?.let(server::absolute)
            val stored = poster == null || fetcher.fetchSmall(poster, files.poster(id))
            if (stored) download = store.updateDownload(id) { it?.copy(posterStored = true) } ?: return
        }
        val url = server.absolute(manifest.fileUrl) ?: return fail(id, FetchError.GONE)
        // After a crash the file may be longer than what was durably written: cut it back (stage 0 point 6).
        withContext(Dispatchers.IO) { files.truncateMedia(id, download.bytesWritten) }
        val from = withContext(Dispatchers.IO) { files.media(id).length() }.coerceAtMost(download.bytesWritten)
        _progress.value = FetchProgress(id, from, manifest.sizeBytes, title)
        val outcome = fetcher.fetch(
            url = url,
            target = files.media(id),
            from = from,
            total = manifest.sizeBytes,
            etag = download.fileEtag,
            onDurable = { length, etag -> store.setBytes(id, length, etag) },
            onProgress = { bytes -> _progress.value = FetchProgress(id, bytes, manifest.sizeBytes, title) },
            deadline = deadline,
        )
        when (outcome) {
            FetchOutcome.Stopped -> return
            is FetchOutcome.Failed -> {
                if (outcome.code == FetchError.CHANGED) {
                    // Another copy in the server: its manifest first, the file from the start (code review m-2).
                    withContext(Dispatchers.IO) { files.remove(id) }
                    reset(id)
                    return
                }
                return fail(id, outcome.code)
            }
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
        store.updateDownload(id) { it?.takeUnless { d -> d.pendingDelete }?.copy(complete = true, fetchError = null, fetchErrorAt = null) }
        Log.i(TAG, "download complete")
        // The next sync reports it as complete; the server clears its copy 24 h later (9.2 point 4).
        scope.launch { sync() }
    }

    private suspend fun reset(id: String) = store.updateDownload(id) {
        it?.copy(manifest = null, fileReadyAt = null, bytesWritten = 0, fileEtag = null, subtitlesStored = emptyList(), trickplayStored = emptyList(), posterStored = false, complete = false, fetchError = null, fetchErrorAt = null)
    }

    // ---------- profiles and the offline PIN ----------

    /**
     * After a correct PIN online (the profile choice): the device derives its own verifier of the
     * normal PIN with the salt of the last sync, only when it holds a download of the profile (9.5).
     */
    suspend fun pinEntered(profileId: Long, pin: CharArray) {
        val account = account()?.key ?: store.meta(META_ACCOUNT) ?: return
        val profile = store.profile(account, profileId) ?: return
        val salt = profile.snapshot.pinSalt ?: return
        if (profile.pinVerifier?.salt == salt) return
        if ("$account:$profileId" !in profilesWithDownloads(store.allDownloads())) return
        val verifier = withContext(Dispatchers.Default) { OfflinePin.deriveLocal(pin, salt, profile.snapshot.pinIterations ?: OFFLINE_PIN_ITERATIONS) } ?: return
        store.updateProfile(account, profileId) { it?.copy(pinVerifier = verifier) }
    }

    /**
     * The offline PIN of [profileId] with the brake against trying (9.5); never on the main thread.
     * Every check stores the rest of a running pause: after a restart it starts again (security review M-1).
     */
    suspend fun checkPin(profileId: Long, pin: CharArray): PinCheck {
        val account = account()?.key ?: return PinCheck.Unavailable
        val profile = store.profile(account, profileId) ?: return PinCheck.Unavailable
        val now = elapsed()
        val bootNow = boot()
        val lock = OfflinePin.anchored(profile.lock, now, bootNow)
        val remaining = OfflinePin.remainingMs(lock, now, bootNow)
        if (remaining > 0) {
            store.updateProfile(account, profileId) { it?.copy(lock = lock.copy(remaining = remaining)) }
            return PinCheck.Locked(remaining)
        }
        if (OfflinePin.requirement(profile) == OfflinePin.Requirement.Unavailable) return PinCheck.Unavailable
        val ok = withContext(Dispatchers.Default) { OfflinePin.verify(profile, pin) }
        val updated = store.updateProfile(account, profileId) { stored ->
            stored?.copy(lock = if (ok) PinLock() else OfflinePin.registerFailure(lock, elapsed(), bootNow))
        }
        if (ok) return PinCheck.Ok
        val wait = updated?.let { OfflinePin.remainingMs(it.lock, elapsed(), bootNow) } ?: 0
        return if (wait > 0) PinCheck.Locked(wait) else PinCheck.Wrong
    }

    // ---------- rules ----------

    suspend fun rules(): ApiResult<DownloadRules> = server.rules()

    suspend fun setRules(rules: DownloadRules): ApiResult<DownloadRules> {
        val result = server.setRules(rules)
        val who = account()
        if (result is ApiResult.Ok && who?.profileId != null) {
            store.updateProfile(who.key, who.profileId) { it?.copy(rules = result.value) }
            scope.launch { sync() }
        }
        return result
    }

    // ---------- sign-out ----------

    /** Downloads of the signed-in account on this device (the question of the sign-out sheet). */
    suspend fun accountDownloads(): List<StoredDownload> {
        val account = account()?.key ?: return emptyList()
        return store.allDownloads().filter { it.account == account && !it.pendingDelete }
    }

    /**
     * Before the sign-out (9.7, F-25): [deleteAll] deletes the account's downloads on the device and
     * in the server, in parallel and each with a short limit while the token still works; what the
     * server does not confirm stays as a tombstone of the account and goes out at its next sign-in,
     * before the first sync (code review M-2). Without [deleteAll] they stay locked until the same
     * account signs in again (the PWA's rule). Either way the background work stops.
     */
    suspend fun beforeSignOut(deleteAll: Boolean) {
        stopFetching()
        if (deleteAll) {
            val account = account()?.key
            val ids = accountDownloads().map { it.id } + store.allDownloads().filter { it.pendingDelete && it.account == account }.map { it.id }
            for (id in ids) store.updateDownload(id) { it?.copy(pendingDelete = true) }
            withContext(Dispatchers.IO) { ids.forEach { runCatching { files.remove(it) } } }
            val confirmed = coroutineScope {
                ids.map { id -> async { id.takeIf { withTimeoutOrNull(SIGN_OUT_DELETE_MS) { server.delete(id) }.let { it is ApiResult.Ok || it == ApiResult.NotFound } } } }.awaitAll()
            }.filterNotNull().toSet()
            val unconfirmed = ids.filter { it !in confirmed }
            if (account != null && unconfirmed.isNotEmpty()) {
                val all = tombstones()
                saveTombstones(all + (account to (all[account].orEmpty() + unconfirmed).distinct()))
            }
            store.deleteDownloads(ids)
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
        const val META_ACCOUNT_ID = "accountId:"
        const val META_OFFSET = "serverOffsetMs"
        const val META_TOMBSTONES = "tombstones"

        /** Failed attempts in a row before the contact counts as offline (code review M-3). */
        const val OFFLINE_AFTER = 2

        /** How long the sign-out waits for each delete in the server. */
        const val SIGN_OUT_DELETE_MS = 5_000L
    }
}
