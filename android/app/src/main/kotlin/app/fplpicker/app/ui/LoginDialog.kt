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
    var manualTokenInput by remember { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Log into FPL", color = FplText, fontWeight = FontWeight.Bold) },
        text = {
            Column(modifier = Modifier.fillMaxWidth().height(440.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(statusText, color = FplSubtext, fontSize = 12.sp)
                if (cookies.isNotBlank()) {
                    Surface(color = FplDarkPurple, shape = MaterialTheme.shapes.small) {
                        Column(modifier = Modifier.padding(8.dp)) {
                            Text("🔑 Credentials / Cookies (${cookies.length} chars)", color = FplGreen, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                            Text("🏟️ Team ID: $teamId", color = FplCyan, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                        }
                    }
                }

                OutlinedTextField(
                    value = manualTokenInput,
                    onValueChange = { manualTokenInput = it },
                    label = { Text("Or paste JWT / Bearer token here", fontSize = 10.sp) },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    colors = OutlinedTextFieldDefaults.colors(
                        focusedBorderColor = FplGreen,
                        unfocusedBorderColor = FplBorder,
                        focusedTextColor = FplText,
                        unfocusedTextColor = FplText,
                        focusedLabelColor = FplGreen,
                        unfocusedLabelColor = FplSubtext,
                        focusedContainerColor = FplDarkPurple,
                        unfocusedContainerColor = FplDarkPurple
                    )
                )

                Button(
                    onClick = {
                        if (manualTokenInput.isNotBlank()) {
                            viewModel.updateCookies(manualTokenInput.trim())
                            statusText = "✅ Bearer token applied!"
                            viewModel.autoDetectTeamId { success ->
                                if (success) statusText = "🎉 Team ID auto-detected ($teamId)!"
                                else statusText = "⚠️ Token saved, but Team ID detection failed. Check ID."
                            }
                        }
                    },
                    colors = ButtonDefaults.buttonColors(containerColor = FplGreen),
                    modifier = Modifier.fillMaxWidth().height(36.dp)
                ) {
                    Text("Apply Token", color = FplPurple, fontSize = 11.sp, fontWeight = FontWeight.Bold)
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
                                    val c2 = cookieManager.getCookie("https://users.premierleague.com") ?: ""
                                    val c3 = cookieManager.getCookie("https://account.premierleague.com") ?: ""
                                    val c4 = cookieManager.getCookie(url) ?: ""
                                    val combined = listOf(c1, c2, c3, c4).filter { it.isNotBlank() }.joinToString("; ")
                                    
                                    if (combined.contains("pl_profile") || combined.contains("sessionid") || combined.contains("csrftoken") || combined.contains("idserv")) {
                                        viewModel.updateCookies(combined)
                                        viewModel.autoDetectTeamId()
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
                            statusText = "🔄 Refreshed cookies (${combined.take(30)}...)!"
                            viewModel.autoDetectTeamId { success ->
                                if (success) {
                                    statusText = "🎉 Team ID auto-detected ($teamId)!"
                                } else {
                                    statusText = "⚠️ Got cookies but failed to get Team ID. Check login."
                                }
                            }
                        } else {
                            statusText = "⚠️ No cookies found. Please log in on the page first."
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
