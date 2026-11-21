package com.slouchcat.app.mobile

import android.app.Application
import android.content.Context
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SessionEvents
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.api.log.LogSink
import com.slouchcat.core.session.AndroidTrustInspector
import com.slouchcat.core.session.ApiServerRepository
import com.slouchcat.core.session.DataStoreServerStore
import com.slouchcat.core.session.ServerRepository
import com.slouchcat.core.session.HealthServerProbe
import com.slouchcat.core.session.HostResolver
import com.slouchcat.core.session.ServerProbe
import com.slouchcat.core.session.ServerStore
import okhttp3.OkHttpClient

/**
 * The dependencies of the app, built by hand once per process (no Hilt, no Koin; build log,
 * Android architecture). A screen gets what it needs from here through its ViewModel factory.
 */
class AppGraph(context: Context) {
    val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, BuildConfig.VERSION_NAME)

    /** The one HTTP client for API, images and player (concept 27, 5.2). */
    val http: OkHttpClient = SlouchcatHttp.client(identity)

    /** «Sitzung weg» and «App zu alt» from any call; SlouchcatNavHost reacts to them. */
    val sessionEvents = SessionEvents()

    /** Every call of the generated client goes through it: `graph.api.call { … }`. */
    val api = ApiCaller(sessionEvents)

    val serverStore: ServerStore = DataStoreServerStore(context)
    val serverProbe: ServerProbe = HealthServerProbe(http, AndroidTrustInspector())
    val serverRepository: ServerRepository = ApiServerRepository(api, http)
    val hostResolver: HostResolver = HostResolver.System
}

class SlouchcatApplication : Application() {
    lateinit var graph: AppGraph
        private set

    override fun onCreate() {
        super.onCreate()
        Log.sink = AndroidLogSink(BuildConfig.DEBUG)
        graph = AppGraph(this)
    }
}

/**
 * Logcat. The only place of the app that calls android.util.Log (lint rule SlouchcatDirectLog);
 * what arrives here went through the redaction of [Log]. Release builds keep warnings and errors.
 */
class AndroidLogSink(private val verbose: Boolean) : LogSink {
    override fun write(priority: Int, tag: String, message: String) {
        if (!verbose && priority < Log.WARN) return
        android.util.Log.println(priority, "Slouchcat/$tag", message)
    }
}
