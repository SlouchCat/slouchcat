package com.slouchcat.buildlogic

import com.android.build.api.dsl.LibraryExtension
import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.kotlin.dsl.configure

/** An Android library of `:core:*`: SDK levels, Java level, lint with the own rules, JVM tests. */
class AndroidLibraryConventionPlugin : Plugin<Project> {
    override fun apply(target: Project) = with(target) {
        pluginManager.apply("com.android.library")
        extensions.configure<LibraryExtension> {
            compileSdk {
                version = release(Sdk.COMPILE) { minorApiLevel = Sdk.COMPILE_MINOR }
            }
            defaultConfig {
                minSdk = Sdk.MIN
            }
            compileOptions {
                sourceCompatibility = JAVA_TARGET
                targetCompatibility = JAVA_TARGET
            }
            testOptions {
                unitTests {
                    isIncludeAndroidResources = true
                    isReturnDefaultValues = true
                }
            }
            configureLint(lint)
        }
        configureTests()
    }
}
