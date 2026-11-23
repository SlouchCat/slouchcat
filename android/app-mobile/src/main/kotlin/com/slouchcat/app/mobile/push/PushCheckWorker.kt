package com.slouchcat.app.mobile.push

import android.content.Context
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
import androidx.work.PeriodicWorkRequest
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import com.slouchcat.app.mobile.SlouchcatApplication
import java.util.concurrent.TimeUnit

/**
 * The check of the push subscription in the background (concept 27, 7.3: «Nur beim Start zu
 * prüfen reicht nicht»; code review M-4): about every 12 hours with a network, it asks the server
 * whether the subscription still stands and the VAPID key is the same; if not, it enters it
 * again. Sparing: two small requests when nothing changed, nothing while push is off. Its own job,
 * so the downloads' background sync of `:core:offline` stays as it is.
 */
class PushCheckWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        (applicationContext as? SlouchcatApplication)?.graph?.push?.sync()
        return Result.success()
    }

    companion object {
        const val NAME = "push-check"
        private const val HOURS = 12L

        fun request(): PeriodicWorkRequest = PeriodicWorkRequestBuilder<PushCheckWorker>(HOURS, TimeUnit.HOURS)
            .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
            .build()

        fun schedule(context: Context) {
            WorkManager.getInstance(context).enqueueUniquePeriodicWork(NAME, ExistingPeriodicWorkPolicy.KEEP, request())
        }
    }
}
