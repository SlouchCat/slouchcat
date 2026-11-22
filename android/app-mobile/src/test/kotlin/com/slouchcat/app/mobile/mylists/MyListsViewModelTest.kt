package com.slouchcat.app.mobile.mylists

import com.slouchcat.app.mobile.testing.FakeMyLists
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.session
import com.slouchcat.app.mobile.testing.settle
import com.slouchcat.app.mobile.testing.ulid
import com.slouchcat.app.mobile.testing.watchedEntry
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
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
class MyListsViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun TestScope.watchlist(repository: FakeMyLists, count: LiveCount = LiveCount()): WatchlistViewModel {
        val s = session(backgroundScope)
        settle()
        val vm = WatchlistViewModel(s, repository, count)
        settle()
        return vm
    }

    private fun TestScope.watched(repository: FakeMyLists): WatchedViewModel {
        val s = session(backgroundScope)
        settle()
        val vm = WatchedViewModel(s, repository)
        settle()
        return vm
    }

    private fun <T> MyListUiState<T>.ids(id: (T) -> String) = (shown(id) as Loadable.Ready).value.map(id)

    @Test
    fun theWatchlistLoadsAndCountsForTheMenu() = runTest(dispatcher) {
        val count = LiveCount()
        val vm = watchlist(FakeMyLists(mutableListOf(itemRef(1), itemRef(2))), count)
        assertEquals(listOf(ulid(1), ulid(2)), vm.state.value.ids { it.id })
        assertEquals(2, vm.state.value.total)
        assertEquals(2, count.count.value)
    }

    @Test
    fun anEmptyWatchlistIsEmptyAndAFailureCanBeRetried() = runTest(dispatcher) {
        val repository = FakeMyLists()
        val vm = watchlist(repository)
        assertEquals(Loadable.Empty, vm.state.value.shown { it.id })
        repository.failure = ApiResult.Offline
        vm.retry()
        settle()
        assertEquals(Loadable.Failed(LoadError.Offline), vm.state.value.shown { it.id })
        repository.failure = null
        repository.watchlist += itemRef(3)
        vm.retry()
        settle()
        assertEquals(listOf(ulid(3)), vm.state.value.ids { it.id })
    }

    @Test
    fun removingTakesTheTitleOffAndUndoPutsItBack() = runTest(dispatcher) {
        val repository = FakeMyLists(mutableListOf(itemRef(1, "Silo"), itemRef(2)))
        val count = LiveCount()
        val vm = watchlist(repository, count)
        val messages = mutableListOf<ListMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        vm.onToggleEdit()
        settle()
        assertTrue(vm.state.value.editing)
        val silo = (vm.state.value.shown { it.id } as Loadable.Ready).value.first()
        vm.onRemove(silo)
        settle()
        assertEquals(listOf(ulid(2)), vm.state.value.ids { it.id })
        assertEquals(1, count.count.value)
        assertEquals(ListMessage.Removed(ulid(1), "Silo"), messages.single())
        vm.onUndo(ulid(1))
        settle()
        assertEquals(listOf(ulid(1), ulid(2)), vm.state.value.ids { it.id })
        assertEquals(2, count.count.value)
        assertEquals(listOf("set:${ulid(1)}:false", "set:${ulid(1)}:true"), repository.calls.filter { it.startsWith("set:") })
        vm.onToggleEdit()
        settle()
        assertFalse(vm.state.value.editing)
    }

    @Test
    fun aRefusedRemovalKeepsTheTitle() = runTest(dispatcher) {
        val repository = FakeMyLists(mutableListOf(itemRef(1)))
        val vm = watchlist(repository)
        val messages = mutableListOf<ListMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        repository.changeFailure = ApiResult.Offline
        vm.onRemove((vm.state.value.shown { it.id } as Loadable.Ready).value.first())
        settle()
        assertEquals(listOf(ulid(1)), vm.state.value.ids { it.id })
        assertEquals(ListMessage.Failed, messages.single())
        assertTrue(vm.state.value.busy.isEmpty())
    }

    @Test
    fun theLastTitleRemovedLeavesTheEmptyList() = runTest(dispatcher) {
        val vm = watchlist(FakeMyLists(mutableListOf(itemRef(1))))
        vm.onRemove((vm.state.value.shown { it.id } as Loadable.Ready).value.first())
        settle()
        assertEquals(Loadable.Empty, vm.state.value.shown { it.id })
    }

    @Test
    fun watchedLoadsAndAFilmLosesItsMarkAtOnce() = runTest(dispatcher) {
        val repository = FakeMyLists(watched = mutableListOf(watchedEntry(1, "Film"), watchedEntry(2, "Series", episodes = 10, watchedCount = 4)))
        val vm = watched(repository)
        val messages = mutableListOf<ListMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        assertEquals(listOf(ulid(1), ulid(2)), vm.state.value.ids { it.id })
        val film = (vm.state.value.shown { it.id } as Loadable.Ready).value.first()
        vm.onAsk(film)
        settle()
        assertNull(vm.state.value.asking)
        assertEquals(listOf(ulid(2)), vm.state.value.ids { it.id })
        assertEquals(ListMessage.Unwatched("Film"), messages.single())
    }

    @Test
    fun aSeriesAsksFirstAndCanBeKept() = runTest(dispatcher) {
        val repository = FakeMyLists(watched = mutableListOf(watchedEntry(2, "Series", episodes = 10, watchedCount = 4)))
        val vm = watched(repository)
        val series = (vm.state.value.shown { it.id } as Loadable.Ready).value.first()
        assertEquals(4, series.watchedCount)
        vm.onAsk(series)
        settle()
        assertEquals(series, vm.state.value.asking)
        vm.onDismiss()
        settle()
        assertNull(vm.state.value.asking)
        assertTrue(repository.calls.none { it.endsWith(":false") })
        vm.onAsk(series)
        vm.onConfirm()
        settle()
        assertEquals(Loadable.Empty, vm.state.value.shown { it.id })
        assertEquals("watched:${ulid(2)}:false", repository.calls.last())
    }

    @Test
    fun watchedEmptyFailedAndARefusedChange() = runTest(dispatcher) {
        val repository = FakeMyLists()
        val vm = watched(repository)
        assertEquals(Loadable.Empty, vm.state.value.shown { it.id })
        repository.failure = ApiResult.Failed(500, null)
        vm.retry()
        settle()
        assertEquals(Loadable.Failed(LoadError.Failed), vm.state.value.shown { it.id })
        repository.failure = null
        repository.watched += watchedEntry(1)
        vm.retry()
        settle()
        val messages = mutableListOf<ListMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        repository.changeFailure = ApiResult.Offline
        vm.onAsk((vm.state.value.shown { it.id } as Loadable.Ready).value.first())
        settle()
        assertEquals(listOf(ulid(1)), vm.state.value.ids { it.id })
        assertEquals(ListMessage.Failed, messages.single())
    }
}
