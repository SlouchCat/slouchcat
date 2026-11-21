plugins {
    `kotlin-dsl`
}

dependencies {
    // Compile-only: the root build puts the real plugins on the classpath (android/build.gradle.kts).
    compileOnly(libs.android.gradlePlugin)
    compileOnly(libs.kotlin.gradlePlugin)
    compileOnly(libs.compose.gradlePlugin)
}

gradlePlugin {
    plugins {
        register("androidApplication") {
            id = "slouchcat.android.application"
            implementationClass = "com.slouchcat.buildlogic.AndroidApplicationConventionPlugin"
        }
        register("androidLibrary") {
            id = "slouchcat.android.library"
            implementationClass = "com.slouchcat.buildlogic.AndroidLibraryConventionPlugin"
        }
        register("androidCompose") {
            id = "slouchcat.android.compose"
            implementationClass = "com.slouchcat.buildlogic.AndroidComposeConventionPlugin"
        }
        register("jvmLibrary") {
            id = "slouchcat.jvm.library"
            implementationClass = "com.slouchcat.buildlogic.JvmLibraryConventionPlugin"
        }
    }
}
