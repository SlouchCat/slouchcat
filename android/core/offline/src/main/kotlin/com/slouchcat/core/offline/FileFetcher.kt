package com.slouchcat.core.offline

import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
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

    /** [code] of [FetchError]; [status] the HTTP answer when there was one. */
    data class Failed(val code: String, val status: Int? = null) : FetchOutcome
}

/**
 * Fetches a download's file with Range and If-Range, resuming at the durably written length (stage 0
 * point 6): the device writes, `sync()`s every [syncEvery] bytes and records that length, so after a
 * crash it cuts the file back to it and asks for the rest. A 200 to a resume (the server prepared
 * another file, the strong ETag no longer matches, or a proxy without Range) starts the file over.
 * Every request goes through [http], the app's one client (gate «Server weist sich aus», the
 * download token `dt` in the address, never the bearer in a URL).
 */
class FileFetcher(
    private val http: Call.Factory,
    private val freeBytes: () -> Long,
    private val syncEvery: Long = SYNC_EVERY,
    /** What stays free on the volume at least; below it the fetch stops with `quota`. */
    private val reserve: Long = RESERVE,
) {
    /**
     * Fetches [url] into [target] from byte [from] up to [total] bytes. [onDurable] gets every
     * durably written length with the ETag it belongs to; [onProgress] the bytes so far.
     */
    suspend fun fetch(
        url: HttpUrl,
        target: File,
        from: Long,
        total: Long,
        etag: String?,
        onDurable: suspend (length: Long, etag: String?) -> Unit,
        onProgress: (Long) -> Unit = {},
    ): FetchOutcome = withContext(Dispatchers.IO) {
        if (from >= total && from > 0) return@withContext FetchOutcome.Done(etag)
        val builder = Request.Builder().url(url).get()
        if (from > 0) {
            builder.header("Range", "bytes=$from-")
            if (etag != null) builder.header("If-Range", etag)
        }
        val call = http.newCall(builder.build())
        val cancel = currentCoroutineContext()[Job]?.invokeOnCompletion { call.cancel() }
        try {
            call.execute().use { response ->
                val status = response.code
                val start = when (status) {
                    206 -> from
                    200 -> 0L
                    410 -> return@withContext FetchOutcome.Failed(FetchError.EVICTED, status)
                    404 -> return@withContext FetchOutcome.Failed(FetchError.GONE, status)
                    401, 403 -> return@withContext FetchOutcome.Failed(FetchError.UNAUTHORIZED, status)
                    416 -> return@withContext FetchOutcome.Failed(FetchError.SIZE, status)
                    else -> return@withContext FetchOutcome.Failed(FetchError.NETWORK, status)
                }
                val tag = response.header("ETag") ?: etag
                if (status == 206 && !contentRangeStartsAt(response.header("Content-Range"), from)) {
                    return@withContext FetchOutcome.Failed(FetchError.RANGE, status)
                }
                if (start == 0L && from > 0) Log.i(TAG, "file starts over", "status" to status)
                val body = response.body
                var written = start
                FileOutputStream(target, start > 0).use { out ->
                    if (start == 0L) out.channel.truncate(0)
                    var sinceSync = 0L
                    val buffer = ByteArray(BUFFER)
                    val input = body.byteStream()
                    while (true) {
                        currentCoroutineContext().ensureActive()
                        val read = try {
                            input.read(buffer)
                        } catch (e: IOException) {
                            currentCoroutineContext().ensureActive()
                            out.durable()
                            onDurable(written, tag)
                            return@withContext FetchOutcome.Failed(FetchError.NETWORK)
                        }
                        if (read < 0) break
                        if (written + read > total) return@withContext FetchOutcome.Failed(FetchError.SIZE, status)
                        try {
                            out.write(buffer, 0, read)
                        } catch (e: IOException) {
                            return@withContext FetchOutcome.Failed(if (isFull(e)) FetchError.QUOTA else FetchError.STORAGE)
                        }
                        written += read
                        sinceSync += read
                        onProgress(written)
                        if (sinceSync >= syncEvery) {
                            out.durable()
                            onDurable(written, tag)
                            sinceSync = 0
                            if (freeBytes() < reserve) return@withContext FetchOutcome.Failed(FetchError.QUOTA)
                        }
                    }
                    out.durable()
                }
                onDurable(written, tag)
                if (written != total) FetchOutcome.Failed(FetchError.SIZE, status) else FetchOutcome.Done(tag)
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: IOException) {
            currentCoroutineContext().ensureActive()
            Log.i(TAG, "no answer", error = e)
            FetchOutcome.Failed(if (isFull(e)) FetchError.QUOTA else FetchError.NETWORK)
        } finally {
            cancel?.dispose()
        }
    }

    /** A small file (subtitle, preview sheet, poster) as a whole; `null` with the code in [failed]. */
    suspend fun fetchSmall(url: HttpUrl, target: File, failed: (String) -> Unit = {}): Boolean = withContext(Dispatchers.IO) {
        val call = http.newCall(Request.Builder().url(url).get().build())
        val cancel = currentCoroutineContext()[Job]?.invokeOnCompletion { call.cancel() }
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
                    return@withContext false
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
        } finally {
            cancel?.dispose()
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
        private const val BUFFER = 64 * 1024
    }
}
