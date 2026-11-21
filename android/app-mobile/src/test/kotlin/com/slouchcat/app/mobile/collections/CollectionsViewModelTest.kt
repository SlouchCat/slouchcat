package com.slouchcat.app.mobile.collections

import com.slouchcat.app.mobile.testing.FakeCollections
import com.slouchcat.app.mobile.testing.collectionRef
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.session
import com.slouchcat.app.mobile.testing.settle
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
class CollectionsViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun fake() = FakeCollections(
        list = ApiResult.Ok(listOf(collectionRef("c1", "28 Days Later", 3), collectionRef("c2", "A Quiet Place", 2))),
        parts = mutableMapOf("c1" to mutableListOf(itemRef(1), itemRef(2, inWatchlist = true), itemRef(3))),
    )

    private fun TestScope.overview(repository: FakeCollections, count: LiveCount = LiveCount()): CollectionsViewModel {
        val s = session(backgroundScope)
        settle()
        return CollectionsViewModel(s, repository, count).also { settle() }
    }

    private fun TestScope.page(repository: FakeCollections, id: String = "c1"): CollectionViewModel {
        val s = session(backgroundScope)
        settle()
        return CollectionViewModel(s, repository, id).also { settle() }
    }

    private fun CollectionViewModel.page() = (state.value.page as Loadable.Ready).value

    @Test
    fun theOverviewLoadsCountsAndHandlesEmptyAndFailure() = runTest(dispatcher) {
        val repository = fake()
        val count = LiveCount()
        val vm = overview(repository, count)
        assertEquals(listOf("c1", "c2"), (vm.state.value.collections as Loadable.Ready).value.map { it.id })
        assertEquals(2, count.count.value)
        repository.list = ApiResult.Ok(emptyList())
        vm.retry()
        settle()
        assertEquals(Loadable.Empty, vm.state.value.collections)
        repository.list = ApiResult.Offline
        vm.retry()
        settle()
        assertEquals(Loadable.Failed(LoadError.Offline), vm.state.value.collections)
    }

    @Test
    fun aCollectionLoadsEveryPartAndAnUnknownOneIsNotFound() = runTest(dispatcher) {
        val repository = fake()
        repository.parts["c1"] = (1..250).map { itemRef(it) }.toMutableList()
        val vm = page(repository)
        assertEquals(250, vm.page().parts.size)
        assertEquals(listOf("get:c1:0", "get:c1:200"), repository.calls)
        val gone = page(repository, "nope")
        assertEquals(Loadable.Failed(LoadError.NotFound), gone.state.value.page)
    }

    @Test
    fun listAllAddsTheOthersAndASecondPressTakesAllOff() = runTest(dispatcher) {
        val repository = fake()
        val vm = page(repository)
        val messages = mutableListOf<CollectionMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        assertFalse(vm.page().allListed)
        vm.onListAll()
        settle()
        assertTrue(vm.page().allListed)
        assertEquals(CollectionMessage.Listed(2), messages.last())
        assertEquals(2, repository.calls.count { it.startsWith("watchlist:") && it.endsWith(":true") })
        vm.onListAll()
        settle()
        assertFalse(vm.page().allListed)
        assertTrue(vm.page().parts.none { it.inWatchlist })
        assertEquals(CollectionMessage.Unlisted, messages.last())
    }

    @Test
    fun aRefusedListingSaysSo() = runTest(dispatcher) {
        val repository = fake()
        val vm = page(repository)
        val messages = mutableListOf<CollectionMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        repository.changeFailure = ApiResult.Offline
        vm.onListAll()
        settle()
        assertEquals(CollectionMessage.Failed, messages.single())
        assertFalse(vm.state.value.listing)
    }

    @Test
    fun watchedMarksAllAndTakingTheMarksOffAsksFirst() = runTest(dispatcher) {
        val repository = fake()
        val vm = page(repository)
        val messages = mutableListOf<CollectionMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        assertEquals(CollectionWatched.None, vm.page().watched)
        vm.onWatched()
        settle()
        assertEquals(CollectionWatched.Full, vm.page().watched)
        assertEquals(CollectionMessage.Watched, messages.last())
        // Full: the question, then «Abbrechen» changes nothing.
        vm.onWatched()
        assertEquals(CollectionWatched.Full, vm.state.value.asking)
        vm.onDismiss()
        assertNull(vm.state.value.asking)
        vm.onWatched()
        vm.onConfirm(false)
        settle()
        assertEquals(CollectionWatched.None, vm.page().watched)
        assertEquals(CollectionMessage.Unwatched, messages.last())
        assertEquals(listOf("watched:c1:true", "watched:c1:false"), repository.calls.filter { it.startsWith("watched:") })
    }

    @Test
    fun aHalfMarkedCollectionOffersBothWays() = runTest(dispatcher) {
        val repository = fake()
        repository.seen["c1"] = 1
        val vm = page(repository)
        assertEquals(CollectionWatched.Half, vm.page().watched)
        vm.onWatched()
        assertEquals(CollectionWatched.Half, vm.state.value.asking)
        vm.onConfirm(true)
        settle()
        assertEquals(CollectionWatched.Full, vm.page().watched)
    }

    @Test
    fun aRefusedMarkKeepsThePage() = runTest(dispatcher) {
        val repository = fake()
        val vm = page(repository)
        val messages = mutableListOf<CollectionMessage>()
        backgroundScope.launch { vm.messages.toList(messages) }
        repository.changeFailure = ApiResult.Failed(500, null)
        vm.onWatched()
        settle()
        assertEquals(CollectionMessage.Failed, messages.single())
        assertEquals(CollectionWatched.None, vm.page().watched)
        assertFalse(vm.state.value.marking)
    }
}
