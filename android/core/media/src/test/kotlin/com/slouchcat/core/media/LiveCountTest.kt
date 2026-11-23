package com.slouchcat.core.media

import com.slouchcat.core.api.ApiResult
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class LiveCountTest {
    @Test
    fun aReadKeepsTheNumberUpToDateUnlessAPageChangedItMeanwhile() = runTest {
        val count = LiveCount()
        count.refresh { ApiResult.Ok(3) }
        assertEquals(3, count.count.value)
        count.refresh { ApiResult.Offline }
        assertEquals(3, count.count.value)
        // A slow read of the rail, «Alle gelesen» of the page answers first: the 0 stays.
        val answer = CompletableDeferred<ApiResult<Int>>()
        launch { count.refresh { answer.await() } }
        runCurrent()
        count.set(0)
        answer.complete(ApiResult.Ok(3))
        runCurrent()
        assertEquals(0, count.count.value)
        count.adjust(+1)
        assertEquals(1, count.count.value)
        count.set(null)
        count.adjust(+1)
        assertNull(count.count.value)
    }
}
