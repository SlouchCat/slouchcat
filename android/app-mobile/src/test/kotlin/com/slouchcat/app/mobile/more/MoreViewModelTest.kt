package com.slouchcat.app.mobile.more

import com.slouchcat.app.mobile.common.SessionHooks
import com.slouchcat.app.mobile.common.SignOutQuestion
import com.slouchcat.app.mobile.common.SignOutStep
import com.slouchcat.app.mobile.testing.EMAIL
import com.slouchcat.app.mobile.testing.FakeAuth
import com.slouchcat.app.mobile.testing.FakeTokens
import com.slouchcat.app.mobile.testing.HTTP_SERVER
import com.slouchcat.app.mobile.testing.PRIMARY
import com.slouchcat.app.mobile.testing.actions
import com.slouchcat.app.mobile.testing.session
import com.slouchcat.app.mobile.testing.settle
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.StandardTestDispatcher
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
class MoreViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val auth = FakeAuth()
    private val hooks = SessionHooks()
    private val tokens = FakeTokens(StoredCredentials("token", "device-1"))

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    /** What stage 6 will register: a question and a step that runs while the token still works. */
    private class Downloads(val tokens: FakeTokens) : SignOutStep {
        var ran: Boolean? = null
        var tokenAtRun: String? = null
        override suspend fun question() = SignOutQuestion(R.string.nav_downloads, default = true)
        override suspend fun beforeSignOut(answer: Boolean) {
            ran = answer
            tokenAtRun = tokens.stored?.token
        }
    }

    @Test
    fun showsProfileAccountAndServer() = runTest(dispatcher) {
        val session = session(backgroundScope, tokens = tokens)
        settle()
        val keys = com.slouchcat.app.mobile.testing.FakeKeys()
        val vm = MoreViewModel(session, hooks, actions(session, auth, hooks), keys)
        settle()
        assertEquals(PRIMARY, vm.state.value.profile)
        assertEquals(EMAIL, vm.state.value.email)
        assertEquals(HTTP_SERVER, vm.state.value.server)
        vm.onServer()
        assertEquals(MoreSheet.Server, vm.state.value.sheet)
        vm.onDismissSheet()
        assertNull(vm.state.value.sheet)
        // No new key: no check to open.
        vm.onKeyCheck()
        assertNull(vm.state.value.sheet)
    }

    @Test
    fun aNewKeyIsConfirmedOnlyAfterTheOwnNetworkIsStated() = runTest(dispatcher) {
        val session = session(backgroundScope, tokens = tokens)
        settle()
        val keys = com.slouchcat.app.mobile.testing.FakeKeys("new-key")
        val vm = MoreViewModel(session, hooks, actions(session, auth, hooks), keys)
        settle()
        assertEquals("new-key", vm.state.value.newKey)
        // Nothing opens by itself (ADR 0040).
        assertNull(vm.state.value.sheet)
        vm.onKeyCheck()
        assertEquals(MoreSheet.KeyCheck, vm.state.value.sheet)
        val confirm = backgroundScope.launch { vm.confirmKey.first() }
        vm.onConfirmKey()
        settle()
        assertTrue("confirmed without the own network", !confirm.isCompleted)
        vm.onOwnNetwork(true)
        vm.onConfirmKey()
        settle()
        assertTrue(confirm.isCompleted)
        assertNull(vm.state.value.sheet)
    }

    @Test
    fun signingOutAsksTheStepsRunsThemFirstAndKeepsServerAndDevice() = runTest(dispatcher) {
        val step = Downloads(tokens)
        hooks.addSignOutStep(step)
        var cleaned = 0
        hooks.onProfileOrSessionEnd { cleaned++ }
        val session = session(backgroundScope, tokens = tokens)
        settle()
        val vm = MoreViewModel(session, hooks, actions(session, auth, hooks), com.slouchcat.app.mobile.testing.FakeKeys())
        settle()
        vm.onSignOutAsk()
        settle()
        assertEquals(MoreSheet.SignOut, vm.state.value.sheet)
        assertEquals(1, vm.state.value.choices.size)
        vm.onChoice(0, false)
        val epoch = session.epoch.value
        vm.onSignOut()
        settle()
        assertEquals("a new session epoch", epoch + 1, session.epoch.value)
        assertEquals(false, step.ran)
        assertEquals("the step ran before the token went", "token", step.tokenAtRun)
        assertEquals(listOf("logout"), auth.calls)
        assertEquals(1, cleaned)
        assertEquals(StoredCredentials(null, "device-1"), tokens.stored)
        assertEquals(SessionState.SignedOut(HTTP_SERVER), session.state.value)
    }

    @Test
    fun withoutAnAnswerOfTheServerTheAppStillSignsOutHere() = runTest(dispatcher) {
        auth.logout = ApiResult.Offline
        val session = session(backgroundScope, tokens = tokens)
        settle()
        val vm = MoreViewModel(session, hooks, actions(session, auth, hooks), com.slouchcat.app.mobile.testing.FakeKeys())
        vm.onSignOut()
        settle()
        assertTrue(session.state.value is SessionState.SignedOut)
    }
}
