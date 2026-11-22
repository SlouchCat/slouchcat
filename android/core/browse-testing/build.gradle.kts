// Test doubles of the shared browsing logic, for the tests of :core:browse and of both apps (only as
// testImplementation): a session over fakes, sample titles and the fakes of the :core:media repositories.
// Never part of an app.
plugins {
    id("slouchcat.android.library")
}

android {
    namespace = "com.slouchcat.core.browse.testing"
}

dependencies {
    api(project(":core:api"))
    api(project(":core:session"))
    api(project(":core:media"))
    api(libs.coroutines.test)
}
