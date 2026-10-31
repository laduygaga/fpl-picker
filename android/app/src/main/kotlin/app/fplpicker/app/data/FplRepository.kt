package app.fplpicker.app.data

import android.content.Context
import app.fplpicker.core.api.BootstrapStaticResponse
import app.fplpicker.core.api.Fixture
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.OkHttpClient
import okhttp3.Request
import java.io.File
import java.util.concurrent.TimeUnit

@Serializable
data class CacheMeta(
    val fetchedAt: Long,
    val etag: String?
)

class FplRepository(context: Context) {
    private val client = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(15, TimeUnit.SECONDS)
        .build()

    private val json = Json { ignoreUnknownKeys = true; isLenient = true }
    private val cacheDir = File(context.cacheDir, "fpl_cache").apply { mkdirs() }
    private val cacheTtlMs = 60 * 60 * 1000L // 1 hour

    fun fetchBootstrap(fresh: Boolean = false): BootstrapStaticResponse {
        val file = File(cacheDir, "bootstrap.json")
        val metaFile = File(cacheDir, "bootstrap.meta.json")

        if (!fresh && file.exists() && metaFile.exists()) {
            try {
                val meta = json.decodeFromString<CacheMeta>(metaFile.readText())
                if (System.currentTimeMillis() - meta.fetchedAt < cacheTtlMs) {
                    return json.decodeFromString(file.readText())
                }
            } catch (_: Exception) {}
        }

        val requestBuilder = Request.Builder().url("https://fantasy.premierleague.com/api/bootstrap-static/")
        val metaEtag = if (!fresh && metaFile.exists()) {
            try {
                json.decodeFromString<CacheMeta>(metaFile.readText()).etag
            } catch (_: Exception) { null }
        } else null

        if (metaEtag != null) {
            requestBuilder.header("If-None-Match", metaEtag)
        }

        val response = client.newCall(requestBuilder.build()).execute()
        val etag = response.header("ETag")

        if (response.code == 304 && file.exists()) {
            if (metaFile.exists()) {
                try {
                    val oldMeta = json.decodeFromString<CacheMeta>(metaFile.readText())
                    metaFile.writeText(json.encodeToString(CacheMeta(System.currentTimeMillis(), etag ?: oldMeta.etag)))
                } catch (_: Exception) {}
            }
            return json.decodeFromString(file.readText())
        }

        val body = response.body
        if (!response.isSuccessful || body == null) {
            if (file.exists()) {
                return json.decodeFromString(file.readText())
            }
            throw RuntimeException("Failed to fetch bootstrap-static: HTTP ${response.code}")
        }

        val bodyStr = body.string()
        file.writeText(bodyStr)
        metaFile.writeText(json.encodeToString(CacheMeta(System.currentTimeMillis(), etag)))

        return json.decodeFromString(bodyStr)
    }

    fun fetchFixtures(fresh: Boolean = false): List<Fixture> {
        val file = File(cacheDir, "fixtures.json")
        val metaFile = File(cacheDir, "fixtures.meta.json")

        if (!fresh && file.exists() && metaFile.exists()) {
            try {
                val meta = json.decodeFromString<CacheMeta>(metaFile.readText())
                if (System.currentTimeMillis() - meta.fetchedAt < cacheTtlMs) {
                    return json.decodeFromString(file.readText())
                }
            } catch (_: Exception) {}
        }

        val requestBuilder = Request.Builder().url("https://fantasy.premierleague.com/api/fixtures/")
        val metaEtag = if (!fresh && metaFile.exists()) {
            try {
                json.decodeFromString<CacheMeta>(metaFile.readText()).etag
            } catch (_: Exception) { null }
        } else null

        if (metaEtag != null) {
            requestBuilder.header("If-None-Match", metaEtag)
        }

        val response = client.newCall(requestBuilder.build()).execute()
        val etag = response.header("ETag")

        if (response.code == 304 && file.exists()) {
            if (metaFile.exists()) {
                try {
                    val oldMeta = json.decodeFromString<CacheMeta>(metaFile.readText())
                    metaFile.writeText(json.encodeToString(CacheMeta(System.currentTimeMillis(), etag ?: oldMeta.etag)))
                } catch (_: Exception) {}
            }
            return json.decodeFromString(file.readText())
        }

        val body = response.body
        if (!response.isSuccessful || body == null) {
            if (file.exists()) {
                return json.decodeFromString(file.readText())
            }
            throw RuntimeException("Failed to fetch fixtures: HTTP ${response.code}")
        }

        val bodyStr = body.string()
        file.writeText(bodyStr)
        metaFile.writeText(json.encodeToString(CacheMeta(System.currentTimeMillis(), etag)))

        return json.decodeFromString(bodyStr)
    }
}
