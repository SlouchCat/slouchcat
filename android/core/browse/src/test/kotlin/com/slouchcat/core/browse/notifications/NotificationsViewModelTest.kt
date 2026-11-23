package com.slouchcat.core.browse.notifications

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Channel
import com.slouchcat.core.api.generated.models.EventType
import com.slouchcat.core.api.generated.models.Notification
import com.slouchcat.core.browse.testing.CHILD
import com.slouchcat.core.browse.testing.FakeChannels
import com.slouchcat.core.browse.testing.FakeDevicePush
import com.slouchcat.core.browse.testing.FakeNotifications
import com.slouchcat.core.browse.testing.announcement
import com.slouchcat.core.browse.testing.notification
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.settle
import com.slouchcat.core.browse.testing.ulid
import com.slouchcat.core.media.DevicePushState
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.NotificationTarget
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class NotificationsViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun TestScope.inbox(repository: FakeNotifications, badge: LiveCount = LiveCount()): NotificationsViewModel {
        val s = session(backgroundScope)
        settle()
        val vm = NotificationsViewModel(s, repository, badge)
        settle()
        return vm
    }

    private fun sample() = FakeNotifications(
        entries = mutableListOf(
            notification(1),
            notification(2, Notification.Kind.WISH_AVAILABLE, item = null, wishTitle = "Hidden title"),
            notification(3, read = true),
        ),
        open = mutableListOf(announcement(9)),
    )

    @Test
    fun theInboxCountsUnreadAndAnnouncementsForTheBell() = runTest(dispatcher) {
        val badge = LiveCount()
        val vm = inbox(sample(), badge)
        val state = vm.state.value
        assertEquals(3, state.entries.loaded.size)
        assertEquals(2, state.unread)
        assertEquals(listOf(9L), state.announcements.map { it.id })
        assertEquals(3, badge.count.value)
    }

    @Test
    fun openingAnEntryMarksItReadAndSaysWhereItLeads() = runTest(dispatcher) {
        val repository = sample()
        val badge = LiveCount()
        val vm = inbox(repository, badge)
        val first = vm.state.value.entries.loaded[0]
        assertEquals(NotificationTarget.Item(ulid(1)), vm.onOpen(first))
        settle()
        assertTrue("read:1" in repository.calls)
        assertFalse(vm.state.value.isUnread(first))
        assertEquals(1, vm.state.value.unread)
        assertEquals(2, badge.count.value)
        // A wish whose title the profile does not see leads to «Meine Wünsche».
        assertEquals(NotificationTarget.Wishes, vm.onOpen(vm.state.value.entries.loaded[1]))
        // A read entry is not marked again.
        vm.onOpen(vm.state.value.entries.loaded[2])
        settle()
        assertFalse("read:3" in repository.calls)
    }

    @Test
    fun markAllReadClearsTheBadge() = runTest(dispatcher) {
        val repository = sample()
        val badge = LiveCount()
        val vm = inbox(repository, badge)
        vm.onMarkAllRead()
        settle()
        assertEquals(0, vm.state.value.unread)
        assertTrue(vm.state.value.entries.loaded.none { vm.state.value.isUnread(it) })
        // The announcement still counts until it is acknowledged.
        assertEquals(1, badge.count.value)
        vm.onAcknowledge(vm.state.value.announcements.single())
        settle()
        assertTrue(vm.state.value.announcements.isEmpty())
        assertEquals(0, badge.count.value)
    }

    @Test
    fun aWithdrawnAnnouncementGoesToo() = runTest(dispatcher) {
        val repository = sample()
        val vm = inbox(repository)
        repository.open.clear()
        vm.onAcknowledge(vm.state.value.announcements.single())
        settle()
        assertTrue(vm.state.value.announcements.isEmpty())
    }

    @Test
    fun aFailedChangeTellsIt() = runTest(dispatcher) {
        val repository = sample()
        val vm = inbox(repository)
        val messages = mutableListOf<NotificationsMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        repository.changeFailure = ApiResult.Offline
        vm.onMarkAllRead()
        settle()
        assertEquals(listOf<NotificationsMessage>(NotificationsMessage.Failed), messages)
        assertEquals(2, vm.state.value.unread)
    }

    @Test
    fun anEmptyInboxIsEmpty() = runTest(dispatcher) {
        val vm = inbox(FakeNotifications())
        assertEquals(Loadable.Empty, vm.state.value.entries.items)
        assertTrue(vm.state.value.empty)
    }
}

@OptIn(ExperimentalCoroutinesApi::class)
class ChannelSettingsViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    @Test
    fun thePrimaryProfileChangesAChannel() = runTest(dispatcher) {
        val repository = FakeChannels()
        val push = FakeDevicePush(DevicePushState.On)
        val s = session(backgroundScope)
        settle()
        val vm = ChannelSettingsViewModel(s, repository, push)
        val messages = mutableListOf<ChannelMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        settle()
        assertTrue(vm.state.value.canChange)
        assertEquals(DevicePushState.On, vm.state.value.device)
        assertEquals(listOf("refresh"), push.calls)
        vm.onOpen(Channel.EMAIL)
        settle()
        assertEquals(true, vm.state.value.draft?.enabled)
        vm.onDraftEnabled(false)
        vm.onDraftEvent(EventType.WISH_STATUS, true)
        settle()
        vm.onSave()
        settle()
        assertTrue("update:email:false" in repository.calls)
        assertNull(vm.state.value.draft)
        assertEquals(false, vm.state.value.channel(Channel.EMAIL)?.enabled)
        assertTrue(EventType.WISH_STATUS in vm.state.value.channel(Channel.EMAIL)!!.events)
        vm.onTest(Channel.EMAIL)
        settle()
        assertEquals(listOf(ChannelMessage.Saved, ChannelMessage.TestSent), messages)
    }

    @Test
    fun anotherProfileOnlyLooks() = runTest(dispatcher) {
        val repository = FakeChannels()
        val s = session(backgroundScope, profile = CHILD)
        settle()
        val vm = ChannelSettingsViewModel(s, repository, null)
        settle()
        assertFalse(vm.state.value.canChange)
        assertEquals(DevicePushState.NotInBuild, vm.state.value.device)
        vm.onOpen(Channel.EMAIL)
        settle()
        assertNull(vm.state.value.draft)
        vm.onLinkTelegram()
        settle()
        assertFalse("link" in repository.calls)
    }

    @Test
    fun theServerRefusesAndThePageSaysWhy() = runTest(dispatcher) {
        val repository = FakeChannels().apply { changeFailure = ApiResult.NotAllowed("primary_profile_required") }
        val s = session(backgroundScope)
        settle()
        val vm = ChannelSettingsViewModel(s, repository, null)
        val messages = mutableListOf<ChannelMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        settle()
        vm.onOpen(Channel.PUSH)
        settle()
        vm.onSave()
        settle()
        assertEquals(listOf<ChannelMessage>(ChannelMessage.Failed("primary_profile_required")), messages)
        assertEquals(Channel.PUSH, vm.state.value.draft?.channel)
    }

    @Test
    fun theTelegramLinkCodeComesAndGoes() = runTest(dispatcher) {
        val repository = FakeChannels()
        val s = session(backgroundScope)
        settle()
        val vm = ChannelSettingsViewModel(s, repository, null)
        settle()
        vm.onLinkTelegram()
        settle()
        assertEquals("ABC123", vm.state.value.channel(Channel.TELEGRAM)?.telegram?.linkCode)
        vm.onUnlinkTelegram()
        settle()
        assertNull(vm.state.value.channel(Channel.TELEGRAM)?.telegram?.linkCode)
    }

    @Test
    fun theDeviceSwitchGoesToTheDevicePush() = runTest(dispatcher) {
        val push = FakeDevicePush(DevicePushState.Off)
        val s = session(backgroundScope)
        settle()
        val vm = ChannelSettingsViewModel(s, FakeChannels(channelsWithPushOff()), push)
        settle()
        assertTrue(vm.state.value.pushChannelOff)
        vm.onDevicePush(true)
        settle()
        assertEquals(DevicePushState.On, vm.state.value.device)
        vm.onDevicePush(false)
        settle()
        assertEquals(listOf("refresh", "enable", "disable"), push.calls)
    }

    private fun channelsWithPushOff() = com.slouchcat.core.browse.testing.channelSettings(pushEnabled = false).toMutableList()
}
