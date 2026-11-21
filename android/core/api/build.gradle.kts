// The Kotlin client of the contract (concept 27, 5.3) and the one HTTP client of the apps: generated
// at build time from api/openapi.yaml (trimmed to what the apps may call), never checked in.
import com.slouchcat.tools.TrimOpenApi
import org.openapitools.generator.gradle.plugin.tasks.GenerateTask

plugins {
    id("slouchcat.jvm.library")
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.openapi.generator)
    id("slouchcat.generators")
}

val contractFile = rootProject.layout.projectDirectory.file("../api/openapi.yaml")
val trimmed = layout.buildDirectory.file("openapi/mobile.yaml")
val generated = layout.buildDirectory.dir("generated/openapi")

val trimOpenApi = tasks.register<TrimOpenApi>("trimOpenApi") {
    contract.set(contractFile)
    visibilities.set(listOf("public", "account", "scoped"))
    client.set("app")
    output.set(trimmed)
}

// openapi-generator 7.27.0, generator "kotlin" with OkHttp and kotlinx.serialization (stage 0,
// point 3). enumUnknownDefaultCase: an unknown value of an enumeration becomes
// UNKNOWN_DEFAULT_OPEN_API instead of an exception (11.5); UPPERCASE names keep the value `name`
// apart from Enum.name.
openApiGenerate {
    generatorName.set("kotlin")
    library.set("jvm-okhttp4")
    inputSpec.set(trimmed.get().asFile.path)
    outputDir.set(generated.get().asFile.path)
    packageName.set("com.slouchcat.core.api.generated")
    cleanupOutput.set(true)
    generateApiTests.set(false)
    generateModelTests.set(false)
    generateApiDocumentation.set(false)
    generateModelDocumentation.set(false)
    validateSpec.set(false)
    configOptions.set(
        mapOf(
            "serializationLibrary" to "kotlinx_serialization",
            "enumUnknownDefaultCase" to "true",
            "enumPropertyNaming" to "UPPERCASE",
            "omitGradleWrapper" to "true",
            "omitGradlePluginVersions" to "true",
            "sourceFolder" to "src/main/kotlin",
            "dateLibrary" to "java8",
            "nullableReturnType" to "false",
        ),
    )
}

tasks.named<GenerateTask>("openApiGenerate") {
    dependsOn(trimOpenApi)
    inputs.file(trimmed).withPathSensitivity(PathSensitivity.NONE)
}
sourceSets["main"].kotlin.srcDir(generated.map { it.dir("src/main/kotlin") })
tasks.named("compileKotlin") { dependsOn("openApiGenerate") }

dependencies {
    api(libs.okhttp)
    api(libs.serialization.json)
    api(libs.coroutines.core)
    implementation(libs.tink.android)
    testImplementation(libs.junit)
    testImplementation(libs.coroutines.test)
    testImplementation(libs.okhttp.mockwebserver)
}

// The shared test vectors of the repository (testdata/crypto, format slouchcat-crypto-vectors-v1).
tasks.named<Test>("test") {
    val vectors = rootProject.layout.projectDirectory.dir("../testdata/crypto")
    inputs.dir(vectors).withPathSensitivity(PathSensitivity.RELATIVE)
    systemProperty("slouchcat.vectors", vectors.asFile.absolutePath)
}

// The contract test (task android:contract): the generated client against a real server the task
// starts. Its own suite, so `task android:test` never needs a server.
testing {
    suites {
        register<JvmTestSuite>("contractTest") {
            useJUnit(libs.versions.junit.get())
            dependencies {
                implementation(project())
            }
            targets.all {
                testTask.configure {
                    val base = providers.gradleProperty("slouchcat.contract.base")
                    inputs.property("base", base.orElse(""))
                    systemProperty("slouchcat.contract.base", base.getOrElse(""))
                    outputs.upToDateWhen { false }
                    testLogging { events("passed", "failed"); showStandardStreams = true }
                }
            }
        }
    }
}
