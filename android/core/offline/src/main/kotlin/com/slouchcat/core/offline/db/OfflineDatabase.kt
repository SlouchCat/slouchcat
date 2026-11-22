package com.slouchcat.core.offline.db

import android.content.Context
import androidx.room.Dao
import androidx.room.Database
import androidx.room.Entity
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.Room
import androidx.room.RoomDatabase
import androidx.room.Upsert
import kotlinx.coroutines.flow.Flow
import java.io.File

/**
 * A download as the device keeps it ([com.slouchcat.core.offline.StoredDownload] as JSON in [data]):
 * the server's last state, the title, how far the files are, the positions per profile. The manifest
 * with its fragment index has a column of its own and so do the durable bytes and the ETag: the fetch
 * writes those every 8 MiB without writing the fragment index again (code review M-5).
 */
@Entity(tableName = "downloads")
data class DownloadRow(
    @PrimaryKey val id: String,
    val account: String,
    val createdAt: String,
    val data: String,
    val manifest: String? = null,
    val bytes: Long = 0,
    val etag: String? = null,
)

/** A profile as the offline library needs it ([com.slouchcat.core.offline.StoredProfile] as JSON): snapshot, verifier, brake, watch time, rule. */
@Entity(tableName = "profiles", primaryKeys = ["account", "id"])
data class ProfileRow(
    val account: String,
    val id: Long,
    val data: String,
)

/** A progress event of offline playback waiting for the upload (`OfflineEvent` as JSON). */
@Entity(tableName = "events")
data class EventRow(
    @PrimaryKey(autoGenerate = true) val key: Long = 0,
    val account: String,
    val data: String,
)

/** Small values of the device: the account of the downloads, the last sync, the server's clock offset. */
@Entity(tableName = "meta")
data class MetaRow(
    @PrimaryKey val key: String,
    val value: String,
)

@Dao
interface OfflineDao {
    @Query("SELECT * FROM downloads ORDER BY createdAt DESC")
    fun observeDownloads(): Flow<List<DownloadRow>>

    @Query("SELECT * FROM downloads ORDER BY createdAt DESC")
    suspend fun downloads(): List<DownloadRow>

    @Query("SELECT * FROM downloads WHERE id = :id")
    suspend fun download(id: String): DownloadRow?

    @Upsert
    suspend fun putDownload(row: DownloadRow)

    @Query("UPDATE downloads SET bytes = :bytes, etag = :etag WHERE id = :id")
    suspend fun setBytes(id: String, bytes: Long, etag: String?): Int

    @Query("DELETE FROM downloads WHERE id IN (:ids)")
    suspend fun deleteDownloads(ids: List<String>)

    @Query("SELECT * FROM profiles")
    fun observeProfiles(): Flow<List<ProfileRow>>

    @Query("SELECT * FROM profiles")
    suspend fun profiles(): List<ProfileRow>

    @Query("SELECT * FROM profiles WHERE account = :account AND id = :id")
    suspend fun profile(account: String, id: Long): ProfileRow?

    @Upsert
    suspend fun putProfile(row: ProfileRow)

    @Query("DELETE FROM profiles WHERE account = :account AND id IN (:ids)")
    suspend fun deleteProfiles(account: String, ids: List<Long>)

    @Query("DELETE FROM profiles WHERE account NOT IN (:keep)")
    suspend fun deleteProfilesExcept(keep: List<String>)

    @Insert(onConflict = OnConflictStrategy.ABORT)
    suspend fun addEvent(row: EventRow)

    @Query("SELECT * FROM events WHERE account = :account ORDER BY `key` LIMIT :limit")
    suspend fun events(account: String, limit: Int): List<EventRow>

    @Query("SELECT * FROM events ORDER BY `key`")
    suspend fun allEvents(): List<EventRow>

    @Query("DELETE FROM events WHERE `key` IN (:keys)")
    suspend fun deleteEvents(keys: List<Long>)

    @Query("DELETE FROM events WHERE account NOT IN (:keep)")
    suspend fun deleteEventsExcept(keep: List<String>)

    @Query("SELECT value FROM meta WHERE `key` = :key")
    suspend fun meta(key: String): String?

    @Upsert
    suspend fun putMeta(row: MetaRow)

    @Query("DELETE FROM meta WHERE `key` = :key")
    suspend fun deleteMeta(key: String)
}

@Database(entities = [DownloadRow::class, ProfileRow::class, EventRow::class, MetaRow::class], version = 1, exportSchema = true)
abstract class OfflineDatabase : RoomDatabase() {
    abstract fun dao(): OfflineDao

    companion object {
        /**
         * The database lives in `noBackupFilesDir` (concept 27 9.4, security review stage 5 L-5): no
         * backup and no device transfer takes it along, whatever the backup rules say, so neither the
         * verifiers nor the brake of the offline PIN come back with a restore.
         */
        fun file(context: Context): File = File(context.noBackupFilesDir, "offline.db")

        fun open(context: Context): OfflineDatabase =
            Room.databaseBuilder(context.applicationContext, OfflineDatabase::class.java, file(context).path).build()

        /** For tests: a database in memory. */
        fun inMemory(context: Context): OfflineDatabase =
            Room.inMemoryDatabaseBuilder(context.applicationContext, OfflineDatabase::class.java).allowMainThreadQueries().build()
    }
}
