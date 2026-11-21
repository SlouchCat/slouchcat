package com.slouchcat.core.media

import com.slouchcat.core.api.ApiResult
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** The pager of the lists: first page, empty, failure, further pages, refresh. */
@OptIn(ExperimentalCoroutinesApi::class)
class PageLoaderTest {
    private val dispatcher = StandardTestDispatcher()
    private val all = (1..120).toList()
    private val asked = mutableListOf<Int>()
    private var answer: (Int, Int) -> ApiResult<Page<Int>> = { offset, limit -> ApiResult.Ok(Page(all.drop(offset).take(limit), all.size)) }
    private var gate: CompletableDeferred<Unit>? = null

    private fun TestScope.loader() = PageLoader<Int, String>(this, pageSize = 50, map = { "#$it" }) { offset, limit ->
        asked += offset
        gate?.await()
        answer(offset, limit)
    }

    @Test
    fun theFirstPageThenTheNextUntilTheEnd() = runTest(dispatcher) {
        val pager = loader()
        pager.refresh()
        assertEquals(Loadable.Loading, pager.state.value.items)
        advanceUntilIdle()
        assertEquals(50, pager.state.value.loaded.size)
        assertEquals("#1", pager.state.value.loaded.first())
        assertTrue(pager.state.value.hasMore)
        pager.loadMore()
        // A second call while the page loads asks nothing more.
        pager.loadMore()
        assertEquals(MoreState.Loading, pager.state.value.more)
        advanceUntilIdle()
        pager.loadMore()
        advanceUntilIdle()
        assertEquals(120, pager.state.value.loaded.size)
        assertFalse(pager.state.value.hasMore)
        pager.loadMore()
        advanceUntilIdle()
        assertEquals(listOf(0, 50, 100), asked)
    }

    @Test
    fun nothingThereIsEmpty() = runTest(dispatcher) {
        answer = { _, _ -> ApiResult.Ok(Page(emptyList(), 0)) }
        val pager = loader()
        pager.refresh()
        advanceUntilIdle()
        assertEquals(Loadable.Empty, pager.state.value.items)
    }

    @Test
    fun aFailedFirstPageIsTheErrorOfThePage() = runTest(dispatcher) {
        answer = { _, _ -> ApiResult.Offline }
        val pager = loader()
        pager.refresh()
        advanceUntilIdle()
        assertEquals(Loadable.Failed(LoadError.Offline), pager.state.value.items)
        answer = { _, _ -> ApiResult.NotFound }
        pager.refresh()
        advanceUntilIdle()
        assertEquals(Loadable.Failed(LoadError.NotFound), pager.state.value.items)
    }

    @Test
    fun aFailedNextPageKeepsTheListAndCanBeTriedAgain() = runTest(dispatcher) {
        val pager = loader()
        pager.refresh()
        advanceUntilIdle()
        answer = { _, _ -> ApiResult.Offline }
        pager.loadMore()
        advanceUntilIdle()
        assertEquals(MoreState.Failed, pager.state.value.more)
        assertEquals(50, pager.state.value.loaded.size)
        answer = { offset, limit -> ApiResult.Ok(Page(all.drop(offset).take(limit), all.size)) }
        pager.loadMore()
        advanceUntilIdle()
        assertEquals(100, pager.state.value.loaded.size)
        assertEquals(MoreState.Idle, pager.state.value.more)
    }

    @Test
    fun aRefreshStartsAgainAndDropsThePageUnderWay() = runTest(dispatcher) {
        val pager = loader()
        pager.refresh()
        advanceUntilIdle()
        gate = CompletableDeferred()
        pager.loadMore()
        dispatcher.scheduler.runCurrent()
        // A filter changed: the page under way must not land in the new list.
        pager.refresh()
        gate!!.complete(Unit)
        advanceUntilIdle()
        assertEquals(50, pager.state.value.loaded.size)
        assertEquals(MoreState.Idle, pager.state.value.more)
    }

    @Test
    fun aRefreshThatKeepsShowsTheOldListWhileItLoadsAndAfterAFailure() = runTest(dispatcher) {
        val pager = loader()
        pager.refresh()
        advanceUntilIdle()
        answer = { _, _ -> ApiResult.Offline }
        pager.refresh(keep = true)
        assertEquals(50, pager.state.value.loaded.size)
        advanceUntilIdle()
        assertEquals(50, pager.state.value.loaded.size)
    }
}
