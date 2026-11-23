package com.slouchcat.app.mobile

import android.content.Context
import android.graphics.drawable.AdaptiveIconDrawable
import androidx.test.core.app.ApplicationProvider
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.xmlpull.v1.XmlPullParser
import java.io.File

/**
 * The launcher icon: the manifest names it as icon and round icon, it is adaptive with a monochrome
 * layer (themed icons), and its two colours are the tokens of web/public/icon.svg.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class LauncherIconTest {
    private val context = ApplicationProvider.getApplicationContext<Context>()

    @Test
    fun theManifestNamesTheAdaptiveIcon() {
        val icon = context.applicationInfo.icon
        assertTrue("the manifest names no android:icon", icon != 0)
        assertEquals("mipmap", context.resources.getResourceTypeName(icon))
        assertEquals("ic_launcher", context.resources.getResourceEntryName(icon))
        val drawable = context.packageManager.getApplicationIcon(context.applicationInfo)
        assertTrue("the icon is not adaptive: $drawable", drawable is AdaptiveIconDrawable)
        // The layers as compiled (Robolectric's drawable leaves the monochrome layer out).
        val layers = buildList {
            val xml = context.resources.getXml(icon)
            while (xml.next() != XmlPullParser.END_DOCUMENT) {
                if (xml.eventType == XmlPullParser.START_TAG) add(xml.name)
            }
        }
        assertEquals(listOf("adaptive-icon", "background", "foreground", "monochrome"), layers)
    }

    @Test
    fun theManifestNamesTheRoundIcon() {
        val manifest = File("src/main/AndroidManifest.xml").readText()
        assertTrue("no android:roundIcon", "android:roundIcon=\"@mipmap/ic_launcher\"" in manifest)
    }

    @Test
    fun theColoursAreTheTokens() {
        val colors = Json.parseToJsonElement(File("../../packages/design-tokens/tokens/color.json").readText()).jsonObject
        fun token(vararg path: String): String {
            var node: JsonObject = colors["color"]!!.jsonObject
            path.forEach { node = node[it]!!.jsonObject }
            return node["\$value"]!!.jsonPrimitive.content
        }
        assertEquals(listOf(token("bg", "base")), fills("ic_launcher_background"))
        assertEquals(listOf(token("accent", "ember", "base")), fills("ic_launcher_foreground"))
    }

    private fun fills(name: String): List<String> =
        Regex("android:fillColor=\"([^\"]+)\"").findAll(File("src/main/res/drawable/$name.xml").readText())
            .map { it.groupValues[1] }.toList()
}
