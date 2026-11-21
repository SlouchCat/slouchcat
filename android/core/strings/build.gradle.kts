// The texts of the apps (concept 27, 5.4): string resources generated from web/src/locales for the
// namespaces in android/strings-namespaces.txt. Use them as com.slouchcat.core.strings.R.string.<ns>_<key>.
import com.slouchcat.tools.GenerateStrings

plugins {
    id("slouchcat.android.library")
    id("slouchcat.generators")
}

android {
    namespace = "com.slouchcat.core.strings"
}

val repo = rootProject.layout.projectDirectory.dir("..")
val generateStrings = tasks.register<GenerateStrings>("generateStrings") {
    locales.from(repo.dir("web/src/locales").asFileTree.matching { include("*.json") })
    namespaces.set(rootProject.layout.projectDirectory.file("strings-namespaces.txt"))
    outputDir.set(layout.buildDirectory.dir("generated/slouchcat/res"))
}

androidComponents {
    onVariants { variant ->
        variant.sources.res?.addGeneratedSourceDirectory(generateStrings, GenerateStrings::outputDir)
    }
}
