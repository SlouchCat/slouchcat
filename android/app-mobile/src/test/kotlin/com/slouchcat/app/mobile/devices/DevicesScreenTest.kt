package com.slouchcat.app.mobile.devices

import android.content.Context
import androidx.compose.material3.SnackbarHostState
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.app.mobile.testing.device
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.api.generated.models.SessionKind
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private const val PHONE = "w412dp-h915dp"
private const val TABLET = "w1280dp-h800dp-land"

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/** «Meine Geräte» (rule 8): two widths, 200 % font, every control labelled. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class DevicesScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val mine = device("a", "Pixel 9 · Android 16", current = true)
    private val tv = device("b", "Living room", type = MyDevice.Type.TV, kind = SessionKind.DEVICE)
    private val old = device("c", null, type = MyDevice.Type.DESKTOP, kind = null, downloads = 3)

    private fun show(state: DevicesUiState) = compose.setContent {
        SlouchcatTheme {
            DevicesScreen(
                state = state,
                snackbar = SnackbarHostState(),
                onBack = { events += "back" },
                onRetry = { events += "retry" },
                onAsk = { events += "ask:${it.id}" },
                onConfirm = { events += "confirm:$it" },
                onDismiss = { events += "dismiss" },
            )
        }
    }

    private fun list() {
        show(DevicesUiState(Loadable.Ready(listOf(mine, tv, old)), canSignOut = true))
        compose.onNodeWithText(s(R.string.app_devices_lede)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.settings_devices_thisDevice)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.settings_devices_downloadsOnly)).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(s(R.string.settings_devices_unnamed)).performScrollTo().assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        // Only the other devices can be signed out here.
        compose.onNodeWithContentDescription(s(R.string.settings_devices_signOut_label, "Pixel 9 · Android 16")).assertDoesNotExist()
        compose.onNodeWithContentDescription(s(R.string.settings_devices_signOut_label, "Living room")).performScrollTo().performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("ask:b", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun listPhone() = list()

    @Test @Config(qualifiers = TABLET)
    fun listTablet() = list()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun listPhoneLargeFont() = list()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun listTabletLargeFont() = list()

    private fun readOnly() {
        show(DevicesUiState(Loadable.Ready(listOf(mine, tv)), canSignOut = false))
        compose.onNodeWithText(s(R.string.settings_devices_onlyPrimary_body)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.settings_devices_signOut_action)).assertDoesNotExist()
        compose.assertEveryControlLabelledAndTappable()
    }

    @Test @Config(qualifiers = PHONE)
    fun readOnlyPhone() = readOnly()

    @Test @Config(qualifiers = TABLET)
    fun readOnlyTablet() = readOnly()

    private fun ask() {
        show(DevicesUiState(Loadable.Ready(listOf(mine, tv)), canSignOut = true, asking = tv, error = DeviceSignOutError.PrimaryOnly))
        compose.onNodeWithText(s(R.string.settings_devices_signOut_title, "Living room")).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.errors_code_primary_profile_required)).performScrollTo().assertIsDisplayed()
        compose.onNodeWithText(s(R.string.settings_devices_signOut_confirm)).performScrollTo().performClick()
        compose.onNodeWithText(s(R.string.common_cancel)).performScrollTo().performClick()
        assertEquals(listOf("confirm:${s(R.string.settings_devices_unnamed)}", "dismiss"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun askPhone() = ask()

    @Test @Config(qualifiers = TABLET)
    fun askTablet() = ask()

    private fun empty() {
        show(DevicesUiState(Loadable.Empty, canSignOut = true))
        compose.onNodeWithText(s(R.string.settings_devices_none)).assertIsDisplayed()
    }

    @Test @Config(qualifiers = PHONE)
    fun emptyPhone() = empty()

    @Test @Config(qualifiers = TABLET)
    fun emptyTablet() = empty()
}
