package com.slouchcat.core.player

import android.content.Context
import android.net.Uri
import android.os.Handler
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.Format
import androidx.media3.common.MediaItem
import androidx.media3.common.MediaMetadata
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.PlaybackParameters
import androidx.media3.common.Player
import androidx.media3.common.TrackSelectionOverride
import androidx.media3.common.Tracks
import androidx.media3.common.VideoSize
import androidx.annotation.OptIn
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.HttpDataSource
import androidx.media3.datasource.okhttp.OkHttpDataSource
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ExoPlaybackException
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.Renderer
import androidx.media3.exoplayer.audio.AudioRendererEventListener
import androidx.media3.exoplayer.audio.AudioSink
import androidx.media3.exoplayer.audio.MediaCodecAudioRenderer
import androidx.media3.exoplayer.mediacodec.MediaCodecInfo
import androidx.media3.exoplayer.mediacodec.MediaCodecSelector
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.session.MediaSession
import com.google.common.util.concurrent.Futures
import com.google.common.util.concurrent.ListenableFuture
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import okhttp3.Call
import java.util.concurrent.atomic.AtomicInteger

/**
 * [PlayerEngine] with Media3's ExoPlayer over the shared OkHttp client of the app (concept 27 5.2:
 * the same headers and the gate «Server weist sich aus» for every media request). Direct play and
 * HLS (fMP4, remux and transcode) go through one [DefaultMediaSourceFactory]; a WebVTT file of the
 * server comes as a side-loaded subtitle.
 *
 * The [MediaSession] makes headphone and Bluetooth buttons work while the page is in front; the
 * player pauses when a headphone is unplugged and takes the audio focus. Stage 7 lifts [player]
 * into a `MediaSessionService` for sound in the background (then without [withSession]).
 */
@OptIn(UnstableApi::class)
class Media3Engine(
    context: Context,
    http: Call.Factory,
    withSession: Boolean = true,
) : PlayerEngine {
    private val signals = MutableSharedFlow<EngineSignal>(extraBufferCapacity = 64)
    override val events: Flow<EngineSignal> = signals.asSharedFlow()

    /** The number of the current load; [stop] sets [PlayerEngine.NO_LOAD], so its own events belong to no load. */
    private var current = PlayerEngine.NO_LOAD

    private fun emit(event: EngineEvent) {
        signals.tryEmit(EngineSignal(current, event))
    }

    /** The Media3 player: for the surface of a page and the media session of stage 7. */
    val player: ExoPlayer = ExoPlayer.Builder(context.applicationContext, SlouchcatRenderersFactory(context.applicationContext))
        .setMediaSourceFactory(DefaultMediaSourceFactory(OkHttpDataSource.Factory(http)))
        .setAudioAttributes(
            AudioAttributes.Builder().setUsage(C.USAGE_MEDIA).setContentType(C.AUDIO_CONTENT_TYPE_MOVIE).build(),
            /* handleAudioFocus = */ true,
        )
        .setHandleAudioBecomingNoisy(true)
        .build()

    private val session: MediaSession? = if (withSession) {
        MediaSession.Builder(context.applicationContext, player)
            .setId("slouchcat-video-${SESSIONS.incrementAndGet()}")
            .setCallback(TrustedControllers(context.applicationContext.packageName))
            .build()
    } else {
        null
    }

    private var lastTracks: Tracks = Tracks.EMPTY

    init {
        Media3Logs.install()
        player.addListener(
            object : Player.Listener {
                override fun onTracksChanged(tracks: Tracks) {
                    lastTracks = tracks
                    emit(EngineEvent.Tracks(groups(tracks, C.TRACK_TYPE_AUDIO), groups(tracks, C.TRACK_TYPE_TEXT)))
                }

                override fun onRenderedFirstFrame() {
                    emit(EngineEvent.FirstFrame)
                }

                override fun onIsPlayingChanged(isPlaying: Boolean) {
                    emit(EngineEvent.Playing(isPlaying))
                }

                override fun onPlayWhenReadyChanged(playWhenReady: Boolean, reason: Int) {
                    emit(EngineEvent.WantsToPlay(playWhenReady))
                }

                override fun onPlaybackStateChanged(playbackState: Int) {
                    when (playbackState) {
                        Player.STATE_BUFFERING -> emit(EngineEvent.Buffering(true))
                        Player.STATE_READY -> emit(EngineEvent.Buffering(false))
                        Player.STATE_ENDED -> emit(EngineEvent.Ended)
                        else -> Unit
                    }
                }

                override fun onVideoSizeChanged(videoSize: VideoSize) {
                    if (videoSize.width > 0) emit(EngineEvent.VideoSize(videoSize.width, videoSize.height))
                }

                override fun onPlayerError(error: PlaybackException) {
                    emit(failure(error))
                }
            },
        )
    }

    override val positionMs: Long get() = player.currentPosition
    override val durationMs: Long get() = player.duration.takeIf { it != C.TIME_UNSET } ?: 0
    override val bufferedMs: Long get() = player.bufferedPosition

    override fun load(media: EngineMedia, startMs: Long, play: Boolean, load: Long) {
        current = load
        val item = MediaItem.Builder()
            .setUri(media.url)
            .setMimeType(if (media.hls) MimeTypes.APPLICATION_M3U8 else null)
            .setMediaMetadata(MediaMetadata.Builder().setTitle(media.title).setSubtitle(media.subtitle).build())
            .setSubtitleConfigurations(
                listOfNotNull(
                    media.subtitleUrl?.let {
                        MediaItem.SubtitleConfiguration.Builder(Uri.parse(it))
                            .setId(SIDELOADED)
                            .setMimeType(MimeTypes.TEXT_VTT)
                            .setLanguage(media.subtitleLanguage)
                            .build()
                    },
                ),
            )
            .build()
        // A new start chooses its tracks again (the server's choice comes with the next Tracks).
        player.trackSelectionParameters = player.trackSelectionParameters.buildUpon()
            .clearOverrides()
            .setTrackTypeDisabled(C.TRACK_TYPE_TEXT, false)
            .build()
        player.setMediaItem(item, startMs.coerceAtLeast(0))
        player.playWhenReady = play
        player.prepare()
    }

    override fun play() {
        if (player.playbackState == Player.STATE_IDLE) player.prepare()
        player.play()
    }

    override fun pause() = player.pause()

    override fun seekTo(positionMs: Long) = player.seekTo(positionMs)

    override fun setSpeed(speed: Float) {
        player.playbackParameters = PlaybackParameters(speed)
    }

    override fun selectAudio(index: Int) {
        val group = typeGroups(lastTracks, C.TRACK_TYPE_AUDIO).getOrNull(index) ?: return
        player.trackSelectionParameters = player.trackSelectionParameters.buildUpon()
            .setOverrideForType(TrackSelectionOverride(group.mediaTrackGroup, 0))
            .build()
    }

    override fun selectText(index: Int?) {
        val group = index?.let { typeGroups(lastTracks, C.TRACK_TYPE_TEXT).getOrNull(it) }
        val params = player.trackSelectionParameters.buildUpon()
        if (group == null) {
            params.clearOverridesOfType(C.TRACK_TYPE_TEXT).setTrackTypeDisabled(C.TRACK_TYPE_TEXT, true)
        } else {
            params.setTrackTypeDisabled(C.TRACK_TYPE_TEXT, false).setOverrideForType(TrackSelectionOverride(group.mediaTrackGroup, 0))
        }
        player.trackSelectionParameters = params.build()
    }

    override fun stop() {
        current = PlayerEngine.NO_LOAD
        player.stop()
        player.clearMediaItems()
        lastTracks = Tracks.EMPTY
    }

    override fun release() {
        session?.release()
        player.release()
    }

    private fun typeGroups(tracks: Tracks, type: Int): List<Tracks.Group> = tracks.groups.filter { it.type == type }

    private fun groups(tracks: Tracks, type: Int): List<EngineTrack> = typeGroups(tracks, type).map { group ->
        val format = group.getTrackFormat(0)
        EngineTrack(
            language = format.language,
            label = format.label,
            mime = format.sampleMimeType,
            channels = format.channelCount.takeIf { it != Format.NO_VALUE } ?: 0,
            supported = group.isTrackSupported(0),
            selected = group.isSelected,
            sideloaded = format.id?.contains(SIDELOADED) == true,
        )
    }

    private fun failure(error: PlaybackException): EngineEvent.Failed {
        val status = generateSequence(error.cause) { it.cause }.filterIsInstance<HttpDataSource.InvalidResponseCodeException>().firstOrNull()?.responseCode
        val rendererMime = (error as? ExoPlaybackException)?.rendererFormat?.sampleMimeType
        val failure = when (error.errorCode) {
            PlaybackException.ERROR_CODE_IO_NETWORK_CONNECTION_FAILED,
            PlaybackException.ERROR_CODE_IO_NETWORK_CONNECTION_TIMEOUT,
            PlaybackException.ERROR_CODE_IO_UNSPECIFIED,
            PlaybackException.ERROR_CODE_TIMEOUT,
            -> EngineFailure.Network
            PlaybackException.ERROR_CODE_IO_BAD_HTTP_STATUS -> EngineFailure.Refused
            PlaybackException.ERROR_CODE_DECODER_INIT_FAILED,
            PlaybackException.ERROR_CODE_DECODER_QUERY_FAILED,
            PlaybackException.ERROR_CODE_DECODING_FAILED,
            PlaybackException.ERROR_CODE_DECODING_FORMAT_EXCEEDS_CAPABILITIES,
            PlaybackException.ERROR_CODE_DECODING_FORMAT_UNSUPPORTED,
            -> if (MimeTypes.isAudio(rendererMime)) EngineFailure.AudioDecoder else EngineFailure.VideoDecoder
            PlaybackException.ERROR_CODE_AUDIO_TRACK_INIT_FAILED,
            PlaybackException.ERROR_CODE_AUDIO_TRACK_WRITE_FAILED,
            -> EngineFailure.AudioDecoder
            PlaybackException.ERROR_CODE_PARSING_CONTAINER_UNSUPPORTED,
            PlaybackException.ERROR_CODE_PARSING_CONTAINER_MALFORMED,
            -> EngineFailure.Container
            else -> EngineFailure.Other
        }
        return EngineEvent.Failed(failure, status)
    }

    private companion object {
        const val SIDELOADED = "slouchcat-subtitle"
        val SESSIONS = AtomicInteger()
    }
}

/**
 * Who may control the player's [MediaSession]: this app, the system's media notification and the
 * trusted controllers of the system (headphone and Bluetooth buttons, the system UI). Another app
 * with access to notifications is turned away. Accepted controllers get the player commands without
 * setting or changing the media: the stream addresses come only from the controller of the app.
 */
@OptIn(UnstableApi::class)
internal class TrustedControllers(private val ownPackage: String) : MediaSession.Callback {
    override fun onConnect(session: MediaSession, controller: MediaSession.ControllerInfo): MediaSession.ConnectionResult {
        if (!accepts(ownPackage, controller.packageName, session.isMediaNotificationController(controller), controller.isTrusted)) {
            return MediaSession.ConnectionResult.reject()
        }
        return MediaSession.ConnectionResult.AcceptedResultBuilder(session).setAvailablePlayerCommands(COMMANDS).build()
    }

    override fun onAddMediaItems(
        mediaSession: MediaSession,
        controller: MediaSession.ControllerInfo,
        mediaItems: MutableList<MediaItem>,
    ): ListenableFuture<MutableList<MediaItem>> = Futures.immediateFailedFuture(UnsupportedOperationException("no media from controllers"))

    companion object {
        fun accepts(ownPackage: String, packageName: String, notification: Boolean, trusted: Boolean): Boolean =
            packageName == ownPackage || notification || trusted

        /** Play, pause, seek and the like; no command that sets or changes the media items. */
        val COMMANDS: Player.Commands = MediaSession.ConnectionResult.DEFAULT_PLAYER_COMMANDS.buildUpon()
            .remove(Player.COMMAND_SET_MEDIA_ITEM)
            .remove(Player.COMMAND_CHANGE_MEDIA_ITEMS)
            .build()
    }
}

/**
 * Media3 writes its own messages straight to logcat; a parser error may quote a line of a playlist
 * with the stream token `st`. They go through the app's [Log] instead, which scrubs it (rule 10);
 * debug and info lines are dropped.
 */
@OptIn(UnstableApi::class)
object Media3Logs : androidx.media3.common.util.Log.Logger {
    @Volatile
    private var installed = false

    fun install() {
        if (installed) return
        installed = true
        androidx.media3.common.util.Log.setLogger(this)
    }

    override fun d(tag: String, message: String, throwable: Throwable?) = Unit

    override fun i(tag: String, message: String, throwable: Throwable?) = Unit

    override fun w(tag: String, message: String, throwable: Throwable?) = Log.w("media3/$tag", message, error = throwable)

    override fun e(tag: String, message: String, throwable: Throwable?) = Log.e("media3/$tag", message, error = throwable)
}

/**
 * Media3 1.11.1 leaves the input buffer of FLAC at the decoder's default (32 KiB); 5.1 FLAC frames
 * are larger (concept 26 stage 0: «Buffer too small»). This factory raises it for FLAC.
 */
@OptIn(UnstableApi::class)
class SlouchcatRenderersFactory(context: Context) : DefaultRenderersFactory(context) {
    override fun buildAudioRenderers(
        context: Context,
        extensionRendererMode: Int,
        mediaCodecSelector: MediaCodecSelector,
        enableDecoderFallback: Boolean,
        audioSink: AudioSink,
        eventHandler: Handler,
        eventListener: AudioRendererEventListener,
        out: ArrayList<Renderer>,
    ) {
        out += object : MediaCodecAudioRenderer(context, codecAdapterFactory, mediaCodecSelector, enableDecoderFallback, eventHandler, eventListener, audioSink) {
            override fun getCodecMaxInputSize(codecInfo: MediaCodecInfo, format: Format, streamFormats: Array<out Format>): Int {
                val base = super.getCodecMaxInputSize(codecInfo, format, streamFormats)
                return if (format.sampleMimeType == MimeTypes.AUDIO_FLAC) maxOf(base, FLAC_MAX_INPUT) else base
            }
        }
    }

    private companion object {
        /** FLAC frame upper bound for 8 channels, 24 bit, 4608 samples (uncompressed worst case). */
        const val FLAC_MAX_INPUT = 8 * 3 * 4608 + 1024
    }
}
