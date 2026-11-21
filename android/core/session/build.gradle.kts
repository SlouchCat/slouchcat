// Server address, home network, compatibility with the server (concept 27, 5.1 and 6), the session:
// token store in the Android Keystore, account and profile, the repositories of sign-in, profiles
// and «Meine Geräte», the network monitor of «Server weist sich aus».
plugins {
    id("slouchcat.android.library")
    alias(libs.plugins.kotlin.serialization)
}

android {
    namespace = "com.slouchcat.core.session"
}

dependencies {
    api(project(":core:api"))
    implementation(libs.androidx.datastore.preferences)
    implementation(libs.coroutines.android)
    implementation(libs.serialization.json)
    testImplementation(libs.junit)
    testImplementation(libs.coroutines.test)
    testImplementation(libs.okhttp.mockwebserver)
}
