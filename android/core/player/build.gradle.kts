// The player of both apps (concept 27, 8; concept 26, 5): what the device reports to the server,
// the start and the session of a playback, Media3 over the shared OkHttp client, tracks, chapters,
// marks, trickplay, the fallbacks after a silent track or a decoder error, and the state a page
// draws. No user interface: the phone and the TV app each draw their own on top of it.
plugins {
    id("slouchcat.android.library")
}

android {
    namespace = "com.slouchcat.core.player"
}

dependencies {
    api(project(":core:api"))
    api(libs.media3.exoplayer)
    api(libs.media3.session)
    implementation(libs.media3.exoplayer.hls)
    implementation(libs.media3.datasource.okhttp)
    implementation(libs.androidx.datastore.preferences)
    implementation(libs.coroutines.android)
    testImplementation(project(":core:player-testing"))
    testImplementation(libs.junit)
    testImplementation(libs.coroutines.test)
    testImplementation(libs.robolectric)
    testImplementation(libs.androidx.test.core)
}

// The capability profile of the contract test (testdata/android/phone-capabilities.json) is the one
// the rules make for the emulator's decoders; the unit test compares both.
tasks.withType<Test>().configureEach {
    val fixtures = rootProject.layout.projectDirectory.dir("../testdata/android")
    inputs.dir(fixtures).withPathSensitivity(PathSensitivity.RELATIVE)
    systemProperty("slouchcat.fixtures", fixtures.asFile.absolutePath)
}
