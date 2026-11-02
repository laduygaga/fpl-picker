package app.fplpicker.app.data

import android.content.Context
import app.fplpicker.core.api.*
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
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

    fun getMyEntryId(cookies: String): Int? {
        try {
            val bearer = if (cookies.startsWith("eyJ") || cookies.contains("eyJhbGci")) cookies else ""
            val urls = listOf(
                "https://fantasy.premierleague.com/api/me/",
                "https://fantasy.premierleague.com/api/entry/"
            )
            for (u in urls) {
                try {
                    val requestBuilder = Request.Builder()
                        .url(u)
                        .header("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
                        .header("Referer", "https://fantasy.premierleague.com/")

                    if (bearer.isNotBlank()) {
                        requestBuilder.header("x-api-authorization", "Bearer $bearer")
                        requestBuilder.header("Authorization", "Bearer $bearer")
                    } else {
                        requestBuilder.header("Cookie", cookies)
                    }

                    val response = client.newCall(requestBuilder.build()).execute()
                    val body = response.body?.string() ?: continue
                    if (!response.isSuccessful) continue

                    val root = json.parseToJsonElement(body) as? kotlinx.serialization.json.JsonObject
                    
                    val player = root?.get("player") as? kotlinx.serialization.json.JsonObject
                    val entry1 = player?.get("entry")?.toString()?.toIntOrNull()
                    if (entry1 != null) return entry1

                    val entry2 = root?.get("entry")?.toString()?.toIntOrNull()
                    if (entry2 != null) return entry2

                    val id3 = root?.get("id")?.toString()?.toIntOrNull()
                    if (id3 != null) return id3

                    val teams = root?.get("teams") as? kotlinx.serialization.json.JsonArray
                    if (teams != null && teams.isNotEmpty()) {
                        val firstTeam = teams[0] as? kotlinx.serialization.json.JsonObject
                        val id = firstTeam?.get("id")?.toString()?.toIntOrNull()
                        if (id != null) return id
                    }
                } catch (_: Exception) {}
            }
            return null
        } catch (_: Exception) {
            return null
        }
    }

    fun getMyTeam(teamId: Int, cookies: String): MyTeam {
        val bearer = if (cookies.startsWith("eyJ") || cookies.contains("eyJhbGci")) cookies else ""
        val requestBuilder = Request.Builder()
            .url("https://fantasy.premierleague.com/api/my-team/$teamId/")
            .header("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
            .header("Referer", "https://fantasy.premierleague.com/")

        if (bearer.isNotBlank()) {
            requestBuilder.header("x-api-authorization", "Bearer $bearer")
            requestBuilder.header("Authorization", "Bearer $bearer")
        } else {
            requestBuilder.header("Cookie", cookies)
            val csrfToken = cookies.split(";").map { it.trim() }.find { it.startsWith("csrftoken=") }?.substringAfter("=")
            if (csrfToken != null) {
                requestBuilder.header("X-CSRFToken", csrfToken)
            }
        }

        val response = client.newCall(requestBuilder.build()).execute()
        val body = response.body?.string() ?: throw RuntimeException("Empty response from my-team")
        if (!response.isSuccessful) {
            throw RuntimeException("Failed to fetch my-team: HTTP ${response.code} - $body")
        }
        return json.decodeFromString(body)
    }

    fun postTransfers(req: TransferRequest, cookies: String): String {
        val bodyStr = json.encodeToString(req)
        val requestBody = bodyStr.toRequestBody("application/json".toMediaType())

        val csrfToken = cookies.split(";").map { it.trim() }.find { it.startsWith("csrftoken=") }?.substringAfter("=")

        val requestBuilder = Request.Builder()
            .url("https://fantasy.premierleague.com/api/transfers/")
            .header("Cookie", cookies)
            .header("Content-Type", "application/json")
            .header("User-Agent", "Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36")
            .post(requestBody)

        if (csrfToken != null) {
            requestBuilder.header("X-CSRFToken", csrfToken)
        }

        val response = client.newCall(requestBuilder.build()).execute()
        val body = response.body?.string() ?: ""
        if (!response.isSuccessful) {
            throw RuntimeException("Transfer request failed: HTTP ${response.code} - $body")
        }
        return body
    }

    fun postLineup(teamId: Int, lineup: LineupUpdate, cookies: String): String {
        val bodyStr = json.encodeToString(lineup)
        val requestBody = bodyStr.toRequestBody("application/json".toMediaType())
        val csrfToken = cookies.split(";").map { it.trim() }.find { it.startsWith("csrftoken=") }?.substringAfter("=")

        val requestBuilder = Request.Builder()
            .url("https://fantasy.premierleague.com/api/my-team/$teamId/")
            .header("Cookie", cookies)
            .header("Content-Type", "application/json")
            .header("User-Agent", "Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36")
            .post(requestBody)

        if (csrfToken != null) {
            requestBuilder.header("X-CSRFToken", csrfToken)
        }

        val response = client.newCall(requestBuilder.build()).execute()
        val body = response.body?.string() ?: ""
        if (!response.isSuccessful) {
            throw RuntimeException("Lineup update failed: HTTP ${response.code} - $body")
        }
        return body
    }
}
