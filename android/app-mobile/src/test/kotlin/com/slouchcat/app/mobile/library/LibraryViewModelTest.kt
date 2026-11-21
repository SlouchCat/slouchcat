package com.slouchcat.app.mobile.library

import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.testing.FakeLibrary
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.library
import com.slouchcat.app.mobile.testing.session
import com.slouchcat.app.mobile.testing.settle
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.LibraryItemStatus
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.api.generated.models.LibrarySort
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.test.StandardTestDispatcher
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
class LibraryViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val films = (1..70).map { itemRef(it, title = "Film $it") }
    private val repository = FakeLibrary(ApiResult.Ok(films)).apply { genreOf = { if (it.title.endsWith("0")) "Drama" else null } }
    private val libraries = MutableStateFlow<Loadable<List<LibraryNav>>>(Loadable.Ready(listOf(library(4, "Films", count = 70), library(5, "Books", LibraryKind.AUDIOBOOKS))))

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    @Test
    fun theFirstPageWithTheLibraryAndItsGenresThenTheNext() = runTest(dispatcher) {
        val vm = LibraryViewModel(session(backgroundScope), repository, 4, libraries)
        settle()
        val state = vm.state.value
        assertEquals("Films", state.library?.name)
        assertEquals(50, state.items.loaded.size)
        assertEquals(70, state.items.total)
        assertEquals(listOf("Drama", "Comedy"), state.genres.map { it.name })
        // The mockup opens a library by «Zuletzt hinzugefügt».
        assertEquals(LibrarySort.ADDED, repository.queries.first().first.sort)
        vm.onNearEnd()
        settle()
        assertEquals(70, vm.state.value.items.loaded.size)
        assertFalse(vm.state.value.items.hasMore)
    }

    @Test
    fun filtersAndOrderGoToTheServerAndStartAgain() = runTest(dispatcher) {
        val vm = LibraryViewModel(session(backgroundScope), repository, 4, libraries)
        settle()
        vm.onSheet(LibrarySheet.Genre)
        assertEquals(LibrarySheet.Genre, vm.state.value.sheet)
        vm.onGenre("Drama")
        settle()
        assertNull("choosing closes the sheet", vm.state.value.sheet)
        assertEquals(7, vm.state.value.items.total)
        assertTrue(vm.state.value.filters.active)
        vm.onUnseen()
        vm.onDecade(Decade(2010))
        vm.onSort(LibrarySort.TITLE)
        settle()
        val query = repository.queries.last().first
        assertEquals("Drama", query.genre)
        assertEquals(listOf(LibraryItemStatus.UNWATCHED), query.status)
        assertEquals(2010, query.yearFrom)
        assertEquals(2019, query.yearTo)
        assertEquals(LibrarySort.TITLE, query.sort)
        assertEquals(0, repository.queries.last().second)
        vm.onAll()
        settle()
        assertFalse(vm.state.value.filters.active)
        assertEquals(LibrarySort.TITLE, vm.state.value.filters.sort)
        assertEquals(70, vm.state.value.items.total)
    }

    @Test
    fun nothingMatchesIsEmptyAndTheYearsBeforeTheOldestDecadeCount() = runTest(dispatcher) {
        val vm = LibraryViewModel(session(backgroundScope), repository, 4, libraries)
        settle()
        vm.onGenre("Western")
        settle()
        assertEquals(Loadable.Empty, vm.state.value.items.items)
        val before = vm.state.value.decades.last()
        assertTrue(before.before)
        assertNull(before.from)
        assertEquals(1959, before.to)
    }

    @Test
    fun aLibraryTheProfileDoesNotSeeIsNotFound() = runTest(dispatcher) {
        repository.all = ApiResult.NotFound
        val vm = LibraryViewModel(session(backgroundScope), repository, 9, libraries)
        settle()
        assertEquals(Loadable.Failed(LoadError.NotFound), vm.state.value.items.items)
        assertNull(vm.state.value.library)
    }

    @Test
    fun theRowNewInOpensByAddedAndAudiobooksSortByAuthor() = runTest(dispatcher) {
        val vm = LibraryViewModel(session(backgroundScope), repository, 5, libraries, initialSort = LibrarySort.RELEASED)
        settle()
        assertEquals(LibrarySort.RELEASED, vm.state.value.filters.sort)
        assertTrue(vm.state.value.audio)
        assertTrue(LibrarySort.AUTHOR in vm.state.value.sorts)
        assertFalse(LibrarySort.VOTE in vm.state.value.sorts)
    }

    @Test
    fun aFailureCanBeTriedAgain() = runTest(dispatcher) {
        repository.all = ApiResult.Offline
        val vm = LibraryViewModel(session(backgroundScope), repository, 4, libraries)
        settle()
        assertEquals(Loadable.Failed(LoadError.Offline), vm.state.value.items.items)
        repository.all = ApiResult.Ok(films)
        vm.retry()
        settle()
        assertEquals(50, vm.state.value.items.loaded.size)
    }
}
