package com.slouchcat.app.mobile.devices

import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.testing.CHILD
import com.slouchcat.app.mobile.testing.FakeDevices
import com.slouchcat.app.mobile.testing.device
import com.slouchcat.app.mobile.testing.session
import com.slouchcat.app.mobile.testing.settle
import com.slouchcat.core.api.ApiResult
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.async
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
class DevicesViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val mine = device("a", "Pixel 9 · Android 16", current = true)
    private val tv = device("b", "Living room", type = com.slouchcat.core.api.generated.models.MyDevice.Type.TV, kind = com.slouchcat.core.api.generated.models.SessionKind.DEVICE)
    private val repository = FakeDevices(ApiResult.Ok(listOf(mine, tv)))

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    @Test
    fun theListOfTheAccountWithTheOwnDevice() = runTest(dispatcher) {
        val s = session(backgroundScope)
        settle()
        val vm = DevicesViewModel(s, repository)
        settle()
        assertEquals(listOf(mine, tv), (vm.state.value.devices as Loadable.Ready).value)
        assertTrue(vm.state.value.canSignOut)
    }

    @Test
    fun anotherDeviceIsSignedOutAfterTheQuestion() = runTest(dispatcher) {
        val s = session(backgroundScope)
        settle()
        val vm = DevicesViewModel(s, repository)
        settle()
        // The own device is signed out on «Mehr», not here.
        vm.onAsk(mine)
        assertNull(vm.state.value.asking)
        vm.onAsk(tv)
        assertEquals(tv, vm.state.value.asking)
        val name = async { vm.signedOut.first() }
        repository.list = ApiResult.Ok(listOf(mine))
        vm.onConfirm("Unnamed")
        settle()
        assertEquals("Living room", name.await())
        assertEquals(listOf("b"), repository.signedOut)
        assertNull(vm.state.value.asking)
        assertEquals(listOf(mine), (vm.state.value.devices as Loadable.Ready).value)
    }

    @Test
    fun anotherProfileOnlyReads() = runTest(dispatcher) {
        val s = session(backgroundScope, profile = CHILD)
        settle()
        val vm = DevicesViewModel(s, repository)
        settle()
        assertFalse(vm.state.value.canSignOut)
        vm.onAsk(tv)
        assertNull(vm.state.value.asking)
    }

    @Test
    fun aRefusalOfTheServerStaysInTheSheet() = runTest(dispatcher) {
        val s = session(backgroundScope)
        settle()
        val vm = DevicesViewModel(s, repository)
        settle()
        repository.signOut = ApiResult.NotAllowed("primary_profile_required")
        vm.onAsk(tv)
        vm.onConfirm("Unnamed")
        settle()
        assertEquals(DeviceSignOutError.PrimaryOnly, vm.state.value.error)
        assertEquals(tv, vm.state.value.asking)
        repository.signOut = ApiResult.Offline
        vm.onConfirm("Unnamed")
        settle()
        assertEquals(DeviceSignOutError.Unreachable, vm.state.value.error)
    }

    @Test
    fun aFailedLoadOffersRetry() = runTest(dispatcher) {
        repository.list = ApiResult.Offline
        val s = session(backgroundScope)
        settle()
        val vm = DevicesViewModel(s, repository)
        settle()
        assertEquals(Loadable.Failed(LoadError.Offline), vm.state.value.devices)
        repository.list = ApiResult.Ok(emptyList())
        vm.retry()
        settle()
        assertEquals(Loadable.Empty, vm.state.value.devices)
    }
}
