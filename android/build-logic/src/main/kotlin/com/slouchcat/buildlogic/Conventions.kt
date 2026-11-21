package com.slouchcat.buildlogic

import com.android.build.api.dsl.Lint
import org.gradle.api.JavaVersion
import org.gradle.api.Project
import org.gradle.api.artifacts.VersionCatalog
import org.gradle.api.artifacts.VersionCatalogsExtension
import org.gradle.api.tasks.testing.Test
import org.gradle.kotlin.dsl.getByType
import org.gradle.kotlin.dsl.withType

/** The SDK levels of the build log (concept 27; A-5: compileSdk 37.2 because of the libraries). */
internal object Sdk {
    const val COMPILE = 37
    const val COMPILE_MINOR = 2
    const val MIN = 28
    const val TARGET_MOBILE = 36
}

/** Byte code level of every module: Java 17 classes, compiled by the JDK 21 that runs Gradle. */
internal val JAVA_TARGET = JavaVersion.VERSION_17

internal val Project.libs: VersionCatalog
    get() = extensions.getByType<VersionCatalogsExtension>().named("libs")

internal fun VersionCatalog.lib(alias: String) = findLibrary(alias).get()

/**
 * Lint of every module (concept 27, 5.6): the own rules of `:lint` as errors, everything else as
 * Android ships it. `android/lint.xml` silences what the generated resources trip over.
 */
internal fun Project.configureLint(lint: Lint) {
    lint.abortOnError = true
    lint.checkReleaseBuilds = false
    lint.lintConfig = rootProject.file("lint.xml")
    lint.htmlReport = true
    lint.textReport = true
    lint.textOutput = file("build/reports/lint-results.txt")
    dependencies.add("lintChecks", project(":lint"))
}

/** One test JVM at a time in the whole build: Robolectric needs 1.5 GB each (gradle.properties). */
abstract class TestSlot : org.gradle.api.services.BuildService<org.gradle.api.services.BuildServiceParameters.None>

/** JVM tests: Robolectric needs memory, but not more than the machine can give several builds. */
internal fun Project.configureTests() {
    val slot = gradle.sharedServices.registerIfAbsent("slouchcatTestSlot", TestSlot::class.java) {
        maxParallelUsages.set(1)
    }
    tasks.withType<Test>().configureEach {
        usesService(slot)
        maxHeapSize = "1536m"
        testLogging {
            events("failed")
            exceptionFormat = org.gradle.api.tasks.testing.logging.TestExceptionFormat.FULL
        }
    }
}
