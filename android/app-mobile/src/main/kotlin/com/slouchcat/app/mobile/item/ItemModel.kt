package com.slouchcat.app.mobile.item

import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.NextUp
import com.slouchcat.core.api.generated.models.Progress
import java.math.BigDecimal
import java.time.OffsetDateTime

/*
 * What a detail page reads from the server's answer: the check mark, the big button and the
 * optimistic change of the check mark (web app `watched.tsx`, `DetailActions.tsx`). Every state
 * comes from the server (rule 12); the app only shows the change it asked for until the answer.
 */

/** The check mark of a title (concept 20 F-12): none, half (a group with some episodes) or full. */
enum class WatchedMark { None, Half, Full }

/** A season, series or podcast: its check mark stands for its visible episodes. */
fun isGroup(kind: ItemKind): Boolean = kind == ItemKind.SEASON || kind == ItemKind.SERIES || kind == ItemKind.PODCAST

/** Audiobooks, podcasts and their episodes are heard, not watched. */
fun isAudio(kind: ItemKind): Boolean = kind == ItemKind.AUDIOBOOK || kind == ItemKind.PODCAST || kind == ItemKind.PODCAST_EPISODE

/** The kinds with a check mark (every title but extras). */
fun hasCheck(kind: ItemKind): Boolean = kind != ItemKind.EXTRA && kind != ItemKind.UNKNOWN_DEFAULT_OPEN_API

/** The kinds that go on the watchlist (every kind but seasons and extras, web E-5). */
fun listable(kind: ItemKind): Boolean = kind != ItemKind.SEASON && kind != ItemKind.EXTRA && kind != ItemKind.UNKNOWN_DEFAULT_OPEN_API

private fun mark(kind: ItemKind, progress: Progress?, watched: Boolean?, count: Int?, children: Int?): WatchedMark {
    if (!isGroup(kind)) return if (progress?.watchedAt != null) WatchedMark.Full else WatchedMark.None
    val seen = count ?: 0
    val full = if (kind == ItemKind.SEASON) children != null && children > 0 && seen >= children else watched == true
    return when {
        full -> WatchedMark.Full
        seen > 0 -> WatchedMark.Half
        else -> WatchedMark.None
    }
}

val ItemDetail.watchedMark: WatchedMark get() = mark(kind, progress, watched, watchedCount, childCount)

val ItemRef.watchedMark: WatchedMark get() = mark(kind, progress, watched, watchedCount, childCount)

/** A started, not finished position (the bar and «noch …»). */
fun Progress?.started(): Progress? = this?.takeIf { !it.completed && it.positionMs > 0 }

/** Minutes left of [progress], at least 1. */
fun minutesLeft(progress: Progress): Int = maxOf(1, Math.round((progress.durationMs - progress.positionMs) / 60_000.0).toInt())

/** Played share 0..1. */
fun share(progress: Progress): Float =
    if (progress.durationMs > 0) (progress.positionMs.toFloat() / progress.durationMs).coerceIn(0f, 1f) else (progress.percent.toFloat() / 100f).coerceIn(0f, 1f)

/** The progress after setting or clearing «gesehen» (web `changed`): a finished title starts at 0 again. */
private fun progressAfter(progress: Progress?, on: Boolean, now: OffsetDateTime): Progress? = when {
    on -> Progress(
        positionMs = progress?.durationMs ?: 0,
        durationMs = progress?.durationMs ?: 0,
        percent = BigDecimal(100),
        completed = true,
        updatedAt = now,
        watchedAt = now,
        playbackRate = progress?.playbackRate,
    )
    progress == null -> null
    progress.completed -> progress.copy(watchedAt = null, completed = false, positionMs = 0, percent = BigDecimal.ZERO)
    else -> progress.copy(watchedAt = null)
}

/** [this] as the server will have it after `setItemWatched(on)`. */
fun ItemDetail.withWatched(on: Boolean, now: OffsetDateTime): ItemDetail = when {
    !isGroup(kind) -> copy(progress = progressAfter(progress, on, now))
    kind == ItemKind.SEASON -> copy(
        watchedCount = if (on) childCount ?: watchedCount else 0,
        episodes = episodes?.map { it.withWatched(on, now) },
    )
    else -> copy(watched = on, watchedCount = if (on) watchedCount else 0)
}

fun ItemRef.withWatched(on: Boolean, now: OffsetDateTime): ItemRef = when {
    !isGroup(kind) -> copy(progress = progressAfter(progress, on, now))
    kind == ItemKind.SEASON -> copy(watchedCount = if (on) childCount ?: watchedCount else 0)
    else -> copy(watched = on, watchedCount = if (on) watchedCount else 0)
}

/** What the big button starts and says (web `playTarget`, mockup `cta`). */
data class PlayTarget(
    /** The title the player opens: the item itself or the episode of a series. */
    val id: String,
    val kind: Kind,
    /** The episode of a series or season. */
    val episode: ItemRef? = null,
    /** The position the button resumes, with the bar and «noch …». */
    val resume: Progress? = null,
) {
    enum class Kind { Play, Resume, Again, Episode, EpisodeResume }
}

/**
 * The big button of a title: a series or season by `nextUp`, a film or an episode by its own
 * progress; none for audio (stage 7 brings its pages), extras and a series without episodes.
 */
fun playTarget(item: ItemDetail): PlayTarget? {
    if (isAudio(item.kind) || item.kind == ItemKind.EXTRA || item.kind == ItemKind.UNKNOWN_DEFAULT_OPEN_API) return null
    if (item.kind == ItemKind.SERIES || item.kind == ItemKind.SEASON) {
        val next = item.nextUp ?: return null
        val resume = if (next.reason == NextUp.Reason.REWATCH) null else next.episode.progress.started()
        return PlayTarget(next.episode.id, if (resume != null) PlayTarget.Kind.EpisodeResume else PlayTarget.Kind.Episode, next.episode, resume)
    }
    if (item.mediaFiles.none { it.available }) return null
    val resume = item.progress.started()
    return when {
        resume != null -> PlayTarget(item.id, PlayTarget.Kind.Resume, resume = resume)
        item.progress?.completed == true -> PlayTarget(item.id, PlayTarget.Kind.Again)
        else -> PlayTarget(item.id, PlayTarget.Kind.Play)
    }
}
