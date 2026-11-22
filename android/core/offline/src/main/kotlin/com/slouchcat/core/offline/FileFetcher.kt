package com.slouchcat.core.offline

import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.awaitCancellation
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.Call
import okhttp3.HttpUrl
import okhttp3.Request
import java.io.File
import java.io.FileOutputStream
import java.io.IOException

/** How one fetch of a file ended. */
sealed interface FetchOutcome {
    /** The whole file is on the device and durably written. */
    data class Done(val etag: String?) : FetchOutcome

    /**
     * Stopped on purpose with the bytes so far durable: the run's deadline passed, or the download
     * is no longer wanted (deleted, paused, revoked). Not an error: the next run resumes or nothing does.
     */
    data object Stopped : FetchOutcome

    /** [code] of [FetchError]; [status] the HTTP answer when there was one. */
    data class Failed(val code: String, val status: Int? = null) : FetchOutcome
}

/**
 * Fetches a download's file with Range and If-Range, resuming at the durably written length (stage 0
 * point 6): the device writes, `sync()`s every [syncEvery] bytes and records that length, so after a
 * crash it cuts the file back to it and asks for the rest. A 200 to a resume means another prepared
 * file (the strong ETag no longer matches): `changed`, the manifest has to be fetched again (ADR 0043
 * point 3). Every request goes through [http], the app's one client.
 *
 * A cancelled coroutine cancels the HTTP call at once (also while a read blocks), and the function
 * returns only after the file is closed: two writers on one file cannot happen.
 */
class FileFetcher(
    private val http: Call.Factory,
    private val freeBytes: () -> Long,
    private val syncEvery: Long = SYNC_EVERY,
    /** What stays free on the volume at least; below it the fetch stops with `quota`. */
    private val reserve: Long = RESERVE,
    private val clock: () -> Long = System::currentTimeMillis,
    /** Least time between two progress reports (notification and page, about twice a second). */
    private val progressEvery: Long = PROGRESS_MS,
) {
    /**
     * Fetches [url] into [target] from byte [from] up to [total] bytes. [onDurable] gets every durably
     * written length with the ETag it belongs to and answers whether the download is still wanted
     * (`false` stops the fetch). [onProgress] gets the bytes so far, at most every [progressEvery] ms.
     * [deadline] (device time) stops the fetch with the bytes durable.
     */
    suspend fun fetch(
        url: HttpUrl,
        target: File,
        from: Long,
        total: Long,
        etag: String?,
        onDurable: suspend (length: Long, etag: String?) -> Boolean,
        onProgress: (Long) -> Unit = {},
        deadline: Long? = null,
    ): FetchOutcome = withContext(Dispatchers.IO) {
        if (from >= total && from > 0) return@withContext FetchOutcome.Done(etag)
        val builder = Request.Builder().url(url).get()
        if (from > 0) {
            builder.header("Range", "bytes=$from-")
            if (etag != null) builder.header("If-Range", etag)
        }
        val call = http.newCall(builder.build())
        cancelling(call) {
            try {
                transfer(call, target, from, total, etag, onDurable, onProgress, deadline)
            } catch (e: CancellationException) {
                throw e
            } catch (e: IOException) {
                currentCoroutineContext().ensureActive()
                Log.i(TAG, "no answer", error = e)
                FetchOutcome.Failed(if (isFull(e)) FetchError.QUOTA else FetchError.NETWORK)
            }
        }
    }

    private suspend fun transfer(
        call: Call,
        target: File,
        from: Long,
        total: Long,
        etag: String?,
        onDurable: suspend (Long, String?) -> Boolean,
        onProgress: (Long) -> Unit,
        deadline: Long?,
    ): FetchOutcome = call.execute().use { response ->
        val status = response.code
        when (status) {
            206 -> Unit
            // A whole file to a resume: another preparation in the server.
            200 -> if (from > 0) return FetchOutcome.Failed(FetchError.CHANGED, status)
            410 -> return FetchOutcome.Failed(FetchError.EVICTED, status)
            404 -> return FetchOutcome.Failed(FetchError.GONE, status)
            401, 403 -> return FetchOutcome.Failed(FetchError.UNAUTHORIZED, status)
            416 -> return FetchOutcome.Failed(FetchError.SIZE, status)
            else -> return FetchOutcome.Failed(FetchError.NETWORK, status)
        }
        val start = if (status == 206) from else 0L
        val tag = response.header("ETag") ?: etag
        if (status == 206 && !contentRangeStartsAt(response.header("Content-Range"), from)) {
            return FetchOutcome.Failed(FetchError.RANGE, status)
        }
        var written = start
        var lastProgress = 0L
        FileOutputStream(target, start > 0).use { out ->
            if (start == 0L) out.channel.truncate(0)
            var sinceSync = 0L
            val buffer = ByteArray(BUFFER)
            val input = response.body.byteStream()
            while (true) {
                currentCoroutineContext().ensureActive()
                val read = try {
                    input.read(buffer)
                } catch (e: IOException) {
                    currentCoroutineContext().ensureActive()
                    out.durable()
                    onDurable(written, tag)
                    return FetchOutcome.Failed(FetchError.NETWORK)
                }
                if (read < 0) break
                if (written + read > total) return FetchOutcome.Failed(FetchError.SIZE, status)
                try {
                    out.write(buffer, 0, read)
                } catch (e: IOException) {
                    return FetchOutcome.Failed(if (isFull(e)) FetchError.QUOTA else FetchError.STORAGE)
                }
                written += read
                sinceSync += read
                val now = clock()
                if (now - lastProgress >= progressEvery) {
                    lastProgress = now
                    onProgress(written)
                }
                if (sinceSync >= syncEvery) {
                    out.durable()
                    sinceSync = 0
                    if (!onDurable(written, tag)) return FetchOutcome.Stopped
                    if (freeBytes() < reserve) return FetchOutcome.Failed(FetchError.QUOTA)
                    if (deadline != null && now >= deadline) return FetchOutcome.Stopped
                }
            }
            out.durable()
        }
        onProgress(written)
        if (!onDurable(written, tag)) return FetchOutcome.Stopped
        if (written != total) FetchOutcome.Failed(FetchError.SIZE, status) else FetchOutcome.Done(tag)
    }

    /** A small file (subtitle, preview sheet, poster) as a whole; `false` with the code in [failed]. */
    suspend fun fetchSmall(url: HttpUrl, target: File, failed: (String) -> Unit = {}): Boolean = withContext(Dispatchers.IO) {
        val call = http.newCall(Request.Builder().url(url).get().build())
        cancelling(call) {
            try {
                call.execute().use { response ->
                    if (!response.isSuccessful) {
                        failed(
                            when (response.code) {
                                410 -> FetchError.EVICTED
                                404 -> FetchError.GONE
                                401, 403 -> FetchError.UNAUTHORIZED
                                else -> FetchError.NETWORK
                            },
                        )
                        return@use false
                    }
                    val temp = File(target.parentFile, target.name + ".part")
                    FileOutputStream(temp).use { out ->
                        response.body.byteStream().copyTo(out)
                        out.durable()
                    }
                    temp.renameTo(target)
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: IOException) {
                currentCoroutineContext().ensureActive()
                failed(if (isFull(e)) FetchError.QUOTA else FetchError.NETWORK)
                false
            }
        }
    }

    /**
     * Runs the blocking [block] with a watcher on another thread: when the coroutine is cancelled,
     * the watcher cancels [call] at once, so a read that hangs (a dead connection) ends now and not
     * after the read timeout (code review M-4).
     */
    private suspend fun <T> cancelling(call: Call, block: suspend CoroutineScope.() -> T): T = coroutineScope {
        val watcher = launch(Dispatchers.Default) {
            try {
                awaitCancellation()
            } finally {
                call.cancel()
            }
        }
        try {
            block()
        } finally {
            watcher.cancel()
        }
    }

    private fun FileOutputStream.durable() {
        flush()
        fd.sync()
    }

    private fun contentRangeStartsAt(header: String?, from: Long): Boolean {
        val value = header ?: return true
        val start = Regex("""bytes (\d+)-""").find(value)?.groupValues?.get(1)?.toLongOrNull() ?: return false
        return start == from
    }

    private fun isFull(e: IOException): Boolean = e.message?.contains("ENOSPC") == true || e.message?.contains("No space", ignoreCase = true) == true

    companion object {
        private const val TAG = "offline-fetch"
        const val SYNC_EVERY = 8L * 1024 * 1024
        const val RESERVE = 200L * 1024 * 1024
        const val PROGRESS_MS = 500L
        private const val BUFFER = 64 * 1024
    }
}
