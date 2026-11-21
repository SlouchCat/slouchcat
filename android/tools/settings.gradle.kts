// The generators of the Android build (concept 27, 5.3 to 5.5): the pre-pass of the Kotlin client,
// the texts from web/src/locales and the call of the Kotlin output of packages/design-tokens.
dependencyResolutionManagement {
    repositories {
        mavenCentral()
        gradlePluginPortal()
    }
    versionCatalogs {
        create("libs") {
            from(files("../gradle/libs.versions.toml"))
        }
    }
}

rootProject.name = "tools"
