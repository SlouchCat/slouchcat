package com.slouchcat.core.session

import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.security.GeneralSecurityException
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** A [SecretBox] with a key in memory, AES-GCM like the Keystore box. */
class MemoryBox : SecretBox {
    var key: SecretKey? = null
    var forgotten = 0

    private fun key(): SecretKey = key ?: KeyGenerator.getInstance("AES").apply { init(256) }.generateKey().also { key = it }

    override fun seal(plain: ByteArray): ByteArray {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, key())
        return cipher.iv + cipher.doFinal(plain)
    }

    override fun open(sealed: ByteArray): ByteArray {
        val k = key ?: throw GeneralSecurityException("key gone")
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, k, GCMParameterSpec(128, sealed, 0, 12))
        return cipher.doFinal(sealed, 12, sealed.size - 12)
    }

    override fun forget() {
        forgotten++
        key = null
    }
}

class EncryptedFileTokenStoreTest {
    @get:Rule
    val folder = TemporaryFolder()

    private val token = "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_abcde"
    private val device = "0123456789abcdef0123456789abcdef"

    @Test
    fun storesEncryptedAndReadsBack() = runTest {
        val file = folder.root.resolve("session.bin")
        val box = MemoryBox()
        val store = EncryptedFileTokenStore(file, box)
        assertNull(store.read())
        store.write(StoredCredentials(token, device))
        val raw = file.readBytes().decodeToString(throwOnInvalidSequence = false)
        assertFalse("the token lies in the file in plain text", raw.contains(token))
        assertFalse(raw.contains(device))
        assertEquals(StoredCredentials(token, device), store.read())
    }

    @Test
    fun signingOutKeepsTheDeviceId() = runTest {
        val store = EncryptedFileTokenStore(folder.root.resolve("session.bin"), MemoryBox())
        store.write(StoredCredentials(token, device))
        store.write(StoredCredentials(null, device))
        assertEquals(StoredCredentials(null, device), store.read())
        store.write(StoredCredentials(null, null))
        assertNull(store.read())
    }

    @Test
    fun aLostKeyMeansSigningInAgain() = runTest {
        val file = folder.root.resolve("session.bin")
        val box = MemoryBox()
        val store = EncryptedFileTokenStore(file, box)
        store.write(StoredCredentials(token, device))
        box.key = null
        assertNull(store.read())
        assertFalse("the unreadable file stays", file.exists())
        assertEquals(1, box.forgotten)
        // A new sign-in works with a new key.
        store.write(StoredCredentials(token, device))
        assertEquals(StoredCredentials(token, device), store.read())
    }

    @Test
    fun aChangedFileIsNotTrusted() = runTest {
        val file = folder.root.resolve("session.bin")
        val store = EncryptedFileTokenStore(file, MemoryBox())
        store.write(StoredCredentials(token, device))
        val bytes = file.readBytes()
        bytes[bytes.size - 1] = (bytes[bytes.size - 1].toInt() xor 1).toByte()
        file.writeBytes(bytes)
        assertNull(store.read())
    }

    @Test
    fun theValuesNeverReachAString() {
        val text = StoredCredentials(token, device).toString()
        assertFalse(text.contains(token))
        assertFalse(text.contains(device))
        assertTrue(text.contains("[redacted]"))
    }
}
