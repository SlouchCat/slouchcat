// The logic of the browsing pages both apps share (concept 27, 4; concept 26, 5): the ViewModels of the
// start page, the libraries, the lists, the detail pages, people, the search, «Merkliste», «Gesehen»,
// the collections and «Profile verwalten» with their states, messages and events. No user interface,
// no Compose: the phone and the TV app each draw these states with their own screens.
plugins {
    id("slouchcat.android.library")
}

android {
    namespace = "com.slouchcat.core.browse"
}

dependencies {
    api(project(":core:api"))
    api(project(":core:session"))
    api(project(":core:media"))
    api(libs.androidx.lifecycle.viewmodel)
    implementation(libs.coroutines.android)
    testImplementation(project(":core:browse-testing"))
    testImplementation(libs.junit)
    testImplementation(libs.coroutines.test)
}
