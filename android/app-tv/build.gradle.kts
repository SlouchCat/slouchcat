// The TV app (concept 26). Package com.slouchcat.app like the phone app (E-20), its own artifact
// with its own version (version.properties) in the same store entry; debug builds carry `.dev`
// and are never uploaded. No flavours (build log, Android architecture): no Google libraries.
import java.util.Properties

plugins {
    id("slouchcat.android.application")
    id("slouchcat.android.compose")
    alias(libs.plugins.kotlin.serialization)
}

// The version of the TV app, apart from the phone app's (tags `tv-v…`, concept 26 3.2).
// From 1000001 on: the phone app counts below 1000000 (one store entry, every bundle needs a code of its own).
val tvVersion = Properties().apply { file("version.properties").inputStream().use(::load) }
check(tvVersion.getProperty("versionCode").toInt() > 1_000_000) { "app-tv/version.properties: versionCode must be above 1000000" }

android {
    namespace = "com.slouchcat.app.tv"
    defaultConfig {
        applicationId = "com.slouchcat.app"
        // TV: target 34 (build log; Google's minimum for TV), min 28 (K26 stage 0, point 8).
        targetSdk = 34
        versionCode = tvVersion.getProperty("versionCode").toInt()
        versionName = tvVersion.getProperty("versionName")
        // The flow in the TV emulator (scripts/android-tv-e2e.sh).
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }
    // R8 from the start, like the phone app. Unsigned unless the four `slouchcat.signing.*` values
    // are set (build-logic, Signing.kt).
    buildTypes {
        getByName("release") {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
        }
    }
}

dependencies {
    implementation(project(":core:api"))
    implementation(project(":core:session"))
    implementation(project(":core:design"))
    implementation(project(":core:strings"))
    implementation(project(":core:pairing"))
    implementation(project(":core:media"))
    implementation(project(":core:browse"))
    testImplementation(project(":core:browse-testing"))
    // The player of stage 3 (concept 26, 5): the shared playback, Media3's surface for the picture.
    implementation(project(":core:player"))
    implementation(libs.media3.ui)
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.navigation.compose)
    implementation(libs.androidx.datastore.preferences)
    implementation(libs.serialization.json)
    implementation(libs.coroutines.android)
    implementation(libs.coil.compose)
    implementation(libs.coil.network.okhttp)
    implementation(libs.tv.material)
    implementation(libs.zxing.core)
    testImplementation(project(":core:player-testing"))
    testImplementation(libs.coroutines.test)
    testImplementation(libs.okhttp.mockwebserver)
    testImplementation(libs.androidx.test.junit)
    androidTestImplementation(platform(libs.compose.bom))
    androidTestImplementation(libs.compose.ui.test.junit4)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.junit)
    androidTestImplementation(libs.androidx.test.core)
    // Stage 4: the showcase of the sound plays on the attrappes (made-up titles, players without sound).
    androidTestImplementation(project(":core:browse-testing"))
    androidTestImplementation(project(":core:player-testing"))
}
