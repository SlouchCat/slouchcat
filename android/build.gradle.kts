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
    alias(libs.plugins.ksp) apply false
}

// The entry points of android/Taskfile.yml. Named here once so `task android:check` and
// `task android:test` stay one Gradle call each and a new module only has to be listed in the
// module's own build file.
// ciCheck is the two halves the CI runs as two jobs side by side (ci.yml, job "android"): the
// release builds with R8 alone took 5.5 of 11 minutes after a change to core:api (docs/perf/ci.md).
// ciCheck depends on the halves and on nothing else, so what the two jobs run together is ciCheck
// by construction; a task added belongs into one of the halves.
tasks.register("ciCheckDebug") {
    group = "verification"
    description = "Debug builds of both flavours and of the TV app, lint of every module, the generators (task android:check:debug)"
    dependsOn(":app-mobile:assemblePlayDebug", ":app-mobile:assembleFossDebug")
    dependsOn(":app-mobile:lintPlayDebug", ":app-mobile:lintFossDebug", ":app-mobile:checkFlavourSeparation")
    dependsOn(":core:api:lint", ":core:session:lintDebug", ":core:design:lintDebug", ":core:strings:lintDebug", ":core:player:lintDebug", ":core:player-testing:lintDebug", ":core:media:lintDebug", ":core:offline:lintDebug", ":core:browse:lintDebug", ":core:browse-testing:lintDebug")
    // The TV app (concept 26): debug build, lint; the pairing module.
    dependsOn(":app-tv:assembleDebug", ":app-tv:lintDebug", ":core:pairing:lint")
}

tasks.register("ciCheckRelease") {
    group = "verification"
    description = "Unsigned release builds (R8) of both flavours and of the TV app (task android:check:release)"
    dependsOn(":app-mobile:assemblePlayRelease", ":app-mobile:assembleFossRelease", ":app-tv:assembleRelease")
}

tasks.register("ciCheck") {
    group = "verification"
    description = "Debug and unsigned release builds (R8) of both flavours and of the TV app, lint of every module, the generators (task android:check)"
    dependsOn("ciCheckDebug", "ciCheckRelease")
}

tasks.register("ciTest") {
    group = "verification"
    description = "All JVM tests: modules, lint rules and generators (task android:test)"
    dependsOn(":app-mobile:testPlayDebugUnitTest", ":app-mobile:testFossDebugUnitTest")
    dependsOn(":core:api:test", ":core:session:testDebugUnitTest", ":core:design:testDebugUnitTest", ":core:player:testDebugUnitTest", ":core:media:testDebugUnitTest", ":core:offline:testDebugUnitTest", ":core:browse:testDebugUnitTest")
    dependsOn(":app-tv:testDebugUnitTest", ":core:pairing:test")
    dependsOn(":lint:test")
    dependsOn(gradle.includedBuild("tools").task(":test"))
}

tasks.register("fastCheck") {
    group = "verification"
    description = "The quick part of task check:fast: compile the play debug build and the TV debug build and run their JVM tests"
    dependsOn(":app-mobile:assemblePlayDebug", ":app-mobile:testPlayDebugUnitTest")
    dependsOn(":core:api:test", ":core:session:testDebugUnitTest", ":core:design:testDebugUnitTest", ":core:player:testDebugUnitTest", ":core:media:testDebugUnitTest", ":core:offline:testDebugUnitTest", ":core:browse:testDebugUnitTest")
    dependsOn(":app-tv:assembleDebug", ":app-tv:testDebugUnitTest", ":core:pairing:test")
}
