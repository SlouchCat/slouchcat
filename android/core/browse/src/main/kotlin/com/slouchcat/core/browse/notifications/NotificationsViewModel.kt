package com.slouchcat.core.browse.notifications

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Notification
import com.slouchcat.core.api.generated.models.ProfileAnnouncement
import com.slouchcat.core.api.map
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.NotificationTarget
import com.slouchcat.core.media.Page
import com.slouchcat.core.media.PageLoader
import com.slouchcat.core.media.Paged
import com.slouchcat.core.media.data.NotificationsRepository
import com.slouchcat.core.media.target
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import okhttp3.HttpUrl

/** «Benachrichtigungen»: the open announcements on top, the entries page by page, how many are unread. */
data class NotificationsUiState(
    val announcements: List<ProfileAnnouncement> = emptyList(),
    val entries: Paged<Notification> = Paged(),
    val unread: Int = 0,
    /** Entries read on this page (the list keeps them until it is read again). */
    val read: Set<Long> = emptySet(),
    /** Announcements whose «Verstanden» is on its way. */
    val acknowledging: Set<Long> = emptySet(),
    val markingAll: Boolean = false,
) {
    fun isUnread(entry: Notification): Boolean = entry.readAt == null && entry.id !in read
}

sealed interface NotificationsMessage {
    data object Failed : NotificationsMessage
}

/** The number of the bell on «Start»: unread entries and open announcements together. */
suspend fun NotificationsRepository.badge(server: HttpUrl): ApiResult<Int> {
    val unread = page(server, 0, 1).map { it.unread }
    if (unread !is ApiResult.Ok) return unread
    val announcements = announcements(server)
    return ApiResult.Ok(unread.value + ((announcements as? ApiResult.Ok)?.value?.size ?: 0))
}

/**
 * «Benachrichtigungen» (mockup `/notifications`, web `NotificationsPage` and `Announcements`):
 * the profile's inbox page by page (`listNotifications`), opening an entry marks it read and leads
 * to its title or to «Meine Wünsche» ([target]), «Alle als gelesen markieren», and the open
 * announcements of the main user with «Verstanden» (`ackAnnouncement`). [badge]: the number of the
 * bell; the page keeps it current.
 */
class NotificationsViewModel(
    private val session: SessionManager,
    private val repository: NotificationsRepository,
    private val badge: LiveCount,
) : ViewModel() {
    private val local = MutableStateFlow(NotificationsUiState())
    private val pager = PageLoader(viewModelScope, map = { it: Notification -> it }, key = { it.id }) { offset, limit ->
        val server = session.server ?: return@PageLoader ApiResult.Offline
        repository.page(server, offset, limit).also { result ->
            if (offset == 0 && result is ApiResult.Ok) {
                local.update { it.copy(unread = result.value.unread, read = emptySet()) }
                loaded = true
                publishBadge()
            }
        }.map { Page(it.notifications, it.total) }
    }

    val state: StateFlow<NotificationsUiState> = combine(local, pager.state) { own, entries -> own.copy(entries = entries) }
        .stateIn(viewModelScope, SharingStarted.Eagerly, NotificationsUiState())

    private val _messages = Channel<NotificationsMessage>(Channel.BUFFERED)
    val messages: Flow<NotificationsMessage> = _messages.receiveAsFlow()

    /** The first page has arrived; before that the bell's number says nothing about this list. */
    private var loaded = false

    init {
        refresh()
        // The bell reads a higher number than this list shows (a push came while the page is open,
        // the start page read the bell again): the list is old, read it again. The web app does the
        // same on focus and with its 30 s poll of the bell.
        viewModelScope.launch {
            badge.count.collect { count ->
                if (loaded && count != null && count > local.value.unread + local.value.announcements.size) refresh()
            }
        }
    }

    fun refresh() {
        pager.refresh(keep = true)
        val server = session.server ?: return
        viewModelScope.launch {
            (repository.announcements(server) as? ApiResult.Ok)?.let { result ->
                local.update { it.copy(announcements = result.value) }
                publishBadge()
            }
        }
    }

    fun retry() = pager.refresh()

    fun onMore() = pager.loadMore()

    private fun publishBadge() = badge.set(local.value.unread + local.value.announcements.size)

    /** Marks [entry] read (when it was not) and says where it leads. */
    fun onOpen(entry: Notification): NotificationTarget {
        if (local.value.isUnread(entry)) {
            local.update { it.copy(read = it.read + entry.id, unread = (it.unread - 1).coerceAtLeast(0)) }
            publishBadge()
            session.server?.let { server -> viewModelScope.launch { repository.markRead(server, entry.id) } }
        }
        return entry.target()
    }

    fun onMarkAllRead() {
        val server = session.server ?: return
        if (local.value.markingAll) return
        local.update { it.copy(markingAll = true) }
        viewModelScope.launch {
            when (repository.markAllRead(server)) {
                is ApiResult.Ok -> {
                    local.update { now ->
                        val ids = pager.state.value.loaded.map { it.id }.toSet()
                        now.copy(unread = 0, read = now.read + ids, markingAll = false)
                    }
                    publishBadge()
                }
                else -> {
                    local.update { it.copy(markingAll = false) }
                    _messages.send(NotificationsMessage.Failed)
                }
            }
        }
    }

    /** «Verstanden»: the announcement goes for this profile (also when it was withdrawn meanwhile). */
    fun onAcknowledge(announcement: ProfileAnnouncement) {
        val server = session.server ?: return
        if (announcement.id in local.value.acknowledging) return
        local.update { it.copy(acknowledging = it.acknowledging + announcement.id) }
        viewModelScope.launch {
            when (repository.acknowledge(server, announcement.id)) {
                is ApiResult.Ok, ApiResult.NotFound -> local.update {
                    it.copy(announcements = it.announcements.filterNot { a -> a.id == announcement.id }, acknowledging = it.acknowledging - announcement.id)
                }.also { publishBadge() }
                else -> {
                    local.update { it.copy(acknowledging = it.acknowledging - announcement.id) }
                    _messages.send(NotificationsMessage.Failed)
                }
            }
        }
    }
}

/** Whether the inbox of [state] has nothing (no entries and no announcements). */
val NotificationsUiState.empty: Boolean get() = entries.items is Loadable.Empty && announcements.isEmpty()
