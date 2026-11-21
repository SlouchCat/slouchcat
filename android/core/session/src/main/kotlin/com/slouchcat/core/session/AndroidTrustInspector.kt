package com.slouchcat.core.session

import android.net.http.X509TrustManagerExtensions
import java.security.KeyStore
import java.security.cert.Certificate
import java.security.cert.X509Certificate
import javax.net.ssl.TrustManagerFactory
import javax.net.ssl.X509TrustManager

/**
 * Builds the trusted chain of a server the way Android does and asks whether its anchor is a CA
 * the user installed (`X509TrustManagerExtensions.isUserAddedCertificate`). Any failure counts as
 * «no»: the connection itself already passed the same check.
 */
class AndroidTrustInspector : TrustInspector {
    private val manager: X509TrustManager? by lazy {
        runCatching {
            TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
                .apply { init(null as KeyStore?) }
                .trustManagers.filterIsInstance<X509TrustManager>().firstOrNull()
        }.getOrNull()
    }

    override fun usesUserCertificate(chain: List<Certificate>, host: String): Boolean {
        val tm = manager ?: return false
        val x509 = chain.filterIsInstance<X509Certificate>()
        if (x509.isEmpty()) return false
        return runCatching {
            val extensions = X509TrustManagerExtensions(tm)
            val authType = if (x509.first().publicKey.algorithm == "EC") "ECDHE_ECDSA" else "ECDHE_RSA"
            extensions.checkServerTrusted(x509.toTypedArray(), authType, host).any { extensions.isUserAddedCertificate(it) }
        }.getOrDefault(false)
    }
}
