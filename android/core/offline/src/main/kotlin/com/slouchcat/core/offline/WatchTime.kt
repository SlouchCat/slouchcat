package com.slouchcat.core.offline

import com.slouchcat.core.api.generated.models.OfflineWatchTime

/**
 * Watch time offline (K-5, AN-18; declared exception 2 of plan principle 1, concept 27 9.5): the
 * server says at the sync how many minutes the profile has left today and when the day ends.
 * Offline the device subtracts what it plays by its own clock; from `resetsAt` on the full limit
 * applies again, every further 24 h as well. All times are server time (device time plus the offset
 * of the sync). The same rules as the PWA's watchTime.ts.
 */
object WatchTime {
    const val DAY_MS = 86_400_000L
    private const val MINUTE_MS = 60_000L

    /** The stored watch time of a sync answer; [pendingMs] is play of today the server has not booked yet. */
    fun fromSnapshot(snapshot: OfflineWatchTime?, syncedAt: Long, pendingMs: Long = 0): StoredWatchTime? {
        snapshot ?: return null
        return StoredWatchTime(
            limitMinutes = snapshot.limitMinutes,
            remainingMinutes = snapshot.remainingMinutes,
            resetsAt = snapshot.resetsAt.toInstant().toEpochMilli(),
            syncedAt = syncedAt,
            period = -1,
            playedMs = pendingMs.coerceAtLeast(0),
        )
    }

    /** -1 before `resetsAt`, else the number of the 24-h period after it that [now] lies in. */
    fun periodAt(watch: StoredWatchTime, now: Long): Int =
        if (now < watch.resetsAt) -1 else ((now - watch.resetsAt) / DAY_MS).toInt()

    /** Milliseconds the profile may still play at [now]. */
    fun remainingMs(watch: StoredWatchTime, now: Long): Long {
        val period = periodAt(watch, now)
        val played = if (watch.period == period) watch.playedMs else 0
        val allowance = if (period == -1) watch.remainingMinutes * MINUTE_MS else watch.limitMinutes * MINUTE_MS
        return (allowance - played).coerceAtLeast(0)
    }

    /** Adds played time at [now]; play in a new period starts that period's count. */
    fun addPlayed(watch: StoredWatchTime, ms: Long, now: Long): StoredWatchTime {
        val period = periodAt(watch, now)
        val add = ms.coerceAtLeast(0)
        return if (period == watch.period) watch.copy(playedMs = watch.playedMs + add) else watch.copy(period = period, playedMs = add)
    }

    /** Start of the day the snapshot describes (server time), for counting queued play. */
    fun snapshotDayStart(snapshot: OfflineWatchTime): Long = snapshot.resetsAt.toInstant().toEpochMilli() - DAY_MS
}
