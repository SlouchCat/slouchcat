package com.slouchcat.app.mobile.downloads

import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.File
import javax.xml.parsers.DocumentBuilderFactory

/**
 * Nothing of the downloads goes into a backup or a device transfer (concept 27 9.4, review L-5):
 * the rules name only the server address, and the database (verifiers, PIN brake) and the files
 * live in `noBackupFilesDir`, which no rule can include.
 */
@org.junit.runner.RunWith(org.robolectric.RobolectricTestRunner::class)
@org.robolectric.annotation.Config(sdk = [36])
class OfflineStorageRulesTest {
    private fun includes(file: String): List<String> {
        val doc = DocumentBuilderFactory.newInstance().newDocumentBuilder().parse(File("src/main/res/xml/$file"))
        val nodes = doc.getElementsByTagName("include")
        return (0 until nodes.length).map { nodes.item(it).attributes.getNamedItem("domain").nodeValue + ":" + nodes.item(it).attributes.getNamedItem("path").nodeValue }
    }

    @Test
    fun backupAndTransferTakeOnlyTheServerAddress() {
        val only = "file:datastore/server.preferences_pb"
        assertEquals(listOf(only, only), includes("data_extraction_rules.xml"))
        assertEquals(listOf(only), includes("backup_rules.xml"))
    }

    @Test
    fun theFilesLiveInNoBackupFiles() {
        val context = org.robolectric.RuntimeEnvironment.getApplication()
        assertEquals(context.noBackupFilesDir, OfflineData.filesRoot(context).parentFile)
    }
}
