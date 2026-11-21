// Own lint rules of the apps (concept 27, 5.6): rule 9 (no fixed text, no fixed colour) and rule 10
// (no log call past the redacting logger). Every Android and JVM module runs them through
// lintChecks(project(":lint")) (build-logic, configureLint).
plugins {
    alias(libs.plugins.kotlin.jvm)
}

kotlin {
    jvmToolchain(21)
    compilerOptions.jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
}

java {
    sourceCompatibility = JavaVersion.VERSION_17
    targetCompatibility = JavaVersion.VERSION_17
}

dependencies {
    compileOnly(libs.lint.api)
    compileOnly(libs.lint.checks)
    testImplementation(libs.lint.api)
    testImplementation(libs.lint.checks)
    testImplementation(libs.lint.tests)
    testImplementation(libs.junit)
}

tasks.jar {
    manifest { attributes("Lint-Registry-v2" to "com.slouchcat.lint.SlouchcatIssueRegistry") }
}

tasks.test {
    maxHeapSize = "1536m"
}
