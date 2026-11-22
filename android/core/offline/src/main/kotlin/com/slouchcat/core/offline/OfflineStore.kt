package com.slouchcat.core.offline

import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.models.DownloadManifest
import com.slouchcat.core.api.generated.models.OfflineEvent
import com.slouchcat.core.offline.db.DownloadRow
import com.slouchcat.core.offline.db.EventRow
import com.slouchcat.core.offline.db.MetaRow
import com.slouchcat.core.offline.db.OfflineDao
import com.slouchcat.core.offline.db.ProfileRow
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

/**
 * The offline database in the app's terms: [StoredDownload] and [StoredProfile] as JSON in their
 * rows (the generated models serialize with the client's JSON), the queue of progress events and a
 * few values of the device. Updates of one record go through [updateDownload]/[updateProfile] under
 * one lock, so a fetch writing its bytes and a sync writing the server's state never lose each other.
 */
class OfflineStore(private val dao: OfflineDao) {
    private val json = SlouchcatApi.json
    private val lock = Mutex()

    /** Decoded manifests by download id with their JSON: a row read again decodes its fragment index once. */
    private val manifests = java.util.concurrent.ConcurrentHashMap<String, Pair<String, DownloadManifest>>()

    private fun manifestOf(id: String, text: String?): DownloadManifest? {
        text ?: return null
        manifests[id]?.takeIf { it.first == text }?.let { return it.second }
        val decoded = runCatching { json.decodeFromString(DownloadManifest.serializer(), text) }.getOrNull() ?: return null
        manifests[id] = text to decoded
        return decoded
    }

    private fun decode(row: DownloadRow): StoredDownload? =
        runCatching { json.decodeFromString(StoredDownload.serializer(), row.data) }.getOrNull()
            ?.copy(manifest = manifestOf(row.id, row.manifest), bytesWritten = row.bytes, fileEtag = row.etag)

    private fun encode(download: StoredDownload) = DownloadRow(
        id = download.id,
        account = download.account,
        createdAt = download.createdAt,
        data = json.encodeToString(StoredDownload.serializer(), download.copy(manifest = null, bytesWritten = 0, fileEtag = null)),
        manifest = download.manifest?.let { m -> manifests[download.id]?.takeIf { it.second === m }?.first ?: json.encodeToString(DownloadManifest.serializer(), m) },
        bytes = download.bytesWritten,
        etag = download.fileEtag,
    )

    /**
     * The durable length and ETag of a fetch, without the rest of the record; `false` when the
     * download is gone or no longer wanted (deleted, paused): the fetch stops.
     */
    suspend fun setBytes(id: String, bytes: Long, etag: String?): Boolean = lock.withLock {
        val row = dao.download(id) ?: return@withLock false
        val download = decode(row) ?: return@withLock false
        if (download.pendingDelete || download.paused) return@withLock false
        dao.setBytes(id, bytes, etag) > 0
    }

    private fun decodeProfile(row: ProfileRow): StoredProfile? =
        runCatching { json.decodeFromString(StoredProfile.serializer(), row.data) }.getOrNull()

    private fun encodeProfile(profile: StoredProfile) =
        ProfileRow(profile.account, profile.id, json.encodeToString(StoredProfile.serializer(), profile))

    val downloads: Flow<List<StoredDownload>> = dao.observeDownloads().map { rows -> rows.mapNotNull(::decode) }
    val profiles: Flow<List<StoredProfile>> = dao.observeProfiles().map { rows -> rows.mapNotNull(::decodeProfile) }

    suspend fun allDownloads(): List<StoredDownload> = dao.downloads().mapNotNull(::decode)

    suspend fun download(id: String): StoredDownload? = dao.download(id)?.let(::decode)

    /** Reads, changes and writes one download under the store's lock; `null` from [change] deletes nothing and writes nothing. */
    suspend fun updateDownload(id: String, change: (StoredDownload?) -> StoredDownload?): StoredDownload? = lock.withLock {
        val next = change(dao.download(id)?.let(::decode)) ?: return@withLock null
        dao.putDownload(encode(next))
        next
    }

    suspend fun putDownload(download: StoredDownload) = lock.withLock { dao.putDownload(encode(download)) }

    suspend fun deleteDownloads(ids: Collection<String>) {
        if (ids.isEmpty()) return
        lock.withLock { ids.chunked(CHUNK).forEach { dao.deleteDownloads(it) } }
        ids.forEach(manifests::remove)
    }

    suspend fun allProfiles(): List<StoredProfile> = dao.profiles().mapNotNull(::decodeProfile)

    suspend fun profile(account: String, id: Long): StoredProfile? = dao.profile(account, id)?.let(::decodeProfile)

    suspend fun updateProfile(account: String, id: Long, change: (StoredProfile?) -> StoredProfile?): StoredProfile? = lock.withLock {
        val next = change(dao.profile(account, id)?.let(::decodeProfile)) ?: return@withLock null
        dao.putProfile(encodeProfile(next))
        next
    }

    suspend fun deleteProfiles(account: String, ids: Collection<Long>) {
        if (ids.isNotEmpty()) lock.withLock { dao.deleteProfiles(account, ids.toList()) }
    }

    /** Drops profiles and queued events of every account but [keep] (an account that left the device). */
    suspend fun keepOnly(keep: Collection<String>) = lock.withLock {
        dao.deleteProfilesExcept(keep.toList())
        dao.deleteEventsExcept(keep.toList())
    }

    suspend fun queue(account: String, event: OfflineEvent) =
        dao.addEvent(EventRow(account = account, data = json.encodeToString(OfflineEvent.serializer(), event)))

    suspend fun events(account: String, limit: Int): List<QueuedEvent> = dao.events(account, limit).mapNotNull { row ->
        runCatching { QueuedEvent(row.key, row.account, json.decodeFromString(OfflineEvent.serializer(), row.data)) }.getOrNull()
    }

    suspend fun allEvents(): List<QueuedEvent> = dao.allEvents().mapNotNull { row ->
        runCatching { QueuedEvent(row.key, row.account, json.decodeFromString(OfflineEvent.serializer(), row.data)) }.getOrNull()
    }

    suspend fun deleteEvents(keys: Collection<Long>) = keys.chunked(CHUNK).forEach { dao.deleteEvents(it) }

    suspend fun meta(key: String): String? = dao.meta(key)

    suspend fun setMeta(key: String, value: String?) = if (value == null) dao.deleteMeta(key) else dao.putMeta(MetaRow(key, value))

    companion object {
        /** The database of the device in `noBackupFilesDir` ([com.slouchcat.core.offline.db.OfflineDatabase.file]). */
        fun open(context: android.content.Context) = OfflineStore(com.slouchcat.core.offline.db.OfflineDatabase.open(context).dao())

        /** For tests of the apps: a database in memory. */
        fun inMemory(context: android.content.Context) = OfflineStore(com.slouchcat.core.offline.db.OfflineDatabase.inMemory(context).dao())

        /** SQLite's limit of bound variables per statement is 999 on old versions. */
        private const val CHUNK = 500
    }
}
