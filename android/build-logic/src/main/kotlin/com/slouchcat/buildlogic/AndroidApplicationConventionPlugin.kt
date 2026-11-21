package com.slouchcat.buildlogic

import com.android.build.api.dsl.ApplicationExtension
import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.kotlin.dsl.configure

/**
 * An app module: SDK levels, Java level, lint with the own rules, JVM tests, and the debug build
 * under its own application id (`.dev`, concept 27 11.1: development builds are never uploaded).
 * The target SDK differs per app (phone 36, TV 34) and stays in the app's own build file.
 */
class AndroidApplicationConventionPlugin : Plugin<Project> {
    override fun apply(target: Project) = with(target) {
        pluginManager.apply("com.android.application")
        extensions.configure<ApplicationExtension> {
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
            buildTypes {
                getByName("debug") {
                    // The version name stays plain: the app sends it in X-Slouchcat-Client.
                    applicationIdSuffix = ".dev"
                }
            }
            buildFeatures {
                buildConfig = true
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
