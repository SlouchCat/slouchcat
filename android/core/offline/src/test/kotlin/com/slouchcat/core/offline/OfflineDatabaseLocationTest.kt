package com.slouchcat.core.offline

import androidx.test.core.app.ApplicationProvider
import com.slouchcat.core.offline.db.OfflineDatabase
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/** The database with the verifiers and the PIN brake lives in `noBackupFilesDir` (9.4, review L-5). */
@RunWith(RobolectricTestRunner::class)
class OfflineDatabaseLocationTest {
    @Test
    fun theDatabaseIsOutsideEveryBackup() {
        val context = ApplicationProvider.getApplicationContext<android.content.Context>()
        assertEquals(context.noBackupFilesDir, OfflineDatabase.file(context).parentFile)
    }
}
