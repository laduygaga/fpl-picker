package app.fplpicker.app.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.fplpicker.app.viewmodel.MainViewModel
import app.fplpicker.app.viewmodel.UiState
import app.fplpicker.core.model.ScoredPlayer
import app.fplpicker.core.model.SquadResult
import app.fplpicker.core.model.TransferSuggestion

val FplPurple = Color(0xFF38003C)
val FplDarkPurple = Color(0xFF240029)
val FplCardBg = Color(0xFF2D0032)
val FplGreen = Color(0xFF00FF87)
val FplPink = Color(0xFFE90052)
val FplCyan = Color(0xFF04F5FF)
val FplText = Color(0xFFFFFFFF)
val FplSubtext = Color(0xFFA393A5)
val FplBorder = Color(0x1AFFFFFF)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MainScreen(viewModel: MainViewModel) {
    val uiState by viewModel.uiState.collectAsState()
    val budget by viewModel.budget.collectAsState()
    val formula by viewModel.formula.collectAsState()
    val teamId by viewModel.teamId.collectAsState()
    val cookies by viewModel.cookies.collectAsState()

    var selectedTab by remember { mutableStateOf(0) } // 0 = XI & Bench, 1 = Transfers

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text("⚽ FPL Picker AI", color = FplGreen, fontWeight = FontWeight.ExtraBold, fontSize = 18.sp)
                    }
                },
                actions = {
                    when (val state = uiState) {
                        is UiState.Success -> {
                            Surface(
                                color = FplPink,
                                shape = MaterialTheme.shapes.small,
                                modifier = Modifier.padding(end = 12.dp)
                            ) {
                                Text(
                                    "GW${state.nextGw}",
                                    color = Color.White,
                                    fontWeight = FontWeight.Bold,
                                    fontSize = 12.sp,
                                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp)
                                )
                            }
                        }
                        else -> {}
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = FplDarkPurple)
            )
        },
        containerColor = FplPurple
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
                .background(FplPurple)
        ) {
            // Controls Card
            Card(
                colors = CardDefaults.cardColors(containerColor = FplCardBg),
                shape = MaterialTheme.shapes.medium,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(12.dp)
            ) {
                Column(
                    modifier = Modifier.padding(12.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedTextField(
                            value = budget,
                            onValueChange = { viewModel.updateBudget(it) },
                            label = { Text("Budget (£M)", color = FplSubtext, fontSize = 11.sp) },
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                            singleLine = true,
                            colors = OutlinedTextFieldDefaults.colors(
                                focusedBorderColor = FplGreen,
                                unfocusedBorderColor = FplBorder,
                                focusedTextColor = FplText,
                                unfocusedTextColor = FplText,
                                focusedLabelColor = FplGreen,
                                unfocusedLabelColor = FplSubtext,
                                cursorColor = FplGreen,
                                focusedContainerColor = FplDarkPurple,
                                unfocusedContainerColor = FplDarkPurple
                            ),
                            modifier = Modifier.weight(1f)
                        )

                        var expanded by remember { mutableStateOf(false) }
                        val formulaNames = mapOf("1" to "1 - Balanced", "2" to "2 - Attacker", "3" to "3 - Defender")

                        ExposedDropdownMenuBox(
                            expanded = expanded,
                            onExpandedChange = { expanded = !expanded },
                            modifier = Modifier.weight(1.3f)
                        ) {
                            OutlinedTextField(
                                value = formulaNames[formula] ?: "1 - Balanced",
                                onValueChange = {},
                                readOnly = true,
                                label = { Text("Formula", color = FplSubtext, fontSize = 11.sp) },
                                trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = expanded) },
                                colors = OutlinedTextFieldDefaults.colors(
                                    focusedBorderColor = FplGreen,
                                    unfocusedBorderColor = FplBorder,
                                    focusedTextColor = FplText,
                                    unfocusedTextColor = FplText,
                                    focusedLabelColor = FplGreen,
                                    unfocusedLabelColor = FplSubtext,
                                    focusedContainerColor = FplDarkPurple,
                                    unfocusedContainerColor = FplDarkPurple
                                ),
                                modifier = Modifier.menuAnchor().fillMaxWidth()
                            )

                            ExposedDropdownMenu(
                                expanded = expanded,
                                onDismissRequest = { expanded = false },
                                modifier = Modifier.background(FplCardBg)
                            ) {
                                formulaNames.forEach { (key, label) ->
                                    DropdownMenuItem(
                                        text = { Text(label, color = FplText) },
                                        onClick = {
                                            viewModel.updateFormula(key)
                                            expanded = false
                                        }
                                    )
                                }
                            }
                        }
                    }

                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedTextField(
                            value = teamId,
                            onValueChange = { viewModel.updateTeamId(it) },
                            label = { Text("Team ID", color = FplSubtext, fontSize = 11.sp) },
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                            singleLine = true,
                            colors = OutlinedTextFieldDefaults.colors(
                                focusedBorderColor = FplGreen,
                                unfocusedBorderColor = FplBorder,
                                focusedTextColor = FplText,
                                unfocusedTextColor = FplText,
                                focusedLabelColor = FplGreen,
                                unfocusedLabelColor = FplSubtext,
                                cursorColor = FplGreen,
                                focusedContainerColor = FplDarkPurple,
                                unfocusedContainerColor = FplDarkPurple
                            ),
                            modifier = Modifier.weight(1f)
                        )

                        var showLoginDialog by remember { mutableStateOf(false) }

                        Button(
                            onClick = { showLoginDialog = true },
                            colors = ButtonDefaults.buttonColors(containerColor = FplCyan),
                            shape = MaterialTheme.shapes.small,
                            modifier = Modifier.weight(1.3f).height(50.dp)
                        ) {
                            Text("🔑 Log in with FPL", color = FplPurple, fontWeight = FontWeight.Bold, fontSize = 12.sp)
                        }

                        if (showLoginDialog) {
                            LoginDialog(viewModel, onDismiss = { showLoginDialog = false })
                        }
                    }

                    Button(
                        onClick = { viewModel.optimizeSquad(false) },
                        colors = ButtonDefaults.buttonColors(containerColor = FplGreen),
                        shape = MaterialTheme.shapes.small,
                        modifier = Modifier.fillMaxWidth().height(42.dp)
                    ) {
                        Text("Optimize Squad", color = FplPurple, fontWeight = FontWeight.ExtraBold, fontSize = 14.sp)
                    }
                }
            }

            // Tabs
            TabRow(
                selectedTabIndex = selectedTab,
                containerColor = FplDarkPurple,
                contentColor = FplGreen
            ) {
                Tab(
                    selected = selectedTab == 0,
                    onClick = { selectedTab = 0 },
                    text = { Text("Starting XI & Bench", color = if (selectedTab == 0) FplGreen else FplSubtext, fontSize = 12.sp, fontWeight = FontWeight.Bold) }
                )
                Tab(
                    selected = selectedTab == 1,
                    onClick = { selectedTab = 1 },
                    text = { Text("Transfers", color = if (selectedTab == 1) FplGreen else FplSubtext, fontSize = 12.sp, fontWeight = FontWeight.Bold) }
                )
            }

            // Content Area
            Box(modifier = Modifier.fillMaxSize().weight(1f).padding(12.dp)) {
                when (val state = uiState) {
                    is UiState.Initial -> {
                        Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                            Text("Tap 'Optimize Squad' to run optimizer...", color = FplSubtext, fontSize = 13.sp)
                        }
                    }
                    is UiState.Loading -> {
                        Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp)) {
                                CircularProgressIndicator(color = FplGreen)
                                Text("Scoring players & running DP optimizer...", color = FplGreen, fontWeight = FontWeight.SemiBold, fontSize = 13.sp)
                            }
                        }
                    }
                    is UiState.Error -> {
                        Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                            Text("Error: ${state.message}", color = Color(0xFFFF5555), fontSize = 13.sp, textAlign = androidx.compose.ui.text.style.TextAlign.Center)
                        }
                    }
                    is UiState.Success -> {
                        if (state.message != null) {
                            Surface(
                                color = FplCardBg,
                                shape = MaterialTheme.shapes.small,
                                border = BorderStroke(1.dp, FplGreen),
                                modifier = Modifier.fillMaxWidth().padding(bottom = 8.dp)
                            ) {
                                Text(state.message, color = FplGreen, fontWeight = FontWeight.Bold, fontSize = 13.sp, modifier = Modifier.padding(10.dp))
                            }
                        }

                        if (selectedTab == 0) {
                            SquadResultContent(state.result)
                        } else {
                            TransfersContent(state.transfers, onApply = { viewModel.applyChanges() })
                        }
                    }
                }
            }
        }
    }
}

@Composable
fun SquadResultContent(res: SquadResult) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.spacedBy(10.dp),
        contentPadding = PaddingValues(bottom = 20.dp)
    ) {
        // Summary Banner
        item {
            Card(
                colors = CardDefaults.cardColors(containerColor = FplCardBg),
                shape = MaterialTheme.shapes.medium,
                border = BorderStroke(1.dp, FplBorder),
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth().padding(14.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column {
                        Text("Starting XI Score", color = FplSubtext, fontSize = 11.sp)
                        Text(String.format("%.3f", res.totalScore), color = FplGreen, fontSize = 22.sp, fontWeight = FontWeight.ExtraBold)
                    }
                    Column(horizontalAlignment = Alignment.End) {
                        Text("${res.formation} Formation", color = FplCyan, fontSize = 13.sp, fontWeight = FontWeight.Bold)
                        Text("XI: £${String.format("%.1f", res.xiCost)}M | Squad: £${String.format("%.1f", res.totalCost)}M", color = FplSubtext, fontSize = 11.sp)
                    }
                }
            }
        }

        // Captain Cards
        item {
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                CaptainCard("Captain (©)", res.captain, FplGreen, Modifier.weight(1f))
                CaptainCard("Vice Captain (V)", res.viceCaptain, FplPink, Modifier.weight(1f))
            }
        }

        // Starting XI Section Header
        item {
            Text("STARTING XI", color = FplGreen, fontSize = 12.sp, fontWeight = FontWeight.ExtraBold, modifier = Modifier.padding(top = 4.dp))
        }

        items(res.starters) { sp ->
            val isC = res.captain?.player?.id == sp.player.id
            val isVc = res.viceCaptain?.player?.id == sp.player.id
            PlayerRow(sp, isC, isVc)
        }

        // Bench Section Header
        item {
            Text("BENCH", color = FplSubtext, fontSize = 12.sp, fontWeight = FontWeight.ExtraBold, modifier = Modifier.padding(top = 8.dp))
        }

        items(res.bench) { sp ->
            PlayerRow(sp, false, false)
        }
    }
}

@Composable
fun TransfersContent(transfers: List<TransferSuggestion>, onApply: () -> Unit) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.spacedBy(10.dp),
        contentPadding = PaddingValues(bottom = 20.dp)
    ) {
        if (transfers.isEmpty()) {
            item {
                Box(modifier = Modifier.fillMaxWidth().padding(40.dp), contentAlignment = Alignment.Center) {
                    Text("Enter Team ID and Cookies to generate transfer recommendations, or your squad is already optimal!", color = FplGreen, fontSize = 13.sp, textAlign = androidx.compose.ui.text.style.TextAlign.Center)
                }
            }
        } else {
            item {
                Button(
                    onClick = onApply,
                    colors = ButtonDefaults.buttonColors(containerColor = FplPink),
                    shape = MaterialTheme.shapes.small,
                    modifier = Modifier.fillMaxWidth().height(42.dp)
                ) {
                    Text("⚡ Apply ${transfers.size} Transfer(s) & Lineup to FPL", color = Color.White, fontWeight = FontWeight.ExtraBold, fontSize = 13.sp)
                }
            }
            item {
                Text("PROPOSED TRANSFERS (${transfers.size})", color = FplGreen, fontSize = 12.sp, fontWeight = FontWeight.ExtraBold, modifier = Modifier.padding(top = 4.dp))
            }
            items(transfers) { t ->
                TransferItemRow(t)
            }
        }
    }
}

@Composable
fun TransferItemRow(t: TransferSuggestion) {
    Card(
        colors = CardDefaults.cardColors(containerColor = FplCardBg),
        shape = MaterialTheme.shapes.small,
        border = BorderStroke(1.dp, FplBorder),
        modifier = Modifier.fillMaxWidth()
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(12.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(2.dp), modifier = Modifier.weight(1f)) {
                Text("OUT: ${t.out.player.web_name} (£${String.format("%.1f", t.sellingPrice / 10.0)}M)", color = Color(0xFFFF5555), fontSize = 12.sp, fontWeight = FontWeight.Bold)
                Text("IN: ${t.inPlayer.player.web_name} (${t.inPlayer.teamName}, ${t.inPlayer.positionName}, £${String.format("%.1f", t.purchasePrice / 10.0)}M)", color = FplGreen, fontSize = 12.sp, fontWeight = FontWeight.Bold)
            }
            Column(horizontalAlignment = Alignment.End) {
                Text("+${String.format("%.3f", t.scoreUplift)}", color = FplGreen, fontSize = 14.sp, fontWeight = FontWeight.ExtraBold)
                Text("Score Uplift", color = FplSubtext, fontSize = 9.sp)
            }
        }
    }
}

@Composable
fun CaptainCard(role: String, sp: ScoredPlayer?, accentColor: Color, modifier: Modifier = Modifier) {
    Card(
        colors = CardDefaults.cardColors(containerColor = FplCardBg),
        shape = MaterialTheme.shapes.medium,
        border = BorderStroke(1.dp, FplBorder),
        modifier = modifier
    ) {
        Column(modifier = Modifier.padding(10.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(role, color = accentColor, fontSize = 10.sp, fontWeight = FontWeight.ExtraBold)
            val nameText = if (sp != null) "${sp.player.web_name} (${sp.teamName})" else "--"
            Text(nameText, color = FplText, fontSize = 13.sp, fontWeight = FontWeight.Bold, maxLines = 1)
        }
    }
}

@Composable
fun PlayerRow(sp: ScoredPlayer, isC: Boolean, isVc: Boolean) {
    Card(
        colors = CardDefaults.cardColors(containerColor = FplCardBg),
        shape = MaterialTheme.shapes.small,
        border = BorderStroke(1.dp, FplBorder),
        modifier = Modifier.fillMaxWidth()
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(10.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.weight(1f)) {
                Surface(
                    color = FplDarkPurple,
                    shape = MaterialTheme.shapes.small
                ) {
                    Text(sp.positionName, color = FplSubtext, fontSize = 10.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp))
                }
                Column {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text(sp.player.web_name, color = FplText, fontSize = 13.sp, fontWeight = FontWeight.Bold)
                        if (isC) {
                            Surface(color = FplGreen, shape = MaterialTheme.shapes.extraSmall) {
                                Text("C", color = FplPurple, fontSize = 9.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(horizontal = 3.dp))
                            }
                        } else if (isVc) {
                            Surface(color = FplPink, shape = MaterialTheme.shapes.extraSmall) {
                                Text("VC", color = Color.White, fontSize = 9.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(horizontal = 3.dp))
                            }
                        }
                    }
                    Text("${sp.teamName} • £${String.format("%.1f", sp.player.now_cost / 10.0)}M", color = FplSubtext, fontSize = 11.sp)
                }
            }

            Column(horizontalAlignment = Alignment.End) {
                Text(String.format("%.3f", sp.score), color = FplGreen, fontSize = 13.sp, fontWeight = FontWeight.Bold)
                Text(sp.oppDesc, color = FplSubtext, fontSize = 9.sp, maxLines = 1)
            }
        }
    }
}
