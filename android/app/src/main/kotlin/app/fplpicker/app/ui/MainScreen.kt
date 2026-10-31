package app.fplpicker.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.BorderStroke
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

    var selectedTab by remember { mutableStateOf(0) } // 0 = XI & Bench, 1 = Settings/Info

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
                            modifier = Modifier.weight(1.1f)
                        )

                        var expanded by remember { mutableStateOf(false) }
                        val formulaNames = mapOf("1" to "1 - Balanced", "2" to "2 - Attacker", "3" to "3 - Defender")

                        ExposedDropdownMenuBox(
                            expanded = expanded,
                            onExpandedChange = { expanded = !expanded },
                            modifier = Modifier.weight(1.9f)
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

            // Content Area
            Box(modifier = Modifier.fillMaxSize().weight(1f).padding(horizontal = 12.dp)) {
                when (val state = uiState) {
                    is UiState.Initial -> {
                        Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                            Text("Tap 'Optimize Squad' to generate optimal XI & bench.", color = FplSubtext, fontSize = 13.sp)
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
                            Text("Error: ${state.message}", color = Color(0xFFFF5555), fontSize = 13.sp)
                        }
                    }
                    is UiState.Success -> {
                        SquadResultContent(state.result)
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
