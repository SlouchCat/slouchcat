package com.slouchcat.app.mobile.push

import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.push.PushKeys
import com.slouchcat.core.session.SecretBox
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
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
    )

    private val json = Json { ignoreUnknownKeys = true }

    private suspend fun load(): Stored = withContext(Dispatchers.IO) {
        if (!file.exists()) return@withContext Stored()
        try {
            json.decodeFromString(Stored.serializer(), box.open(file.readBytes()).toString(Charsets.UTF_8))
        } catch (e: GeneralSecurityException) {
            // The key is gone (a restore on another device): start over, the next sync enters a new subscription.
            box.forget()
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
        val keys = runCatching {
            PushKeys(Base64Url.decode(s.privateKey ?: return null)!!, Base64Url.decode(s.publicKey ?: return null)!!, Base64Url.decode(s.auth ?: return null)!!)
        }.getOrNull() ?: return null
        return PushRecord(s.server ?: return null, keys, s.fcmToken, s.vapidKey, s.endpoint, s.subscriptionId)
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
                    privateKey = Base64Url.encode(record.keys.privateKey),
                    publicKey = Base64Url.encode(record.keys.publicKey),
                    auth = Base64Url.encode(record.keys.auth),
                    fcmToken = record.fcmToken,
                    vapidKey = record.vapidKey,
                    endpoint = record.endpoint,
                    subscriptionId = record.subscriptionId,
                )
            },
        )
    }
}
