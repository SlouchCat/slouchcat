package com.slouchcat.core.session

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import java.io.File
import java.io.IOException
import java.security.GeneralSecurityException
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** What the sign-in leaves on the device: the bearer token and the device id the server handed out. */
data class StoredCredentials(val token: String?, val deviceId: String?) {
    override fun toString(): String = "StoredCredentials(token=${if (token == null) "none" else "[redacted]"}, deviceId=${if (deviceId == null) "none" else "[redacted]"})"
}

/** Where token and device id live (concept 27, 4.2); a seam for tests. */
interface TokenStore {
    /** The stored values; `null` when there are none or they cannot be read any more (key lost). */
    suspend fun read(): StoredCredentials?

    suspend fun write(credentials: StoredCredentials)
}

/** Encrypts and decrypts small secrets; the app's box holds its key in the Android Keystore. */
interface SecretBox {
    /** Throws [GeneralSecurityException] when the key is gone or the data was changed. */
    fun seal(plain: ByteArray): ByteArray

    fun open(sealed: ByteArray): ByteArray

    /** Forgets the key (after it failed); the next [seal] makes a new one. */
    fun forget()
}

/**
 * AES-256-GCM with a key that never leaves the Android Keystore (concept 27, 4.2; no
 * `security-crypto`). Sealed data is the 12-byte IV followed by the ciphertext and tag.
 */
class KeystoreSecretBox(private val alias: String = ALIAS) : SecretBox {
    private val keyStore: KeyStore by lazy { KeyStore.getInstance(PROVIDER).apply { load(null) } }

    private fun key(create: Boolean): SecretKey? {
        (keyStore.getKey(alias, null) as? SecretKey)?.let { return it }
        if (!create) return null
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, PROVIDER)
        generator.init(
            KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build(),
        )
        return generator.generateKey()
    }

    override fun seal(plain: ByteArray): ByteArray {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, key(create = true))
        cipher.updateAAD(alias.toByteArray(Charsets.UTF_8))
        return cipher.iv + cipher.doFinal(plain)
    }

    override fun open(sealed: ByteArray): ByteArray {
        val key = key(create = false) ?: throw GeneralSecurityException("the key of the token store is gone")
        if (sealed.size <= IV_BYTES) throw GeneralSecurityException("sealed data too short")
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(TAG_BITS, sealed, 0, IV_BYTES))
        // The alias as associated data: a blob sealed for another purpose does not open here.
        cipher.updateAAD(alias.toByteArray(Charsets.UTF_8))
        return cipher.doFinal(sealed, IV_BYTES, sealed.size - IV_BYTES)
    }

    override fun forget() {
        runCatching { keyStore.deleteEntry(alias) }
    }

    private companion object {
        const val PROVIDER = "AndroidKeyStore"
        const val ALIAS = "slouchcat-session-v1"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val IV_BYTES = 12
        const val TAG_BITS = 128
    }
}

/**
 * [TokenStore] in one encrypted file. Put [file] into `noBackupFilesDir`: Android leaves it out of
 * the backup, and the data extraction rules of the app name only the server address, so a device
 * transfer leaves it behind too (a new phone signs in again, 4.2). If the key is lost (a restored
 * device, a reset lock screen on some devices) the file cannot be opened: it is deleted and the
 * app shows the sign-in, the server address stays.
 */
class EncryptedFileTokenStore(private val file: File, private val box: SecretBox) : TokenStore {
    @Serializable
    private data class Content(val token: String? = null, val deviceId: String? = null)

    private val json = Json { ignoreUnknownKeys = true }

    override suspend fun read(): StoredCredentials? = withContext(Dispatchers.IO) {
        if (!file.exists()) return@withContext null
        try {
            val content = json.decodeFromString(Content.serializer(), box.open(file.readBytes()).decodeToString())
            StoredCredentials(content.token, content.deviceId)
        } catch (e: GeneralSecurityException) {
            lost(e)
        } catch (e: IOException) {
            lost(e)
        } catch (e: SerializationException) {
            lost(e)
        } catch (e: IllegalArgumentException) {
            lost(e)
        }
    }

    private fun lost(e: Exception): StoredCredentials? {
        Log.w(TAG, "token store unreadable, signing in again", error = e)
        file.delete()
        box.forget()
        return null
    }

    override suspend fun write(credentials: StoredCredentials) = withContext(Dispatchers.IO) {
        if (credentials.token == null && credentials.deviceId == null) {
            file.delete()
            return@withContext
        }
        val sealed = box.seal(json.encodeToString(Content.serializer(), Content(credentials.token, credentials.deviceId)).encodeToByteArray())
        file.parentFile?.mkdirs()
        val next = File(file.parentFile, "${file.name}.new")
        next.writeBytes(sealed)
        if (!next.renameTo(file)) {
            file.delete()
            if (!next.renameTo(file)) throw IOException("could not store the session")
        }
    }

    private companion object {
        const val TAG = "session"
    }
}
