package com.slouchcat.app.mobile.search

import com.slouchcat.app.mobile.testing.FakeSearch
import com.slouchcat.app.mobile.testing.collectionRef
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.personRef
import com.slouchcat.app.mobile.testing.session
import com.slouchcat.app.mobile.testing.settle
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MoreState
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/** The search: the pause while typing and the cancelled request run on the test clock, never on real time. */
@OptIn(ExperimentalCoroutinesApi::class)
class SearchViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun TestScope.model(repository: FakeSearch, genre: String? = null): SearchViewModel {
        val s = session(backgroundScope)
        settle()
        val vm = SearchViewModel(s, repository, genre)
        runCurrent()
        return vm
    }

    private fun films() = FakeSearch(
        titles = listOf(itemRef(1, "Spider-Man"), itemRef(2, "Spider Island"), itemRef(3, "Night Train")),
        people = listOf(personRef(7, "Spike Lee")),
        collections = listOf(collectionRef("c1", "Spider-Man Collection", 3)),
    )

    @Test
    fun withoutTextThePageShowsTheGenresAndAsksNothing() = runTest(dispatcher) {
        val repository = films()
        val vm = model(repository)
        settle()
        assertNull(vm.state.value.hits)
        assertEquals(listOf("Drama", "Comedy"), (vm.state.value.genres as Loadable.Ready).value.map { it.name })
        assertTrue(repository.asked.isEmpty())
    }

    @Test
    fun typingWaitsForThePauseBeforeItAsks() = runTest(dispatcher) {
        val repository = films()
        val vm = model(repository)
        vm.onText("sp")
        advanceTimeBy(SearchViewModel.DEBOUNCE_MS - 1)
        runCurrent()
        assertTrue(repository.asked.isEmpty())
        vm.onText("spi")
        advanceTimeBy(SearchViewModel.DEBOUNCE_MS - 1)
        runCurrent()
        // The second key started the pause again.
        assertTrue(repository.asked.isEmpty())
        advanceTimeBy(1)
        runCurrent()
        assertEquals(listOf(Triple("spi", null, 0)), repository.asked)
        val hits = (vm.state.value.hits as Loadable.Ready).value
        assertEquals(listOf("Spider-Man", "Spider Island"), hits.items.map { it.title })
        assertEquals(1, hits.people.size)
        assertEquals(1, hits.collections.size)
        assertEquals(4, hits.count)
    }

    /** A space more asks nothing new; the hits mark the text they were found for (code review L-3). */
    @Test
    fun aSpaceMoreAsksNothingNewAndTheHitsKeepTheirText() = runTest(dispatcher) {
        val repository = films()
        val vm = model(repository)
        vm.onText("spi")
        settle()
        val asked = repository.asked.size
        vm.onText("spi ")
        settle()
        assertEquals(asked, repository.asked.size)
        assertEquals("spi", (vm.state.value.hits as Loadable.Ready).value.query)
    }

    @Test
    fun aNewTextCancelsTheRequestUnderWay() = runTest(dispatcher) {
        val repository = films()
        val gate = CompletableDeferred<Unit>()
        repository.gate = gate
        val vm = model(repository)
        vm.onText("spi")
        advanceTimeBy(SearchViewModel.DEBOUNCE_MS)
        runCurrent()
        assertEquals(Loadable.Loading, vm.state.value.hits)
        vm.onText("night")
        runCurrent()
        assertEquals(listOf<String?>("spi"), repository.cancelled)
        gate.complete(Unit)
        advanceTimeBy(SearchViewModel.DEBOUNCE_MS)
        runCurrent()
        assertEquals(listOf("spi", "night"), repository.asked.map { it.first })
        assertEquals(listOf("Night Train"), (vm.state.value.hits as Loadable.Ready).value.items.map { it.title })
    }

    @Test
    fun oneCharacterIsTooShortAndClearingReturnsToTheWaysIn() = runTest(dispatcher) {
        val repository = films()
        val vm = model(repository)
        vm.onText("s")
        advanceTimeBy(SearchViewModel.DEBOUNCE_MS * 2)
        runCurrent()
        assertTrue(vm.state.value.tooShort)
        assertTrue(repository.asked.isEmpty())
        vm.onText("spi")
        vm.onSubmit()
        runCurrent()
        assertTrue(vm.state.value.hits is Loadable.Ready)
        vm.onClear()
        runCurrent()
        assertNull(vm.state.value.hits)
        assertEquals("", vm.state.value.text)
    }

    @Test
    fun nothingFoundIsEmptyAndAFailureCanBeRetried() = runTest(dispatcher) {
        val repository = films()
        val vm = model(repository)
        vm.onText("zzz")
        vm.onSubmit()
        runCurrent()
        assertEquals(Loadable.Empty, vm.state.value.hits)
        repository.failure = ApiResult.Offline
        vm.onText("spi")
        vm.onSubmit()
        runCurrent()
        assertEquals(Loadable.Failed(LoadError.Offline), vm.state.value.hits)
        repository.failure = null
        vm.retry()
        runCurrent()
        assertTrue(vm.state.value.hits is Loadable.Ready)
    }

    @Test
    fun aGenreFiltersWithoutTextAndCanBeTakenOff() = runTest(dispatcher) {
        val repository = films()
        repository.genreOf = { if (it.title == "Night Train") "Drama" else null }
        val vm = model(repository, genre = "Drama")
        settle()
        assertEquals(Triple(null, "Drama", 0), repository.asked.single())
        assertEquals(listOf("Night Train"), (vm.state.value.hits as Loadable.Ready).value.items.map { it.title })
        vm.onClearGenre()
        runCurrent()
        assertNull(vm.state.value.genre)
        assertNull(vm.state.value.hits)
        vm.onGenre("Drama")
        runCurrent()
        assertEquals("Drama", vm.state.value.genre)
        assertEquals(2, repository.asked.size)
    }

    @Test
    fun theNextPageComesOnceAndAFailedPageCanBeRetried() = runTest(dispatcher) {
        val repository = FakeSearch(titles = (1..60).map { itemRef(it, "Spider $it") })
        val vm = model(repository)
        vm.onText("spider")
        vm.onSubmit()
        runCurrent()
        assertEquals(50, (vm.state.value.hits as Loadable.Ready).value.items.size)
        vm.onMore()
        vm.onMore()
        runCurrent()
        val hits = (vm.state.value.hits as Loadable.Ready).value
        assertEquals(60, hits.items.size)
        assertEquals(listOf(0, 50), repository.asked.map { it.third })
        // A failed page: the foot offers «Erneut versuchen».
        val more = FakeSearch(titles = (1..60).map { itemRef(it, "Spider $it") })
        val vm2 = model(more)
        vm2.onText("spider")
        vm2.onSubmit()
        runCurrent()
        more.failure = ApiResult.Offline
        vm2.onMore()
        runCurrent()
        assertEquals(MoreState.Failed, (vm2.state.value.hits as Loadable.Ready).value.more)
    }

    @Test
    fun genresThatFailCanBeRetried() = runTest(dispatcher) {
        val repository = films()
        repository.genres = ApiResult.Offline
        val vm = model(repository)
        settle()
        assertEquals(Loadable.Failed(LoadError.Offline), vm.state.value.genres)
        repository.genres = ApiResult.Ok(emptyList())
        vm.retryGenres()
        settle()
        assertEquals(Loadable.Empty, vm.state.value.genres)
    }
}
