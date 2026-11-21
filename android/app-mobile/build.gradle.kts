// The phone app (concept 27). Package com.slouchcat.app for phone and TV (E-20); debug builds carry
// `.dev` and are never uploaded (11.1).
plugins {
    id("slouchcat.android.application")
    id("slouchcat.android.compose")
    alias(libs.plugins.kotlin.serialization)
}

android {
    namespace = "com.slouchcat.app.mobile"
    defaultConfig {
        applicationId = "com.slouchcat.app"
        targetSdk = 36
        versionCode = 1
        versionName = "0.1.0"
    }
    // Two flavours (F-33): `play` will carry Google's push and code scanner, `foss` builds without
    // Google's libraries. Today both are the same; only `play` is ever published.
    flavorDimensions += "distribution"
    productFlavors {
        create("play") { dimension = "distribution" }
        // Its own id, so both builds can sit side by side on one device.
        create("foss") {
            dimension = "distribution"
            applicationIdSuffix = ".foss"
        }
    }
    // R8 from the start, so a page that needs a keep rule shows it in `task android:apk` and
    // not first in the store build. Unsigned here; signing comes with the release (11.2).
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
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.navigation.compose)
    implementation(libs.serialization.json)
    implementation(libs.coroutines.android)
    testImplementation(libs.coroutines.test)
}
