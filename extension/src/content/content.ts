import type { SquadResult } from '../lib/model/recommender';
import type { TransferSuggestion } from '../lib/model/transfers';

let drawerOpen = false;

let lastDrawerData: {
  optimal: SquadResult;
  transfers: TransferSuggestion[];
  nextGw: number;
} | null = null;

function initContentWidget(): void {
  if (document.getElementById('fpl-picker-widget-toggle')) return;

  // Create Toggle Button
  const toggleBtn = document.createElement('button');
  toggleBtn.id = 'fpl-picker-widget-toggle';
  toggleBtn.innerHTML = `⚽ FPL Picker AI`;
  toggleBtn.addEventListener('click', toggleDrawer);
  document.body.appendChild(toggleBtn);

  // Create Side Drawer
  const drawer = document.createElement('div');
  drawer.id = 'fpl-picker-drawer';
  drawer.innerHTML = `
    <div class="fpl-drawer-header">
      <h3>⚽ FPL Picker AI</h3>
      <button class="fpl-drawer-close" id="fpl-drawer-close-btn">&times;</button>
    </div>
    <div class="fpl-drawer-body">
      <div class="fpl-card">
        <div class="fpl-card-title">Strategy Controls</div>
        <div style="display: flex; gap: 10px; margin-bottom: 10px;">
          <div style="flex: 1;">
            <label style="font-size: 11px; color: #a393a5; display: block; margin-bottom: 4px;">Formula</label>
            <select id="fpl-drawer-formula" style="width: 100%; padding: 8px; background: #240029; color: #fff; border: 1px solid rgba(255,255,255,0.15); border-radius: 6px;">
              <option value="1">1 - Balanced</option>
              <option value="2">2 - Attacker</option>
              <option value="3">3 - Defender</option>
            </select>
          </div>
          <div style="flex: 1;">
            <label style="font-size: 11px; color: #a393a5; display: block; margin-bottom: 4px;">Budget (£M)</label>
            <input type="number" id="fpl-drawer-budget" value="100.0" step="0.1" style="width: 100%; padding: 8px; background: #240029; color: #fff; border: 1px solid rgba(255,255,255,0.15); border-radius: 6px;">
          </div>
        </div>
        <div style="margin-bottom: 12px;">
          <label style="font-size: 11px; color: #a393a5; display: block; margin-bottom: 4px;">FPL Team ID</label>
          <input type="number" id="fpl-drawer-team-id" placeholder="Auto-detected or enter ID" style="width: 100%; padding: 8px; background: #240029; color: #fff; border: 1px solid rgba(255,255,255,0.15); border-radius: 6px;">
        </div>
        <button class="fpl-btn-primary" id="fpl-drawer-run-btn">Run Squad Optimizer</button>
      </div>

      <div id="fpl-drawer-results">
        <div style="text-align: center; color: #a393a5; padding: 20px 0;">
          Click "Run Squad Optimizer" to analyze current gameweek picks.
        </div>
      </div>
    </div>
  `;

  document.body.appendChild(drawer);

  document.getElementById('fpl-drawer-close-btn')?.addEventListener('click', toggleDrawer);
  document.getElementById('fpl-drawer-run-btn')?.addEventListener('click', runOptimizer);

  // Try auto-extracting team ID from URL if on fantasy.premierleague.com/entry/12345 or /pick-team
  detectAndAutoFill();
}

function toggleDrawer(): void {
  drawerOpen = !drawerOpen;
  const drawer = document.getElementById('fpl-picker-drawer');
  if (drawer) {
    if (drawerOpen) {
      drawer.classList.add('open');
      runOptimizer();
    } else {
      drawer.classList.remove('open');
    }
  }
}

async function detectAndAutoFill(): Promise<void> {
  const match = window.location.pathname.match(/\/entry\/(\d+)/);
  if (match) {
    const teamId = parseInt(match[1], 10);
    sessionStorage.setItem('fpl_picker_my_team_id', teamId.toString());
    if (typeof chrome !== 'undefined' && chrome.storage?.local) {
      chrome.storage.local.set({ fpl_entry_id: teamId });
    }
  }

  try {
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key && key.startsWith('oidc.user')) {
        const val = localStorage.getItem(key);
        if (val) {
          const parsed = JSON.parse(val);
          if (parsed.access_token) {
            chrome.storage.local.set({ fpl_bearer_token: parsed.access_token });
            if (parsed.profile?.entry) {
              sessionStorage.setItem('fpl_picker_my_team_id', parsed.profile.entry.toString());
              chrome.storage.local.set({ fpl_entry_id: Number(parsed.profile.entry) });
            }
            break;
          }
        }
      }
    }
  } catch (e) {
    console.warn('FPL Picker: failed to scan localStorage bearer token:', e);
  }
}

function runOptimizer(): void {
  const resultsDiv = document.getElementById('fpl-drawer-results');
  if (!resultsDiv) return;

  resultsDiv.innerHTML = `<div style="text-align: center; color: #00ff87; padding: 20px 0;">Calculating optimal XI with opponent conditioning...</div>`;

  const formula = (document.getElementById('fpl-drawer-formula') as HTMLSelectElement)?.value || '1';
  const budget = parseFloat((document.getElementById('fpl-drawer-budget') as HTMLInputElement)?.value || '100.0');
  const savedTeamId = sessionStorage.getItem('fpl_picker_my_team_id');
  const teamId = savedTeamId ? parseInt(savedTeamId, 10) : undefined;

  chrome.runtime.sendMessage(
    {
      type: 'OPTIMIZE_SQUAD',
      budget,
      formula,
      teamId,
    },
    (response) => {
      if (!response || !response.success) {
        resultsDiv.innerHTML = `<div style="color: #ff4d4d; padding: 10px;">Error: ${
          response?.error || 'Failed to connect to extension background'
        }</div>`;
        return;
      }

      renderDrawerResults(response.data);
    }
  );
}

interface OptimizeData {
  optimal: SquadResult;
  transfers: TransferSuggestion[];
  nextGw: number;
  detectedTeamId?: number | null;
}

function renderDrawerResults(data: OptimizeData): void {
  const resultsDiv = document.getElementById('fpl-drawer-results');
  if (!resultsDiv) return;

  const { optimal, transfers, nextGw, detectedTeamId } = data;
  lastDrawerData = { optimal, transfers, nextGw };

  if (detectedTeamId) {
    sessionStorage.setItem('fpl_picker_my_team_id', detectedTeamId.toString());
    const drawerTeamInput = document.getElementById('fpl-drawer-team-id') as HTMLInputElement;
    if (drawerTeamInput && !drawerTeamInput.value) {
      drawerTeamInput.value = detectedTeamId.toString();
    }
  }

  const savedTeamId = sessionStorage.getItem('fpl_picker_my_team_id');
  const teamId = savedTeamId ? parseInt(savedTeamId, 10) : null;

  let applyBtnHtml = '';
  if (teamId) {
    applyBtnHtml = `
      <button id="fpl-drawer-apply-btn" class="fpl-btn-primary" style="background: linear-gradient(135deg, #e90052 0%, #ff1a6c 100%); color: #fff; margin-bottom: 12px; font-weight: 800;">
        ⚡ Apply Changes to FPL Team
      </button>
    `;
  }

  let html = `
    ${applyBtnHtml}
    <div class="fpl-card">
      <div class="fpl-card-title">GW${nextGw} Optimal XI (${optimal.formation})</div>
      <div style="font-size: 20px; font-weight: 800; color: #00ff87; margin-bottom: 8px;">
        ${optimal.totalScore.toFixed(3)} pts
      </div>
      <div style="font-size: 12px; color: #a393a5; margin-bottom: 12px;">
        XI Cost: £${optimal.xiCost.toFixed(1)}M | Total: £${optimal.totalCost.toFixed(1)}M / £${optimal.budget.toFixed(1)}M
      </div>

      <div style="font-size: 13px; margin-bottom: 8px;">
        <strong>Captain:</strong> ${optimal.captain?.player.web_name || 'N/A'} <span class="fpl-badge-c">C</span>
      </div>
      <div style="font-size: 13px; margin-bottom: 12px;">
        <strong>Vice:</strong> ${optimal.viceCaptain?.player.web_name || 'N/A'} <span class="fpl-badge-vc">VC</span>
      </div>
    </div>

    <div class="fpl-card">
      <div class="fpl-card-title">Starting XI Lineup</div>
  `;

  optimal.starters.forEach((sp) => {
    const isC = optimal.captain?.player.id === sp.player.id;
    const isVc = optimal.viceCaptain?.player.id === sp.player.id;
    const badge = isC ? `<span class="fpl-badge-c">C</span>` : isVc ? `<span class="fpl-badge-vc">VC</span>` : '';

    html += `
      <div class="fpl-player-row">
        <div>
          <strong>${sp.positionName}</strong> ${sp.player.web_name} (${sp.teamName}) ${badge}
          <div style="font-size: 11px; color: #a393a5;">${sp.oppDesc}</div>
        </div>
        <div style="text-align: right; font-weight: 600; color: #00ff87;">
          ${sp.score.toFixed(3)}
        </div>
      </div>
    `;
  });

  html += `</div>`;

  if (transfers && transfers.length > 0) {
    html += `
      <div class="fpl-card">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
          <div class="fpl-card-title" style="margin-bottom: 0;">Recommended Transfers (${transfers.length})</div>
          <button id="fpl-drawer-apply-transfers-btn" style="background: linear-gradient(135deg, #e90052 0%, #ff1a6c 100%); color: #fff; border: none; padding: 6px 12px; border-radius: 6px; font-weight: 700; font-size: 11px; cursor: pointer;">
            ⚡ Apply Transfers
          </button>
        </div>
    `;

    transfers.forEach((t) => {
      html += `
        <div class="fpl-player-row" style="flex-direction: column; gap: 4px;">
          <div style="color: #ff4d4d; font-size: 12px;">OUT: ${t.out.player.web_name || `#${t.out.player.id}`} (£${(t.sellingPrice / 10).toFixed(1)}M)</div>
          <div style="color: #00ff87; font-size: 12px;">IN: ${t.in.player.web_name} (£${(t.purchasePrice / 10).toFixed(1)}M)</div>
          <div style="font-size: 11px; color: #a393a5;">Score Uplift: +${t.scoreUplift.toFixed(3)}</div>
        </div>
      `;
    });

    html += `</div>`;
  }

  resultsDiv.innerHTML = html;

  const drawerApplyBtn = document.getElementById('fpl-drawer-apply-btn') as HTMLButtonElement;
  if (drawerApplyBtn) {
    drawerApplyBtn.addEventListener('click', applyDrawerChanges);
  }

  const drawerApplyTransfersBtn = document.getElementById('fpl-drawer-apply-transfers-btn') as HTMLButtonElement;
  if (drawerApplyTransfersBtn) {
    drawerApplyTransfersBtn.addEventListener('click', applyDrawerChanges);
  }
}

function applyDrawerChanges(): void {
  if (!lastDrawerData) return;

  const savedTeamId = sessionStorage.getItem('fpl_picker_my_team_id');
  if (!savedTeamId) {
    alert('No FPL Team ID detected.');
    return;
  }

  const teamId = parseInt(savedTeamId, 10);
  const btn = document.getElementById('fpl-drawer-apply-btn') as HTMLButtonElement;
  if (btn) {
    btn.disabled = true;
    btn.textContent = '⏳ Applying changes to FPL...';
  }

  const formula = (document.getElementById('fpl-drawer-formula') as HTMLSelectElement)?.value || '1';

  chrome.runtime.sendMessage(
    {
      type: 'APPLY_CHANGES',
      teamId,
      nextGw: lastDrawerData.nextGw,
      formula,
      optimal: lastDrawerData.optimal,
      transfers: lastDrawerData.transfers,
    },
    (res) => {
      if (btn) {
        if (res && res.success) {
          btn.textContent = '✅ Applied Successfully!';
          btn.style.background = '#00ff87';
          btn.style.color = '#38003c';
        } else {
          btn.disabled = false;
          btn.textContent = '⚡ Apply Changes to FPL Team';
          alert(`Failed to apply changes: ${res?.error || 'Unknown error'}`);
        }
      }
    }
  );
}

// Run widget setup on load
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initContentWidget);
} else {
  initContentWidget();
}
