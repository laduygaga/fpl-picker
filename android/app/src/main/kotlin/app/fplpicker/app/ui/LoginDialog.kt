package app.fplpicker.app.ui

import android.webkit.CookieManager
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import app.fplpicker.app.viewmodel.MainViewModel

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LoginDialog(viewModel: MainViewModel, onDismiss: () -> Unit) {
    var statusText by remember { mutableStateOf("Please log into your FPL account...") }
    val cookies by viewModel.cookies.collectAsState()
    val teamId by viewModel.teamId.collectAsState()

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Log into FPL", color = FplText, fontWeight = FontWeight.Bold) },
        text = {
            Column(modifier = Modifier.fillMaxWidth().height(420.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(statusText, color = FplSubtext, fontSize = 12.sp)
                if (cookies.isNotBlank()) {
                    Surface(color = FplDarkPurple, shape = MaterialTheme.shapes.small) {
                        Column(modifier = Modifier.padding(8.dp)) {
                            Text("🔑 Credentials Active (${cookies.length} chars)", color = FplGreen, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                            Text("🏟️ Team ID: $teamId", color = FplCyan, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                        }
                    }
                }

                AndroidView(
                    factory = { context ->
                        WebView(context).apply {
                            settings.javaScriptEnabled = true
                            settings.domStorageEnabled = true
                            settings.loadsImagesAutomatically = true
                            webViewClient = object : WebViewClient() {
                                override fun onPageFinished(view: WebView?, url: String?) {
                                    super.onPageFinished(view, url)
                                    view?.evaluateJavascript(
                                        "(function() { " +
                                        "  var token = ''; " +
                                        "  for (var i = 0; i < localStorage.length; i++) { " +
                                        "    var key = localStorage.key(i); " +
                                        "    if (key && key.includes('oidc.user')) { " +
                                        "      try { " +
                                        "        var parsed = JSON.parse(localStorage.getItem(key)); " +
                                        "        if (parsed && parsed.access_token) { token = parsed.access_token; break; } " +
                                        "      } catch(e) {} " +
                                        "    } " +
                                        "  } " +
                                        "  return token; " +
                                        "})();"
                                    ) { tokenVal ->
                                        if (tokenVal != null && tokenVal.length > 10) {
                                            val cleanToken = tokenVal.trim('"')
                                            if (cleanToken.startsWith("eyJ")) {
                                                viewModel.updateCookies(cleanToken)
                                                statusText = "🎉 JWT Bearer token auto-captured!"
                                                viewModel.autoDetectTeamId { success ->
                                                    if (success) statusText = "🎉 Team ID auto-detected ($teamId)!"
                                                }
                                            }
                                        }
                                    }

                                    val cookieManager = CookieManager.getInstance()
                                    val c1 = cookieManager.getCookie("https://fantasy.premierleague.com") ?: ""
                                    val c2 = cookieManager.getCookie("https://users.premierleague.com") ?: ""
                                    val c3 = cookieManager.getCookie("https://account.premierleague.com") ?: ""
                                    val c4 = cookieManager.getCookie(url) ?: ""
                                    val combined = listOf(c1, c2, c3, c4).filter { it.isNotBlank() }.joinToString("; ")
                                    
                                    if (combined.contains("pl_profile") || combined.contains("sessionid") || combined.contains("csrftoken") || combined.contains("idserv")) {
                                        if (!cookies.startsWith("eyJ")) {
                                            viewModel.updateCookies(combined)
                                            viewModel.autoDetectTeamId()
                                        }
                                    }
                                }
                            }
                            loadUrl("https://fantasy.premierleague.com/")
                        }
                    },
                    modifier = Modifier.fillMaxWidth().weight(1f)
                )
            }
        },
        confirmButton = {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                Button(
                    onClick = {
                        val cookieManager = CookieManager.getInstance()
                        val c1 = cookieManager.getCookie("https://fantasy.premierleague.com") ?: ""
                        val c2 = cookieManager.getCookie("https://users.premierleague.com") ?: ""
                        val c3 = cookieManager.getCookie("https://account.premierleague.com") ?: ""
                        val combined = listOf(c1, c2, c3).filter { it.isNotBlank() }.joinToString("; ")
                        
                        if (combined.isNotBlank()) {
                            viewModel.updateCookies(combined)
                            statusText = "🔄 Refreshed session!"
                            viewModel.autoDetectTeamId { success ->
                                if (success) {
                                    statusText = "🎉 Team ID auto-detected ($teamId)!"
                                } else {
                                    statusText = "⚠️ Session found, but Team ID fetch failed. (Check login/cookies)"
                                }
                            }
                        } else {
                            statusText = "⚠️ No session found. Please log in on the page first."
                        }
                    },
                    colors = ButtonDefaults.buttonColors(containerColor = FplCyan)
                ) {
                    Text("🔄 Refresh Session", color = FplPurple, fontWeight = FontWeight.Bold)
                }
                Button(
                    onClick = onDismiss,
                    colors = ButtonDefaults.buttonColors(containerColor = FplGreen)
                ) {
                    Text("Done", color = FplPurple, fontWeight = FontWeight.Bold)
                }
            }
        },
        containerColor = FplCardBg
    )
}
