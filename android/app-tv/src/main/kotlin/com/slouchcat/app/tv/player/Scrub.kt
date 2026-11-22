package com.slouchcat.app.tv.player

/**
 * Spulen with Links and Rechts (mockup `brPlSeek`): every press moves the target; presses in quick
 * succession in the same direction grow the step (10, 10, 20, 30, 30, 60, 60, 120 s), a held key
 * (its repeats) as well, but at most one step per [repeatMs] so the target stays readable. The
 * player seeks once, when the presses stop ([PlayerScreen] commits after a pause), not on every
 * step: Media3 would load each one.
 *
 * Pure: the time comes from the caller (`KeyEvent.eventTime` on the device, the test's clock in
 * a test).
 */
class Scrub(
    private val stepsMs: List<Long> = STEPS_MS,
    private val streakMs: Long = STREAK_MS,
    private val repeatMs: Long = REPEAT_MS,
) {
    /** Where the bar will seek to; `null` while nobody spools. */
    var targetMs: Long? = null
        private set

    /** The way from where the spooling began to [targetMs] (the «+2:30» of the bubble). */
    var deltaMs: Long = 0
        private set

    private var streak = 0
    private var lastAt: Long? = null
    private var direction = 0
    private var startMs = 0L

    /**
     * One press (or repeat, [repeat]) towards [direction] (−1, +1) at [nowMs], starting from
     * [positionMs] when nothing is pending, within `0 ..` [durationMs]. `false` when the press was
     * dropped (a repeat that came too soon).
     */
    fun step(direction: Int, nowMs: Long, positionMs: Long, durationMs: Long, repeat: Boolean = false): Boolean {
        val last = lastAt
        if (repeat && last != null && nowMs - last < repeatMs && direction == this.direction) return false
        val again = last != null && nowMs - last < streakMs && direction == this.direction
        streak = if (again) streak + 1 else 0
        lastAt = nowMs
        this.direction = direction
        val from = targetMs ?: positionMs.also { startMs = it }
        val end = if (durationMs > 0) (durationMs - END_GAP_MS).coerceAtLeast(0) else Long.MAX_VALUE
        val to = (from + direction * stepsMs[minOf(streak, stepsMs.lastIndex)]).coerceIn(0, end)
        targetMs = to
        deltaMs = to - startMs
        return true
    }

    /** The target to seek to now, and the spooling ends. */
    fun commit(): Long? = targetMs.also { reset() }

    fun reset() {
        targetMs = null
        deltaMs = 0
        streak = 0
        lastAt = null
        direction = 0
    }

    companion object {
        /** The steps of the mockup, in milliseconds. */
        val STEPS_MS = listOf(10_000L, 10_000L, 20_000L, 30_000L, 30_000L, 60_000L, 60_000L, 120_000L)

        /** Presses closer than this make a streak (mockup: 700 ms). */
        const val STREAK_MS = 700L

        /** At most one step per this while a key is held. */
        const val REPEAT_MS = 150L

        /** How long after the last press the player seeks. */
        const val COMMIT_MS = 800L

        /** The target stays this far before the end (the mockup's `dur - 1`). */
        const val END_GAP_MS = 1_000L
    }
}
