package com.slouchcat.core.player

import com.slouchcat.core.api.generated.models.Chapter
import kotlin.math.roundToInt

/**
 * The pure rules of the audio player (web `features/player/audio.ts`, concept 27 8.3): jumps,
 * speeds, chapters on the one timeline of a title (an audiobook of several files plays as one
 * HLS timeline, its chapters and positions are on it), and the sleep timer.
 */
object AudioRules {
    /** «15 Sekunden zurück» (web `AUDIO_SEEK_S` and the left button). */
    const val BACK_MS = 15_000L

    /** «30 Sekunden vor». */
    const val FORWARD_MS = 30_000L

    /** Within this time after a chapter's start «Vorheriges Kapitel» goes to the one before. */
    const val CHAPTER_RESTART_MS = 3_000L

    const val MIN_SPEED = 0.5f
    const val MAX_SPEED = 3f

    /** The presets of the sheet «Tempo» (web `SPEEDS`). */
    val SPEEDS = listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 1.75f, 2f, 2.5f, 3f)

    /** The step of «Langsamer» and «Schneller». */
    const val SPEED_STEP = 0.1f

    /** [speed] within the range, on a hundredth (1.25 and 1.75 are presets; the server keeps 0.5 to 3.0). */
    fun clampSpeed(speed: Float): Float = ((speed.coerceIn(MIN_SPEED, MAX_SPEED) * 100).roundToInt() / 100f)

    /** The chapter [positionMs] lies in; `null` without chapters or before the first. */
    fun chapterIndex(chapters: List<Chapter>, positionMs: Long): Int? {
        if (chapters.isEmpty()) return null
        val index = chapters.indexOfLast { it.startMs <= positionMs }
        return index.takeIf { it >= 0 }
    }

    /**
     * Where «Vorheriges Kapitel» jumps: the start of the current chapter, or the one before when
     * the position is within [CHAPTER_RESTART_MS] of the start; 0 without chapters.
     */
    fun previousChapterStart(chapters: List<Chapter>, positionMs: Long): Long {
        val index = chapterIndex(chapters, positionMs) ?: return 0
        val current = chapters[index]
        return if (positionMs - current.startMs < CHAPTER_RESTART_MS && index > 0) chapters[index - 1].startMs else current.startMs
    }

    /** Where «Nächstes Kapitel» jumps; `null` in the last chapter (the button is off). */
    fun nextChapterStart(chapters: List<Chapter>, positionMs: Long): Long? {
        if (chapters.isEmpty()) return null
        val index = chapterIndex(chapters, positionMs) ?: return chapters.first().startMs.takeIf { it > positionMs }
        return chapters.getOrNull(index + 1)?.startMs
    }
}

/** A choice of the sheet «Schlaftimer» (web `SleepChoice`). */
sealed interface SleepChoice {
    data object Off : SleepChoice

    data class Minutes(val minutes: Int) : SleepChoice

    /** At the end of the current chapter, or of the title when it has none (web «Ende des Titels»). */
    data object EndOfChapter : SleepChoice

    companion object {
        val MINUTES = listOf(15, 30, 45, 60)
    }
}

/** The running sleep timer: [leftMs] counts down (media time scaled by the speed for a chapter end). */
data class SleepState(val choice: SleepChoice, val leftMs: Long)

/**
 * The sleep timer of the audio player: it runs on the device (as in the web app) and pauses when
 * it is due; the pause reports the progress like any other. [minutes] count down while the title
 * plays (a pause stops the clock); the end of a chapter is measured in media time and turned
 * into wall time by the speed.
 */
class SleepTimer {
    var state: SleepState? = null
        private set

    private var lastTick: Long? = null

    /** The media time the chapter chosen at [set] ends at (a seek does not move it). */
    private var chapterEnd: Long = 0

    fun set(choice: SleepChoice, positionMs: Long, chapters: List<Chapter>, durationMs: Long, speed: Float) {
        lastTick = null
        chapterEnd = AudioRules.chapterIndex(chapters, positionMs)?.let { chapters[it].endMs } ?: durationMs
        state = when (choice) {
            SleepChoice.Off -> null
            is SleepChoice.Minutes -> SleepState(choice, choice.minutes * 60_000L)
            SleepChoice.EndOfChapter -> SleepState(choice, chapterLeft(positionMs, speed))
        }
    }

    /**
     * One step at [nowMs] (a monotonic clock): `true` when the timer is due and the player should
     * pause. [playing]: the clock of [SleepChoice.Minutes] runs only while the title plays.
     */
    fun tick(nowMs: Long, playing: Boolean, positionMs: Long, chapters: List<Chapter>, durationMs: Long, speed: Float): Boolean {
        val running = state ?: return false
        val last = lastTick
        lastTick = if (playing) nowMs else null
        val left = when (running.choice) {
            is SleepChoice.Minutes -> if (playing && last != null) running.leftMs - (nowMs - last) else running.leftMs
            SleepChoice.EndOfChapter -> chapterLeft(positionMs, speed)
            SleepChoice.Off -> return false
        }
        if (left <= 0) {
            state = null
            lastTick = null
            return true
        }
        state = running.copy(leftMs = left)
        return false
    }

    fun cancel() {
        state = null
        lastTick = null
    }

    private fun chapterLeft(positionMs: Long, speed: Float): Long =
        ((chapterEnd - positionMs).coerceAtLeast(0) / speed.coerceAtLeast(AudioRules.MIN_SPEED)).toLong()
}
