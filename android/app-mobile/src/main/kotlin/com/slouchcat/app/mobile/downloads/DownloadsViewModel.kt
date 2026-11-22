package com.slouchcat.app.mobile.downloads

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.DownloadQuality
import com.slouchcat.core.api.generated.models.DownloadRules
import com.slouchcat.core.api.generated.models.DownloadStateValue
import com.slouchcat.core.offline.Contact
import com.slouchcat.core.offline.FetchError
import com.slouchcat.core.offline.FetchProgress
import com.slouchcat.core.offline.NetworkKind
import com.slouchcat.core.offline.OfflineEngine
import com.slouchcat.core.offline.OfflinePin
import com.slouchcat.core.offline.PinCheck
import com.slouchcat.core.offline.StoredDownload
import com.slouchcat.core.offline.StoredProfile
import com.slouchcat.core.offline.downloadsForProfile
import com.slouchcat.core.offline.offlineProfiles
import com.slouchcat.core.offline.waitsForWifi
import com.slouchcat.core.player.DeviceSettingsStore
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.io.File

/** Where a row of «Downloads» stands (mockup `/downloads`, the PWA's offline.state). */
sealed interface RowState {
    data class Fetching(val percent: Int, val bytes: Long, val total: Long) : RowState

    /** Paused on this device; [percent] of the file is here. */
    data class Paused(val percent: Int) : RowState

    data class Preparing(val percent: Int) : RowState

    data object Queued : RowState

    data object BudgetFull : RowState

    data object ServerPaused : RowState

    data object WaitsForWifi : RowState

    data object Failed : RowState

    /** A fetch error of the device or the way to the server ([FetchError]). */
    data class Error(val code: String) : RowState

    data object Done : RowState
}

/** The three groups of the mockup: «Wird geladen», «Wartet», «Fertig». */
enum class Section { Active, Waiting, Done }

data class DownloadRowUi(
    val id: String,
    val itemId: String,
    /** The title, for an episode «S1 · F2 · Titel». */
    val title: String,
    /** The series an episode belongs to (the group), its id and title. */
    val groupId: String?,
    val groupTitle: String?,
    val quality: DownloadQuality,
    /** Bytes of the whole file (prepared size or estimate). */
    val size: Long,
    val state: RowState,
    val auto: Boolean,
    val poster: File?,
    val season: Int?,
    val episode: Int?,
    /** An audiobook or a podcast episode: it plays in the audio player (stage 7). */
    val audio: Boolean = false,
) {
    val section: Section get() = when (state) {
        RowState.Done -> Section.Done
        is RowState.Fetching, is RowState.Paused, is RowState.Error -> Section.Active
        else -> Section.Waiting
    }
    val playable: Boolean get() = state == RowState.Done
}

data class Storage(val downloads: Long, val free: Long, val total: Long)

/** The rule of auto-downloads of the active profile (9.6) as the form edits it. */
data class RuleForm(val rules: DownloadRules, val saving: Boolean = false, val saved: Boolean = false, val failed: Boolean = false)

/** The offline PIN pad of the offline library (9.5). */
data class OfflinePinUi(
    val profile: StoredProfile,
    val digits: String = "",
    val checking: Boolean = false,
    val wrong: Boolean = false,
    val lockedSeconds: Long = 0,
    val unavailable: Boolean = false,
) {
    val offlinePin: Boolean get() = profile.snapshot.offlinePin != null
    override fun toString() = "OfflinePinUi(profile=${profile.id}, digits=${digits.length})"
}

enum class DownloadsSheet { Delete, Profiles, Pin }

data class DownloadsUiState(
    val rows: List<DownloadRowUi> = emptyList(),
    val loaded: Boolean = false,
    val offline: Boolean = false,
    val storage: Storage? = null,
    /** The profile whose downloads the page shows. */
    val profileName: String? = null,
    val viewerIsSession: Boolean = true,
    /** The rule of the profile on this device is on (from the last sync; the fold's second line). */
    val ruleOn: Boolean = false,
    /** Profiles that may open the offline library (for «Profil wechseln» offline). */
    val offlineProfiles: List<StoredProfile> = emptyList(),
    val wifiOnly: Boolean = false,
    val autoOpen: Boolean = false,
    val settingsOpen: Boolean = false,
    val rule: RuleForm? = null,
    val ruleFailed: Boolean = false,
    val sheet: DownloadsSheet? = null,
    /** The rows a delete would remove (one, or a group of episodes). */
    val deleting: List<DownloadRowUi> = emptyList(),
    val pin: OfflinePinUi? = null,
) {
    fun section(section: Section) = rows.filter { it.section == section }
}

sealed interface DownloadsMessage {
    data object Paused : DownloadsMessage

    data object Resumed : DownloadsMessage

    data class Deleted(val count: Int) : DownloadsMessage

    data object RuleSaved : DownloadsMessage
}

interface DownloadsEvents {
    fun onPause(id: String)
    fun onResume(id: String)
    fun onDeleteAsk(rows: List<DownloadRowUi>)
    fun onDeleteConfirm()
    fun onDismissSheet()
    fun onToggleAuto()
    fun onToggleSettings()
    fun onWifiOnly(on: Boolean)
    fun onRule(change: (DownloadRules) -> DownloadRules)
    fun onSaveRule()
    fun onSwitchProfile()
    fun onPickProfile(profile: StoredProfile)
    fun onPinDigit(digit: Char)
    fun onPinDelete()
    fun onPinSubmit()
}

private data class Local(
    val autoOpen: Boolean = false,
    val settingsOpen: Boolean = false,
    val rule: RuleForm? = null,
    val ruleFailed: Boolean = false,
    val sheet: DownloadsSheet? = null,
    val deleting: List<DownloadRowUi> = emptyList(),
    val pin: OfflinePinUi? = null,
)

/**
 * «Downloads» (mockup `/downloads`, concept 27 9.2 to 9.6): the downloads of the active profile (or
 * the profile opened offline with its PIN) by section with progress, pause, resume and delete, the
 * storage of the device, the offline state, the rule of auto-downloads and «nur im WLAN». The list
 * filters only by `allowedProfileIds` (rule 12).
 */
class DownloadsViewModel(
    private val engine: OfflineEngine,
    private val session: SessionManager,
    private val settings: DeviceSettingsStore,
    private val viewer: MutableStateFlow<Long?>,
    private val network: () -> NetworkKind = { NetworkKind.Unknown },
    private val clock: () -> Long = System::currentTimeMillis,
) : ViewModel(), DownloadsEvents {
    private val local = MutableStateFlow(Local())
    private val messages = Channel<DownloadsMessage>(Channel.BUFFERED)
    val events: Flow<DownloadsMessage> = messages.receiveAsFlow()

    /**
     * The list and the storage follow the records, not the progress: the folder is walked when the
     * downloads change, not with every step of a fetch (code review M-5).
     */
    private val data = combine(engine.downloads, engine.profiles, viewer) { downloads, profiles, viewing ->
        Triple(downloads, profiles, viewing)
    }.map { (downloads, profiles, viewing) ->
        val ready = session.state.value as? SessionState.Ready
        val account = engine.account()?.key
        val profileId = viewing ?: ready?.profile?.id
        val own = if (account != null && profileId != null) downloadsForProfile(downloads, account, profileId) else emptyList()
        val wifiOnly = runCatching { settingsWifi() }.getOrDefault(false)
        val rows = own.map { row(it, profiles, wifiOnly) }
        val storage = Storage(engine.files.usedBytes(), engine.files.freeBytes(), engine.files.totalBytes())
        val name = profiles.firstOrNull { it.account == account && it.id == profileId }?.snapshot?.name ?: ready?.profile?.name
        val rule = profiles.firstOrNull { it.account == account && it.id == profileId }?.rules
        Snapshot(rows, storage, name, viewing == null || viewing == ready?.profile?.id, offlineProfiles(profiles, downloads, account), rule?.enabled == true)
    }.flowOn(Dispatchers.IO)

    private suspend fun settingsWifi(): Boolean = settings.settings.first().downloadsWifiOnly

    val state: StateFlow<DownloadsUiState> = combine(data, engine.progress, engine.contact, settings.settings, local) { snap, progress, contact, device, l ->
        DownloadsUiState(
            rows = snap.rows.map { it.with(progress) },
            loaded = true,
            offline = contact == Contact.Offline || network() == NetworkKind.None,
            storage = snap.storage,
            profileName = snap.name,
            viewerIsSession = snap.ownProfile,
            ruleOn = l.rule?.rules?.enabled ?: snap.ruleOn,
            offlineProfiles = snap.profiles,
            wifiOnly = device.downloadsWifiOnly,
            autoOpen = l.autoOpen,
            settingsOpen = l.settingsOpen,
            rule = l.rule,
            ruleFailed = l.ruleFailed,
            sheet = l.sheet,
            deleting = l.deleting,
            pin = l.pin,
        )
    }.stateIn(viewModelScope, SharingStarted.Eagerly, DownloadsUiState())

    init {
        viewModelScope.launch { engine.sync() }
    }

    /** The running fetch over its row: a cheap step, at most twice a second (the fetcher's pace). */
    private fun DownloadRowUi.with(progress: FetchProgress?): DownloadRowUi {
        if (progress == null || progress.id != id || state == RowState.Done || state is RowState.Paused) return this
        val percent = if (progress.total > 0) (progress.bytes * 100 / progress.total).toInt() else 0
        return copy(state = RowState.Fetching(percent, progress.bytes, progress.total))
    }

    private fun row(d: StoredDownload, profiles: List<StoredProfile>, wifiOnly: Boolean): DownloadRowUi {
        val item = d.item
        val size = d.manifest?.sizeBytes ?: d.sizeBytes ?: d.estimatedBytes
        val percent = if (size > 0) (d.bytesWritten * 100 / size).toInt().coerceIn(0, 100) else 0
        val state = when {
            d.complete -> RowState.Done
            d.paused -> RowState.Paused(percent)
            d.fetchError != null && d.fetchError != FetchError.NETWORK -> RowState.Error(d.fetchError!!)
            d.state == DownloadStateValue.READY && waitsForWifi(d, profiles, network(), wifiOnly) -> RowState.WaitsForWifi
            d.state == DownloadStateValue.READY -> RowState.Fetching(percent, d.bytesWritten, size)
            d.state == DownloadStateValue.FAILED -> RowState.Failed
            d.state == DownloadStateValue.PAUSED -> RowState.ServerPaused
            d.state == DownloadStateValue.PREPARING -> RowState.Preparing(d.segmentsTotal?.takeIf { it > 0 }?.let { d.segmentsDone * 100 / it } ?: 0)
            d.error == "budget_full" -> RowState.BudgetFull
            else -> RowState.Queued
        }
        val episode = item?.episodeNumber
        val groupId = item?.seriesRef?.id ?: item?.parentId?.takeIf { episode != null }
        return DownloadRowUi(
            id = d.id,
            itemId = item?.id ?: d.manifest?.itemId ?: d.id,
            title = item?.title.orEmpty(),
            groupId = groupId,
            groupTitle = item?.seriesTitle ?: item?.seriesRef?.title,
            quality = d.quality,
            size = size,
            state = state,
            auto = d.auto,
            poster = engine.files.poster(d.id).takeIf { d.posterStored && it.exists() },
            season = item?.seasonNumber,
            episode = episode,
            audio = item?.kind?.let { com.slouchcat.core.media.isAudio(it) } == true,
        )
    }

    override fun onPause(id: String) {
        viewModelScope.launch {
            engine.pause(id)
            messages.send(DownloadsMessage.Paused)
        }
    }

    override fun onResume(id: String) {
        viewModelScope.launch {
            engine.resume(id)
            messages.send(DownloadsMessage.Resumed)
        }
    }

    override fun onDeleteAsk(rows: List<DownloadRowUi>) = local.update { it.copy(sheet = DownloadsSheet.Delete, deleting = rows) }

    override fun onDeleteConfirm() {
        val rows = local.value.deleting
        if (rows.isEmpty()) return
        local.update { it.copy(sheet = null, deleting = emptyList()) }
        viewModelScope.launch {
            engine.delete(rows.map { it.id })
            messages.send(DownloadsMessage.Deleted(rows.size))
        }
    }

    override fun onDismissSheet() {
        if (local.value.pin?.checking == true) return
        local.update { it.copy(sheet = null, deleting = emptyList(), pin = null) }
    }

    override fun onToggleAuto() {
        val open = !local.value.autoOpen
        local.update { it.copy(autoOpen = open) }
        if (open && local.value.rule == null) loadRule()
    }

    private fun loadRule() {
        viewModelScope.launch {
            when (val result = engine.rules()) {
                is ApiResult.Ok -> local.update { it.copy(rule = RuleForm(result.value), ruleFailed = false) }
                else -> local.update { it.copy(ruleFailed = true) }
            }
        }
    }

    override fun onToggleSettings() = local.update { it.copy(settingsOpen = !it.settingsOpen) }

    override fun onWifiOnly(on: Boolean) {
        viewModelScope.launch { settings.update { it.copy(downloadsWifiOnly = on) } }
    }

    override fun onRule(change: (DownloadRules) -> DownloadRules) = local.update { l ->
        l.copy(rule = l.rule?.let { it.copy(rules = change(it.rules).let { r -> r.copy(maxCount = r.maxCount.coerceIn(MIN_COUNT, MAX_COUNT)) }, saved = false, failed = false) })
    }

    override fun onSaveRule() {
        val form = local.value.rule ?: return
        if (form.saving) return
        local.update { it.copy(rule = form.copy(saving = true)) }
        viewModelScope.launch {
            val result = engine.setRules(form.rules)
            local.update { l ->
                l.copy(rule = if (result is ApiResult.Ok) RuleForm(result.value, saved = true) else form.copy(saving = false, failed = true))
            }
            if (result is ApiResult.Ok) messages.send(DownloadsMessage.RuleSaved)
        }
    }

    override fun onSwitchProfile() = local.update { it.copy(sheet = DownloadsSheet.Profiles) }

    override fun onPickProfile(profile: StoredProfile) {
        val ready = session.state.value as? SessionState.Ready
        if (profile.id == ready?.profile?.id || OfflinePin.requirement(profile) == OfflinePin.Requirement.None) {
            viewer.value = profile.id.takeIf { it != ready?.profile?.id }
            local.update { it.copy(sheet = null, pin = null) }
            return
        }
        val unavailable = OfflinePin.requirement(profile) == OfflinePin.Requirement.Unavailable
        local.update { it.copy(sheet = DownloadsSheet.Pin, pin = OfflinePinUi(profile, unavailable = unavailable)) }
    }

    override fun onPinDigit(digit: Char) = local.update { l ->
        val pin = l.pin ?: return@update l
        if (!digit.isDigit() || pin.checking || pin.lockedSeconds > 0 || pin.digits.length >= MAX_PIN) return@update l
        l.copy(pin = pin.copy(digits = pin.digits + digit, wrong = false))
    }

    override fun onPinDelete() = local.update { l -> l.copy(pin = l.pin?.takeUnless { it.checking }?.let { it.copy(digits = it.digits.dropLast(1)) } ?: l.pin) }

    override fun onPinSubmit() {
        val pin = local.value.pin ?: return
        if (pin.checking || pin.lockedSeconds > 0 || pin.digits.length < MIN_PIN || pin.unavailable) return
        local.update { it.copy(pin = pin.copy(checking = true)) }
        viewModelScope.launch {
            val digits = pin.digits.toCharArray()
            val result = engine.checkPin(pin.profile.id, digits)
            digits.fill('0')
            when (result) {
                PinCheck.Ok -> {
                    viewer.value = pin.profile.id
                    local.update { it.copy(sheet = null, pin = null) }
                }
                PinCheck.Wrong -> local.update { it.copy(pin = pin.copy(digits = "", checking = false, wrong = true, lockedSeconds = 0)) }
                is PinCheck.Locked -> {
                    local.update { it.copy(pin = pin.copy(digits = "", checking = false, wrong = false, lockedSeconds = (result.remainingMs + 999) / 1000)) }
                    countDown(pin.profile.id)
                }
                PinCheck.Unavailable -> local.update { it.copy(pin = pin.copy(digits = "", checking = false, unavailable = true)) }
            }
        }
    }

    private var countdown: Job? = null

    /** The pause of the PIN counts down; the pad stays locked until it ends (code review L-3). */
    private fun countDown(profileId: Long) {
        countdown?.cancel()
        countdown = viewModelScope.launch {
            while (true) {
                delay(SECOND)
                var left = 0L
                local.update { l ->
                    val pin = l.pin?.takeIf { it.profile.id == profileId } ?: return@update l
                    left = (pin.lockedSeconds - 1).coerceAtLeast(0)
                    l.copy(pin = pin.copy(lockedSeconds = left))
                }
                if (left == 0L) break
            }
        }
    }

    private data class Snapshot(val rows: List<DownloadRowUi>, val storage: Storage, val name: String?, val ownProfile: Boolean, val profiles: List<StoredProfile>, val ruleOn: Boolean)

    companion object {
        const val MIN_COUNT = 1
        const val MAX_COUNT = 20
        const val MIN_PIN = 4
        const val MAX_PIN = 12
        private const val SECOND = 1_000L
    }
}
