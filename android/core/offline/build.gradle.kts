// Downloads and offline of the apps (concept 27 section 9, stage 6; stage 7 loads audiobooks and
// episodes through it as well): the local database (Room) of downloads, profile snapshots, progress
// events and watch time; the files in the app's own folder outside every backup; the sync with the
// server, the revocation, the fetch with Range and If-Range in the background (user-initiated
// data transfer from API 34, a dataSync foreground service before, WorkManager in short runs for
// auto-downloads); the offline PIN; the local HLS playlist of the offline player. No user interface.
plugins {
    id("slouchcat.android.library")
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.ksp)
}

android {
    namespace = "com.slouchcat.core.offline"
}

ksp {
    // The schema of every database version, checked in (migrations of later versions test against it).
    arg("room.schemaLocation", "$projectDir/schemas")
    arg("room.generateKotlin", "true")
}

dependencies {
    api(project(":core:api"))
    api(project(":core:player"))
    implementation(project(":core:strings"))
    implementation(libs.androidx.core.ktx)
    implementation(libs.room.runtime)
    implementation(libs.room.ktx)
    ksp(libs.room.compiler)
    implementation(libs.work.runtime)
    implementation(libs.coroutines.android)
    implementation(libs.serialization.json)
    testImplementation(libs.junit)
    testImplementation(libs.coroutines.test)
    testImplementation(libs.robolectric)
    testImplementation(libs.androidx.test.core)
    testImplementation(libs.okhttp.mockwebserver)
    testImplementation(libs.work.testing)
}

// The shared test vectors of the repository (testdata/crypto/pbkdf2.json) for the offline PIN.
tasks.withType<Test>().configureEach {
    val vectors = rootProject.layout.projectDirectory.dir("../testdata/crypto")
    inputs.dir(vectors).withPathSensitivity(PathSensitivity.RELATIVE)
    systemProperty("slouchcat.vectors", vectors.asFile.absolutePath)
}
