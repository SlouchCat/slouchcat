package com.slouchcat.app.tv.testing

import com.slouchcat.app.tv.audio.AudioFeature
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.browse.testing.chapters
import com.slouchcat.core.player.AudioPlayback
import com.slouchcat.core.player.AudioSource
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.EngineKind
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.player.testing.FakeEngine
import com.slouchcat.core.player.testing.FakeRadioServer
import com.slouchcat.core.player.testing.FakeServer
import com.slouchcat.core.player.testing.audioInfo

/** The real features without the sound of stage 4 (tests bring [TestAudio.feature]). */
val withoutAudio: List<TvFeature> = com.slouchcat.app.tv.shell.features.filterNot { it is AudioFeature }

/**
 * The sound of the TV tests: [AudioFeature] on the attrappes of `:core:player-testing`. [titles]
 * answers every title as an audiobook with [chapters] (an id in [episodes] as an episode of a
 * podcast); [radio] the radio; [engines] every player made, with its kind.
 */
class TestAudio(val chapters: List<Chapter> = chapters(count = 4, minutes = 10), val episodes: Map<String, ItemRef> = emptyMap()) {
    val engines = mutableListOf<Pair<EngineKind, FakeEngine>>()
    val length: Long = chapters.lastOrNull()?.endMs ?: DEFAULT_LENGTH
    val titles = FakeServer().apply {
        answer = { id, request ->
            val info = audioInfo(id, resumeMs = request.positionMs ?: 0, durationMs = length, chapters = if (id in episodes) emptyList() else chapters)
            ApiResult.Ok(episodes[id]?.let { episode -> info.copy(item = episode) } ?: info)
        }
    }
    val radio = FakeRadioServer()
    val feature = AudioFeature(
        playback = { _, _, scope ->
            AudioPlayback(
                scope = scope,
                engines = { kind -> FakeEngine { 0L }.also { it.duration = length; engines += kind to it } },
                radioServer = { radio },
                capabilities = { CapabilityRules.build(EMULATOR_TV, narrowing = it) },
                settings = MemoryDeviceSettingsStore().settings,
                clock = { 0L },
            )
        },
        source = { AudioSource("online") { titles } },
    )

    /** The player made last. */
    val engine: FakeEngine get() = engines.last().second

    private companion object {
        const val DEFAULT_LENGTH = 600_000L
    }
}

/** An episode as the player gets it: kind, podcast and title. */
fun playedEpisode(id: String, title: String, podcast: String, parent: String) =
    com.slouchcat.core.player.testing.ref(id, title = title, kind = ItemKind.PODCAST_EPISODE, parent = parent).copy(seriesTitle = podcast)
