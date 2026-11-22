package com.slouchcat.core.offline.background

import android.Manifest
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.app.job.JobInfo
import android.app.job.JobParameters
import android.app.job.JobScheduler
import android.app.job.JobService
import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.os.Build
import android.os.IBinder
import androidx.annotation.RequiresApi
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import androidx.core.content.ContextCompat
import androidx.work.BackoffPolicy
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.offline.FetchProgress
import com.slouchcat.core.offline.FetchScheduler
import com.slouchcat.core.offline.NetworkKind
import com.slouchcat.core.offline.OfflineEngine
import com.slouchcat.core.strings.R
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch
import java.util.concurrent.TimeUnit

/** The app gives the background parts its one engine (the Application implements it). */
interface OfflineHost {
    val offlineEngine: OfflineEngine
}

private val Context.engine: OfflineEngine? get() = (applicationContext as? OfflineHost)?.offlineEngine

/** What the device knows of its default network, from the system (concept 27 9.6: Android knows Wi-Fi and cellular reliably). */
class AndroidNetwork(context: Context) {
    private val manager = context.getSystemService(ConnectivityManager::class.java)

    fun kind(): NetworkKind {
        val caps = manager?.let { it.getNetworkCapabilities(it.activeNetwork) } ?: return NetworkKind.None
        if (!caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)) return NetworkKind.None
        return when {
            caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED) -> NetworkKind.Wifi
            else -> NetworkKind.Cellular
        }
    }

    /** Calls [onChange] when the default network comes, goes or changes its kind. */
    fun watch(onChange: (NetworkKind) -> Unit) {
        manager?.registerDefaultNetworkCallback(
            object : ConnectivityManager.NetworkCallback() {
                override fun onAvailable(network: Network) = onChange(kind())

                override fun onLost(network: Network) = onChange(NetworkKind.None)

                override fun onCapabilitiesChanged(network: Network, capabilities: NetworkCapabilities) = onChange(kind())
            },
        )
    }
}

/** The notification of the fetch: progress, and «Anhalten» (store declaration, docs/store/datensicherheit.md 2.2). */
object DownloadNotifications {
    const val CHANNEL = "downloads"
    const val ID = 4207

    fun channel(context: Context) {
        val manager = context.getSystemService(NotificationManager::class.java) ?: return
        manager.createNotificationChannel(NotificationChannel(CHANNEL, context.getString(R.string.app_downloads_channel), NotificationManager.IMPORTANCE_LOW))
    }

    fun build(context: Context, progress: FetchProgress?, title: String?): Notification {
        channel(context)
        val percent = progress?.takeIf { it.total > 0 }?.let { (it.bytes * 100 / it.total).toInt() }
        val launch = context.packageManager.getLaunchIntentForPackage(context.packageName)
        val open = launch?.let { PendingIntent.getActivity(context, 0, it, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT) }
        val pause = PendingIntent.getBroadcast(
            context,
            1,
            Intent(context, PauseReceiver::class.java).setPackage(context.packageName),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        return NotificationCompat.Builder(context, CHANNEL)
            .setSmallIcon(android.R.drawable.stat_sys_download)
            .setContentTitle(title ?: context.getString(R.string.app_downloads_notificationTitle))
            .setContentText(percent?.let { context.getString(R.string.app_downloads_notificationProgress, it.toString()) } ?: context.getString(R.string.app_downloads_notificationWaiting))
            .setProgress(100, percent ?: 0, percent == null)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setSilent(true)
            .setContentIntent(open)
            .addAction(0, context.getString(R.string.app_downloads_pause), pause)
            .build()
    }

    fun update(context: Context, progress: FetchProgress?, title: String?) {
        val allowed = Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
        if (!allowed) return
        context.getSystemService(NotificationManager::class.java)?.notify(ID, build(context, progress, title))
    }
}

/** «Anhalten» in the notification: pauses the download coming in now. */
class PauseReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val engine = context.engine ?: return
        val pending = goAsync()
        CoroutineScope(Dispatchers.Default).launch {
            try {
                engine.progress.value?.id?.let { engine.pause(it) }
            } finally {
                pending.finish()
            }
        }
    }
}

/** Runs the fetch with the notification while [scope] lives; returns whether work is left. */
private suspend fun fetchWithNotification(context: Context, engine: OfflineEngine, scope: CoroutineScope): Boolean {
    val updates = scope.launch {
        engine.progress.collectLatest { progress ->
            val title = progress?.let { engine.download(it.id)?.item?.title }
            DownloadNotifications.update(context, progress, title)
        }
    }
    try {
        engine.sync()
        return engine.runFetches()
    } finally {
        updates.cancel()
    }
}

/**
 * A download the user started, from Android 14 on (stage 0 point 6): a user-initiated data transfer
 * job. It survives swiping the app away and a restart (persisted), shows its progress and has no
 * 6-hour limit; the system wants it scheduled while the app is visible, which holds right after a tap.
 */
class DownloadJobService : JobService() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var running: Job? = null

    override fun onStartJob(params: JobParameters): Boolean {
        // Only scheduled from Android 14 on (AndroidFetchScheduler); the manifest names it for every level.
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.UPSIDE_DOWN_CAKE) return false
        val engine = engine ?: return false
        setNotification(params, DownloadNotifications.ID, DownloadNotifications.build(this, null, null), JOB_END_NOTIFICATION_POLICY_REMOVE)
        running = scope.launch {
            val left = try {
                fetchWithNotification(this@DownloadJobService, engine, this)
            } catch (e: kotlinx.coroutines.CancellationException) {
                throw e
            } catch (e: RuntimeException) {
                Log.w(TAG, "transfer broke", error = e)
                false
            }
            jobFinished(params, false)
            // What waits for Wi-Fi or a later moment goes to WorkManager.
            if (left) engine.scheduler.scheduleBackground(unmetered = engine.waitsForWifiOnly())
        }
        return true
    }

    override fun onStopJob(params: JobParameters): Boolean {
        Log.i(TAG, "transfer stopped by the system", "reason" to if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) params.stopReason else -1)
        running?.cancel()
        // Again later; the next run resumes at the durable length.
        return true
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }

    private companion object {
        const val TAG = "offline-transfer"
    }
}

/** The same fetch before Android 14: a foreground service of the type `dataSync` (stage 0 point 6). */
class DownloadService : Service() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var running: Job? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val type = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC else 0
        ServiceCompat.startForeground(this, DownloadNotifications.ID, DownloadNotifications.build(this, null, null), type)
        val engine = engine
        if (engine == null) {
            stopSelf()
            return START_NOT_STICKY
        }
        if (running?.isActive == true) return START_NOT_STICKY
        running = scope.launch {
            val left = try {
                fetchWithNotification(this@DownloadService, engine, this)
            } catch (e: kotlinx.coroutines.CancellationException) {
                throw e
            } catch (e: RuntimeException) {
                Log.w(TAG, "transfer broke", error = e)
                false
            }
            if (left) engine.scheduler.scheduleBackground(unmetered = engine.waitsForWifiOnly())
            ServiceCompat.stopForeground(this@DownloadService, ServiceCompat.STOP_FOREGROUND_REMOVE)
            stopSelf()
        }
        return START_NOT_STICKY
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }

    private companion object {
        const val TAG = "offline-transfer"
    }
}

/**
 * The background run (concept 27 9.3 and 9.6, stage 0 point 7): a sync, then the pending files in a
 * short run of WorkManager that resumes where the last one stopped. Periodic about every 6 hours
 * with a network (F-34), so the server creates auto-downloads while the app is closed; once more
 * after a run that left work (with Wi-Fi only when a rule or the device says so).
 */
class OfflineWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        val engine = applicationContext.engine ?: return Result.success()
        engine.sync()
        val left = engine.runFetches(deadline = System.currentTimeMillis() + RUN_MS)
        return if (left && runAttemptCount < MAX_ATTEMPTS) Result.retry() else Result.success()
    }

    companion object {
        /** Below the 10 minutes WorkManager gives a run. */
        const val RUN_MS = 9 * 60_000L
        const val MAX_ATTEMPTS = 50
    }
}

/** [FetchScheduler] with the means of Android (UIDT from API 34, the dataSync service before, WorkManager). */
class AndroidFetchScheduler(private val context: Context, private val visible: () -> Boolean) : FetchScheduler {
    override fun startTransfer() {
        if (!visible()) {
            scheduleBackground(unmetered = false)
            return
        }
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) startJob() else ContextCompat.startForegroundService(context, Intent(context, DownloadService::class.java))
        } catch (e: RuntimeException) {
            // Not allowed now (the app left the screen a moment ago): WorkManager takes it.
            Log.w(TAG, "transfer not started", error = e)
            scheduleBackground(unmetered = false)
        }
    }

    @RequiresApi(Build.VERSION_CODES.UPSIDE_DOWN_CAKE)
    private fun startJob() {
        val scheduler = context.getSystemService(JobScheduler::class.java) ?: return
        // A running transfer stays: scheduling the same id again would stop it.
        if (scheduler.getPendingJob(JOB_ID) != null) return
        val job = JobInfo.Builder(JOB_ID, ComponentName(context, DownloadJobService::class.java))
            .setUserInitiated(true)
            .setRequiredNetworkType(JobInfo.NETWORK_TYPE_ANY)
            .setPersisted(true)
            .build()
        scheduler.schedule(job)
    }

    override fun scheduleBackground(unmetered: Boolean) {
        val work = runCatching { WorkManager.getInstance(context) }.getOrNull() ?: return
        val network = if (unmetered) NetworkType.UNMETERED else NetworkType.CONNECTED
        work.enqueueUniqueWork(
            FETCH_WORK,
            ExistingWorkPolicy.KEEP,
            OneTimeWorkRequestBuilder<OfflineWorker>()
                .setConstraints(Constraints.Builder().setRequiredNetworkType(network).build())
                .setBackoffCriteria(BackoffPolicy.LINEAR, BACKOFF_S, TimeUnit.SECONDS)
                .build(),
        )
        work.enqueueUniquePeriodicWork(
            SYNC_WORK,
            ExistingPeriodicWorkPolicy.KEEP,
            PeriodicWorkRequestBuilder<OfflineWorker>(SYNC_HOURS, TimeUnit.HOURS)
                .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
                .build(),
        )
    }

    override fun cancelAll() {
        runCatching { WorkManager.getInstance(context) }.getOrNull()?.let {
            it.cancelUniqueWork(FETCH_WORK)
            it.cancelUniqueWork(SYNC_WORK)
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) context.getSystemService(JobScheduler::class.java)?.cancel(JOB_ID)
        context.stopService(Intent(context, DownloadService::class.java))
    }

    companion object {
        private const val TAG = "offline-schedule"
        const val JOB_ID = 4207
        const val FETCH_WORK = "offline-fetch"
        const val SYNC_WORK = "offline-sync"

        /** About every 6 hours with a network (F-34, the paper's recommendation). */
        const val SYNC_HOURS = 6L
        private const val BACKOFF_S = 30L
    }
}
