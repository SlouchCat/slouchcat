plugins {
    `kotlin-dsl`
}

dependencies {
    // YAML and JSON (a subset of YAML) with the order of the keys kept.
    implementation(libs.snakeyaml.engine)
    testImplementation(libs.junit)
}

gradlePlugin {
    plugins {
        // Applying it puts the task types of this build (TrimOpenApi, GenerateStrings,
        // GenerateTokens) on the module's build classpath; the module registers them itself.
        register("generators") {
            id = "slouchcat.generators"
            implementationClass = "com.slouchcat.tools.GeneratorsPlugin"
        }
    }
}

tasks.test {
    // The test over the real locale files reads them from the repository.
    systemProperty("slouchcat.repo", rootDir.resolve("../..").canonicalPath)
}
