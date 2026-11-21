// Server address, home network, compatibility with the server (concept 27, 5.1 and 6). The sign-in,
// the token store and the profile choice come with stage 3 part 2.
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
    testImplementation(libs.junit)
    testImplementation(libs.coroutines.test)
    testImplementation(libs.okhttp.mockwebserver)
}
