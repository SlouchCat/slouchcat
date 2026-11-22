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
import com.slouchcat.core.offline.FetchRun
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

    /**
     * Calls [onChange] when the kind of the default network changes (none, Wi-Fi, cellular); the
     * frequent capability updates of a cellular network without another kind call nothing (code review M-6).
     */
    fun watch(onChange: (NetworkKind) -> Unit) {
        var last: NetworkKind? = null
        fun report(kind: NetworkKind) = synchronized(this) {
            if (kind == last) return
            last = kind
            onChange(kind)
        }
        manager?.registerDefaultNetworkCallback(
            object : ConnectivityManager.NetworkCallback() {
                override fun onAvailable(network: Network) = report(kind())

                override fun onLost(network: Network) = report(NetworkKind.None)

                override fun onCapabilitiesChanged(network: Network, capabilities: NetworkCapabilities) = report(kind())
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

    fun build(context: Context, progress: FetchProgress?): Notification {
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
        val neutral = context.getString(R.string.app_downloads_notificationTitle)
        val text = percent?.let { context.getString(R.string.app_downloads_notificationProgress, it.toString()) } ?: context.getString(R.string.app_downloads_notificationWaiting)
        fun base() = NotificationCompat.Builder(context, CHANNEL)
            .setSmallIcon(android.R.drawable.stat_sys_download)
            .setContentText(text)
            .setProgress(100, percent ?: 0, percent == null)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setSilent(true)
        // The lock screen shows no title (security review L-4); the title only for a profile that may see it.
        return base()
            .setContentTitle(progress?.title ?: neutral)
            .setVisibility(NotificationCompat.VISIBILITY_PRIVATE)
            .setPublicVersion(base().setContentTitle(neutral).build())
            .setContentIntent(open)
            .addAction(0, context.getString(R.string.app_downloads_pause), pause)
            .build()
    }

    fun update(context: Context, progress: FetchProgress?) {
        val allowed = Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
        if (!allowed) return
        context.getSystemService(NotificationManager::class.java)?.notify(ID, build(context, progress))
    }
}

/** «Anhalten» in the notification: every download the transfer would fetch waits until «Fortsetzen» (code review m-8). */
class PauseReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val engine = context.engine ?: return
        val pending = goAsync()
        CoroutineScope(Dispatchers.Default).launch {
            try {
                engine.pauseAll()
            } finally {
                pending.finish()
            }
        }
    }
}

/**
 * Runs the fetch with the notification while [scope] lives: a sync, then the files. The fetch is a
 * child of this coroutine: a stopped job or service stops it and waits until its file is closed.
 * The notification follows the progress (at most twice a second, no database read per step).
 */
private suspend fun fetchWithNotification(context: Context, engine: OfflineEngine, scope: CoroutineScope): FetchRun {
    val updates = scope.launch {
        engine.progress.collectLatest { progress -> DownloadNotifications.update(context, progress) }
    }
    try {
        engine.sync()
        return engine.runFetches()
    } finally {
        updates.cancel()
    }
}

/** After a run: what is left goes to WorkManager, a device error again after its pause (code review m-11, m-12). */
private suspend fun afterRun(engine: OfflineEngine, run: FetchRun) {
    when (run) {
        FetchRun.Left -> engine.scheduler.scheduleBackground(unmetered = engine.waitsForWifiOnly())
        is FetchRun.RetryAt -> engine.scheduler.scheduleBackground(unmetered = false, delayMs = (run.at - System.currentTimeMillis()).coerceAtLeast(0))
        // A download ordered while the run ended: one more look (code review m-12).
        FetchRun.Done -> if (engine.pendingFetches().isNotEmpty()) engine.scheduler.scheduleBackground(unmetered = false)
    }
}

private suspend fun safeRun(tag: String, block: suspend () -> FetchRun): FetchRun = try {
    block()
} catch (e: kotlinx.coroutines.CancellationException) {
    throw e
} catch (e: RuntimeException) {
    Log.w(tag, "transfer broke", error = e)
    FetchRun.Done
} catch (e: java.io.IOException) {
    Log.w(tag, "transfer broke", error = e)
    FetchRun.Done
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
        setNotification(params, DownloadNotifications.ID, DownloadNotifications.build(this, null), JOB_END_NOTIFICATION_POLICY_REMOVE)
        running = scope.launch {
            // A cold process (after a restart of the phone): the engine waits for the stored session (code review B-2).
            val run = safeRun(TAG) { fetchWithNotification(this@DownloadJobService, engine, this) }
            jobFinished(params, false)
            afterRun(engine, run)
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
        ServiceCompat.startForeground(this, DownloadNotifications.ID, DownloadNotifications.build(this, null), type)
        val engine = engine
        if (engine == null) {
            stopSelf()
            return START_NOT_STICKY
        }
        if (running?.isActive == true) return START_NOT_STICKY
        running = scope.launch {
            val run = safeRun(TAG) { fetchWithNotification(this@DownloadService, engine, this) }
            afterRun(engine, run)
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
    /**
     * A sync, then the files until shortly before WorkManager's limit; the fetch stops at the
     * deadline inside a file with its bytes durable. A worker stopped by the system cancels its fetch
     * (it is a child of this coroutine). The engine waits for the stored session of a cold process.
     */
    override suspend fun doWork(): Result {
        val engine = applicationContext.engine ?: return Result.success()
        val run = safeRun("offline-worker") {
            engine.sync()
            engine.runFetches(deadline = System.currentTimeMillis() + RUN_MS)
        }
        return when (run) {
            // Only what can go on at once comes back soon; a device error waits for its own run (m-11).
            FetchRun.Left -> if (runAttemptCount < MAX_ATTEMPTS) Result.retry() else Result.success()
            is FetchRun.RetryAt -> {
                engine.scheduler.scheduleBackground(unmetered = false, delayMs = (run.at - System.currentTimeMillis()).coerceAtLeast(0))
                Result.success()
            }
            FetchRun.Done -> Result.success()
        }
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

    override fun scheduleBackground(unmetered: Boolean, delayMs: Long) {
        val work = runCatching { WorkManager.getInstance(context) }.getOrNull() ?: return
        val network = if (unmetered) NetworkType.UNMETERED else NetworkType.CONNECTED
        // One work per condition: a run for any network does not wait behind one for Wi-Fi (code review m-13).
        work.enqueueUniqueWork(
            if (unmetered) FETCH_WORK_WIFI else FETCH_WORK,
            ExistingWorkPolicy.KEEP,
            OneTimeWorkRequestBuilder<OfflineWorker>()
                .setConstraints(Constraints.Builder().setRequiredNetworkType(network).build())
                .setInitialDelay(delayMs, TimeUnit.MILLISECONDS)
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
            it.cancelUniqueWork(FETCH_WORK_WIFI)
            it.cancelUniqueWork(SYNC_WORK)
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) context.getSystemService(JobScheduler::class.java)?.cancel(JOB_ID)
        context.stopService(Intent(context, DownloadService::class.java))
    }

    companion object {
        private const val TAG = "offline-schedule"
        const val JOB_ID = 4207
        const val FETCH_WORK = "offline-fetch"
        const val FETCH_WORK_WIFI = "offline-fetch-wifi"
        const val SYNC_WORK = "offline-sync"

        /** About every 6 hours with a network (F-34, the paper's recommendation). */
        const val SYNC_HOURS = 6L
        private const val BACKOFF_S = 30L
    }
}
