// The test double of the player for the tests of :core:player and of both apps (only as
// testImplementation): a player that behaves like Media3, a playback server and sample playback
// descriptions. Never part of an app.
plugins {
    id("slouchcat.android.library")
}

android {
    namespace = "com.slouchcat.core.player.testing"
}

dependencies {
    api(project(":core:api"))
    api(project(":core:player"))
    api(libs.coroutines.test)
}
