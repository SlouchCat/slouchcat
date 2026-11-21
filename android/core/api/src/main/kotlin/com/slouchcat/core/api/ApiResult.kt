package com.slouchcat.core.api

import com.slouchcat.core.api.generated.infrastructure.ClientError
import com.slouchcat.core.api.generated.infrastructure.ClientException
import com.slouchcat.core.api.generated.infrastructure.ServerError
import com.slouchcat.core.api.generated.infrastructure.ServerException
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.runInterruptible
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import java.io.IOException

/**
 * The outcome of one call of the generated client, the same for every page (concept 27, 11.5;
 * rules 2 and 12: not visible is 404 and the app shows «not there», never decides itself).
 */
sealed interface ApiResult<out T> {
    data class Ok<T>(val value: T) : ApiResult<T>

    /** 404: gone, or not visible to this profile. The page says it is not there. */
    data object NotFound : ApiResult<Nothing>

    /** 403: this session may not do it (`client_not_allowed`, a profile without the right). */
    data class NotAllowed(val code: String?) : ApiResult<Nothing>

    /** 401: the session is gone. Reported once to [SessionEvents]; the app returns to the sign-in. */
    data object SignedOut : ApiResult<Nothing>

    /** 426 `app_too_old`: the server wants a newer app. Reported to [SessionEvents]. */
    data object AppTooOld : ApiResult<Nothing>

    /** No answer: no network, server down, timeout. Worth a «try again». */
    data object Offline : ApiResult<Nothing>

    /** Any other answer: 5xx, another 4xx (with the problem `code` when the server sent one), an unreadable body. */
    data class Failed(val status: Int?, val code: String?) : ApiResult<Nothing>
}

/** The value of [ApiResult.Ok] turned into something else; every other case stays as it is. */
inline fun <T, R> ApiResult<T>.map(transform: (T) -> R): ApiResult<R> = when (this) {
    is ApiResult.Ok -> ApiResult.Ok(transform(value))
    ApiResult.NotFound -> ApiResult.NotFound
    is ApiResult.NotAllowed -> this
    ApiResult.SignedOut -> ApiResult.SignedOut
    ApiResult.AppTooOld -> ApiResult.AppTooOld
    ApiResult.Offline -> ApiResult.Offline
    is ApiResult.Failed -> this
}

/** What the whole app reacts to, wherever a call met it. */
enum class SessionEvent { SignedOut, AppTooOld }

/** One channel per app (held by its AppGraph) for [SessionEvent]s; the app's root collects it. */
class SessionEvents {
    private val flow = MutableSharedFlow<SessionEvent>(extraBufferCapacity = 8)
    val events: SharedFlow<SessionEvent> = flow.asSharedFlow()

    fun report(event: SessionEvent) {
        flow.tryEmit(event)
    }
}

/**
 * Runs calls of the generated client and turns their outcome into [ApiResult].
 *
 * ```
 * val result = api.call { SlouchcatApi.create(::ItemsApi, server, http).getItem(id) }
 * when (result) { is ApiResult.Ok -> …; ApiResult.NotFound -> …; else -> … }
 * ```
 * The block runs on the IO dispatcher and is interrupted when the coroutine is cancelled (the
 * generated client calls OkHttp blocking). A `null` body of a 2xx answer is [ApiResult.Failed].
 */
class ApiCaller(private val events: SessionEvents) {
    suspend fun <T : Any> call(tag: String = "api", block: () -> T?): ApiResult<T> = try {
        val value = runInterruptible(Dispatchers.IO) { block() }
        if (value == null) ApiResult.Failed(status = null, code = "empty") else ApiResult.Ok(value)
    } catch (e: CancellationException) {
        throw e
    } catch (e: ClientException) {
        val result = fromStatus(e.statusCode, problemCode((e.response as? ClientError<*>)?.body))
        Log.i(tag, "call refused", "status" to e.statusCode)
        result
    } catch (e: ServerException) {
        Log.w(tag, "server error", "status" to e.statusCode, "code" to problemCode((e.response as? ServerError<*>)?.body))
        ApiResult.Failed(e.statusCode, problemCode((e.response as? ServerError<*>)?.body))
    } catch (e: IOException) {
        Log.i(tag, "no answer", error = e)
        ApiResult.Offline
    } catch (e: SerializationException) {
        Log.w(tag, "answer unreadable", error = e)
        ApiResult.Failed(status = null, code = "unreadable")
    } catch (e: UnsupportedOperationException) {
        // The generated client throws this for 1xx and 3xx answers.
        Log.w(tag, "unexpected answer", error = e)
        ApiResult.Failed(status = null, code = "unexpected")
    }

    private fun fromStatus(status: Int, code: String?): ApiResult<Nothing> = when (status) {
        401 -> ApiResult.SignedOut.also { events.report(SessionEvent.SignedOut) }
        426 -> ApiResult.AppTooOld.also { events.report(SessionEvent.AppTooOld) }
        403 -> ApiResult.NotAllowed(code)
        404 -> ApiResult.NotFound
        else -> ApiResult.Failed(status, code)
    }

    /** The stable `code` of an RFC 9457 problem body (`Problem.code`), if the body is one. */
    private fun problemCode(body: Any?): String? {
        val text = body as? String ?: return null
        val obj = runCatching { SlouchcatApi.json.parseToJsonElement(text) as? JsonObject }.getOrNull() ?: return null
        return (obj["code"] as? JsonPrimitive)?.takeIf { it.isString }?.content
    }
}
