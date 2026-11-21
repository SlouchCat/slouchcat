package com.slouchcat.core.player

import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.PlaybackCapabilities
import com.slouchcat.core.api.generated.models.PlaybackInfo
import com.slouchcat.core.api.generated.models.QualityOption
import com.slouchcat.core.api.generated.models.Segment
import com.slouchcat.core.api.generated.models.Track

/** Why the player cannot play (or plays no more); each has its own picture in the app. */
sealed interface PlayerError {
    /** 404: the title is gone or no longer visible to the profile (rule 2: never «forbidden»). */
    data object NotFound : PlayerError

    /** 403 `watch_limit_reached` at the start or `stoppedBy: limit` during it (K-5). */
    data object WatchLimit : PlayerError

    /** 409 `stream_limit_reached` (W-8). */
    data object StreamLimit : PlayerError

    /** The server ended the playback: from another device, the stream overview or the administration. */
    data object StoppedElsewhere : PlayerError

    /** The session or the profile ended (logout, revoked device); the shell returns to the sign-in. */
    data object SignedOut : PlayerError

    /** No connection; [retrying]: the controller tries again by itself. */
    data class Network(val retrying: Boolean) : PlayerError

    /** 503 `transcode_busy`; [retrying]: the controller asks again by itself, else «Erneut versuchen». */
    data class Busy(val retrying: Boolean) : PlayerError

    /** The device cannot decode the picture even after the narrower start. */
    data object Unplayable : PlayerError

    /** Anything else; worth a «try again». */
    data object Failed : PlayerError
}

/** A choice of a sheet: an audio or subtitle track of the server. */
data class TrackChoice(val track: Track, val selected: Boolean)

/** The card of the next episode (W-9): [secondsLeft] counts down, `null` when it does not start by itself. */
data class Countdown(val next: ItemRef, val secondsLeft: Int?)

/** The episodes of the season for the sheet «Folgen»: `null` while loading, empty when there are none. */
data class SeasonEpisodes(val seasonNumber: Int?, val episodes: List<ItemRef>?)

/**
 * The whole state a player page draws (concept 27 8.1). The controller fills it; a page reads it
 * and calls the controller's commands. Times in milliseconds.
 */
data class PlayerState(
    /** The title that plays (a film or an episode with `seriesTitle`, `seasonNumber`, `episodeNumber`). */
    val item: ItemRef? = null,
    val info: PlaybackInfo? = null,
    /** What this start reported to the server (the info sheet names the device). */
    val capabilities: PlaybackCapabilities? = null,
    val loading: Boolean = true,
    /** A start with another track, quality or position runs. */
    val switching: Boolean = false,
    val playing: Boolean = false,
    val buffering: Boolean = false,
    val ended: Boolean = false,
    val positionMs: Long = 0,
    val durationMs: Long = 0,
    val bufferedMs: Long = 0,
    val speed: Float = 1f,
    val audio: List<TrackChoice> = emptyList(),
    val subtitles: List<TrackChoice> = emptyList(),
    /** No subtitles shown. */
    val subtitlesOff: Boolean = true,
    val qualities: List<QualityOption> = emptyList(),
    val chapters: List<Chapter> = emptyList(),
    val intro: Segment? = null,
    val outro: Segment? = null,
    /** «Vorspann überspringen»: inside the intro and the device offers it. */
    val showSkipIntro: Boolean = false,
    /** «Abspann überspringen»: inside the end credits without a next-episode card. */
    val showSkipOutro: Boolean = false,
    val previous: ItemRef? = null,
    val next: ItemRef? = null,
    val countdown: Countdown? = null,
    val trickplay: TrickplayTiles? = null,
    val episodes: SeasonEpisodes? = null,
    val videoWidth: Int? = null,
    val videoHeight: Int? = null,
    val error: PlayerError? = null,
    /** Milliseconds from [PlaybackController.start] to the first picture of this playback (the measure of stage 5). */
    val firstFrameMs: Long? = null,
) {
    val selectedAudio: Track? get() = audio.firstOrNull { it.selected }?.track
    val selectedSubtitle: Track? get() = if (subtitlesOff) null else subtitles.firstOrNull { it.selected }?.track
    val remainingMs: Long get() = (durationMs - positionMs).coerceAtLeast(0)

    /**
     * Without [info] and [trickplay]: the playback description carries the stream and subtitle
     * addresses with the token `st` (rule 10).
     */
    override fun toString() = "PlayerState(item=${item?.id}, loading=$loading, switching=$switching, playing=$playing, " +
        "ended=$ended, positionMs=$positionMs, durationMs=$durationMs, mode=${info?.mode}, error=$error, countdown=${countdown?.secondsLeft})"
    val isEpisode: Boolean get() = item?.seasonNumber != null || item?.episodeNumber != null
}
