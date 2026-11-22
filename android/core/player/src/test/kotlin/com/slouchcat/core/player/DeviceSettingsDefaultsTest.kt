package com.slouchcat.core.player

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/** The defaults of an app before the user chose (the phone keeps its caps, the TV asks for none). */
@RunWith(RobolectricTestRunner::class)
class DeviceSettingsDefaultsTest {
    private val context: Context = ApplicationProvider.getApplicationContext()

    @Test
    fun withoutAChoiceTheDefaultsOfTheAppHold() = runTest {
        val phone = DataStoreDeviceSettingsStore(context).settings.first()
        assertEquals(QualityChoice.Medium, phone.qualityMobile)
        assertEquals(QualityChoice.High, phone.qualityWifi)
        val tv = DataStoreDeviceSettingsStore(context, DeviceSettings(qualityMobile = QualityChoice.Auto, qualityWifi = QualityChoice.Auto)).settings.first()
        assertEquals(QualityChoice.Auto, tv.qualityMobile)
        assertEquals(QualityChoice.Auto, tv.qualityWifi)
        assertEquals(AudioSetting.Auto, tv.audio)
    }

    @Test
    fun aChoiceWinsOverTheDefaults() = runTest {
        val tv = DataStoreDeviceSettingsStore(context, DeviceSettings(qualityMobile = QualityChoice.Auto, qualityWifi = QualityChoice.Auto))
        tv.update { it.copy(qualityWifi = QualityChoice.Low, audio = AudioSetting.Manual(setOf("aac", "ac3"), 6)) }
        val now = tv.settings.first()
        assertEquals(QualityChoice.Low, now.qualityWifi)
        assertEquals(AudioSetting.Manual(setOf("aac", "ac3"), 6), now.audio)
    }
}
