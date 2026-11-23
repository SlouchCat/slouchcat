package com.slouchcat.core.api.log

/** Where log lines end up; the app installs one that writes to logcat. */
interface LogSink {
    /** [priority] as `android.util.Log` counts it (2 verbose … 6 error). */
    fun write(priority: Int, tag: String, message: String)
}

/**
 * The one logger of the apps (rule 10, concept 27 5.6). Token, voucher, device id and stream tokens
 * never reach a log line: values of fields with such names are replaced, and the message and the
 * text of an error are scrubbed of the forms those values travel in (Bearer header, `dt`/`st`/`rt`
 * in an address, the device header). The lint rule `SlouchcatDirectLog` reports every other way to
 * logcat.
 *
 * ```
 * Log.i("setup", "server found", "version" to info.version)
 * Log.w("setup", "probe failed", error = e)
 * ```
 * Put values into fields, not into the message: a field is redacted by its name.
 */
object Log {
    const val VERBOSE = 2
    const val DEBUG = 3
    const val INFO = 4
    const val WARN = 5
    const val ERROR = 6

    @Volatile
    var sink: LogSink = object : LogSink {
        override fun write(priority: Int, tag: String, message: String) = Unit
    }

    fun d(tag: String, message: String, vararg fields: Pair<String, Any?>, error: Throwable? = null) =
        write(DEBUG, tag, message, fields, error)

    fun i(tag: String, message: String, vararg fields: Pair<String, Any?>, error: Throwable? = null) =
        write(INFO, tag, message, fields, error)

    fun w(tag: String, message: String, vararg fields: Pair<String, Any?>, error: Throwable? = null) =
        write(WARN, tag, message, fields, error)

    fun e(tag: String, message: String, vararg fields: Pair<String, Any?>, error: Throwable? = null) =
        write(ERROR, tag, message, fields, error)

    private fun write(priority: Int, tag: String, message: String, fields: Array<out Pair<String, Any?>>, error: Throwable?) {
        sink.write(priority, tag, Redaction.line(message, fields.toList(), error))
    }
}

/** The rules of [Log], separate so they can be tested on their own. */
object Redaction {
    const val REDACTED = "[redacted]"

    /**
     * Key suffixes whose values never appear, compared after lower-casing and dropping `-` and `_`:
     * the list of the server's redaction handler (`server/internal/logging`, `sensitiveKeys`, rule
     * 10) plus what only an app holds. `accessToken`, `push_endpoint`, `pairingVoucher` all end in
     * one of them.
     */
    val SECRET_SUFFIXES = listOf(
        // server/internal/logging/logging.go, sensitiveKeys
        "password", "pin", "token", "apikey", "authorization", "secret", "pinhash", "tokenhash", "passwordhash",
        "youtubeapikey", "tvdbapikey", "tvdbpin", "verifier", "clientid",
        "smtppassword", "bottoken", "passphrase", "vapidprivatekey", "p256dh", "authsecret", "endpoint",
        "listurl", "deviceid", "clientkey",
        "seerrapikey", "newpassword", "radarrapikey", "sonarrapikey",
        "apptoken", "devicetoken", "slouchcatdevice",
        "voucher", "pickup", "pickupsecret", "pickuphash", "pairid", "meetingpoint", "meetingpointid", "packet",
        "sealedtoken", "fcmtoken",
        // the apps: voucher, pickup secret of the meeting point, cookie, Bearer header, device header
        "voucher", "pickup", "cookie", "bearer", "xslouchcatdevice",
    )

    /** Short names that are secret only as the whole name (`st`, but not `host` or `list`). */
    val SECRET_NAMES = setOf("dt", "st", "rt", "device", "key", "sig", "signature")

    fun isSecret(key: String): Boolean {
        val k = key.lowercase().replace("-", "").replace("_", "")
        return k in SECRET_NAMES || SECRET_SUFFIXES.any { k.endsWith(it) }
    }

    private val patterns = listOf(
        Regex("""(?i)(Bearer\s+)[^\s,;"']+""") to "$1$REDACTED",
        Regex("""(?i)(Pickup\s+)[^\s,;"']+""") to "$1$REDACTED",
        Regex("""(?i)(X-Slouchcat-Device\s*[:=]\s*)[^\s,;"']+""") to "$1$REDACTED",
        // The push address holds the sealed Google token (stage 8, security review L-2).
        Regex("""(/v1/push/)[^\s?#"']+""") to "$1$REDACTED",
        // The query parameters of the server's list (logging.go, sensitiveQuery), and `voucher`.
        Regex(
            """(?i)((?:^|[?&;])(?:api_key|apikey|key|client_key|st|dt|rt|auth|token|access_token|secret|sig|signature|voucher)=)[^&;#\s"']*""",
        ) to "$1$REDACTED",
    )

    /** `"name":"value"` in JSON quoted by an error text; the value goes when the name is secret. */
    private val jsonPair = Regex(""""([A-Za-z0-9_-]+)"\s*:\s*"((?:[^"\\]|\\.)*)"?""")

    fun scrub(text: String): String {
        val plain = patterns.fold(text) { acc, (regex, replacement) -> regex.replace(acc, replacement) }
        return jsonPair.replace(plain) { m ->
            if (isSecret(m.groupValues[1])) "\"${m.groupValues[1]}\":\"$REDACTED\"" else m.value
        }
    }

    fun line(message: String, fields: List<Pair<String, Any?>>, error: Throwable?): String = buildString {
        append(scrub(message))
        for ((key, value) in fields) {
            append(' ').append(key).append('=')
            append(if (isSecret(key)) REDACTED else scrub(value.toString()))
        }
        // The class and the scrubbed message of the error, no stack trace: the messages of OkHttp
        // and of the generated client carry addresses, and a stack adds nothing a user could act on.
        // A serialization error quotes the body it could not read («JSON input: …»): only its class.
        var cause = error
        var depth = 0
        while (cause != null && depth < 4) {
            append(if (depth == 0) " error=" else " cause=")
            append(cause.javaClass.name)
            if (!cause.isSerializationError()) cause.message?.let { append(": ").append(scrub(it)) }
            cause = cause.cause
            depth++
        }
    }

    private fun Throwable.isSerializationError(): Boolean =
        generateSequence<Class<*>>(javaClass) { it.superclass }.any { it.name == "kotlinx.serialization.SerializationException" }
}
