package com.slouchcat.tools

import org.gradle.api.DefaultTask
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.provider.ListProperty
import org.gradle.api.provider.Property
import org.gradle.api.tasks.CacheableTask
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.InputFile
import org.gradle.api.tasks.OutputFile
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.TaskAction
import org.snakeyaml.engine.v2.api.Dump
import org.snakeyaml.engine.v2.api.DumpSettings
import org.snakeyaml.engine.v2.api.Load
import org.snakeyaml.engine.v2.api.LoadSettings
import org.snakeyaml.engine.v2.api.RepresentToNode
import org.snakeyaml.engine.v2.common.FlowStyle
import org.snakeyaml.engine.v2.common.ScalarStyle
import org.snakeyaml.engine.v2.nodes.Tag
import org.snakeyaml.engine.v2.representer.StandardRepresenter

/**
 * The pre-pass of concept 27 (5.3): cuts api/openapi.yaml down to the operations one app may call
 * and the components they reach, before openapi-generator runs. Rules in [OpenApiTrimmer].
 */
@CacheableTask
abstract class TrimOpenApi : DefaultTask() {
    @get:InputFile
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val contract: RegularFileProperty

    /** Visibilities kept for operations without `x-clients` (today: public, account, scoped). */
    @get:Input
    abstract val visibilities: ListProperty<String>

    /** The app whose operations stay once the contract names them in `x-clients` (`app`, `tv`). */
    @get:Input
    abstract val client: Property<String>

    @get:OutputFile
    abstract val output: RegularFileProperty

    @TaskAction
    fun trim() {
        val settings = LoadSettings.builder().setCodePointLimit(64 * 1024 * 1024).build()
        @Suppress("UNCHECKED_CAST")
        val root = Load(settings).loadFromString(contract.get().asFile.readText()) as MutableMap<String, Any?>
        val stats = OpenApiTrimmer.trim(root, client.get(), visibilities.get().toSet())
        val dumpSettings = DumpSettings.builder().setDefaultFlowStyle(FlowStyle.BLOCK).setWidth(4096).build()
        val dump = Dump(dumpSettings, Yaml11SafeRepresenter(dumpSettings))
        output.get().asFile.apply { parentFile.mkdirs() }.writeText(dump.dumpToString(root))
        logger.lifecycle(
            "TrimOpenApi: ${stats.kept} operations kept, ${stats.dropped} dropped, " +
                "${stats.components} components reached",
        )
    }
}

/**
 * Writes YAML 1.2 that a YAML 1.1 reader (the parser of openapi-generator) reads the same: a string
 * that YAML 1.1 would resolve to a boolean, a number, a timestamp or null stays quoted. Unquoted,
 * the generator read the enumeration value `off` of `SubtitleMode` as the boolean `false` and the
 * client sent "false" (stage 5); likewise `1:30` would become 90, `1_000` 1000, `2026-11-21` a date.
 */
internal class Yaml11SafeRepresenter(settings: DumpSettings) : StandardRepresenter(settings) {
    init {
        val plain = representers.getValue(String::class.java)
        representers[String::class.java] = RepresentToNode { data ->
            if (isYaml11Special(data as String)) representScalar(Tag.STR, data, ScalarStyle.DOUBLE_QUOTED) else plain.representData(data)
        }
    }

    companion object {
        /** The implicit resolvers of YAML 1.1 (yaml.org/type: bool, int, float, null, timestamp). */
        private val YAML11 = listOf(
            Regex("""y|Y|yes|Yes|YES|n|N|no|No|NO|true|True|TRUE|false|False|FALSE|on|On|ON|off|Off|OFF"""),
            Regex("""~|null|Null|NULL|"""),
            Regex("""[-+]?0b[0-1_]+"""),
            Regex("""[-+]?0[0-7_]+"""),
            Regex("""[-+]?(0|[1-9][0-9_]*)"""),
            Regex("""[-+]?0x[0-9a-fA-F_]+"""),
            Regex("""[-+]?[1-9][0-9_]*(:[0-5]?[0-9])+"""),
            Regex("""[-+]?([0-9][0-9_]*)?\.[0-9.]*([eE][-+][0-9]+)?"""),
            Regex("""[-+]?[0-9][0-9_]*(:[0-5]?[0-9])+\.[0-9_]*"""),
            Regex("""[-+]?\.(inf|Inf|INF)|\.(nan|NaN|NAN)"""),
            Regex("""[0-9][0-9][0-9][0-9]-[0-9][0-9]?-[0-9][0-9]?([Tt]|[ \t]+)[0-9][0-9]?:[0-9][0-9]:[0-9][0-9](\.[0-9]*)?(([ \t]*)Z|[-+][0-9][0-9]?(:[0-9][0-9])?)?"""),
            Regex("""[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]"""),
        )

        /** Whether a YAML 1.1 reader takes [value] for anything but a string. */
        fun isYaml11Special(value: String): Boolean = YAML11.any { it.matches(value) }
    }
}

/** What [OpenApiTrimmer.trim] kept and dropped. */
data class TrimStats(val kept: Int, val dropped: Int, val components: Int)

/**
 * Selection: an operation that carries `x-clients` stays when the list names [client]; one
 * without the list stays when its `x-visibility` is one of the kept ones. Today the contract has no
 * `x-clients` yet and the visibilities decide; from contract 1.42.0 on every `account` and
 * `scoped` operation carries the list, `public` ones never do, and the administration has neither.
 *
 * Besides it rewrites two forms the Kotlin generator cannot handle: `oneOf: [X, {type: "null"}]`
 * becomes X with `nullable: true`, and `additionalProperties: true` next to `properties` goes (the
 * client ignores unknown fields anyway).
 */
object OpenApiTrimmer {
    private val methods = setOf("get", "put", "post", "delete", "patch", "head", "options", "trace")

    @Suppress("UNCHECKED_CAST")
    fun trim(root: MutableMap<String, Any?>, client: String, visibilities: Set<String>): TrimStats {
        val paths = root["paths"] as MutableMap<String, Any?>
        val components = root["components"] as MutableMap<String, Any?>? ?: mutableMapOf()
        var kept = 0
        var dropped = 0
        val pathIter = paths.entries.iterator()
        while (pathIter.hasNext()) {
            val ops = pathIter.next().value as MutableMap<String, Any?>
            val opIter = ops.entries.iterator()
            while (opIter.hasNext()) {
                val (method, op) = opIter.next()
                if (method !in methods) continue
                if (keeps(op as Map<String, Any?>, client, visibilities)) {
                    kept++
                } else {
                    dropped++
                    opIter.remove()
                }
            }
            if (ops.keys.none { it in methods }) pathIter.remove()
        }

        // Every $ref from the remaining paths, then from what they reach.
        val reached = linkedSetOf<String>()
        val queue = ArrayDeque<String>()
        fun scan(node: Any?) {
            when (node) {
                is Map<*, *> -> node.forEach { (k, v) ->
                    if (k == "mapping" && v is Map<*, *>) {
                        // discriminator.mapping: references without a $ref key.
                        v.values.filterIsInstance<String>().filter { it.startsWith("#/components/") }
                            .forEach { if (reached.add(it)) queue.add(it) }
                    } else if (k == "\$ref" && v is String && v.startsWith("#/components/")) {
                        if (reached.add(v)) queue.add(v)
                    } else {
                        scan(v)
                    }
                }
                is List<*> -> node.forEach(::scan)
            }
        }
        scan(paths)
        while (queue.isNotEmpty()) {
            val ref = queue.removeFirst().removePrefix("#/components/").split("/", limit = 2)
            scan((components[ref[0]] as Map<String, Any?>?)?.get(ref[1]))
        }
        for ((section, value) in components) {
            if (section == "securitySchemes") continue
            (value as MutableMap<String, Any?>).keys.retainAll { "#/components/$section/$it" in reached }
        }
        components.entries.removeIf { (it.value as Map<*, *>).isEmpty() }

        normalize(root)
        val info = root["info"] as MutableMap<String, Any?>
        info["description"] = "Cut for client '$client': $kept operations kept, $dropped dropped."
        return TrimStats(kept, dropped, reached.size)
    }

    @Suppress("UNCHECKED_CAST")
    fun keeps(op: Map<String, Any?>, client: String, visibilities: Set<String>): Boolean {
        val clients = op["x-clients"] as List<String>?
        return if (clients != null) client in clients else op["x-visibility"] in visibilities
    }

    @Suppress("UNCHECKED_CAST")
    private fun normalize(node: Any?) {
        when (node) {
            is MutableMap<*, *> -> {
                val map = node as MutableMap<String, Any?>
                if (map["additionalProperties"] == true && map.containsKey("properties")) {
                    map.remove("additionalProperties")
                }
                val oneOf = map["oneOf"] as List<Any?>?
                if (oneOf != null && oneOf.size == 2) {
                    val nullBranch = oneOf.firstOrNull { (it as? Map<*, *>)?.get("type") == "null" }
                    val other = oneOf.firstOrNull { it !== nullBranch } as Map<String, Any?>?
                    if (nullBranch != null && other != null) {
                        map.remove("oneOf")
                        if (other.containsKey("\$ref")) {
                            map["allOf"] = listOf(other)
                        } else {
                            other.forEach { (k, v) -> map.putIfAbsent(k, v) }
                        }
                        map["nullable"] = true
                    }
                }
                map.values.forEach(::normalize)
            }
            is List<*> -> node.forEach(::normalize)
        }
    }
}
