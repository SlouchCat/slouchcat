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
        // The flow in the emulator (task android:e2e, concept 27 12 «Gerät»).
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }
    // Two flavours (F-33): `play` will carry Google's push and code scanner, `foss` builds without
    // Google's libraries. Today both are the same; only `play` is ever published.
    flavorDimensions += "distribution"
    productFlavors {
        // Push (stage 8, concept 27 7.3): Firebase from four Gradle properties or environment
        // variables, never from a checked-in google-services.json. Empty values build `play`
        // without push; the app says so in «Einstellungen › Benachrichtigungen». The hand-over of
        // stage 8 names the values; nothing real is ever checked in.
        create("play") {
            dimension = "distribution"
            buildConfigField("String", "FIREBASE_PROJECT_ID", quoted(setting("slouchcat.firebase.projectId", "SLOUCHCAT_FIREBASE_PROJECT_ID")))
            buildConfigField("String", "FIREBASE_APP_ID", quoted(setting("slouchcat.firebase.appId", "SLOUCHCAT_FIREBASE_APP_ID")))
            buildConfigField("String", "FIREBASE_API_KEY", quoted(setting("slouchcat.firebase.apiKey", "SLOUCHCAT_FIREBASE_API_KEY")))
            buildConfigField("String", "FIREBASE_SENDER_ID", quoted(setting("slouchcat.firebase.senderId", "SLOUCHCAT_FIREBASE_SENDER_ID")))
        }
        // Its own id, so both builds can sit side by side on one device.
        create("foss") {
            dimension = "distribution"
            applicationIdSuffix = ".foss"
        }
    }
    defaultConfig {
        // The push relay of this build (concept 27, 7.3): fixed in the app (F-21: not changeable
        // in the app); a build with its own Firebase project names its own relay here.
        buildConfigField("String", "PUSH_RELAY", quoted(setting("slouchcat.pushRelay", "SLOUCHCAT_PUSH_RELAY").ifEmpty { "https://push.slouchcat.com" }))
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
    // Google's push only in `play` (F-33): `foss` has no Firebase on its class path.
    "playImplementation"(libs.firebase.messaging)
    implementation(project(":core:api"))
    implementation(project(":core:session"))
    implementation(project(":core:design"))
    implementation(project(":core:strings"))
    implementation(project(":core:player"))
    implementation(project(":core:media"))
    implementation(project(":core:browse"))
    testImplementation(project(":core:browse-testing"))
    implementation(project(":core:offline"))
    implementation(libs.media3.ui)
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.navigation.compose)
    implementation(libs.serialization.json)
    implementation(libs.coroutines.android)
    implementation(libs.coil.compose)
    implementation(libs.coil.network.okhttp)
    testImplementation(project(":core:player-testing"))
    testImplementation(libs.coroutines.test)
    androidTestImplementation(platform(libs.compose.bom))
    androidTestImplementation(libs.compose.ui.test.junit4)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.junit)
    androidTestImplementation(libs.androidx.test.core)
}

/** A Gradle property, else an environment variable, else empty (the Firebase values of stage 8). */
fun setting(property: String, environment: String): String =
    (providers.gradleProperty(property).orNull ?: providers.environmentVariable(environment).orNull).orEmpty().trim()

fun quoted(value: String): String = "\"" + value.replace("\\", "\\\\").replace("\"", "\\\"") + "\""
