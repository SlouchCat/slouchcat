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
val tvVersion = Properties().apply { file("version.properties").inputStream().use(::load) }

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
    // R8 from the start, like the phone app. Unsigned here; signing comes with the release.
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
    testImplementation(libs.coroutines.test)
    testImplementation(libs.okhttp.mockwebserver)
    testImplementation(libs.androidx.test.junit)
    androidTestImplementation(platform(libs.compose.bom))
    androidTestImplementation(libs.compose.ui.test.junit4)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.junit)
    androidTestImplementation(libs.androidx.test.core)
}
