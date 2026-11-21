package com.slouchcat.tools

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.snakeyaml.engine.v2.api.Load
import org.snakeyaml.engine.v2.api.LoadSettings

class OpenApiTrimmerTest {
    @Suppress("UNCHECKED_CAST")
    private fun load(yaml: String) = Load(LoadSettings.builder().build()).loadFromString(yaml.trimIndent()) as MutableMap<String, Any?>

    private val contract = """
        openapi: 3.1.0
        info: {title: t, version: 1.0.0}
        paths:
          /healthz:
            get: {operationId: getHealth, x-visibility: public, responses: {'200': {content: {application/json: {schema: {${'$'}ref: '#/components/schemas/Health'}}}}}}
          /home:
            get: {operationId: getHome, x-visibility: scoped, responses: {'200': {content: {application/json: {schema: {${'$'}ref: '#/components/schemas/Home'}}}}}}
          /admin/libraries:
            get: {operationId: listLibraries, x-visibility: admin, responses: {'200': {content: {application/json: {schema: {${'$'}ref: '#/components/schemas/Admin'}}}}}}
        components:
          schemas:
            Health: {type: object, properties: {version: {type: string}}}
            Home: {type: object, additionalProperties: true, properties: {next: {oneOf: [{${'$'}ref: '#/components/schemas/Row'}, {type: 'null'}]}}}
            Row: {type: object, properties: {sub: {oneOf: [{type: string}, {type: 'null'}]}}}
            Admin: {type: object}
          securitySchemes:
            cookie: {type: apiKey, in: cookie, name: sc}
    """

    @Suppress("UNCHECKED_CAST")
    @Test
    fun withoutXClientsTheVisibilityDecides() {
        val root = load(contract)
        val stats = OpenApiTrimmer.trim(root, "app", setOf("public", "account", "scoped"))
        assertEquals(TrimStats(kept = 2, dropped = 1, components = 3), stats)
        val paths = root["paths"] as Map<String, Any?>
        assertEquals(setOf("/healthz", "/home"), paths.keys)
        val schemas = (root["components"] as Map<String, Any?>)["schemas"] as Map<String, Any?>
        assertEquals(setOf("Health", "Home", "Row"), schemas.keys)
        assertTrue("securitySchemes stay", "securitySchemes" in (root["components"] as Map<String, Any?>))
    }

    @Suppress("UNCHECKED_CAST")
    @Test
    fun rewritesNullableOneOfAndOpenObjects() {
        val root = load(contract)
        OpenApiTrimmer.trim(root, "app", setOf("public", "scoped"))
        val schemas = (root["components"] as Map<String, Any?>)["schemas"] as Map<String, Map<String, Any?>>
        val home = schemas.getValue("Home")
        assertFalse("additionalProperties: true next to properties goes", "additionalProperties" in home)
        val next = (home["properties"] as Map<String, Map<String, Any?>>).getValue("next")
        assertEquals(true, next["nullable"])
        assertEquals(listOf(mapOf("\$ref" to "#/components/schemas/Row")), next["allOf"])
        val sub = ((schemas.getValue("Row")["properties"]) as Map<String, Map<String, Any?>>).getValue("sub")
        assertEquals(mapOf("type" to "string", "nullable" to true), sub)
    }

    @Suppress("UNCHECKED_CAST")
    @Test
    fun schemasReachedOnlyThroughADiscriminatorMappingStay() {
        val root = load(
            """
            openapi: 3.1.0
            info: {title: t, version: 1.0.0}
            paths:
              /x:
                get: {x-visibility: public, responses: {'200': {content: {application/json: {schema: {${'$'}ref: '#/components/schemas/Event'}}}}}}
            components:
              schemas:
                Event: {type: object, discriminator: {propertyName: kind, mapping: {a: '#/components/schemas/EventA'}}}
                EventA: {type: object}
                Unused: {type: object}
            """,
        )
        OpenApiTrimmer.trim(root, "app", setOf("public"))
        val schemas = (root["components"] as Map<String, Any?>)["schemas"] as Map<String, Any?>
        assertEquals(setOf("Event", "EventA"), schemas.keys)
    }

    @Test
    fun onceTheContractNamesClientsTheListDecides() {
        val op = { visibility: String, clients: List<String>? ->
            buildMap<String, Any?> {
                put("x-visibility", visibility)
                if (clients != null) put("x-clients", clients)
            }
        }
        val kept = setOf("public", "account", "scoped")
        assertTrue(OpenApiTrimmer.keeps(op("scoped", listOf("app", "tv")), "app", kept))
        assertFalse("tv only", OpenApiTrimmer.keeps(op("scoped", listOf("tv")), "app", kept))
        assertTrue(OpenApiTrimmer.keeps(op("account", listOf("app")), "app", kept))
        assertFalse("account without app", OpenApiTrimmer.keeps(op("account", emptyList()), "app", kept))
        assertTrue("public carries no list", OpenApiTrimmer.keeps(op("public", null), "app", kept))
        assertFalse(OpenApiTrimmer.keeps(op("admin", null), "app", kept))
    }

    /** `off` of `SubtitleMode` must stay a string for the YAML 1.1 reader of openapi-generator. */
    @Test
    fun yaml11BooleanWordsStayQuoted() {
        val settings = org.snakeyaml.engine.v2.api.DumpSettings.builder().build()
        val yaml = org.snakeyaml.engine.v2.api.Dump(settings, Yaml11SafeRepresenter(settings))
            .dumpToString(mapOf("enum" to listOf("off", "forced", "On", "yes"), "minimum" to 0))
        assertTrue(yaml, yaml.contains("\"off\""))
        assertTrue(yaml, yaml.contains("\"On\""))
        assertTrue(yaml, yaml.contains("\"yes\""))
        assertTrue(yaml, yaml.contains("forced") && !yaml.contains("\"forced\""))
        assertTrue(yaml, yaml.contains("minimum: 0"))
    }
}
