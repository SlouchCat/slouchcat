package com.slouchcat.tools

import org.gradle.api.DefaultTask
import org.gradle.api.GradleException
import org.gradle.api.file.ConfigurableFileCollection
import org.gradle.api.file.DirectoryProperty
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.tasks.CacheableTask
import org.gradle.api.tasks.InputFile
import org.gradle.api.tasks.InputFiles
import org.gradle.api.tasks.OutputDirectory
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.TaskAction
import org.snakeyaml.engine.v2.api.Load
import org.snakeyaml.engine.v2.api.LoadSettings
import java.io.File

/**
 * Writes `res/values-<language>/strings.xml` from web/src/locales/<code>.json for the namespaces listed in
 * android/strings-namespaces.txt (concept 27, 5.4). Rules in [StringsGenerator].
 */
@CacheableTask
abstract class GenerateStrings : DefaultTask() {
    /** The language files `web/src/locales/<code>.json`; `en.json` must be among them. */
    @get:InputFiles
    @get:PathSensitive(PathSensitivity.NAME_ONLY)
    abstract val locales: ConfigurableFileCollection

    @get:InputFile
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val namespaces: RegularFileProperty

    @get:OutputDirectory
    abstract val outputDir: DirectoryProperty

    @TaskAction
    fun generate() {
        val files = locales.files.filter { it.name.endsWith(".json") }.associateBy { it.name.removeSuffix(".json") }
        val namespaceList = StringsGenerator.readNamespaces(namespaces.get().asFile.readText())
        val out = outputDir.get().asFile
        out.deleteRecursively()
        val result = try {
            StringsGenerator.generate(files.mapValues { StringsGenerator.load(it.value.readText()) }, namespaceList)
        } catch (e: StringsGenerator.Unsupported) {
            throw GradleException("GenerateStrings: ${e.message}", e)
        }
        for ((dir, xml) in result.files) {
            File(out, "$dir/strings.xml").apply { parentFile.mkdirs() }.writeText(xml)
        }
        logger.lifecycle(
            "GenerateStrings: ${result.files.size} languages, ${result.resources} resources " +
                "(namespaces ${namespaceList.joinToString()})",
        )
    }
}

/**
 * The text generator of concept 27, 5.4: i18next JSON (v4) to Android string resources.
 *
 * - Key `ns.a.b-c` becomes the resource `ns_a_b_c`; two keys that end up with the same name stop
 *   the run.
 * - Plural forms `x_one`, `x_two`, `x_few`, `x_many`, `x_other` become `<plurals name="x">`.
 *   A bare `x` next to `x_other` is the singular (as i18next reads it) when `x_one` is missing.
 *   Every group needs `x_other`. A regional overlay (`de-DE`) gets the forms it lacks from its
 *   base language (`de`): Android takes a <plurals> of one folder as a whole.
 * - `x_zero` becomes a string of its own, `x_zero`: Android never picks the quantity `zero` for
 *   German or English (stage 0, point 8), so a page that shows «none» asks for that text itself.
 *   It needs the same key in English, the default of every string.
 * - Placeholders become positional arguments, the same positions in every language: `{{count}}`
 *   first (`%1$d`), then the other names in the order English uses them (`%2$s` …).
 *   `{{x, number}}` becomes `%N$s`; the page formats the number for the language first.
 * - Folders: `en` is `values`, `de` `values-de`, `de-DE` `values-de-rDE`, `pt-BR` `values-pt-rBR`.
 *   `de-DE.json` is an overlay; Android falls back to `values-de` per text, as the web app does.
 * - Stops with the language and the key on what it cannot carry: nested `$t()`, markup in a text,
 *   another format than `number`, a value that is not a text, a key English does not have.
 */
object StringsGenerator {
    class Unsupported(message: String) : RuntimeException(message)

    data class Result(val files: Map<String, String>, val resources: Int)

    private val pluralForms = listOf("zero", "one", "two", "few", "many", "other")
    private val placeholder = Regex("""\{\{\s*(-\s*)?([A-Za-z0-9_.]+)\s*(?:,\s*([A-Za-z]+)\s*)?}}""")
    private val markup = Regex("""</?[A-Za-z][A-Za-z0-9-]*\s*/?>""")
    private const val REFERENCE = "en"

    fun readNamespaces(text: String): List<String> =
        text.lines().map { it.substringBefore('#').trim() }.filter { it.isNotEmpty() }

    @Suppress("UNCHECKED_CAST")
    fun load(json: String): Map<String, Any?> =
        Load(LoadSettings.builder().setCodePointLimit(64 * 1024 * 1024).build()).loadFromString(json) as Map<String, Any?>

    fun resourceDir(language: String): String {
        if (language == REFERENCE) return "values"
        val parts = language.split('-')
        return if (parts.size == 1) "values-${parts[0]}" else "values-${parts[0]}-r${parts[1].uppercase()}"
    }

    @Suppress("UNCHECKED_CAST")
    private fun exists(tree: Map<String, Any?>, path: String): Boolean {
        var node: Any? = tree
        for (part in path.split('.')) node = (node as? Map<String, Any?>)?.get(part) ?: return false
        return true
    }

    fun resourceName(key: String): String = key.replace(Regex("[^A-Za-z0-9_]"), "_")

    fun generate(languages: Map<String, Map<String, Any?>>, namespaces: List<String>): Result {
        val reference = languages[REFERENCE] ?: throw Unsupported("$REFERENCE.json is missing")
        for (ns in namespaces) {
            if (!exists(reference, ns)) throw Unsupported("$REFERENCE: namespace '$ns' does not exist")
        }
        // An entry is a namespace (`app`) or a part of one (`settings.devices`): only those texts.
        fun selected(key: String) = namespaces.any { key == it || key.startsWith("$it.") }
        val refEntries = flatten(reference).filter { selected(it.first) }.toMap()
        val plan = Plan(refEntries)
        val files = sortedMapOf<String, String>()
        var count = 0
        for ((language, tree) in languages.toSortedMap()) {
            val entries = flatten(tree).filter { selected(it.first) }
            val xml = StringBuilder()
            xml.append("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
            xml.append("<!-- Generated from web/src/locales/$language.json; do not edit. -->\n")
            xml.append("<resources>\n")
            val groups = linkedMapOf<String, MutableMap<String, String>>()
            val bare = mutableMapOf<String, String>()
            for ((key, value) in entries) {
                // A plural form English lacks (`_few` in Russian) belongs to a group English has.
                val base = plan.pluralBase(key)
                if (base == null && key !in refEntries) throw Unsupported("$language: $key: not in $REFERENCE.json")
                if (value !is String) throw Unsupported("$language: $key: not a text")
                if (key.endsWith("_zero") && base != null && key !in refEntries) {
                    // Android needs a default for every string; `values` would lack this one.
                    throw Unsupported("$language: $key: _zero without the same text in $REFERENCE.json")
                }
                if (plan.isPluralBase(key)) {
                    // i18next reads a bare `x` next to `x_other` as the singular.
                    bare[key] = value
                    continue
                }
                if (base != null && !key.endsWith("_zero")) {
                    groups.getOrPut(base) { linkedMapOf() }[key.substringAfterLast('_')] = value
                    continue
                }
                val args = plan.arguments(base ?: key)
                val (text, formatted) = convert(language, key, value, args)
                val attr = if (!formatted && value.contains('%')) " formatted=\"false\"" else ""
                xml.append("    <string name=\"${resourceName(key)}\"$attr>$text</string>\n")
                count++
            }
            for ((base, value) in bare) {
                groups.getOrPut(base) { linkedMapOf() }.putIfAbsent("one", value)
            }
            // Android resolves a <plurals> per configuration as a whole, not per item: a regional
            // overlay (de-DE) with only `x_one` would hide `other` of values-de. Its groups are
            // completed from the base language. pt-BR or zh-TW have no base file: they are whole.
            val baseTree = languages[language.substringBefore('-')].takeIf { '-' in language }
            if (baseTree != null) {
                val baseEntries = flatten(baseTree).toMap()
                for ((base, forms) in groups) {
                    for (form in pluralForms - "zero") {
                        val fallback = baseEntries["${base}_$form"] as? String ?: continue
                        forms.putIfAbsent(form, fallback)
                    }
                }
            }
            for ((base, forms) in groups) {
                if ("other" !in forms) throw Unsupported("$language: $base: plural group without _other")
                xml.append("    <plurals name=\"${resourceName(base)}\">\n")
                for (form in pluralForms) {
                    val value = forms[form] ?: continue
                    val (text, _) = convert(language, "${base}_$form", value, plan.arguments(base), forceFormat = true)
                    xml.append("        <item quantity=\"$form\">$text</item>\n")
                }
                xml.append("    </plurals>\n")
                count++
            }
            xml.append("</resources>\n")
            files[resourceDir(language)] = xml.toString()
        }
        return Result(files, count)
    }

    /** The plural groups and argument positions, decided once from English. */
    private class Plan(reference: Map<String, Any?>) {
        private val pluralBases: Set<String>
        private val arguments: Map<String, List<String>>

        init {
            pluralBases = reference.keys.filter { it.endsWith("_other") }.map { it.removeSuffix("_other") }.toSet()
            val names = linkedMapOf<String, MutableList<String>>()
            for ((key, value) in reference) {
                if (value !is String) continue
                val group = pluralBaseOf(key) ?: key
                val list = names.getOrPut(group) { mutableListOf() }
                for (m in placeholder.findAll(value)) {
                    val name = m.groupValues[2]
                    if (name !in list) list.add(name)
                }
            }
            arguments = names.mapValues { (_, list) ->
                if ("count" in list) listOf("count") + (list - "count") else list
            }
            val seen = mutableMapOf<String, String>()
            for (key in reference.keys) {
                val name = resourceName(pluralBaseOf(key)?.takeUnless { key.endsWith("_zero") } ?: key)
                val owner = pluralBaseOf(key)?.takeUnless { key.endsWith("_zero") } ?: key
                val previous = seen.putIfAbsent(name, owner)
                if (previous != null && previous != owner) {
                    throw Unsupported("$REFERENCE: $key: resource name '$name' is also used by $previous")
                }
            }
        }

        private fun pluralBaseOf(key: String): String? {
            val form = key.substringAfterLast('_', "")
            if (form !in pluralForms) return null
            val base = key.substringBeforeLast('_')
            return base.takeIf { it in pluralBases }
        }

        fun pluralBase(key: String) = pluralBaseOf(key)

        fun isPluralBase(key: String) = key in pluralBases

        fun arguments(group: String): List<String> = arguments[group].orEmpty()
    }

    /**
     * Escapes [value] for aapt2 and turns its placeholders into positional arguments. Returns the
     * text and whether it carries arguments (then `%` is doubled).
     */
    internal fun convert(
        language: String,
        key: String,
        value: String,
        args: List<String>,
        forceFormat: Boolean = false,
    ): Pair<String, Boolean> {
        if ("\$t(" in value) throw Unsupported("$language: $key: nested \$t() reference")
        if (markup.containsMatchIn(value)) throw Unsupported("$language: $key: markup in a text")
        if (value.isEmpty()) throw Unsupported("$language: $key: empty text")
        val formatted = forceFormat || placeholder.containsMatchIn(value)
        val out = StringBuilder()
        var last = 0
        for (m in placeholder.findAll(value)) {
            out.append(escape(value.substring(last, m.range.first), formatted))
            val name = m.groupValues[2]
            val format = m.groupValues[3]
            if (format.isNotEmpty() && format != "number") {
                throw Unsupported("$language: $key: format '$format' of {{$name}}")
            }
            val index = args.indexOf(name)
            if (index < 0) throw Unsupported("$language: $key: placeholder {{$name}} is not in $REFERENCE.json")
            val type = if (name == "count" && format.isEmpty()) 'd' else 's'
            out.append("%${index + 1}\$$type")
            last = m.range.last + 1
        }
        out.append(escape(value.substring(last), formatted))
        val text = out.toString()
        if ("{{" in text) throw Unsupported("$language: $key: placeholder the generator does not know")
        return text.spaced() to formatted
    }

    private fun escape(text: String, formatted: Boolean): String {
        val sb = StringBuilder()
        for (c in text) {
            when (c) {
                '&' -> sb.append("&amp;")
                '<' -> sb.append("&lt;")
                '>' -> sb.append("&gt;")
                '\\' -> sb.append("\\\\")
                '\'' -> sb.append("\\'")
                '"' -> sb.append("\\\"")
                '\n' -> sb.append("\\n")
                '\t' -> sb.append("\\t")
                '%' -> sb.append(if (formatted) "%%" else "%")
                else -> sb.append(c)
            }
        }
        return sb.toString()
    }

    /** aapt2 drops spaces at the edges and folds runs of them; the escape \u0020 keeps them (" · "). */
    private fun String.spaced(): String {
        var s = this
        if (s.startsWith("@") || s.startsWith("?")) s = "\\" + s
        val lead = s.length - s.trimStart(' ').length
        val trail = s.length - s.trimEnd(' ').length
        if (lead == s.length) return "\\u0020".repeat(s.length)
        val core = s.substring(lead, s.length - trail).replace(Regex(" {2,}")) { m ->
            " " + "\\u0020".repeat(m.value.length - 1)
        }
        return "\\u0020".repeat(lead) + core + "\\u0020".repeat(trail)
    }

    @Suppress("UNCHECKED_CAST")
    fun flatten(tree: Map<String, Any?>, prefix: String = ""): List<Pair<String, Any?>> =
        tree.flatMap { (k, v) ->
            if (v is Map<*, *>) flatten(v as Map<String, Any?>, "$prefix$k.") else listOf("$prefix$k" to v)
        }
}
