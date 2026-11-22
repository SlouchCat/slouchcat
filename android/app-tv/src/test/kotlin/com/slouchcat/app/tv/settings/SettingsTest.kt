package com.slouchcat.app.tv.settings

import androidx.activity.ComponentActivity
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performImeAction
import androidx.compose.ui.test.performTextReplacement
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.tv.common.MemoryRelaySettings
import com.slouchcat.app.tv.common.RelaySettings
import com.slouchcat.app.tv.common.TvSessionActions
import com.slouchcat.app.tv.common.TvSessionHooks
import com.slouchcat.app.tv.testing.Dpad
import com.slouchcat.app.tv.testing.FakeAuth
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.HTTP_SERVER
import com.slouchcat.core.browse.testing.session
import com.slouchcat.core.browse.testing.settle
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/** «Einstellungen»: the rows, their panes, the menus with the D-pad, and signing the TV out. */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class SettingsTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val remote = Dpad(compose)
    private val context = ApplicationProvider.getApplicationContext<android.content.Context>()
    private val calls = mutableListOf<String>()
    private val state = SettingsUiState(
        server = HTTP_SERVER,
        email = EMAIL,
        profileName = "Lea",
        profileAccent = "sky",
        tvName = "Shield · Android TV 11",
        appVersion = "0.1.0",
        unconfirmed = true,
    )

    private fun s(id: Int, vararg args: Any) = context.getString(id, *args)

    private fun show(extra: List<SettingsRow> = emptyList()) = remote.show {
        SettingsScreen(
            state = state,
            extra = extra,
            onSwitchProfile = { calls += "profiles" },
            onSignOut = { calls += "signout" },
            onRelay = { calls += "relay:${it ?: "default"}" },
        )
    }

    @Test
    fun theRowsAreReachedAndThePaneFollowsTheFocus() {
        show(listOf(SettingsRow("play", 30, "Wiedergabe", "Automatisch", com.slouchcat.core.design.SlouchcatIcon.Play.vector, { calls += "play" })))
        remote.assertFocused(s(R.string.tv_shell_switchProfile))
        compose.onNodeWithText(s(R.string.tv_settings_switchPane)).assertExists()
        remote.assertEveryControlReachable()
        // A row of a later stage sits by its order.
        remote.down()
        remote.assertFocused("Wiedergabe")
        remote.ok()
        remote.down()
        remote.assertFocused(s(R.string.tv_settings_server))
        // The pane: server, connection, account, this TV, the note about HTTP and the new key.
        compose.onNodeWithText(s(R.string.tv_settings_insecure)).assertExists()
        compose.onNodeWithText(s(R.string.tv_settings_unconfirmedBody)).assertExists()
        compose.onNodeWithText("Shield · Android TV 11").assertExists()
        remote.up(2)
        remote.ok()
        assertEquals(listOf("play", "profiles"), calls)
    }

    @Test
    fun theServerMenuLeadsToTheQuestionAndBackClosesBoth() {
        show()
        remote.down()
        remote.ok()
        remote.assertFocused(s(R.string.common_close))
        val inMenu = hasAnyAncestor(SemanticsMatcher.keyIsDefined(SemanticsProperties.PaneTitle))
        remote.assertEveryControlReachable(scope = inMenu)
        remote.down()
        remote.assertFocused(s(R.string.tv_settings_signOut))
        remote.ok()
        compose.onNodeWithText(s(R.string.tv_settings_signOutBody)).assertExists()
        // The question starts on «Abbrechen».
        remote.assertFocused(s(R.string.common_cancel))
        remote.back()
        remote.assertFocused(s(R.string.common_close))
        remote.down()
        remote.ok()
        remote.left()
        remote.assertFocused(s(R.string.tv_settings_signOutConfirm))
        remote.ok()
        assertEquals(listOf("signout"), calls)
    }

    @Test
    fun aboutAndTheRelayAddress() {
        show()
        remote.down(2)
        remote.ok()
        // In the menu and in the pane behind it.
        assertEquals(2, compose.onAllNodesWithText(s(R.string.tv_settings_licenseValue)).fetchSemanticsNodes().size)
        assertEquals(2, compose.onAllNodesWithText(s(R.string.tv_settings_sourceValue)).fetchSemanticsNodes().size)
        remote.back()
        remote.assertFocused(s(R.string.tv_settings_about))
        remote.down()
        remote.ok()
        remote.assertFocused(s(R.string.tv_relay_label))
        // Back closes the menu and returns to its row.
        remote.back()
        remote.assertFocused(s(R.string.tv_settings_advanced))
        remote.ok()
        remote.assertFocused(s(R.string.tv_relay_label))
        remote.ok()
        compose.onNode(hasSetTextAction()).performTextReplacement("http://10.0.2.2:8180")
        compose.onNode(hasSetTextAction()).performImeAction()
        remote.settle()
        remote.down()
        remote.assertFocused(s(R.string.tv_relay_save))
        remote.ok()
        assertEquals("relay:http://10.0.2.2:8180/", calls.last())
        remote.assertFocused(s(R.string.tv_settings_advanced))
    }

    @OptIn(ExperimentalCoroutinesApi::class)
    @Test
    fun signingOutForgetsEverythingAndSaysWhetherTheServerConfirmed() {
        val dispatcher = StandardTestDispatcher()
        Dispatchers.setMain(dispatcher)
        try {
            runTest(dispatcher) {
                val servers = FakeServers(HTTP_SERVER)
                val auth = FakeAuth()
                val session = session(backgroundScope, servers = servers)
                val relay = MemoryRelaySettings()
                val outcomes = mutableListOf<Boolean>()
                val newKey = MutableStateFlow<String?>(null)
                val vm = SettingsViewModel(session, TvSessionActions(session, servers, auth, TvSessionHooks()), relay, newKey, "TV", "0.1.0", { outcomes += it }, backgroundScope)
                settle()
                assertEquals("Primary", vm.state.value.profileName)
                assertEquals(HTTP_SERVER, vm.state.value.server)
                newKey.value = "other"
                settle()
                assertTrue(vm.state.value.unconfirmed)
                vm.onRelay("http://10.0.2.2:8180".toHttpUrl())
                settle()
                assertEquals("http://10.0.2.2:8180/".toHttpUrl(), vm.state.value.relay)
                vm.onRelay(null)
                settle()
                assertEquals(RelaySettings.DEFAULT, vm.state.value.relay)

                auth.logout = ApiResult.Offline
                vm.onSignOut()
                settle()
                assertEquals(listOf(false), outcomes)
                assertEquals(SessionState.NoServer, session.state.value)
                assertNull(servers.flow.value)
                assertNull(session.bearerToken)
            }
        } finally {
            Dispatchers.resetMain()
        }
    }
}
