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
 * layer (themed icons), its two colours are the tokens of web/public/icon.svg, and its sign is the
 * one of web/public/icon.svg, the middle of that canvas on the middle of the layer.
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

    /**
     * Both layers with a sign draw the two paths of the web icon, and the group around them puts the
     * middle of the web icon's canvas (512 x 512) on the middle of the layer (108 x 108). The web
     * icon has its sign in the middle of its canvas (web/src/ui/logoMark.test.ts), so the launcher
     * shows it in the middle too, whatever mask it cuts.
     */
    @Test
    fun theSignIsTheWebIconInTheMiddleOfTheLayer() {
        val webIcon = File("../../web/public/icon.svg").readText()
        val canvas = Regex("viewBox=\"0 0 (\\d+) (\\d+)\"").find(webIcon)!!.groupValues.drop(1).map { it.toFloat() }
        val paths = Regex("<path d=\"([^\"]+)\"").findAll(webIcon).map { it.groupValues[1] }.toList()
        assertEquals("the head and the play triangle", 2, paths.size)
        for (layer in listOf("ic_launcher_foreground", "ic_launcher_monochrome")) {
            val xml = File("src/main/res/drawable/$layer.xml").readText()
            fun number(name: String) = Regex("android:$name=\"([^\"]+)\"").find(xml)!!.groupValues[1].toFloat()
            assertEquals("$layer: the paths", paths.joinToString(" "), Regex("android:pathData=\"([^\"]+)\"").find(xml)!!.groupValues[1])
            assertEquals("$layer: middle, x", number("viewportWidth") / 2, number("translateX") + canvas[0] / 2 * number("scaleX"), 0.001f)
            assertEquals("$layer: middle, y", number("viewportHeight") / 2, number("translateY") + canvas[1] / 2 * number("scaleY"), 0.001f)
        }
    }

    private fun fills(name: String): List<String> =
        Regex("android:fillColor=\"([^\"]+)\"").findAll(File("src/main/res/drawable/$name.xml").readText())
            .map { it.groupValues[1] }.toList()
}
