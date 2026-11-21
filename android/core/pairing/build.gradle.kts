// The pairing of a TV over the relay (concept 26, 4.3; concept 27, 7.1 and 7.2; build log
// «Protokolle»): one-time X25519 key and pickup secret, the content of the QR code, the client of
// the relay, HPKE of the packet and of the redeem answer, the redeem at the user's server. The TV
// app uses the receiving side, the phone app (stage 9) the sending side. Plain Kotlin, no Android.
plugins {
    id("slouchcat.jvm.library")
    alias(libs.plugins.kotlin.serialization)
}

dependencies {
    api(project(":core:api"))
    implementation(libs.tink.android)
    testImplementation(libs.junit)
    testImplementation(libs.coroutines.test)
    testImplementation(libs.okhttp.mockwebserver)
}

// The shared test vectors of the repository (testdata/crypto, format slouchcat-crypto-vectors-v1),
// the same files the Go tests of the server and the relay read.
tasks.named<Test>("test") {
    val vectors = rootProject.layout.projectDirectory.dir("../testdata/crypto")
    inputs.dir(vectors).withPathSensitivity(PathSensitivity.RELATIVE)
    systemProperty("slouchcat.vectors", vectors.asFile.absolutePath)
}
