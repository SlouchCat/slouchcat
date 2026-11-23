package com.slouchcat.buildlogic

import com.android.build.api.dsl.ApplicationExtension
import org.gradle.api.GradleException
import org.gradle.api.Project

/**
 * The upload key of the release builds (concept 27 11.2), never in the repository: four Gradle
 * properties (`~/.gradle/gradle.properties`, `-P…`) or environment variables, the same pattern as
 * `slouchcat.firebase.*`. All four set: `release` is signed with them. None set: `release` stays
 * unsigned, as `task android:check` builds it. Some set: the build stops and names the missing ones.
 * The values are never printed, only their names.
 */
internal fun Project.configureReleaseSigning(android: ApplicationExtension) {
    // Path and alias lose stray spaces; a password is taken as it is (a space may belong to it).
    val values = SIGNING.associate { (property, environment) ->
        val value = (providers.gradleProperty(property).orNull ?: providers.environmentVariable(environment).orNull).orEmpty()
        property to if (property == STORE_FILE || property == KEY_ALIAS) value.trim() else value
    }
    val missing = SIGNING.filter { (property, _) -> values.getValue(property).isEmpty() }
    if (missing.size == SIGNING.size) return
    if (missing.isNotEmpty()) {
        throw GradleException(
            "Release signing needs all four values; missing: " +
                missing.joinToString { (property, environment) -> "$property ($environment)" },
        )
    }
    val store = rootProject.file(values.getValue(STORE_FILE))
    if (!store.isFile) throw GradleException("$STORE_FILE names no file")
    val config = android.signingConfigs.create("upload") {
        storeFile = store
        storePassword = values.getValue(STORE_PASSWORD)
        keyAlias = values.getValue(KEY_ALIAS)
        keyPassword = values.getValue(KEY_PASSWORD)
    }
    android.buildTypes.getByName("release").signingConfig = config
}

private const val STORE_FILE = "slouchcat.signing.storeFile"
private const val STORE_PASSWORD = "slouchcat.signing.storePassword"
private const val KEY_ALIAS = "slouchcat.signing.keyAlias"
private const val KEY_PASSWORD = "slouchcat.signing.keyPassword"

/** Gradle property and environment variable of each value. */
private val SIGNING = listOf(
    STORE_FILE to "SLOUCHCAT_SIGNING_STORE_FILE",
    STORE_PASSWORD to "SLOUCHCAT_SIGNING_STORE_PASSWORD",
    KEY_ALIAS to "SLOUCHCAT_SIGNING_KEY_ALIAS",
    KEY_PASSWORD to "SLOUCHCAT_SIGNING_KEY_PASSWORD",
)
