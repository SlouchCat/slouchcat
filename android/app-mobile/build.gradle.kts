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
    implementation(project(":core:player"))
    implementation(project(":core:media"))
    implementation(project(":core:browse"))
    testImplementation(project(":core:browse-testing"))
    implementation(project(":core:offline"))
    // «Fernseher verbinden» (stage 9): the phone's side of the pairing, the relay setting, the scanner.
    implementation(project(":core:pairing"))
    implementation(libs.androidx.datastore.preferences)
    implementation(libs.zxing.core)
    "playImplementation"(libs.play.code.scanner)
    "playImplementation"(libs.play.services.base)
    "fossImplementation"(libs.camerax.camera2)
    "fossImplementation"(libs.camerax.lifecycle)
    "fossImplementation"(libs.camerax.view)
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

// The two flavours stay apart (code review m-9, docs/store/datensicherheit.md 1.3): the merged
// manifest of `play` declares no camera permission (Google's code scanner needs none), and `foss`
// carries no library of Google's Play services, ML Kit or Firebase. Part of `ciCheck`.
tasks.register("checkFlavourSeparation") {
    group = "verification"
    description = "The merged manifest of play has no CAMERA, foss has no Google Play services, ML Kit or Firebase"
    val play = layout.buildDirectory.file("intermediates/merged_manifest/playRelease/processPlayReleaseMainManifest/AndroidManifest.xml")
    val foss = layout.buildDirectory.file("intermediates/merged_manifest/fossRelease/processFossReleaseMainManifest/AndroidManifest.xml")
    val fossModules = configurations.named("fossReleaseRuntimeClasspath").map { classpath ->
        classpath.incoming.resolutionResult.allComponents.mapNotNull { (it.id as? ModuleComponentIdentifier)?.let { m -> "${m.group}:${m.module}" } }.sorted()
    }
    dependsOn("processPlayReleaseMainManifest", "processFossReleaseMainManifest")
    inputs.file(play)
    inputs.file(foss)
    inputs.property("fossModules", fossModules)
    doLast {
        val playManifest = play.get().asFile.readText()
        check("android.permission.CAMERA" !in playManifest) { "the merged manifest of play declares the camera permission" }
        val fossManifest = foss.get().asFile.readText()
        check("com.google.android.gms" !in fossManifest && "com.google.mlkit" !in fossManifest) { "the merged manifest of foss names Google Play services or ML Kit" }
        val google = fossModules.get().filter { it.startsWith("com.google.android.gms:") || it.startsWith("com.google.mlkit:") || it.startsWith("com.google.firebase:") }
        check(google.isEmpty()) { "foss carries Google's libraries: $google" }
    }
}
