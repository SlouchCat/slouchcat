// The plugins on the classpath of the whole build; each module applies what it needs through the
// convention plugins in build-logic.
plugins {
    alias(libs.plugins.android.application) apply false
    alias(libs.plugins.android.library) apply false
    alias(libs.plugins.android.lint) apply false
    alias(libs.plugins.kotlin.jvm) apply false
    alias(libs.plugins.kotlin.compose) apply false
    alias(libs.plugins.kotlin.serialization) apply false
    alias(libs.plugins.openapi.generator) apply false
}

// The entry points of android/Taskfile.yml. Named here once so `task android:check` and
// `task android:test` stay one Gradle call each and a new module only has to be listed in the
// module's own build file.
tasks.register("ciCheck") {
    group = "verification"
    description = "Debug builds of both flavours, lint of every module, the generators (task android:check)"
    dependsOn(":app-mobile:assemblePlayDebug", ":app-mobile:assembleFossDebug")
    dependsOn(":app-mobile:lintPlayDebug", ":app-mobile:lintFossDebug")
    dependsOn(":core:api:lint", ":core:session:lintDebug", ":core:design:lintDebug", ":core:strings:lintDebug")
}

tasks.register("ciTest") {
    group = "verification"
    description = "All JVM tests: modules, lint rules and generators (task android:test)"
    dependsOn(":app-mobile:testPlayDebugUnitTest", ":app-mobile:testFossDebugUnitTest")
    dependsOn(":core:api:test", ":core:session:testDebugUnitTest", ":core:design:testDebugUnitTest")
    dependsOn(":lint:test")
    dependsOn(gradle.includedBuild("tools").task(":test"))
}

tasks.register("fastCheck") {
    group = "verification"
    description = "The quick part of task check:fast: compile the play debug build and run its JVM tests"
    dependsOn(":app-mobile:assemblePlayDebug", ":app-mobile:testPlayDebugUnitTest")
    dependsOn(":core:api:test", ":core:session:testDebugUnitTest", ":core:design:testDebugUnitTest")
}
