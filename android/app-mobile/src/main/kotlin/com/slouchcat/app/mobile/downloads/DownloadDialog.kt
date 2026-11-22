package com.slouchcat.app.mobile.downloads

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.DownloadOptions
import com.slouchcat.core.api.generated.models.DownloadQuality
import com.slouchcat.core.api.generated.models.DownloadRequest
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.gutter
import com.slouchcat.core.media.data.ItemRepository
import com.slouchcat.core.offline.OfflineEngine
import com.slouchcat.core.offline.StoredDownload
import com.slouchcat.core.player.DeviceSettingsStore
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import okhttp3.HttpUrl

/** The kinds the video download offers (audiobooks and podcast episodes come with stage 7). */
fun downloadable(kind: ItemKind) = kind == ItemKind.MOVIE || kind == ItemKind.EPISODE || kind == ItemKind.SEASON || kind == ItemKind.SERIES

/** «Diese Folge», «Nächste 3 Folgen», «Ganze Staffel» (mockup `ACT.dlAsk`). */
enum class DownloadScope { One, Next, Season }

/** Where the title stands on the device (the round action of the detail page). */
enum class OnDevice { None, Loading, Complete }

data class DownloadDialogState(
    val open: Boolean = false,
    val loading: Boolean = false,
    /** The episode «Diese Folge» names (the next one of a series), the episodes of «Nächste 3», the season. */
    val episode: ItemRef? = null,
    val next: List<ItemRef> = emptyList(),
    val season: ItemRef? = null,
    val seasonEpisodes: Int = 0,
    val scope: DownloadScope = DownloadScope.One,
    val scopes: List<DownloadScope> = emptyList(),
    /** Options per scope (sizes summed over the episodes of «Nächste 3»). */
    val options: Map<DownloadScope, DownloadOptions> = emptyMap(),
    val quality: DownloadQuality = DownloadQuality.HIGH,
    val audioTrack: Int? = null,
    val free: Long = 0,
    val wifiOnly: Boolean = false,
    val already: Boolean = false,
    val starting: Boolean = false,
    val failed: Boolean = false,
    val deviceRequired: Boolean = false,
) {
    val current: DownloadOptions? get() = options[scope]
    val size: Long get() = current?.qualities?.firstOrNull { it.quality == quality }?.estimatedBytes ?: 0
    val tooLittle: Boolean get() = size > free
}

sealed interface DialogMessage {
    data class Started(val title: String, val count: Int) : DialogMessage
}

/**
 * The download dialog of a detail page (9.2 point 1, mockup photos handy 45 and 46): scope for an
 * episode or series, the three qualities with the estimated size (`getDownloadOptions`), the free
 * space of the device and a warning when it is too little (9.4), Wi-Fi or not, then
 * `createDownloads` (a season in one action).
 */
class DownloadDialogViewModel(
    private val engine: OfflineEngine,
    private val items: ItemRepository,
    private val settings: DeviceSettingsStore,
    private val server: () -> HttpUrl?,
    private val itemId: String,
) : ViewModel() {
    private val _state = MutableStateFlow(DownloadDialogState())
    val state: StateFlow<DownloadDialogState> = _state.asStateFlow()

    private val _messages = MutableStateFlow<DialogMessage?>(null)
    val messages: StateFlow<DialogMessage?> = _messages.asStateFlow()

    /** The title's state on the device, from the stored downloads. */
    val onDevice: StateFlow<OnDevice> = engine.downloads.map { list -> status(list) }.stateIn(viewModelScope, SharingStarted.Eagerly, OnDevice.None)

    private fun status(list: List<StoredDownload>): OnDevice {
        val mine = list.filter { d -> !d.pendingDelete && d.item.let { it?.id == itemId || it?.parentId == itemId || it?.seriesRef?.id == itemId } }
        return when {
            mine.isEmpty() -> OnDevice.None
            mine.all { it.complete } -> OnDevice.Complete
            else -> OnDevice.Loading
        }
    }

    fun consumeMessage() {
        _messages.value = null
    }

    fun open(item: ItemDetail) {
        _state.value = DownloadDialogState(open = true, loading = true)
        viewModelScope.launch {
            val base = server() ?: return@launch _state.update { it.copy(loading = false, failed = true) }
            val device = settings.settings.first()
            val free = engine.files.freeBytes()
            var episode: ItemRef? = null
            var siblings: List<ItemRef> = emptyList()
            var season: ItemRef? = null
            var count = 0
            when (item.kind) {
                ItemKind.EPISODE -> {
                    episode = item.toRef()
                    val parent = item.parentId?.let { (items.item(base, it) as? ApiResult.Ok)?.value }
                    season = parent?.toRef() ?: item.season
                    siblings = parent?.episodes.orEmpty()
                    count = parent?.episodes?.size ?: parent?.episodeCount ?: 0
                }
                ItemKind.SERIES -> {
                    episode = item.nextEpisode ?: item.nextUp?.episode
                    val seasonId = episode?.parentId ?: item.seasons?.firstOrNull()?.id
                    val parent = seasonId?.let { (items.item(base, it) as? ApiResult.Ok)?.value }
                    season = parent?.toRef()
                    siblings = parent?.episodes.orEmpty()
                    if (episode == null) episode = siblings.firstOrNull()
                    count = parent?.episodes?.size ?: 0
                }
                ItemKind.SEASON -> {
                    season = item.toRef()
                    count = item.episodes?.size ?: item.episodeCount ?: 0
                }
                else -> Unit
            }
            val next = episode?.let { e -> siblings.dropWhile { it.id != e.id }.take(NEXT) }.orEmpty()
            val scopes = buildList {
                if (item.kind == ItemKind.MOVIE || episode != null) add(DownloadScope.One)
                if (next.size > 1) add(DownloadScope.Next)
                if (season != null) add(DownloadScope.Season)
            }
            val options = mutableMapOf<DownloadScope, DownloadOptions>()
            val one = if (item.kind == ItemKind.MOVIE) item.id else episode?.id
            var refused = false
            fun keep(result: ApiResult<DownloadOptions>): DownloadOptions? {
                if (result is ApiResult.NotAllowed) refused = true
                return (result as? ApiResult.Ok)?.value
            }
            one?.let { id -> keep(engine.options(id))?.let { options[DownloadScope.One] = it } }
            if (DownloadScope.Next in scopes) {
                val each = next.mapNotNull { keep(engine.options(it.id)) }
                if (each.size == next.size) options[DownloadScope.Next] = sum(each)
            }
            season?.let { s -> keep(engine.options(s.id))?.let { options[DownloadScope.Season] = it } }
            val available = scopes.filter { it in options }
            val first = available.firstOrNull()
            _state.value = DownloadDialogState(
                open = true,
                loading = false,
                episode = episode,
                next = next,
                season = season,
                seasonEpisodes = options[DownloadScope.Season]?.episodeCount ?: count,
                scope = first ?: DownloadScope.One,
                scopes = available,
                options = options,
                quality = DownloadQuality.HIGH,
                audioTrack = options[first]?.audioTracks?.firstOrNull { it.default }?.index,
                free = free,
                wifiOnly = device.downloadsWifiOnly,
                already = first == DownloadScope.One && options[first]?.existing != null,
                failed = available.isEmpty() && !refused,
                deviceRequired = refused,
            )
        }
    }

    private fun sum(each: List<DownloadOptions>): DownloadOptions = each.first().copy(
        qualities = each.first().qualities.map { q -> q.copy(estimatedBytes = each.sumOf { o -> o.qualities.firstOrNull { it.quality == q.quality }?.estimatedBytes ?: 0 }) },
        existing = null,
    )

    fun onScope(scope: DownloadScope) = _state.update { it.copy(scope = scope, already = scope == DownloadScope.One && it.options[scope]?.existing != null) }

    fun onQuality(quality: DownloadQuality) = _state.update { it.copy(quality = quality) }

    fun onAudio(index: Int) = _state.update { it.copy(audioTrack = index) }

    fun onDismiss() {
        if (!_state.value.starting) _state.value = DownloadDialogState()
    }

    /** `createDownloads` for the chosen scope; the fetch starts by itself (9.3). */
    fun onStart(fallbackTitle: String) {
        val s = _state.value
        if (s.starting || s.tooLittle || s.current == null) return
        _state.update { it.copy(starting = true, failed = false) }
        viewModelScope.launch {
            val targets = when (s.scope) {
                DownloadScope.One -> listOfNotNull(s.episode?.id ?: itemId.takeIf { s.episode == null })
                DownloadScope.Next -> s.next.map { it.id }
                DownloadScope.Season -> listOfNotNull(s.season?.id)
            }
            var created = 0
            var failed = false
            for (target in targets) {
                val request = DownloadRequest(quality = s.quality, audioTrack = s.audioTrack.takeIf { s.scope == DownloadScope.One })
                when (val result = engine.create(target, request)) {
                    is ApiResult.Ok -> created += result.value.downloads.size
                    is ApiResult.NotAllowed -> {
                        failed = true
                        _state.update { it.copy(deviceRequired = result.code == DEVICE_REQUIRED) }
                    }
                    else -> failed = true
                }
            }
            if (created > 0) engine.scheduler.startTransfer()
            if (failed && created == 0) {
                _state.update { it.copy(starting = false, failed = true) }
            } else {
                val title = s.episode?.takeIf { s.scope == DownloadScope.One }?.title ?: fallbackTitle
                _messages.value = DialogMessage.Started(title, created)
                _state.value = DownloadDialogState()
            }
        }
    }

    private fun ItemDetail.toRef() = ItemRef(
        id = id, kind = kind, title = title, inWatchlist = inWatchlist, images = images, year = year, parentId = parentId,
        seasonNumber = seasonNumber, episodeNumber = episodeNumber, seriesTitle = seriesTitle, seriesRef = seriesRef, childCount = childCount,
    )

    private companion object {
        const val NEXT = 3
        const val DEVICE_REQUIRED = "device_required"
    }
}

/** The sheet itself (mockup `mdDlSheet`). */
@Composable
fun DownloadDialog(state: DownloadDialogState, title: String, subtitle: String?, vm: DownloadDialogViewModel, onOpenDownloads: (() -> Unit)? = null) {
    if (!state.open) return
    val c = SlouchcatTheme.colors
    val heading = stringResource(R.string.app_downloads_dialogTitle)
    val notify = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { vm.onStart(title) }
    val context = LocalContext.current
    SlouchcatSheet(heading, onDismiss = vm::onDismiss) {
        SheetTitle(heading)
        Column(Modifier.padding(horizontal = gutter(), vertical = 4.dp)) {
            Text(title, style = SlouchcatType.strong, color = c.tx)
            if (subtitle != null) Text(subtitle, style = SlouchcatType.caption, color = c.tx2)
        }
        when {
            state.loading -> Text(stringResource(R.string.common_loading), style = SlouchcatType.body, color = c.tx2, modifier = Modifier.padding(gutter()))
            state.deviceRequired -> Note(stringResource(R.string.app_downloads_deviceRequired), warning = true, modifier = Modifier.padding(gutter()))
            state.scopes.isEmpty() -> Note(stringResource(R.string.app_downloads_optionsFailed), warning = true, modifier = Modifier.padding(gutter()))
            else -> Body(state, vm)
        }
        if (state.failed && state.scopes.isNotEmpty()) Note(stringResource(R.string.app_downloads_failed), warning = true, modifier = Modifier.padding(gutter()))
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = vm::onDismiss, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
            if (state.scopes.isNotEmpty() && !state.already) {
                SlouchcatButton(
                    stringResource(if (state.starting) R.string.app_downloads_starting else R.string.app_downloads_start),
                    onClick = {
                        val ask = Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
                            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
                        if (ask) notify.launch(Manifest.permission.POST_NOTIFICATIONS) else vm.onStart(title)
                    },
                    kind = ButtonKind.Accent,
                    leadingIcon = SlouchcatIcon.Download,
                    busy = state.starting,
                    enabled = !state.tooLittle && !state.loading,
                    modifier = Modifier.weight(1f),
                )
            } else if (state.already && onOpenDownloads != null) {
                SlouchcatButton(
                    stringResource(R.string.app_downloads_toDownloads),
                    onClick = {
                        vm.onDismiss()
                        onOpenDownloads()
                    },
                    modifier = Modifier.weight(1f),
                )
            }
        }
    }
}

@Composable
private fun Body(state: DownloadDialogState, vm: DownloadDialogViewModel) {
    val c = SlouchcatTheme.colors
    if (state.scopes.size > 1 || state.scopes.firstOrNull() != DownloadScope.One) {
        Heading(stringResource(R.string.app_downloads_scope))
        Column(Modifier.selectableGroup()) {
            for (scope in state.scopes) {
                val (label, detail) = when (scope) {
                    DownloadScope.One -> stringResource(R.string.app_downloads_scopeOne) to state.episode?.title
                    DownloadScope.Next -> stringResource(R.string.app_downloads_scopeNext) to stringResource(
                        R.string.app_downloads_scopeNextRange,
                        (state.next.firstOrNull()?.episodeNumber ?: 1).toString(),
                        (state.next.lastOrNull()?.episodeNumber ?: 1).toString(),
                    )
                    DownloadScope.Season -> stringResource(R.string.app_downloads_scopeSeason) to pluralStringResource(
                        R.plurals.app_downloads_scopeSeasonSize,
                        state.seasonEpisodes,
                        state.seasonEpisodes,
                        (state.season?.seasonNumber ?: 1).toString(),
                    )
                }
                RadioRow(label, null, state.scope == scope, detail = detail) { vm.onScope(scope) }
            }
        }
    }
    val options = state.current ?: return
    Heading(stringResource(R.string.offline_dialog_quality))
    Column(Modifier.selectableGroup()) {
        for (q in options.qualities.sortedBy { it.quality.ordinal }) {
            val audio = options.mediaFileId == null && q.height == 0
            val label = if (audio) {
                stringResource(R.string.app_downloads_qualityAudio, qualityName(q.quality), q.audioBitrate.toString())
            } else {
                stringResource(R.string.app_downloads_qualityOption, qualityName(q.quality), q.height.toString())
            }
            RadioRow(label, if (state.already) null else fileSize(q.estimatedBytes), state.quality == q.quality) { vm.onQuality(q.quality) }
        }
    }
    if (state.scope == DownloadScope.One && options.audioTracks.size > 1) {
        Heading(stringResource(R.string.offline_dialog_audio))
        Column(Modifier.selectableGroup()) {
            for (track in options.audioTracks) {
                RadioRow(listOfNotNull(track.title, track.language).joinToString(" · ").ifEmpty { track.index.toString() }, null, state.audioTrack == track.index) { vm.onAudio(track.index) }
            }
        }
    }
    val subtitles = options.subtitles.mapNotNull { it.language ?: it.title }.distinct()
    Info(SlouchcatIcon.Subs, if (subtitles.isEmpty()) stringResource(R.string.offline_dialog_noSubtitles) else stringResource(R.string.offline_dialog_subtitles, subtitles.joinToString(", ")))
    val free = fileSize(state.free)
    when {
        state.already -> Info(SlouchcatIcon.Check, stringResource(R.string.app_downloads_already))
        state.tooLittle -> Info(SlouchcatIcon.Alert, stringResource(R.string.app_downloads_tooLittle, fileSize(state.size), free), warn = true)
        else -> Info(SlouchcatIcon.Storage, stringResource(R.string.app_downloads_freeAfter, free, fileSize(state.free - state.size)))
    }
    Info(SlouchcatIcon.Wifi, stringResource(if (state.wifiOnly) R.string.app_downloads_loadsWifi else R.string.app_downloads_loadsAny))
}

@Composable
private fun Heading(text: String) {
    Text(
        text.uppercase(),
        style = SlouchcatType.section,
        color = SlouchcatTheme.colors.tx3,
        modifier = Modifier.padding(start = gutter(), end = gutter(), top = 14.dp, bottom = 6.dp).semantics { heading() },
    )
}

@Composable
private fun Info(icon: SlouchcatIcon, text: String, warn: Boolean = false) {
    val c = SlouchcatTheme.colors
    Row(Modifier.fillMaxWidth().padding(horizontal = gutter(), vertical = 6.dp), horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.Top) {
        SlouchcatIconImage(icon, contentDescription = null, size = 20.dp, tint = if (warn) c.warn else c.tx3)
        Text(text, style = SlouchcatType.bodySmall, color = if (warn) c.warn else c.tx2)
    }
}

/** The round action on the detail page: «Herunterladen», «Wird geladen», «Auf dem Gerät». */
@Composable
fun DownloadActionLabel(onDevice: OnDevice): Pair<SlouchcatIcon, String> = when (onDevice) {
    OnDevice.None -> SlouchcatIcon.Download to stringResource(R.string.app_downloads_actionDownload)
    OnDevice.Loading -> SlouchcatIcon.Download to stringResource(R.string.app_downloads_actionLoading)
    OnDevice.Complete -> SlouchcatIcon.Check to stringResource(R.string.app_downloads_actionOnDevice)
}

/** Shows [DialogMessage]s once in [show]. */
@Composable
fun DialogMessages(vm: DownloadDialogViewModel, show: (DialogMessage) -> Unit) {
    val message by vm.messages.collectAsStateWithLifecycle()
    LaunchedEffect(message) {
        message?.let(show)
        vm.consumeMessage()
    }
}
