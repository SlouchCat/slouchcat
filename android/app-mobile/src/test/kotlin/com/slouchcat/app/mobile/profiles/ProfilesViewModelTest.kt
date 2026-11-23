package com.slouchcat.app.mobile.profiles

import com.slouchcat.app.mobile.common.SessionHooks
import com.slouchcat.app.mobile.testing.FakeAuth
import com.slouchcat.app.mobile.testing.actions
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.sessionInfo
import com.slouchcat.core.browse.testing.settle
import com.slouchcat.core.browse.testing.summary
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class ProfilesViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val auth = FakeAuth()
    private val hooks = SessionHooks()
    private var cleaned = 0

    @Before
    fun main() {
        Dispatchers.setMain(dispatcher)
        hooks.onProfileOrSessionEnd { cleaned++ }
    }

    @After
    fun reset() = Dispatchers.resetMain()

    private fun TestScope.model(): Pair<ProfilesViewModel, SessionManager> {
        val session = session(backgroundScope, profile = null)
        settle()
        // The lock runs on the test clock (SystemClock answers 0 on the plain JVM).
        val vm = ProfilesViewModel(session, auth, actions(session, auth, hooks), clock = { testScheduler.currentTime })
        settle()
        return vm to session
    }

    @Test
    fun theProfilesOfTheAccountAreShown() = runTest(dispatcher) {
        val (vm, _) = model()
        val ready = vm.state.value.profiles as Loadable.Ready
        assertEquals(listOf("Primary", "Child"), ready.value.map { it.name })
        assertEquals(EMAIL, vm.state.value.email)
        assertEquals("192.0.2.10", vm.state.value.host)
    }

    @Test
    fun aProfileWithoutPinOpensAtOnceAndClearsWhatThePreviousLeft() = runTest(dispatcher) {
        val (vm, session) = model()
        val epoch = session.epoch.value
        vm.onPick(summary(1, "Primary", primary = true))
        settle()
        assertEquals("a new session epoch opens the first page", epoch + 1, session.epoch.value)
        assertEquals(listOf("profiles", "select:1:-"), auth.calls)
        assertEquals(1, cleaned)
        val ready = session.state.value as SessionState.Ready
        assertEquals(1L, ready.profile.id)
        assertTrue(ready.profile.isPrimary)
    }

    /** Stage 7 M-2: the sound's last progress and its end go out before the server opens another profile. */
    @Test
    fun anotherProfileWaitsForTheHooksBeforeTheChange() = runTest(dispatcher) {
        hooks.onBeforeProfileChange { auth.calls += "flush" }
        val session = session(backgroundScope)
        settle()
        val vm = ProfilesViewModel(session, auth, actions(session, auth, hooks), clock = { testScheduler.currentTime })
        settle()
        vm.onPick(summary(2, "Child"))
        settle()
        assertEquals(listOf("flush", "select:2:-"), auth.calls.filter { it != "profiles" })
    }

    @Test
    fun thePinPadTakesFourToEightDigits() = runTest(dispatcher) {
        val (vm, _) = model()
        vm.onPick(summary(2, "Child", hasPin = true))
        val pin = vm.state.value.pin!!
        assertEquals("Child", pin.profile.name)
        "123".forEach(vm::onDigit)
        vm.onSubmitPin()
        settle()
        assertEquals(listOf("profiles"), auth.calls)
        "456789".forEach(vm::onDigit)
        assertEquals("12345678", vm.state.value.pin!!.digits)
        vm.onDelete()
        assertEquals("1234567", vm.state.value.pin!!.digits)
        vm.onCancelPin()
        assertNull(vm.state.value.pin)
    }

    @Test
    fun aWrongPinEmptiesThePadAndTheRightOneOpensTheProfile() = runTest(dispatcher) {
        val (vm, session) = model()
        auth.select = { _, pin -> if (pin == "246810") ApiResult.Ok(sessionInfo()) else ApiResult.NotAllowed("pin_invalid") }
        vm.onPick(summary(2, "Child", hasPin = true))
        "1111".forEach(vm::onDigit)
        vm.onSubmitPin()
        settle()
        assertEquals(PinMessage.Invalid, vm.state.value.pin!!.message)
        assertEquals("", vm.state.value.pin!!.digits)
        "246810".forEach(vm::onDigit)
        vm.onSubmitPin()
        settle()
        assertNull(vm.state.value.pin)
        assertEquals(2L, (session.state.value as SessionState.Ready).profile.id)
    }

    @Test
    fun theLockOfTheServerIsCountedDown() = runTest(dispatcher) {
        val (vm, _) = model()
        auth.select = { _, _ -> ApiResult.Failed(429, "pin_locked", 3) }
        vm.onPick(summary(2, "Child", hasPin = true))
        "1111".forEach(vm::onDigit)
        vm.onSubmitPin()
        testScheduler.runCurrent()
        assertEquals(PinMessage.Locked, vm.state.value.pin!!.message)
        assertEquals(3L, vm.state.value.pin!!.lockedSeconds)
        // Locked: the pad takes nothing.
        vm.onDigit('1')
        assertEquals("", vm.state.value.pin!!.digits)
        advanceTimeBy(2_001)
        assertEquals(1L, vm.state.value.pin!!.lockedSeconds)
        advanceTimeBy(1_000)
        assertEquals(PinMessage.Again, vm.state.value.pin!!.message)
        vm.onDigit('1')
        assertEquals("1", vm.state.value.pin!!.digits)
    }

    @Test
    fun aProfileThatGotAPinMeanwhileAsksForIt() = runTest(dispatcher) {
        val (vm, _) = model()
        auth.select = { _, _ -> ApiResult.NotAllowed("pin_required") }
        vm.onPick(summary(1, "Primary", primary = true))
        settle()
        assertEquals(1L, vm.state.value.pin!!.profile.id)
    }

    @Test
    fun noAnswerIsSaid() = runTest(dispatcher) {
        val (vm, _) = model()
        auth.select = { _, _ -> ApiResult.Offline }
        vm.onPick(summary(1, "Primary", primary = true))
        settle()
        assertEquals(ProfilesError.Unreachable, vm.state.value.error)
        auth.profiles = ApiResult.Offline
        vm.retry()
        settle()
        assertTrue(vm.state.value.profiles is Loadable.Failed)
    }

    @Test
    fun signingOutFromThePickerEndsTheSession() = runTest(dispatcher) {
        val (vm, session) = model()
        vm.onSignOut()
        settle()
        assertTrue(session.state.value is SessionState.SignedOut)
        assertTrue("logout" in auth.calls)
    }
}
