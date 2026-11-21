package com.slouchcat.app.mobile.connected

import android.content.Context
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/** «Verbunden mit …» in its four states, at 412 dp and 1280 dp. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class ConnectedScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()

    private fun show(state: ConnectedUiState) = compose.setContent {
        SlouchcatTheme { ConnectedScreen(state, onRetry = { events += "retry" }, onChangeAddress = { events += "change" }) }
    }

    private fun states() {
        show(ConnectedUiState(Loadable.Ready(ConnectedServer("https://media.example.test/".toHttpUrl(), "1.42.0"))))
        compose.onNodeWithText(s(R.string.app_connected_title, "media.example.test")).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_setup_encrypted), substring = true).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithText(s(R.string.app_connected_change)).performScrollTo().performClick()
        assertEquals(listOf("change"), events)
    }

    private fun failed() {
        show(ConnectedUiState(Loadable.Failed(LoadError.Offline)))
        compose.onNodeWithText(s(R.string.app_state_offline)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithText(s(R.string.app_state_retry)).performClick()
        assertEquals(listOf("retry"), events)
    }

    private fun loading() {
        show(ConnectedUiState(Loadable.Loading))
        compose.onNodeWithContentDescription(s(R.string.app_state_loading)).assertIsDisplayed()
    }

    private fun empty() {
        show(ConnectedUiState(Loadable.Empty))
        compose.onNodeWithText(s(R.string.app_connectedState_empty)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_connectedState_setUp)).performClick()
        assertEquals(listOf("change"), events)
    }

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun readyPhone() = states()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun readyTablet() = states()

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun failedPhone() = failed()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun failedTablet() = failed()

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun loadingPhone() = loading()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun loadingTablet() = loading()

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun emptyPhone() = empty()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun emptyTablet() = empty()
}
