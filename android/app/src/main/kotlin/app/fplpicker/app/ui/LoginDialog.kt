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
            Column(modifier = Modifier.fillMaxWidth().height(400.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(statusText, color = FplSubtext, fontSize = 12.sp)
                if (cookies.isNotBlank()) {
                    Surface(color = FplDarkPurple, shape = MaterialTheme.shapes.small) {
                        Column(modifier = Modifier.padding(8.dp)) {
                            Text("🔑 Cookies Captured: ${cookies.take(40)}...", color = FplGreen, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                            Text("🏟️ Team ID: $teamId", color = FplCyan, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                        }
                    }
                }
                AndroidView(
                    factory = { context ->
                        WebView(context).apply {
                            settings.javaScriptEnabled = true
                            settings.domStorageEnabled = true
                            webViewClient = object : WebViewClient() {
                                override fun onPageFinished(view: WebView?, url: String?) {
                                    super.onPageFinished(view, url)
                                    val cookieManager = CookieManager.getInstance()
                                    val c1 = cookieManager.getCookie("https://fantasy.premierleague.com") ?: ""
                                    val c2 = cookieManager.getCookie(url) ?: ""
                                    val combined = if (c1.isNotBlank()) "$c1; $c2" else c2
                                    if (combined.contains("pl_profile") || combined.contains("sessionid") || combined.contains("csrftoken")) {
                                        viewModel.updateCookies(combined)
                                        statusText = "✅ Login cookies captured successfully!"
                                        viewModel.autoDetectTeamId { success ->
                                            if (success) {
                                                statusText = "🎉 Team ID auto-detected ($teamId)! You can close this."
                                            }
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
                        if (c1.isNotBlank()) {
                            viewModel.updateCookies(c1)
                            statusText = "🔄 Refreshed cookies manually!"
                            viewModel.autoDetectTeamId()
                        } else {
                            statusText = "⚠️ No cookies found yet. Please log in first."
                        }
                    },
                    colors = ButtonDefaults.buttonColors(containerColor = FplCyan)
                ) {
                    Text("🔄 Refresh Cookies", color = FplPurple, fontWeight = FontWeight.Bold)
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
