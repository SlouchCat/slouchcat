package com.slouchcat.core.browse.person

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.browse.testing.FakePeople
import com.slouchcat.core.browse.testing.person
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.settle
import com.slouchcat.core.browse.testing.ulid
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class PersonViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private fun TestScope.model(repository: FakePeople, id: Long = 101): PersonViewModel {
        val s = session(backgroundScope)
        settle()
        val vm = PersonViewModel(s, repository, id)
        settle()
        return vm
    }

    @Test
    fun aPersonWithOneCardPerTitleAndEveryRoleInIt() = runTest(dispatcher) {
        val repository = FakePeople()
        val vm = model(repository)
        val page = (vm.state.value.person as Loadable.Ready).value
        assertEquals(listOf(101L), repository.asked)
        assertEquals("Ann Actor", page.person.name)
        assertEquals(listOf(ulid(1), ulid(2)), page.titles.map { it.item.id })
        assertEquals(2, page.titles.first().roles.size)
    }

    @Test
    fun aPersonWithoutVisibleTitlesIsNotFound() = runTest(dispatcher) {
        val vm = model(FakePeople(ApiResult.NotFound))
        assertEquals(Loadable.Failed(LoadError.NotFound), vm.state.value.person)
    }

    @Test
    fun aFailedLoadIsRetried() = runTest(dispatcher) {
        val repository = FakePeople(ApiResult.Offline)
        val vm = model(repository)
        assertEquals(Loadable.Failed(LoadError.Offline), vm.state.value.person)
        repository.answer = ApiResult.Ok(person())
        vm.retry()
        assertEquals(Loadable.Loading, vm.state.value.person)
        settle()
        assertTrue(vm.state.value.person is Loadable.Ready)
    }

    @Test
    fun theBiographyOpensAndCloses() = runTest(dispatcher) {
        val vm = model(FakePeople())
        assertFalse(vm.state.value.biographyOpen)
        vm.onToggleBiography()
        assertTrue(vm.state.value.biographyOpen)
    }
}
