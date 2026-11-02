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

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Log into FPL", color = FplText, fontWeight = FontWeight.Bold) },
        text = {
            Column(modifier = Modifier.fillMaxWidth().height(400.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(statusText, color = FplSubtext, fontSize = 12.sp)
                AndroidView(
                    factory = { context ->
                        WebView(context).apply {
                            settings.javaScriptEnabled = true
                            settings.domStorageEnabled = true
                            webViewClient = object : WebViewClient() {
                                override fun onPageFinished(view: WebView?, url: String?) {
                                    super.onPageFinished(view, url)
                                    val cookieManager = CookieManager.getInstance()
                                    val cookies = cookieManager.getCookie("https://fantasy.premierleague.com") ?: ""
                                    if (cookies.contains("pl_profile") || cookies.contains("sessionid")) {
                                        viewModel.updateCookies(cookies)
                                        statusText = "✅ Login cookies captured successfully! Checking Team ID..."
                                        viewModel.autoDetectTeamId { success ->
                                            if (success) {
                                                statusText = "🎉 Team ID auto-detected! You can close this window."
                                            }
                                        }
                                    }
                                }
                            }
                            loadUrl("https://fantasy.premierleague.com/a/login")
                        }
                    },
                    modifier = Modifier.fillMaxWidth().weight(1f)
                )
            }
        },
        confirmButton = {
            Button(
                onClick = onDismiss,
                colors = ButtonDefaults.buttonColors(containerColor = FplGreen)
            ) {
                Text("Done", color = FplPurple, fontWeight = FontWeight.Bold)
            }
        },
        containerColor = FplCardBg
    )
}
