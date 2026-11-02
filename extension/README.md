# FPL Picker AI — Chrome Extension (Manifest V3)

A Chrome extension for Fantasy Premier League (FPL) that ports the core `fpl-picker` optimization algorithms, opponent-conditioned scoring engine, and transfer recommender directly into your browser.

---

## Key Features

- **Opponent-Conditioned Scoring Engine**: Calculates per-game / per-90 metrics (FDR, xG, xGA, Form, EP, ICT) adjusted for fixture difficulty, home advantage, and double gameweeks.
- **XI-First Squad Optimizer**: Evaluates all 7 valid FPL formations using team-grouped Dynamic Programming (DP) to maximize Starting XI score while reserving budget for bench fillers.
- **Captains Picker**: Auto-selects optimal Captain (©) and Vice-Captain (VC) based on player scores and playing eligibility.
- **AI Transfer Suggestions**: Compares your current FPL squad against the optimal pick and suggests greedy OUT → IN transfer swaps respecting budget, team limits, and free transfers/hits caps.
- **In-Browser & On-Page Integration**:
  - **Extension Popup**: Quick controls to adjust budget, switch formula presets (`Balanced`, `Attacker`, `Defender`), and run optimizer.
  - **FPL Page Side Drawer Widget**: Floating toggle button on `fantasy.premierleague.com` that opens a side drawer for seamless analysis while managing your squad.

---

## Directory Structure

```
extension/
├── manifest.json              # Chrome Manifest V3 declaration
├── package.json               # Dependencies and build scripts
├── vite.config.ts             # Vite build configuration for Chrome Extension
├── src/
│   ├── api/
│   │   ├── types.ts           # TypeScript FPL API types
│   │   └── client.ts          # FPL API client with chrome.storage caching
│   ├── model/
│   │   ├── scorer.ts          # Port of Go model/scorer.go
│   │   ├── recommender.ts     # Port of Go model/recommender.go (DP solver)
│   │   └── transfers.ts       # Port of Go apply/transfers.go
│   ├── background/
│   │   └── service-worker.ts  # Extension service worker
│   ├── content/
│   │   ├── content.ts         # Injected FPL side drawer widget
│   │   └── content.css        # Content widget styles
│   └── popup/
│       ├── popup.html         # Extension popup markup
│       ├── popup.ts           # Extension popup logic
│       └── popup.css          # Extension popup styling
└── dist/                      # Built extension ready for Chrome
```

---

## How to Build (Chrome & Firefox)

From the `extension/` directory:

```bash
# 1. Install dependencies
npm install

# 2. Build for Chrome (output: dist/)
npm run build

# 3. Build for Firefox / Firefox Android (output: dist-firefox/)
npm run build:firefox

# 4. Package into unsigned .zip for Firefox (output: web-ext-artifacts/)
npm run package:firefox

# 5. Sign extension with Mozilla AMO API (generates installable .xpi for all Firefox/Android versions)
npx web-ext sign --source-dir dist-firefox --artifacts-dir ./web-ext-artifacts --api-key="YOUR_JWT_ISSUER" --api-secret="YOUR_JWT_SECRET"
```

---

## How to Install

### In Google Chrome
1. Open Google Chrome and navigate to `chrome://extensions/`.
2. Enable **Developer mode** using the toggle switch in the top-right corner.
3. Click the **Load unpacked** button and select `extension/dist`.

### In Firefox Desktop
1. Navigate to `about:debugging#/runtime/this-firefox`.
2. Click **Load Temporary Add-on...** and select `extension/dist-firefox/manifest.json`.

### In Firefox Android (Standard, Beta, or Nightly)
1. Build and sign your extension using `npx web-ext sign ...` to get the signed `.xpi` file in `web-ext-artifacts/`.
2. Transfer the signed `.xpi` file to your Android phone.
3. Open the `.xpi` file with **Firefox for Android** to install it directly with zero corrupt or unsigned warnings!

---

## How to Use

1. **Popup Window**:
   - Click the extension icon in the Chrome toolbar.
   - Adjust your total budget (e.g. `102.1`).
   - Select your scoring formula (`Balanced`, `Attacker`, or `Defender`).
   - Optionally enter your **FPL Team ID** (or log in on `fantasy.premierleague.com` for auto-detection).
   - Click **Optimize Squad**.

2. **On fantasy.premierleague.com**:
   - Visit [fantasy.premierleague.com](https://fantasy.premierleague.com).
   - A green **⚽ FPL Picker AI** floating button will appear in the bottom-right corner.
   - Click the button to toggle the side drawer and view live squad recommendations while editing your team.
