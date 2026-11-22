// The Android apps of Slouchcat (concept 27, section 5; concept 26, 3.2). One Gradle build next to
// the server and the web app: the contract, the texts and the tokens it reads live in this repository.
pluginManagement {
    // Convention plugins (slouchcat.android.*, slouchcat.jvm.library) and the generators of the
    // client, the texts and the tokens (slouchcat.generators).
    includeBuild("build-logic")
    includeBuild("tools")
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "slouchcat-android"

include(
    ":app-mobile",
    ":app-tv",
    ":core:api",
    ":core:session",
    ":core:design",
    ":core:player",
    ":core:player-testing",
    ":core:media",
    ":core:browse",
    ":core:browse-testing",
    ":core:offline",
    ":core:strings",
    ":core:pairing",
    ":lint",
)
