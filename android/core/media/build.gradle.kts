// The data both apps show (concept 27, 4 and 5; concept 26, 5): the card model of a title and the
// addresses of its images, the check mark and the big button, the pager, the repositories of the
// start page, the libraries, the detail pages, people, the search, «Merkliste», «Gesehen», the
// collections and «Profile verwalten» over the generated client, the counts of the shell and the
// seam a test uses to put its own data in place (MediaData). No user interface: the phone and the
// TV app draw it each with their own building blocks.
plugins {
    id("slouchcat.android.library")
}

android {
    namespace = "com.slouchcat.core.media"
}

dependencies {
    api(project(":core:api"))
    api(project(":core:session"))
    // Only the stability annotations (@Immutable) for the apps' Compose compiler, not Compose itself.
    api(platform(libs.compose.bom))
    api(libs.compose.runtime.annotation)
    implementation(libs.coroutines.android)
    testImplementation(libs.junit)
    testImplementation(libs.coroutines.test)
    testImplementation(libs.okhttp.mockwebserver)
}
