package com.slouchcat.tools

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

class StringsGeneratorTest {
    private fun gen(vararg languages: Pair<String, String>, namespaces: List<String> = listOf("app")) =
        StringsGenerator.generate(languages.associate { (code, json) -> code to StringsGenerator.load(json) }, namespaces)

    @Test
    fun keysBecomeResourceNamesAndLanguagesFolders() {
        val result = gen(
            "en" to """{"app":{"setup":{"title":"Connect","address-label":"Address"}},"admin":{"x":"X"}}""",
            "de" to """{"app":{"setup":{"title":"Verbinden"}}}""",
            "de-DE" to """{"app":{"setup":{"title":"Verbinden"}}}""",
            "pt-BR" to """{"app":{"setup":{"title":"Conectar"}}}""",
        )
        assertEquals(setOf("values", "values-de", "values-de-rDE", "values-pt-rBR"), result.files.keys)
        val en = result.files.getValue("values")
        assertTrue(en, "<string name=\"app_setup_title\">Connect</string>" in en)
        assertTrue(en, "<string name=\"app_setup_address_label\">Address</string>" in en)
        assertFalse("namespaces not listed stay out", "admin_x" in en)
    }

    @Test
    fun placeholdersTakeTheSamePositionsInEveryLanguage() {
        val result = gen(
            "en" to """{"app":{"t":"{{host}} runs {{version}}","n_one":"{{count}} title in {{lib}}","n_other":"{{count}} titles in {{lib}}"}}""",
            "de" to """{"app":{"t":"Version {{version}} auf {{host}}","n_one":"{{count}} Titel in {{lib}}","n_other":"{{count}} Titel in {{lib}}"}}""",
        )
        val de = result.files.getValue("values-de")
        assertTrue(de, "<string name=\"app_t\">Version %2\$s auf %1\$s</string>" in de)
        assertTrue(de, "<plurals name=\"app_n\">" in de)
        assertTrue(de, "<item quantity=\"one\">%1\$d Titel in %2\$s</item>" in de)
        assertTrue(de, "<item quantity=\"other\">%1\$d Titel in %2\$s</item>" in de)
    }

    @Test
    fun countFirstAndNumberFormatsAsText() {
        val result = gen(
            "en" to """{"app":{"s_one":"{{name}}: {{count, number}} file","s_other":"{{name}}: {{count, number}} files"}}""",
        )
        val en = result.files.getValue("values")
        assertTrue(en, "<item quantity=\"other\">%2\$s: %1\$s files</item>" in en)
    }

    @Test
    fun zeroBecomesATextOfItsOwn() {
        val result = gen(
            "en" to """{"app":{"n_zero":"No titles","n_one":"One title","n_other":"{{count}} titles"}}""",
        )
        val en = result.files.getValue("values")
        assertTrue(en, "<string name=\"app_n_zero\">No titles</string>" in en)
        assertFalse(en, "quantity=\"zero\"" in en)
        assertTrue(en, "<item quantity=\"one\">One title</item>" in en)
    }

    @Test
    fun pluralFormsEnglishLacksBelongToTheGroup() {
        val result = gen(
            "en" to """{"app":{"n_one":"{{count}} title","n_other":"{{count}} titles"}}""",
            "ru" to """{"app":{"n_one":"{{count}} фильм","n_few":"{{count}} фильма","n_many":"{{count}} фильмов","n_other":"{{count}} фильма"}}""",
        )
        val ru = result.files.getValue("values-ru")
        assertTrue(ru, "<item quantity=\"few\">%1\$d фильма</item>" in ru)
        assertTrue(ru, "<item quantity=\"many\">%1\$d фильмов</item>" in ru)
    }

    @Test
    fun aRegionalOverlayGetsTheMissingPluralFormsOfItsBaseLanguage() {
        val result = gen(
            "en" to """{"app":{"n_one":"{{count}} title","n_other":"{{count}} titles"}}""",
            "de" to """{"app":{"n_one":"{{count}} Titel gross","n_other":"{{count}} Titel gross"}}""",
            "de-DE" to """{"app":{"n_one":"{{count}} Titel groß"}}""",
        )
        val deDe = result.files.getValue("values-de-rDE")
        assertTrue(deDe, "<item quantity=\"one\">%1\$d Titel groß</item>" in deDe)
        assertTrue(deDe, "<item quantity=\"other\">%1\$d Titel gross</item>" in deDe)
    }

    @Test
    fun aGroupWithoutOtherStops() {
        val e = assertThrows(StringsGenerator.Unsupported::class.java) {
            gen(
                "en" to """{"app":{"n_one":"{{count}} title","n_other":"{{count}} titles"}}""",
                "fr" to """{"app":{"n_one":"{{count}} titre"}}""",
            )
        }
        assertTrue(e.message, "fr: app.n: plural group without _other" in e.message!!)
    }

    @Test
    fun aBareKeyIsTheSingular() {
        val result = gen("en" to """{"app":{"n":"one title","n_other":"{{count}} titles"}}""")
        val en = result.files.getValue("values")
        assertTrue(en, "<item quantity=\"one\">one title</item>" in en)
        assertFalse(en, "<string name=\"app_n\">" in en)
    }

    @Test
    fun zeroOnlyInATranslationStops() {
        val e = assertThrows(StringsGenerator.Unsupported::class.java) {
            gen(
                "en" to """{"app":{"n_one":"{{count}} title","n_other":"{{count}} titles"}}""",
                "de" to """{"app":{"n_zero":"Keine","n_one":"{{count}} Titel","n_other":"{{count}} Titel"}}""",
            )
        }
        assertTrue(e.message, "de: app.n_zero: _zero without the same text in en.json" in e.message!!)
    }

    @Test
    fun escapesWhatAapt2Reads() {
        val result = gen(
            "en" to """{"app":{"q":"It's \"done\" & a < b > c 100%","sep":" · ","at":"@home","lines":"a\nb","p":"{{n}} at 100%"}}""",
        )
        val en = result.files.getValue("values")
        assertTrue(en, "<string name=\"app_q\" formatted=\"false\">It\\'s \\\"done\\\" &amp; a &lt; b &gt; c 100%</string>" in en)
        assertTrue(en, "<string name=\"app_sep\">\\u0020·\\u0020</string>" in en)
        assertTrue(en, "<string name=\"app_at\">\\@home</string>" in en)
        assertTrue(en, "<string name=\"app_lines\">a\\nb</string>" in en)
        assertTrue(en, "<string name=\"app_p\">%1\$s at 100%%</string>" in en)
    }

    @Test
    fun stopsWithTheKeyOnWhatItCannotCarry() {
        fun fails(json: String, expected: String) {
            val e = assertThrows(StringsGenerator.Unsupported::class.java) { gen("en" to json) }
            assertTrue(e.message, expected in e.message!!)
        }
        fails("""{"app":{"a":"See ${'$'}t(app.b)"}}""", "en: app.a: nested \$t()")
        fails("""{"app":{"a":"Open <path>here</path>"}}""", "en: app.a: markup")
        fails("""{"app":{"a":"{{when, datetime}}"}}""", "en: app.a: format 'datetime'")
        fails("""{"app":{"a":["x"]}}""", "en: app.a: not a text")
        fails("""{"other":{"a":"x"}}""", "namespace 'app' does not exist")
        val e = assertThrows(StringsGenerator.Unsupported::class.java) {
            gen("en" to """{"app":{"a":"A"}}""", "de" to """{"app":{"b":"B"}}""")
        }
        assertTrue(e.message, "de: app.b: not in en.json" in e.message!!)
        val clash = assertThrows(StringsGenerator.Unsupported::class.java) { gen("en" to """{"app":{"a-b":"x","a_b":"y"}}""") }
        assertTrue(clash.message, "resource name 'app_a_b'" in clash.message!!)
    }

    @Test
    fun aPartOfANamespaceTakesOnlyItsTexts() {
        val result = gen(
            "en" to """{"settings":{"devices":{"title":"My devices"},"playback":{"title":"Playback"}}}""",
            namespaces = listOf("settings.devices"),
        )
        val en = result.files.getValue("values")
        assertTrue(en.contains("settings_devices_title"))
        assertFalse(en.contains("settings_playback_title"))
        val missing = assertThrows(StringsGenerator.Unsupported::class.java) {
            gen("en" to """{"settings":{}}""", namespaces = listOf("settings.devices"))
        }
        assertTrue(missing.message!!.contains("settings.devices"))
    }

    @Test
    fun namespacesFileIgnoresCommentsAndBlankLines() {
        assertEquals(listOf("app", "player"), StringsGenerator.readNamespaces("# comment\napp\n\n player # inline\n"))
    }

    /**
     * Every language file of the repository carries the namespaces of the viewer pages (stage 0,
     * point 8): the 22 languages and de-DE without a stop, for the list the apps use today and for
     * the namespaces later pages will add.
     */
    @Test
    fun theRepositoryLocalesCarryTheViewerNamespaces() {
        val repo = File(System.getProperty("slouchcat.repo"))
        val files = File(repo, "web/src/locales").listFiles { f -> f.name.endsWith(".json") }!!
        val languages = files.associate { it.name.removeSuffix(".json") to StringsGenerator.load(it.readText()) }
        assertEquals(23, languages.size)
        val listed = StringsGenerator.readNamespaces(File(repo, "android/strings-namespaces.txt").readText())
        assertEquals(23, StringsGenerator.generate(languages, listed).files.size)
        val viewer = languages.getValue("en").keys - setOf("admin", "setup", "invite")
        val all = StringsGenerator.generate(languages, viewer.toList())
        assertTrue("${all.resources} resources", all.resources > 30_000)
    }
}
