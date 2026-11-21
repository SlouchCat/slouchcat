package com.slouchcat.buildlogic

import com.android.build.api.dsl.Lint
import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.plugins.JavaPluginExtension
import org.gradle.kotlin.dsl.configure
import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.jetbrains.kotlin.gradle.dsl.KotlinJvmProjectExtension

/**
 * A plain Kotlin module without Android (the generated client in `:core:api`): JDK 21 toolchain,
 * Java 17 byte code like the Android modules, and Android lint with the own rules.
 */
class JvmLibraryConventionPlugin : Plugin<Project> {
    override fun apply(target: Project) = with(target) {
        pluginManager.apply("org.jetbrains.kotlin.jvm")
        pluginManager.apply("com.android.lint")
        extensions.configure<KotlinJvmProjectExtension> {
            jvmToolchain(21)
            compilerOptions.jvmTarget.set(JvmTarget.fromTarget(JAVA_TARGET.toString()))
        }
        extensions.configure<JavaPluginExtension> {
            sourceCompatibility = JAVA_TARGET
            targetCompatibility = JAVA_TARGET
        }
        extensions.configure<Lint> { configureLint(this) }
        configureTests()
    }
}
