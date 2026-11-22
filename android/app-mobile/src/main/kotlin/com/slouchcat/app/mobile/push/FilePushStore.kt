package com.slouchcat.app.mobile.push

import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.push.PushKeys
import com.slouchcat.core.session.SecretBox
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import java.io.IOException
import java.security.GeneralSecurityException

/**
 * The push record in one file under `noBackupFilesDir` (outside every backup and device transfer),
 * sealed with an AES key of the Android Keystore ([box]): the private key of the subscription, the
 * Google token and the sealed address never lie in the clear and never in a log (rule 10:
 * `fcmtoken`, `sealedtoken`).
 */
class FilePushStore(private val file: File, private val box: SecretBox) : PushStore {
    @Serializable
    private data class Stored(
        val wanted: Boolean = true,
        val server: String? = null,
        val privateKey: String? = null,
        val publicKey: String? = null,
        val auth: String? = null,
        val fcmToken: String? = null,
        val vapidKey: String? = null,
        val endpoint: String? = null,
        val subscriptionId: Long? = null,
        val pendingDelete: Long? = null,
    ) {
        /** No key, token or address in a log or an exception (rule 10, code review m-10). */
        override fun toString(): String = "Stored(wanted=$wanted, subscriptionId=$subscriptionId, pendingDelete=$pendingDelete)"
    }

    private val json = Json { ignoreUnknownKeys = true }

    private suspend fun load(): Stored = withContext(Dispatchers.IO) {
        if (!file.exists()) return@withContext Stored()
        try {
            json.decodeFromString(Stored.serializer(), box.open(file.readBytes()).toString(Charsets.UTF_8))
        } catch (e: GeneralSecurityException) {
            // The key is gone (a restore on another device): start over, the next sync enters a new subscription.
            box.forget()
            Stored()
        } catch (e: IOException) {
            // Unreadable file: start over as well (code review m-1).
            Stored()
        } catch (e: IllegalArgumentException) {
            Stored()
        }
    }

    private suspend fun save(stored: Stored) = withContext(Dispatchers.IO) {
        file.parentFile?.mkdirs()
        val tmp = File(file.parentFile, file.name + ".tmp")
        tmp.writeBytes(box.seal(json.encodeToString(Stored.serializer(), stored).toByteArray(Charsets.UTF_8)))
        if (!tmp.renameTo(file)) {
            file.delete()
            tmp.renameTo(file)
        }
    }

    override suspend fun wanted(): Boolean = load().wanted

    override suspend fun setWanted(on: Boolean) = save(load().copy(wanted = on))

    override suspend fun read(): PushRecord? {
        val s = load()
        val server = s.server ?: return null
        val keys = runCatching {
            val priv = Base64Url.decode(s.privateKey ?: return@runCatching null)
            val pub = Base64Url.decode(s.publicKey ?: return@runCatching null)
            val auth = Base64Url.decode(s.auth ?: return@runCatching null)
            if (priv == null || pub == null || auth == null) null else PushKeys(priv, pub, auth)
        }.getOrNull()
        if (keys == null && s.pendingDelete == null) return null
        return PushRecord(server, keys, s.fcmToken, s.vapidKey, s.endpoint, s.subscriptionId, s.pendingDelete)
    }

    override suspend fun write(record: PushRecord?) {
        val wanted = load().wanted
        save(
            if (record == null) {
                Stored(wanted = wanted)
            } else {
                Stored(
                    wanted = wanted,
                    server = record.server,
                    privateKey = record.keys?.let { Base64Url.encode(it.privateKey) },
                    publicKey = record.keys?.let { Base64Url.encode(it.publicKey) },
                    auth = record.keys?.let { Base64Url.encode(it.auth) },
                    fcmToken = record.fcmToken,
                    vapidKey = record.vapidKey,
                    endpoint = record.endpoint,
                    subscriptionId = record.subscriptionId,
                    pendingDelete = record.pendingDelete,
                )
            },
        )
    }
}
